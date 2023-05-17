/*
 * Copyright (c) 2022 NetLOX Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package loxinet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"

	api "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/apiutil"
	"github.com/osrg/gobgp/v3/pkg/packet/bgp"
	nlp "github.com/vishvananda/netlink"
	"google.golang.org/grpc"
	apb "google.golang.org/protobuf/types/known/anypb"
)

type goBgpEventType uint8
type goBgpState uint8

const (
	bgpConnected goBgpEventType = iota
	bgpDisconnected
	bgpRtRecvd
	bgpTO
)

type goBgpRouteInfo struct {
	nlri 		bgp.AddrPrefixInterface
	attrs 		[]bgp.PathAttributeInterface
	withdraw 	bool
	pathId		uint32
}

type goBgpEvent struct {
	EventType goBgpEventType
	Src       string
	Data      goBgpRouteInfo
	conn      *grpc.ClientConn
}

// goBGP connected status
const (
	BGPConnected goBgpState = iota
	BGPDisconnected
)

// goCI - Cluster Instance context
type goCI struct {
	name    string
	hastate int
	vip     net.IP
	rules   map[string]int
}

// GoBgpH - context container
type GoBgpH struct {
	eventCh chan goBgpEvent
	host    string
	conn    *grpc.ClientConn
	client  api.GobgpApiClient
	mtx     sync.RWMutex
	state   goBgpState
	noNlp   bool
	ciMap   map[string]*goCI
}

func (gbh *GoBgpH) getPathAttributeString(nlri bgp.AddrPrefixInterface, attrs []bgp.PathAttributeInterface) string {
	s := make([]string, 0)
	for _, a := range attrs {
		switch a.GetType() {
		case bgp.BGP_ATTR_TYPE_NEXT_HOP, bgp.BGP_ATTR_TYPE_MP_REACH_NLRI, bgp.BGP_ATTR_TYPE_AS_PATH, bgp.BGP_ATTR_TYPE_AS4_PATH:
			continue
		default:
			s = append(s, a.String())
		}
	}
	switch n := nlri.(type) {
	case *bgp.EVPNNLRI:
		// We print non route key fields like path attributes.
		switch route := n.RouteTypeData.(type) {
		case *bgp.EVPNMacIPAdvertisementRoute:
			s = append(s, fmt.Sprintf("[ESI: %s]", route.ESI.String()))
		case *bgp.EVPNIPPrefixRoute:
			s = append(s, fmt.Sprintf("[ESI: %s]", route.ESI.String()))
			if route.GWIPAddress != nil {
				s = append(s, fmt.Sprintf("[GW: %s]", route.GWIPAddress.String()))
			}
		}
	}
	return fmt.Sprint(s)
}

func (gbh *GoBgpH) getNextHopFromPathAttributes(attrs []bgp.PathAttributeInterface) net.IP {
	for _, attr := range attrs {
		switch a := attr.(type) {
		case *bgp.PathAttributeNextHop:
			return a.Value
		case *bgp.PathAttributeMpReachNLRI:
			return a.Nexthop
		}
	}
	return nil
}

func (gbh *GoBgpH) makeMonitorRouteArgs(p *goBgpRouteInfo, showIdentifier bgp.BGPAddPathMode) []interface{} {
	pathStr := make([]interface{}, 0)

	// Title
	title := "ADDROUTE"
	if p.withdraw {
		title = "DELROUTE"
	}
	pathStr = append(pathStr, title)

	// NLRI
	// If Add-Path required, append Path Identifier.
	if showIdentifier != bgp.BGP_ADD_PATH_NONE {
		pathStr = append(pathStr, p.pathId)
	}
	pathStr = append(pathStr, p.nlri)

	// Next Hop
	nexthop := "fictitious"
	if n := gbh.getNextHopFromPathAttributes(p.attrs); n != nil {
		nexthop = n.String()
	}
	pathStr = append(pathStr, nexthop)

	// AS_PATH
	aspathstr := func() string {
		for _, attr := range p.attrs {
			switch a := attr.(type) {
			case *bgp.PathAttributeAsPath:
				return bgp.AsPathString(a)
			}
		}
		return ""
	}()
	pathStr = append(pathStr, aspathstr)

	// Path Attributes
	pathStr = append(pathStr, gbh.getPathAttributeString(p.nlri, p.attrs))

	return pathStr
}

func (gbh *GoBgpH) processRouteSingle(p *goBgpRouteInfo, showIdentifier bgp.BGPAddPathMode) {
	//pathStr := make([]interface{}, 1)

	pathStr := gbh.makeMonitorRouteArgs(p, showIdentifier)

	format := time.Now().UTC().Format(time.RFC3339)
	if showIdentifier == bgp.BGP_ADD_PATH_NONE {
		format += " [%s] %s via %s aspath [%s] attrs %s\n"
	} else {
		format += " [%s] %d:%s via %s aspath [%s] attrs %s\n"
	}

	tk.LogIt(tk.LogInfo, format, pathStr...)

	if err := gbh.syncRoute(p, showIdentifier); err != nil {
		tk.LogIt(tk.LogError, " failed to "+format, pathStr...)
	}
}

func (gbh *GoBgpH) syncRoute(p *goBgpRouteInfo, showIdentifier bgp.BGPAddPathMode) error {
	if gbh.noNlp {
		return nil
	}

	_, dstIPN, err := net.ParseCIDR(p.nlri.String())
	if err != nil {
		return err
	}

	// NextHop
	nexthop := gbh.getNextHopFromPathAttributes(p.attrs)

	// Make netlink route and add
	route := &nlp.Route{
		Dst: dstIPN,
		Gw:  nexthop,
	}

	if route.Gw.IsUnspecified() {
		return nil
	}

	if p.withdraw {
		tk.LogIt(tk.LogDebug, "[GoBGP] ip route delete %s via %s\n", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteDel(route); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] failed to ip route delete. err: %s\n", err.Error())
			return err
		}
	} else {
		tk.LogIt(tk.LogDebug, "[GoBGP] ip route add %s via %s\n", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteAdd(route); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] failed to ip route add. err: %s\n", err.Error())
			return err
		}
	}

	return nil
}

func (gbh *GoBgpH) processRoute(pathList []*api.Path) {
	
	for _, p := range pathList {
		// NLRI have destination CIDR info
		nlri, err := apiutil.GetNativeNlri(p)
		if err != nil {
			return
		}
		// NextHop
		attrs, err := apiutil.GetNativePathAttributes(p)
		if err != nil {
			return
		}

		data := goBgpRouteInfo{nlri: nlri, attrs: attrs, withdraw: p.GetIsWithdraw(), pathId: p.GetIdentifier()}

		gbh.eventCh <- goBgpEvent{
			EventType: bgpRtRecvd,
			Src:       "",
			Data:      data,
			conn:      &grpc.ClientConn{},
		}
	}
}

// GetRoutes - get routes in goBGP
func (gbh *GoBgpH) GetRoutes(client api.GobgpApiClient) int {

	processRoutes := func(recver interface {
		Recv() (*api.WatchEventResponse, error)
	}) {
		for {
			r, err := recver.Recv()
			if err == io.EOF {

			} else if err != nil {
				tk.LogIt(tk.LogCritical, "processRoutes failed : %v\n", err)

				gbh.eventCh <- goBgpEvent{
					EventType: bgpDisconnected,
				}

				break
			}
			if t := r.GetTable(); t != nil {
				gbh.processRoute(t.Paths)
			}
		}
	}

	// the change of the peer state and path
	routes, err := client.WatchEvent(context.Background(),
		&api.WatchEventRequest{
			Table: &api.WatchEventRequest_Table{
				Filters: []*api.WatchEventRequest_Table_Filter{
					{
						Type: api.WatchEventRequest_Table_Filter_BEST,
					},
				},
			},
		})

	if err != nil {
		tk.LogIt(tk.LogError, "Get- %v\n", err)
		return -1

	}
	processRoutes(routes)
	return 0
}

// AdvertiseRoute - advertise a new route using goBGP
func (gbh *GoBgpH) AdvertiseRoute(rtPrefix string, pLen int, nh string, pref uint32, ipv4 bool) int {
	var apiFamily *api.Family
	// add routes
	//tk.LogIt(tk.LogDebug, "\n\n\n Advertising Route : %v via %v\n\n\n", rtPrefix, nh)
	nlri, _ := apb.New(&api.IPAddressPrefix{
		Prefix:    rtPrefix,
		PrefixLen: uint32(pLen),
	})

	a1, _ := apb.New(&api.OriginAttribute{
		Origin: 0,
	})

	a2, _ := apb.New(&api.NextHopAttribute{
		NextHop: nh,
	})

	a3, _ := apb.New(&api.LocalPrefAttribute{
		LocalPref: pref,
	})

	if ipv4 == true {
		apiFamily = &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST}
	} else {
		apiFamily = &api.Family{Afi: api.Family_AFI_IP6, Safi: api.Family_SAFI_UNICAST}
	}

	/*
		a3, _ := apb.New(&api.AsPathAttribute{
				Segments: []*api.AsSegment{
						{
								Type:    2,
								Numbers: []uint32{6762, 39919, 65000, 35753, 65000},
						},
				},
		})

		attrs := []*apb.Any{a1, a2, a3}
	*/

	attrs := []*apb.Any{a1, a2, a3}

	_, err := gbh.client.AddPath(context.Background(), &api.AddPathRequest{
		Path: &api.Path{
			Family: apiFamily,
			Nlri:   nlri,
			Pattrs: attrs,
		},
	})

	if err != nil {
		tk.LogIt(tk.LogCritical, "Advertised Route add failed: %v\n", err)
		return -1
	}
	tk.LogIt(tk.LogDebug, "Advertised Route [OK]: %s/%d\n", rtPrefix, pLen)
	return 0
}

