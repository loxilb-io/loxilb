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
	"golang.org/x/sys/unix"
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
	nlri     bgp.AddrPrefixInterface
	attrs    []bgp.PathAttributeInterface
	withdraw bool
	pathID   uint32
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
	ticker  *time.Ticker
	tDone   chan bool
	host    string
	conn    *grpc.ClientConn
	client  api.GobgpApiClient
	mtx     sync.RWMutex
	state   goBgpState
	noNlp   bool
	localAs uint32
	ciMap   map[string]*goCI
	reqRst  bool
	resetTS time.Time
}

func (gbh *GoBgpH) getGlobalConfig() error {
	r, err := gbh.client.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		return err
	}

	gbh.localAs = r.Global.Asn
	return nil
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
		pathStr = append(pathStr, p.pathID)
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

	dstIP, dstIPN, err := net.ParseCIDR(p.nlri.String())
	if err != nil {
		return err
	}

	if IsIPHostNetAddr(dstIP) {
		return nil
	}

	// NextHop
	nexthop := gbh.getNextHopFromPathAttributes(p.attrs)

	// Make netlink route and add
	route := &nlp.Route{
		Dst:      dstIPN,
		Gw:       nexthop,
		Protocol: unix.RTPROT_BGP,
	}

	if p.withdraw {
		tk.LogIt(tk.LogDebug, "[GoBGP] ip route delete %s via %s\n", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteDel(route); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] failed to ip route delete. err: %s\n", err.Error())
			return err
		}
	} else {
		if nexthop == nil || nexthop.IsUnspecified() {
			return nil
		}

		tk.LogIt(tk.LogDebug, "[GoBGP] ip route add %s via %s\n", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteReplace(route); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] failed to ip route add. err: %s\n", err.Error())
			return err
		}
	}

	return nil
}

func (gbh *GoBgpH) processRoute(pathList []*api.Path) {

	for _, p := range pathList {
		if !p.GetIsWithdraw() {
			if !p.Best || p.IsNexthopInvalid {
				continue
			}
		}
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

		data := goBgpRouteInfo{nlri: nlri, attrs: attrs, withdraw: p.GetIsWithdraw(), pathID: p.GetIdentifier()}

		gbh.eventCh <- goBgpEvent{
			EventType: bgpRtRecvd,
			Src:       "",
			Data:      data,
			conn:      &grpc.ClientConn{},
		}
	}
}

// GetgoBGPRoutesEvents - get routes in goBGP
func (gbh *GoBgpH) GetgoBGPRoutesEvents(client api.GobgpApiClient) int {

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
						Type: api.WatchEventRequest_Table_Filter_ADJIN,
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
func (gbh *GoBgpH) AdvertiseRoute(rtPrefix string, pLen int, nh string, pref uint32, med uint32, ipv4 bool) int {
	var apiFamily *api.Family

	if gbh.localAs == 0 {
		if err := gbh.getGlobalConfig(); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Can't get localAS\n")
			return -1
		}
	}
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

	a4, _ := apb.New(&api.MultiExitDiscAttribute{
		Med: med,
	})

	a5, _ := apb.New(&api.AsPathAttribute{
		Segments: []*api.AsSegment{
			{
				Type:    1, // SET
				Numbers: []uint32{gbh.localAs},
			},
		},
	})

	if ipv4 {
		apiFamily = &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST}
	} else {
		apiFamily = &api.Family{Afi: api.Family_AFI_IP6, Safi: api.Family_SAFI_UNICAST}
	}

	attrs := []*apb.Any{a1, a2, a3, a4, a5}

	_, err := gbh.client.AddPath(context.Background(), &api.AddPathRequest{
		Path: &api.Path{
			Family: apiFamily,
			Nlri:   nlri,
			Pattrs: attrs,
		},
	})

	if err != nil {
		tk.LogIt(tk.LogCritical, "[GoBGP] Advertised Route add %s/%d via %s failed: %v\n", rtPrefix, pLen, nh, err)
		return -1
	}
	tk.LogIt(tk.LogDebug, "[GoBGP] Advertised Route [OK]: %s/%d via %s pref(%v):med(%v)\n", rtPrefix, pLen, nh, pref, med)
	return 0
}