// DelAdvertiseRoute - delete previously advertised route in goBGP
func (gbh *GoBgpH) DelAdvertiseRoute(rtPrefix string, pLen int, nh string, pref uint32) int {

	// del routes
	nlri, _ := apb.New(&api.IPAddressPrefix{
		Prefix:    rtPrefix,
		PrefixLen: uint32(pLen),
	})

	a1, _ := apb.New(&api.OriginAttribute{
		Origin: 0,
	})

	a2, _ := apb.New(&api.NextHopAttribute{
		NextHop: nh,
	})

	a3, _ := apb.New(&api.LocalPrefAttribute{
		LocalPref: pref,
	})

	attrs := []*apb.Any{a1, a2, a3}

	_, err := gbh.client.DeletePath(context.Background(), &api.DeletePathRequest{
		Path: &api.Path{
			Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
			Nlri:   nlri,
			Pattrs: attrs,
		},
	})

	if err != nil {
		tk.LogIt(tk.LogCritical, "Advertised Route del failed: %v\n", err)
		return -1
	}
	return 0
}

// GoBgpInit - initialize goBGP client subsystem
func GoBgpInit() *GoBgpH {
	//gbh = new(GoBgpH)
	gbh := new(GoBgpH)

	gbh.eventCh = make(chan goBgpEvent, cmn.RuWorkQLen)
	gbh.host = "127.0.0.1:50051"
	gbh.ciMap = make(map[string]*goCI)
	gbh.state = BGPDisconnected
	go gbh.goBgpSpawn()
	go gbh.goBgpConnect(gbh.host)
	go gbh.goBgpMonitor()
	return gbh
}