// DelAdvertiseRoute - delete previously advertised route in goBGP
func (gbh *GoBgpH) DelAdvertiseRoute(rtPrefix string, pLen int, nh string, pref uint32, med uint32) int {

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

	a4, _ := apb.New(&api.MultiExitDiscAttribute{
		Med: med,
	})

	a5, _ := apb.New(&api.AsPathAttribute{
		Segments: []*api.AsSegment{
			{
				Type:    1, // SET
				Numbers: []uint32{gbh.localAs},
			},
		},
	})

	attrs := []*apb.Any{a1, a2, a3, a4, a5}

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
	} else {
		tk.LogIt(tk.LogDebug, "[GoBGP] Withdraw Route [OK]: %s/%d via %s pref(%v):med(%v)\n", rtPrefix, pLen, nh, pref, med)
	}
	return 0
}

// GoBgpInit - initialize goBGP client subsystem
func GoBgpInit(bgpPeerMode bool) *GoBgpH {
	//gbh = new(GoBgpH)
	gbh := new(GoBgpH)

	gbh.eventCh = make(chan goBgpEvent, cmn.RuWorkQLen)
	gbh.host = "127.0.0.1:50052"
	if gbh.ciMap = make(map[string]*goCI); gbh.ciMap == nil {
		panic("gbh.ciMap alloc failure")
	}
	gbh.state = BGPDisconnected
	gbh.tDone = make(chan bool)
	gbh.ticker = time.NewTicker(30 * time.Second)
	go gbh.goBGPTicker()
	go gbh.goBgpSpawn(bgpPeerMode)
	go gbh.goBgpConnect(gbh.host)
	go gbh.goBgpMonitor()
	return gbh
}