func (gbh *GoBgpH) goBgpSpawn() {
	command := "pkill gobgpd"
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError, "Error in stopping gpbgp:%s\n", err)
	}
	mh.dp.WaitXsyncReady("bgp")
	// We need some cool-off period for loxilb to self sync-up in the cluster
	time.Sleep(GoBGPInitTiVal * time.Second)
	for {
		cfgOpts := ""

		if _, err := os.Stat("/etc/gobgp/gobgp.conf"); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat("/etc/gobgp/gobgp_loxilb.yaml"); errors.Is(err, os.ErrNotExist) {
				time.Sleep(2000 * time.Millisecond)
				continue
			}
			cfgOpts = "-t yaml -f /etc/gobgp/gobgp_loxilb.yaml"
		} else {
			cfgOpts = "-f /etc/gobgp/gobgp.conf"
		}

		command := fmt.Sprintf("gobgpd %s", cfgOpts)
		cmd := exec.Command("bash", "-c", command)
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(3000 * time.Millisecond)
	}
}

func (gbh *GoBgpH) goBgpConnect(host string) {
	for {
		gbh.mtx.Lock()
		conn, err := grpc.DialContext(context.TODO(), host, grpc.WithInsecure())
		if err != nil {
			tk.LogIt(tk.LogNotice, "BGP session %s not alive. Will Retry!\n", gbh.host)
			gbh.mtx.Unlock()
			time.Sleep(2000 * time.Millisecond)
		} else {
			gbh.client = api.NewGobgpApiClient(conn)
			gbh.mtx.Unlock()
			for {
				gbh.mtx.Lock()
				r, err := gbh.client.GetBgp(context.TODO(), &api.GetBgpRequest{})
				if err != nil {
					tk.LogIt(tk.LogInfo, "BGP session %s not ready. Will Retry!\n", gbh.host)
					gbh.mtx.Unlock()
					time.Sleep(2000 * time.Millisecond)
					continue
				}
				tk.LogIt(tk.LogNotice, "BGP session %s ready! Attributes[%v]\n", gbh.host, r)

				gbh.mtx.Unlock()
				break
			}
			gbh.eventCh <- goBgpEvent{
				EventType: bgpConnected,
				Src:       host,
				conn:      conn,
			}
			return
		}
	}
}