func (gbh *GoBgpH) goBgpSpawn(bgpPeerMode bool) {
	command := "pkill gobgpd"
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError, "Error in stopping gpbgp:%s\n", err)
	}
	if !bgpPeerMode {
		mh.dp.WaitXsyncReady("bgp")
	}
	// We need some cool-off period for loxilb to self sync-up in the cluster
	time.Sleep(GoBGPInitTiVal * time.Second)
	for {
		cfgOpts := ""
		confFile := fmt.Sprintf("%s%s.conf", "/etc/gobgp/gobgp", os.Getenv("HOSTNAME"))
		if _, err := os.Stat(confFile); errors.Is(err, os.ErrNotExist) {
			if _, err := os.Stat("/etc/gobgp/gobgp.conf"); errors.Is(err, os.ErrNotExist) {
				if _, err := os.Stat("/etc/gobgp/gobgp_loxilb.yaml"); errors.Is(err, os.ErrNotExist) {
					tk.LogIt(tk.LogError, "No BGP conf file found. Run without it\n")
				} else {
					cfgOpts = "-t yaml -f /etc/gobgp/gobgp_loxilb.yaml"
				}
			} else {
				cfgOpts = "-f /etc/gobgp/gobgp.conf"
			}
		} else {
			cfgOpts = "-f " + confFile
		}

		command := fmt.Sprintf("gobgpd %s --api-hosts=127.0.0.1:50052", cfgOpts)
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
				tk.LogIt(tk.LogNotice, "BGP server %s UP!\n", gbh.host)
				if r.Global.Asn == 0 {
					tk.LogIt(tk.LogInfo, "BGP Global Config %s not done. Will wait!\n", gbh.host)
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
	var med uint32

	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	ci := gbh.ciMap[instance]
	if ci == nil {
		ci = new(goCI)
		ci.rules = make(map[string]int)
		ci.name = instance
		ci.hastate = cmn.CIStateNotDefined
		ci.vip = net.IPv4zero
		gbh.ciMap[instance] = ci
	}

	for _, ip := range IP {
		ci.rules[ip]++

		if gbh.state == BGPConnected && ci.rules[ip] == 1 {
			if ci.hastate == cmn.CIStateBackup {
				pref = cmn.LowLocalPref
				med = cmn.LowMed
			} else if ci.hastate == cmn.CIStateMaster {
				pref = cmn.HighLocalPref
				med = cmn.HighMed
			} else {
				pref = 0
				med = 0
			}
			if net.ParseIP(ip).To4() != nil {
				gbh.AdvertiseRoute(ip, 32, "0.0.0.0", pref, med, true)
			} else {
				gbh.AdvertiseRoute(ip, 128, "::", pref, med, false)
			}
		}
	}
}

// DelBGPRule - delete a bgp rule in goBGP
func (gbh *GoBgpH) DelBGPRule(instance string, IP []string) {
	var pref uint32
	var med uint32
	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	ci := gbh.ciMap[instance]
	if ci == nil {
		tk.LogIt(tk.LogError, "[GoBGP] Del BGP Rule - Invalid instance %s\n", instance)
		return
	}

	for _, ip := range IP {
		if ci.rules[ip] > 0 {
			ci.rules[ip]--
		}
		if gbh.state == BGPConnected && ci.rules[ip] == 0 {
			if ci.hastate == cmn.CIStateBackup {
				pref = cmn.LowLocalPref
				med = cmn.LowMed
			} else if ci.hastate == cmn.CIStateMaster {
				pref = cmn.HighLocalPref
				med = cmn.HighMed
			} else {
				pref = 0
				med = 0
			}
			if net.ParseIP(ip).To4() != nil {
				gbh.DelAdvertiseRoute(ip, 32, "0.0.0.0", pref, med)
			} else {
				gbh.DelAdvertiseRoute(ip, 128, "::", pref, med)
			}
			if ci.rules[ip] == 0 {
				delete(ci.rules, ip)
			}
			tk.LogIt(tk.LogDebug, "[GoBGP] Del BGP Rule %s\n", ip)
		}
	}
}

// AddCurrBgpRoutesToIPRoute - add bgp routes to OS
func (gbh *GoBgpH) AddCurrBgpRoutesToIPRoute() error {
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
		dstIP, dstIPN, err := net.ParseCIDR(r.GetPrefix())
		if err != nil {
			tk.LogIt(tk.LogError, "%s is invalid prefix\n", r.GetPrefix())
			return err
		}

		if IsIPHostNetAddr(dstIP) {
			continue
		}

		var nlpRoute *nlp.Route
		var nexthopIP net.IP
		for _, p := range r.Paths {
			if !p.Best || p.IsNexthopInvalid {
				continue
			}
			attrs, err := apiutil.GetNativePathAttributes(p)
			if err != nil {
				continue
			}
			if nexthopIP = gbh.getNextHopFromPathAttributes(attrs); nexthopIP == nil {
				//tk.LogIt(tk.LogDebug, "prefix %s neighbor %s is invalid\n", r.GetPrefix(), p.GetNeighborIp())
				continue
			}

			nlpRoute = &nlp.Route{
				Dst:      dstIPN,
				Gw:       nexthopIP,
				Protocol: unix.RTPROT_BGP,
			}
		}

		if nlpRoute == nil || nlpRoute.Gw.IsUnspecified() {
			tk.LogIt(tk.LogDebug, "prefix %s is invalid\n", r.GetPrefix())
			continue
		}
		//tk.LogIt(tk.LogDebug, "[GoBGP] ip route add %s via %s\n", dstIPN.String(), nlpRoute.Gw.String())
		nlp.RouteReplace(nlpRoute)
	}

	return nil
}

func (gbh *GoBgpH) advertiseAllVIPs(instance string) {
	var pref uint32
	var med uint32
	add := true
	ci := gbh.ciMap[instance]
	if ci == nil {
		tk.LogIt(tk.LogError, "[GoBGP] Instance %s is invalid\n", instance)
		return
	}
	if ci.hastate == cmn.CIStateBackup {
		pref = cmn.LowLocalPref
		med = cmn.LowMed
	} else if ci.hastate == cmn.CIStateMaster {
		pref = cmn.HighLocalPref
		med = cmn.HighMed
	}

	if !ci.vip.IsUnspecified() {
		if add {
			gbh.AdvertiseRoute(ci.vip.String(), 32, "0.0.0.0", pref, med, true)
		} else {
			gbh.DelAdvertiseRoute(ci.vip.String(), 32, "0.0.0.0", pref, med)
		}
	}

	for ip, count := range ci.rules {
		tk.LogIt(tk.LogDebug, "[GoBGP] connected BGP rules ip %s ref count(%d)\n", ip, count)
		if add {
			if net.ParseIP(ip).To4() != nil {
				gbh.AdvertiseRoute(ip, 32, "0.0.0.0", pref, med, true)
			} else {
				gbh.AdvertiseRoute(ip, 128, "::", pref, med, false)
			}
		} else {
			if net.ParseIP(ip).To4() != nil {
				gbh.DelAdvertiseRoute(ip, 32, "0.0.0.0", pref, med)
			} else {
				gbh.DelAdvertiseRoute(ip, 128, "::", pref, med)
			}
		}
	}
}

func (gbh *GoBgpH) initBgpClient() {

	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	for ciname, ci := range gbh.ciMap {
		if ci != nil {
			gbh.advertiseAllVIPs(ciname)
		}

		if ciname == cmn.CIDefault {
			if ci.hastate == cmn.CIStateBackup {
				gbh.resetBGPMed(true)
			} else if ci.hastate == cmn.CIStateMaster {
				gbh.resetBGPMed(false)
			}
		}
	}

	/* Get local routes and advertise */
	//getRoutesAndAdvertise()

	if err := gbh.AddCurrBgpRoutesToIPRoute(); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s\n", err.Error())
	}

	go gbh.GetgoBGPRoutesEvents(gbh.client)
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
		gbh.initBgpClient()
	case bgpRtRecvd:
		gbh.processRouteSingle(&e.Data, bgp.BGP_ADD_PATH_RECEIVE)
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

// UpdateCIState - Routine to update CI state for this module and re-advertise with appropriate priority
func (gbh *GoBgpH) UpdateCIState(instance string, state int, vip net.IP) {
	update := false
	ci := gbh.ciMap[instance]
	if ci == nil {
		ci = new(goCI)
		ci.rules = make(map[string]int)
	} else {
		if ci.hastate != state {
			update = true
		}
	}
	ci.name = instance
	ci.hastate = state
	ci.vip = vip
	gbh.ciMap[instance] = ci

	gbh.advertiseAllVIPs(instance)
	if update {
		if instance == cmn.CIDefault {
			if ci.hastate == cmn.CIStateBackup {
				gbh.resetBGPMed(true)
			} else if ci.hastate == cmn.CIStateMaster {
				gbh.resetBGPMed(false)
			}
		}
	}
	tk.LogIt(tk.LogNotice, "[BGP] Instance %s(%v) HA state updated : %d\n", instance, vip, state)
}

// resetNeighAdj - Reset BGP Neighbor's adjacencies
func (gbh *GoBgpH) resetNeighAdj() error {

	stream, err := gbh.client.ListPeer(context.Background(), &api.ListPeerRequest{
		Address:          "",
		EnableAdvertised: false,
	})
	if err != nil {
		return err
	}

	l := make([]*api.Peer, 0, 1024)
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		l = append(l, r.Peer)
	}

	if len(l) == 0 {
		return nil
	}

	for _, nb := range l {
		if nb.Conf.PeerAsn != gbh.localAs {
			gbh.resetSingleNeighAdj(nb.Conf.NeighborAddress)
		}
	}
	return nil
}

// resetBGPMed - Reset BGP Med attribute
func (gbh *GoBgpH) resetBGPMed(toLow bool) error {

	if !toLow {
		if _, err := gbh.removeExportPolicy("global", "set-med-export-gpolicy"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error removing set-med-export policy%s\n", err.Error())
			// return err
		} else {
			tk.LogIt(tk.LogInfo, "[GoBGP] Removed set-med-export policy\n")
		}
	} else {
		if _, err := gbh.applyExportPolicy("global", "set-med-export-gpolicy"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error applying set-med-export policy%s\n", err.Error())
			//return err
		} else {
			tk.LogIt(tk.LogInfo, "[GoBGP] Applied set-med-export policy\n")
		}
	}

	gbh.reqRst = true
	gbh.resetTS = time.Now()

	return nil
}