// AddBGPRule - add a bgp rule in goBGP
func (gbh *GoBgpH) AddBGPRule(instance string, IP []string) {
	var pref uint32
	gbh.mtx.Lock()
	ci := gbh.ciMap[instance]
	if ci == nil {
		ci = new(goCI)
		ci.rules = make(map[string]int)
		ci.name = instance
		ci.hastate = cmn.CIStateBackup
		ci.vip = net.IPv4zero
		gbh.ciMap[instance] = ci
	}

	for _, ip := range IP {
		ci.rules[ip]++
		
		if gbh.state == BGPConnected {
			if ci.hastate == cmn.CIStateBackup {
				pref = cmn.LowLocalPref
			} else {
				pref = cmn.HighLocalPref
			}
			if net.ParseIP(ip).To4() != nil {
				gbh.AdvertiseRoute(ip, 32, "0.0.0.0", pref, true)
			} else {
				gbh.AdvertiseRoute(ip, 128, "::", pref, false)
			}
		}
	}

	gbh.mtx.Unlock()
}

// DelBGPRule - delete a bgp rule in goBGP
func (gbh *GoBgpH) DelBGPRule(instance string, IP []string) {
	var pref uint32
	gbh.mtx.Lock()
	ci := gbh.ciMap[instance]
	if ci == nil {
		tk.LogIt(tk.LogError, "[GoBGP] Del BGP Rule - Invalid instance %s\n", instance)
		gbh.mtx.Unlock()
		return
	}

	for _, ip := range IP {
		if ci.rules[ip] > 0 {
			ci.rules[ip]--
		}
		if gbh.state == BGPConnected && ci.rules[ip] == 0 {
			if ci.hastate == cmn.CIStateBackup {
				pref = cmn.LowLocalPref
			} else {
				pref = cmn.HighLocalPref
			}
			if net.ParseIP(ip).To4() != nil {
				gbh.DelAdvertiseRoute(ip, 32, "0.0.0.0", pref)
			} else {
				gbh.DelAdvertiseRoute(ip, 128, "::", pref)
			}
			tk.LogIt(tk.LogDebug, "[GoBGP] Del BGP Rule %s\n", ip)
		}
	}
	gbh.mtx.Unlock()
}

// AddCurrentBgpRoutesToIPRoute - add bgp routes to OS
func (gbh *GoBgpH) AddCurrentBgpRoutesToIPRoute() error {
	ipv4UC := &api.Family{
		Afi:  api.Family_AFI_IP,
		Safi: api.Family_SAFI_UNICAST,
	}

	stream, err := gbh.client.ListPath(context.TODO(), &api.ListPathRequest{
		TableType: api.TableType_GLOBAL,
		Family:    ipv4UC,
	})
	if err != nil {
		gbh.eventCh <- goBgpEvent{
			EventType: bgpDisconnected,
		}
		return err
	}

	rib := make([]*api.Destination, 0)
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		rib = append(rib, r.Destination)
	}

	for _, r := range rib {
		_, dstIPN, err := net.ParseCIDR(r.GetPrefix())
		if err != nil {
			return fmt.Errorf("%s is invalid prefix\n", r.GetPrefix())
		}

		var nlpRoute *nlp.Route
		for _, p := range r.Paths {
			if !p.Best {
				continue
			}

			nexthopIP := net.ParseIP(p.GetNeighborIp())
			if nexthopIP == nil {
				tk.LogIt(tk.LogDebug, "prefix %s neighbor_ip %s is invalid\n", r.GetPrefix(), p.GetNeighborIp())
				continue
			}

			nlpRoute = &nlp.Route{
				Dst: dstIPN,
				Gw:  nexthopIP,
			}
		}

		if nlpRoute == nil && len(r.Paths) > 0 {
			nlpRoute = &nlp.Route{
				Dst: dstIPN,
				Gw:  net.ParseIP(r.Paths[0].GetNeighborIp()),
			}
		}
		if nlpRoute.Gw.IsUnspecified() {
			continue
		}
		tk.LogIt(tk.LogDebug, "[GoBGP] ip route add %s via %s\n", dstIPN.String(), nlpRoute.Gw.String())
		nlp.RouteAdd(nlpRoute)
	}

	return nil
}