// BGPNeighMod - Routine to add BGP neigh to goBGP server
func (gbh *GoBgpH) BGPNeighMod(add bool, neigh net.IP, ras uint32, rPort uint32) (int, error) {
	var peer *api.Peer
	var err error

	peer = &api.Peer{
		Conf:           &api.PeerConf{},
		State:          &api.PeerState{},
		RouteServer:    &api.RouteServer{},
		RouteReflector: &api.RouteReflector{},
		Transport:      &api.Transport{},
	}
	peer.Conf.NeighborAddress = neigh.String()
	peer.State.NeighborAddress = neigh.String()
	peer.Conf.PeerAsn = ras
	peer.Conf.AllowOwnAsn = 1
	if rPort != 0 {
		peer.Transport.RemotePort = rPort
	} else {
		peer.Transport.RemotePort = 179
	}

	if add {
		_, err = gbh.client.AddPeer(context.Background(),
			&api.AddPeerRequest{
				Peer: peer,
			})

	} else {
		_, err = gbh.client.DeletePeer(context.Background(),
			&api.DeletePeerRequest{
				Address: neigh.String(),
			})
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// createSelfNHpolicy - Routine to create policy statement
func (gbh *GoBgpH) createNHpolicyStmt(name string, addr string) (int, error) {
	st := &api.Statement{
		Name:    name,
		Actions: &api.Actions{},
	}
	st.Actions.Nexthop = &api.NexthopAction{}
	st.Actions.Nexthop.Address = addr
	_, err := gbh.client.AddStatement(context.Background(),
		&api.AddStatementRequest{
			Statement: st,
		})
	return 0, err
}

// createSetMedPolicy - Routine to create set med-policy statement
func (gbh *GoBgpH) createSetMedPolicy(name string, val int64) (int, error) {
	st := &api.Statement{
		Name:    name,
		Actions: &api.Actions{},
	}
	st.Actions.Med = &api.MedAction{}
	st.Actions.Med.Type = api.MedAction_MOD
	st.Actions.Med.Value = val
	_, err := gbh.client.AddStatement(context.Background(),
		&api.AddStatementRequest{
			Statement: st,
		})
	return 0, err
}

// addPolicy - Routine to apply global policy statement
func (gbh *GoBgpH) addPolicy(name string, stmt string) (int, error) {
	stmts := make([]*api.Statement, 0, 1)
	stmts = append(stmts, &api.Statement{Name: stmt})
	p := &api.Policy{
		Name:       name,
		Statements: stmts,
	}

	_, err := gbh.client.AddPolicy(context.Background(),
		&api.AddPolicyRequest{
			Policy:                  p,
			ReferExistingStatements: true,
		})
	return 0, err
}

// addPolicy - Routine to apply global policy statement
func (gbh *GoBgpH) applyExportPolicy(remoteIP string, name string) (int, error) {
	assign := &api.PolicyAssignment{Name: remoteIP}
	assign.Direction = api.PolicyDirection_EXPORT
	assign.DefaultAction = api.RouteAction_NONE
	ps := make([]*api.Policy, 0, 1)
	ps = append(ps, &api.Policy{Name: name})
	assign.Policies = ps
	_, err := gbh.client.AddPolicyAssignment(context.Background(),
		&api.AddPolicyAssignmentRequest{
			Assignment: assign,
		})

	return 0, err
}

// removePolicy - Routine to apply global policy statement
func (gbh *GoBgpH) removeExportPolicy(remoteIP string, name string) (int, error) {
	assign := &api.PolicyAssignment{Name: remoteIP}
	assign.Direction = api.PolicyDirection_EXPORT
	assign.DefaultAction = api.RouteAction_NONE
	ps := make([]*api.Policy, 0, 1)
	ps = append(ps, &api.Policy{Name: name})
	assign.Policies = ps
	_, err := gbh.client.DeletePolicyAssignment(context.Background(),
		&api.DeletePolicyAssignmentRequest{
			Assignment: assign,
		})

	return 0, err
}

// resetSingleNeighAdj - Routine to reset a bgp neighbor
func (gbh *GoBgpH) resetSingleNeighAdj(remoteIP string) error {
	var comm string
	soft := true
	dir := api.ResetPeerRequest_OUT
	_, err := gbh.client.ResetPeer(context.Background(), &api.ResetPeerRequest{
		Address:       remoteIP,
		Communication: comm,
		Soft:          soft,
		Direction:     dir,
	})

	tk.LogIt(tk.LogInfo, "[GoBGP] Soft reset neigh %s:%s\n", remoteIP, err.Error())
	return err
}

// BGPGlobalConfigAdd - Routine to add global config in goBGP server
func (gbh *GoBgpH) BGPGlobalConfigAdd(config cmn.GoBGPGlobalConfig) (int, error) {
	lalist := make([]string, 0, 1)
	lalist = append(lalist, "0.0.0.0")

	_, err := gbh.client.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:             uint32(config.LocalAs),
			RouterId:        config.RouterID,
			ListenPort:      int32(config.ListenPort),
			ListenAddresses: lalist,
		},
	})

	if err != nil {
		return -1, err
	}

	gbh.localAs = uint32(config.LocalAs)

	if config.SetNHSelf {
		// Create the set-next-hop-self policy statement
		if _, err := gbh.createNHpolicyStmt("set-next-hop-self-gstmt", "self"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error creating set-next-hop-self stmt %s\n", err.Error())
			return 0, err
		}

		// Create the global policy
		if _, err := gbh.addPolicy("set-next-hop-self-gpolicy", "set-next-hop-self-gstmt"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error creating set-next-hop-self policy%s\n", err.Error())
			return 0, err
		}

		// Apply the global policy
		if _, err := gbh.applyExportPolicy("global", "set-next-hop-self-gpolicy"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error applying set-next-hop-self policy%s\n", err.Error())
			return 0, err
		}
	}

	// Create the set-med policy statement
	if _, err := gbh.createSetMedPolicy("set-med-export-gstmt", 50); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error creating set-med-export-gstmt stmt %s\n", err.Error())
		return 0, err
	}
	// Create the global policy
	if _, err := gbh.addPolicy("set-med-export-gpolicy", "set-med-export-gstmt"); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error creating set-med-export policy%s\n", err.Error())
		return 0, err
	}
	// Apply the global policy
	//if _, err := gbh.applyExportPolicy("global", "set-med-export-gpolicy"); err != nil {
	//	tk.LogIt(tk.LogError, "[GoBGP] Error applying set-med-export policy%s\n", err.Error())
	//	return 0, err
	//}

	return 0, err
}

func getRoutesAndAdvertise() {

	var ipNet net.IPNet
	var nh string

	/* Get Routes and advertise */
	routes, err := nlp.RouteList(nil, nlp.FAMILY_ALL)
	if err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error getting route list %v\n", err)
		return
	}

	if len(routes) != 0 {
		for _, route := range routes {
			/* Filter already added BGP routes */
			if route.Protocol == unix.RTPROT_BGP {
				continue
			}

			if route.Dst == nil {
				r := net.IPv4(0, 0, 0, 0)
				m := net.CIDRMask(0, 32)
				r = r.Mask(m)
				ipNet = net.IPNet{IP: r, Mask: m}
			} else {
				ipNet = *route.Dst
			}
			prefix := ipNet.IP.String()
			plen, _ := ipNet.Mask.Size()

			if ipNet.IP.To4() != nil {
				if route.Gw != nil {
					nh = route.Gw.String()
				} else {
					nh = "0.0.0.0"
				}
				mh.bgp.AdvertiseRoute(prefix, plen, nh, cmn.HighLocalPref, cmn.HighMed, true)
			} else {
				if route.Gw != nil {
					nh = route.Gw.String()
				} else {
					nh = "0.0.0.0"
				}
				mh.bgp.AdvertiseRoute(prefix, plen, nh, cmn.HighLocalPref, cmn.HighMed, false)
			}
		}
	}
}

// goBGPHouseKeeper - Periodic house keeping operations
func (gbh *GoBgpH) goBGPHouseKeeper() {
	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	if err := gbh.AddCurrBgpRoutesToIPRoute(); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s\n", err.Error())
	}

	if gbh.reqRst {
		if time.Duration(time.Since(gbh.resetTS).Seconds()) > time.Duration(4) {
			gbh.reqRst = false
			//gbh.resetNeighAdj()
		}
	}

}

// goBGPTicker - Perform periodic operations related to gobgp
func (gbh *GoBgpH) goBGPTicker() {
	for {
		select {
		case <-gbh.tDone:
			return
		case <-gbh.ticker.C:
			gbh.goBGPHouseKeeper()
		}
	}
}