func (gbh *GoBgpH) advertiseAllRoutes(instance string) {
	var pref uint32
	ci := gbh.ciMap[instance]
	if ci == nil {
		tk.LogIt(tk.LogError, "[GoBGP] Instance %s is invalid\n", instance)
		return
	}
	if ci.hastate == cmn.CIStateBackup {
		pref = cmn.LowLocalPref
	} else {
		pref = cmn.HighLocalPref
	}

	if !ci.vip.IsUnspecified() {
		gbh.AdvertiseRoute(ci.vip.String(), 32, "0.0.0.0", pref, true)
	}

	for ip, count := range ci.rules {
		tk.LogIt(tk.LogDebug, "[GoBGP] connected BGP rules ip %s ref count(%d)\n", ip, count)
		if count > 0 {
			if net.ParseIP(ip).To4() != nil {
				gbh.AdvertiseRoute(ip, 32, "0.0.0.0", pref, true)
			} else {
				gbh.AdvertiseRoute(ip, 128, "::", pref, false)
			}
		} else {
			if net.ParseIP(ip).To4() != nil {
				gbh.DelAdvertiseRoute(ip, 32, "0.0.0.0", pref)
			} else {
				gbh.DelAdvertiseRoute(ip, 128, "::", pref)
			}
		}
	}
}

func (gbh *GoBgpH) getGoBgpRoutes() {
	gbh.mtx.Lock()

	for ciname, ci := range gbh.ciMap {
		if ci != nil {
			gbh.advertiseAllRoutes(ciname)
		}
	}

	if err := gbh.AddCurrentBgpRoutesToIPRoute(); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s\n", err.Error())
	}
	gbh.mtx.Unlock()

	go gbh.GetRoutes(gbh.client)
}

func (gbh *GoBgpH) processBgpEvent(e goBgpEvent) {

	switch e.EventType {
	case bgpDisconnected:
		tk.LogIt(tk.LogNotice, "******************* BGP %s disconnected *******************\n", gbh.host)
		gbh.conn.Close()
		gbh.conn = nil
		gbh.state = BGPDisconnected
		go gbh.goBgpConnect(gbh.host)
	case bgpConnected:
		tk.LogIt(tk.LogNotice, "******************* BGP %s connected *******************\n", gbh.host)
		gbh.conn = e.conn
		gbh.state = BGPConnected
		gbh.getGoBgpRoutes()
	case bgpRtRecvd:
		gbh.processRouteSingle(&e.Data, bgp.BGP_ADD_PATH_NONE)
	}
}

func (gbh *GoBgpH) goBgpMonitor() {
	tk.LogIt(tk.LogNotice, "******************* BGP Monitor start *******************\n")
	for {
		for n := 0; n < cmn.RuWorkQLen; n++ {
			select {
			case e := <-gbh.eventCh:
				gbh.processBgpEvent(e)
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func (gbh *GoBgpH) UpdateCIState(instance string, state int, vip net.IP) {
	ci := gbh.ciMap[instance]
	if ci == nil {
		ci = new(goCI)
		ci.rules = make(map[string]int)
	}
	ci.name = instance
	ci.hastate = state
	ci.vip = vip
	gbh.ciMap[instance] = ci

	gbh.advertiseAllRoutes(instance)
	tk.LogIt(tk.LogNotice, "[BGP] Instance %s(%v) HA state updated : %d\n", instance, vip, state)
}
