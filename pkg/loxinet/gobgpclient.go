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
	"strconv"
	"strings"
	"sync"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	utils "github.com/loxilb-io/loxilb/pkg/utils"
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
	fTicker *time.Ticker
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
	reSync  bool
	pMode   bool
	resetTS time.Time
}

func (gbh *GoBgpH) getGlobalConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	r, err := gbh.client.GetBgp(ctx, &api.GetBgpRequest{})
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

	if err := gbh.syncRoute(p); err != nil {
		tk.LogIt(tk.LogError, " failed to "+format, pathStr...)
	}
}

func (gbh *GoBgpH) syncRoute(p *goBgpRouteInfo) error {
	if gbh.noNlp {
		return nil
	}

	dstIP, dstIPN, err := net.ParseCIDR(p.nlri.String())
	if err != nil {
		return err
	}

	if utils.IsIPHostNetAddr(dstIP) {
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
		gbh.reSync = true
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	_, err := gbh.client.AddPath(ctx, &api.AddPathRequest{
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	_, err := gbh.client.DeletePath(ctx, &api.DeletePathRequest{
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
	gbh.pMode = bgpPeerMode
	gbh.ticker = time.NewTicker(30 * time.Second)
	gbh.fTicker = time.NewTicker(5 * time.Second)
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
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				gbh.mtx.Lock()
				r, err := gbh.client.GetBgp(ctx, &api.GetBgpRequest{})
				if err != nil {
					tk.LogIt(tk.LogInfo, "BGP session %s not ready. Will Retry!\n", gbh.host)
					gbh.mtx.Unlock()
					cancel()
					time.Sleep(2000 * time.Millisecond)
					continue
				}
				cancel()
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	stream, err := gbh.client.ListPath(ctx, &api.ListPathRequest{
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
			tk.LogIt(tk.LogError, "%s is invalid prefix %s\n", r.GetPrefix(), err)
			return err
		}

		if utils.IsIPHostNetAddr(dstIP) {
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
			if nlpRoute != nil {
				nlp.RouteDel(nlpRoute)
			} else {
				tk.LogIt(tk.LogDebug, "prefix %s is invalid\n", r.GetPrefix())
			}
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

		ciState, err := mh.has.CIStateGetInst(cmn.CIDefault)
		if err == nil {
			if ciState == "BACKUP" {
				gbh.resetBGPPolicy(true)
			} else if ciState == "MASTER" {
				gbh.resetBGPPolicy(false)
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
		if gbh.conn != nil {
			gbh.conn.Close()
		}
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
		ciState, err := mh.has.CIStateGetInst(cmn.CIDefault)
		if err == nil {
			if ciState == "BACKUP" {
				gbh.resetBGPPolicy(true)
			} else if ciState == "MASTER" {
				gbh.resetBGPPolicy(false)
			}
		}
	}
	tk.LogIt(tk.LogNotice, "[BGP] Instance %s(%v) HA state updated : %d\n", instance, vip, state)
}

// resetNeighAdj - Reset BGP Neighbor's adjacencies
func (gbh *GoBgpH) resetNeighAdj() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	stream, err := gbh.client.ListPeer(ctx, &api.ListPeerRequest{
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
		//if nb.Conf.PeerAsn != gbh.localAs {
		gbh.resetSingleNeighAdj(nb.Conf.NeighborAddress)
		//}
	}
	return nil
}

// resetBGPPolicy - Reset BGP Policy attributes
func (gbh *GoBgpH) resetBGPPolicy(toLow bool) error {

	if !toLow {
		if _, err := gbh.removeExportPolicy("global", "set-llb-export-gpolicy"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error removing set-llb-export policy%s\n", err.Error())
			// return err
		} else {
			tk.LogIt(tk.LogInfo, "[GoBGP] Removed set-llb-export policy\n")
		}
	} else {
		if _, err := gbh.applyExportPolicy("global", "set-llb-export-gpolicy"); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] Error applying set-llb-export policy%s\n", err.Error())
			//return err
		} else {
			tk.LogIt(tk.LogInfo, "[GoBGP] Applied set-llb-export policy\n")
		}
	}

	gbh.reqRst = true
	gbh.resetTS = time.Now()

	return nil
}

// BGPNeighGet - Routine to get BGP neigh from goBGP server
func (gbh *GoBgpH) BGPNeighGet(address string, enableAdv bool) ([]cmn.GoBGPNeighGetMod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	stream, err := gbh.client.ListPeer(ctx, &api.ListPeerRequest{
		Address:          address,
		EnableAdvertised: enableAdv,
	})
	if err != nil {
		return nil, err
	}

	b := make([]cmn.GoBGPNeighGetMod, 0, 1024)
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		tmpPeer := cmn.GoBGPNeighGetMod{}
		tmpPeer.Addr = r.Peer.State.NeighborAddress
		tmpPeer.RemoteAS = r.Peer.State.PeerAsn
		tmpPeer.State = r.Peer.State.SessionState.String()
		timeStr := "never"
		maxtimelen := len("Up/Down")
		if r.Peer.Timers.State.Uptime != nil {
			t := r.Peer.Timers.State.Downtime.AsTime()
			if r.Peer.State.SessionState == api.PeerState_ESTABLISHED {
				t = r.Peer.Timers.State.Uptime.AsTime()
			}
			timeStr = FormatTimedelta(t)
		}
		if len(timeStr) > maxtimelen {
			maxtimelen = len(timeStr)
		}

		tmpPeer.Uptime = timeStr
		b = append(b, tmpPeer)
	}
	if address != "" && len(b) == 0 {
		return b, fmt.Errorf("not found neighbor %s", address)
	}

	return b, err
}

// BGPNeighMod - Routine to add BGP neigh to goBGP server
func (gbh *GoBgpH) BGPNeighMod(add bool, neigh net.IP, ras uint32, rPort uint32, mhop bool) (int, error) {
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

	if mhop {
		peer.EbgpMultihop = &api.EbgpMultihop{
			Enabled:     true,
			MultihopTtl: uint32(8),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if add {
		_, err = gbh.client.AddPeer(ctx,
			&api.AddPeerRequest{
				Peer: peer,
			})

	} else {
		_, err = gbh.client.DeletePeer(ctx,
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	st := &api.Statement{
		Name:    name,
		Actions: &api.Actions{},
	}
	st.Actions.Nexthop = &api.NexthopAction{}
	st.Actions.Nexthop.Address = addr
	_, err := gbh.client.AddStatement(ctx,
		&api.AddStatementRequest{
			Statement: st,
		})
	return 0, err
}

// createSetMedPolicy - Routine to create set med-policy statement
func (gbh *GoBgpH) createSetMedPolicy(name string, val int64) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	st := &api.Statement{
		Name:    name,
		Actions: &api.Actions{},
	}
	st.Actions.Med = &api.MedAction{}
	st.Actions.Med.Type = api.MedAction_MOD
	st.Actions.Med.Value = val
	_, err := gbh.client.AddStatement(ctx,
		&api.AddStatementRequest{
			Statement: st,
		})
	return 0, err
}

// createSetLocalPrefPolicy - Routine to create set local-pref statement
func (gbh *GoBgpH) createSetLocalPrefPolicy(name string, val uint32) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	st := &api.Statement{
		Name:    name,
		Actions: &api.Actions{},
	}
	st.Actions.LocalPref = &api.LocalPrefAction{}
	st.Actions.LocalPref.Value = val
	_, err := gbh.client.AddStatement(ctx,
		&api.AddStatementRequest{
			Statement: st,
		})
	return 0, err
}

// MakePrefixDefinedSet - Make Prefix DefinedSet
func (gbh *GoBgpH) MakePrefixDefinedSet(prefixList []cmn.Prefix) ([]*api.Prefix, error) {
	var ret []*api.Prefix
	for _, prefix := range prefixList {
		// Make Prefix
		Prefix := api.Prefix{}
		Prefix.IpPrefix = prefix.IpPrefix
		// Parse prefix.MasklengthRange
		Masks := strings.Split(prefix.MasklengthRange, "..")
		if len(Masks) == 2 {
			MaskLengthMin, _ := strconv.Atoi(Masks[0])
			Prefix.MaskLengthMin = uint32(MaskLengthMin)
			MaskLengthMax, _ := strconv.Atoi(Masks[1])
			Prefix.MaskLengthMax = uint32(MaskLengthMax)
		} else {
			return nil, errors.New("Mask format is wrong")
		}
		ret = append(ret, &Prefix)
	}

	return ret, nil
}

// GetPolicyDefinedSet - Get Policy Defined Set
func (gbh *GoBgpH) GetPolicyDefinedSet(name string, DefinedTypeString string) ([]cmn.GoBGPPolicyDefinedSetMod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var DefinedType api.DefinedType
	var ret []cmn.GoBGPPolicyDefinedSetMod
	var req api.ListDefinedSetRequest
	switch DefinedTypeString {
	case "prefix", "Prefix":
		DefinedType = api.DefinedType_PREFIX
	case "neigh", "neighbor", "nei", "Neighbor", "Neigh", "Nei":
		DefinedType = api.DefinedType_NEIGHBOR
	case "Community", "community":
		DefinedType = api.DefinedType_COMMUNITY
	case "ExtCommunity", "extCommunity", "extcommunity":
		DefinedType = api.DefinedType_EXT_COMMUNITY
	case "LargeCommunity", "largecommunity", "largeCommunity", "Largecommunity":
		DefinedType = api.DefinedType_LARGE_COMMUNITY
	case "AsPath", "asPath", "ASPath", "aspath":
		DefinedType = api.DefinedType_AS_PATH
	default:
		return ret, fmt.Errorf("Unsupported type")
	}

	if name == "all" {
		req.DefinedType = DefinedType
	} else {
		req.DefinedType = DefinedType
		req.Name = name
	}
	stream, err := gbh.client.ListDefinedSet(ctx, &req)
	if err != nil {
		return ret, err
	}

	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println(err)
			return ret, err
		}
		var tmp cmn.GoBGPPolicyDefinedSetMod
		switch DefinedTypeString {
		case "prefix", "Prefix":
			tmp.Name = r.DefinedSet.Name
			for _, prefix := range r.DefinedSet.Prefixes {
				if prefix != nil {
					tmpprefix := cmn.Prefix{
						IpPrefix:        prefix.IpPrefix,
						MasklengthRange: fmt.Sprintf("%d..%d", prefix.MaskLengthMin, prefix.MaskLengthMax),
					}
					tmp.PrefixList = append(tmp.PrefixList, tmpprefix)
				}
			}
		default:
			tmp.Name = r.DefinedSet.Name
			tmp.List = r.DefinedSet.List
		}
		ret = append(ret, tmp)
	}
	return ret, nil
}

// GetPolicyDefinitions - Get Policy Definitions
func (gbh *GoBgpH) GetPolicyDefinitions() ([]cmn.GoBGPPolicyDefinitionsMod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var ret []cmn.GoBGPPolicyDefinitionsMod
	stream, err := gbh.client.ListPolicy(ctx, &api.ListPolicyRequest{})
	if err != nil {
		return nil, err
	}
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		tmpPolicy := cmn.GoBGPPolicyDefinitionsMod{
			Name: r.Policy.Name,
		}

		for _, statement := range r.Policy.GetStatements() {
			tmpStatement := cmn.Statement{
				Name: statement.Name,
			}
			// Condition Match
			var PrefixSet cmn.MatchPrefixSet
			var NeighborSet cmn.MatchNeighborSet
			var BGPConditions cmn.BGPConditions

			fmt.Println(statement)
			if t := statement.Conditions.GetPrefixSet(); t != nil {
				PrefixSet.PrefixSet = t.Name
				PrefixSet.MatchSetOption = gbh.GetTypeMatchSet(t.Type)
				tmpStatement.Conditions.PrefixSet = PrefixSet
			}

			if t := statement.Conditions.GetNeighborSet(); t != nil {
				NeighborSet.NeighborSet = t.Name
				NeighborSet.MatchSetOption = gbh.GetTypeMatchSet(t.Type)
				tmpStatement.Conditions.NeighborSet = NeighborSet
			}
			if t := statement.Conditions.GetAfiSafiIn(); t != nil {
				for _, afi := range t {
					BGPConditions.AfiSafiIn = append(BGPConditions.AfiSafiIn, bgp.AfiSafiToRouteFamily(uint16(afi.Afi), uint8(afi.Safi)).String())
				}
				tmpStatement.Conditions.BGPConditions.AfiSafiIn = BGPConditions.AfiSafiIn
			}
			if t := statement.Conditions.GetAsPathLength(); t != nil {
				tmpStatement.Conditions.BGPConditions.AsPathLength.Value = int(t.Length)
				switch t.Type {
				case api.AsPathLength_EQ:
					tmpStatement.Conditions.BGPConditions.AsPathLength.Operator = "eq"
				case api.AsPathLength_GE:
					tmpStatement.Conditions.BGPConditions.AsPathLength.Operator = "ge"
				case api.AsPathLength_LE:
					tmpStatement.Conditions.BGPConditions.AsPathLength.Operator = "le"
				}
			}
			if t := statement.Conditions.GetAsPathSet(); t != nil {
				tmpStatement.Conditions.BGPConditions.AsPathSet.AsPathSet = t.Name
				tmpStatement.Conditions.BGPConditions.AsPathSet.MatchSetOptions = gbh.GetTypeMatchSet(t.Type)
			}
			if t := statement.Conditions.GetCommunitySet(); t != nil {
				tmpStatement.Conditions.BGPConditions.CommunitySet.CommunitySet = t.Name
				tmpStatement.Conditions.BGPConditions.CommunitySet.MatchSetOptions = gbh.GetTypeMatchSet(t.Type)
			}
			if t := statement.Conditions.GetExtCommunitySet(); t != nil {
				tmpStatement.Conditions.BGPConditions.ExtCommunitySet.CommunitySet = t.Name
				tmpStatement.Conditions.BGPConditions.ExtCommunitySet.MatchSetOptions = gbh.GetTypeMatchSet(t.Type)
			}
			if t := statement.Conditions.GetLargeCommunitySet(); t != nil {
				tmpStatement.Conditions.BGPConditions.LargeCommunitySet.CommunitySet = t.Name
				tmpStatement.Conditions.BGPConditions.LargeCommunitySet.MatchSetOptions = gbh.GetTypeMatchSet(t.Type)
			}

			if t := statement.Conditions.GetNextHopInList(); t != nil {
				tmpStatement.Conditions.BGPConditions.NextHopInList = t
			}

			if t := statement.Conditions.GetRouteType(); t != api.Conditions_ROUTE_TYPE_NONE {
				switch t {
				case api.Conditions_ROUTE_TYPE_INTERNAL:
					tmpStatement.Conditions.BGPConditions.RouteType = "internal"
				case api.Conditions_ROUTE_TYPE_EXTERNAL:
					tmpStatement.Conditions.BGPConditions.RouteType = "external"
				case api.Conditions_ROUTE_TYPE_LOCAL:
					tmpStatement.Conditions.BGPConditions.RouteType = "local"
				}
			}

			if t := statement.Conditions.GetRpkiResult(); t != 0 {
				switch t {
				case 1: // RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND
					tmpStatement.Conditions.BGPConditions.Rpki = "not-found"
				case 2: //RPKI_VALIDATION_RESULT_TYPE_VALID
					tmpStatement.Conditions.BGPConditions.Rpki = "valid"
				case 3: //RPKI_VALIDATION_RESULT_TYPE_INVALID
					tmpStatement.Conditions.BGPConditions.Rpki = "invalid"
				}
			}
			// Action Match
			if t := statement.Actions.GetAsPrepend(); t != nil {
				tmpStatement.Actions.BGPActions.SetAsPathPrepend.ASN = fmt.Sprintf("%d", t.Asn)
				tmpStatement.Actions.BGPActions.SetAsPathPrepend.RepeatN = int(t.Repeat)
			}
			if t := statement.Actions.GetCommunity(); t != nil {
				tmpStatement.Actions.BGPActions.SetCommunity.Options = gbh.GetTypeCommunityAction(t.Type)
				tmpStatement.Actions.BGPActions.SetCommunity.SetCommunityMethod = t.Communities
			}
			if t := statement.Actions.GetExtCommunity(); t != nil {
				tmpStatement.Actions.BGPActions.SetExtCommunity.Options = gbh.GetTypeCommunityAction(t.Type)
				tmpStatement.Actions.BGPActions.SetExtCommunity.SetCommunityMethod = t.Communities
			}
			if t := statement.Actions.GetLargeCommunity(); t != nil {
				tmpStatement.Actions.BGPActions.SetLargeCommunity.Options = gbh.GetTypeCommunityAction(t.Type)
				tmpStatement.Actions.BGPActions.SetLargeCommunity.SetCommunityMethod = t.Communities
			}
			if t := statement.Actions.GetLocalPref(); t != nil {
				tmpStatement.Actions.BGPActions.SetLocalPerf = int(t.Value)
			}
			if t := statement.Actions.GetMed(); t != nil {
				tmpStatement.Actions.BGPActions.SetMed = fmt.Sprintf("%d", t.Value)
			}
			if t := statement.Actions.GetNexthop(); t != nil {
				tmpStatement.Actions.BGPActions.SetNextHop = t.Address
			}

			if t := statement.Actions.GetRouteAction(); t != api.RouteAction_NONE {
				tmpStatement.Actions.RouteDisposition = gbh.GetActionRoute(t)
			}

			tmpPolicy.Statement = append(tmpPolicy.Statement, tmpStatement)
		}

		ret = append(ret, tmpPolicy)
	}
	fmt.Println(ret)
	return ret, nil
}

// AddPolicyDefinedSets - Add Policy Defined Set like a Prefix, neighbor. etc
func (gbh *GoBgpH) AddPolicyDefinedSets(df cmn.GoBGPPolicyDefinedSetMod) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var DefinedType api.DefinedType
	var DefinedSet api.DefinedSet
	switch df.DefinedTypeString {
	case "prefix", "Prefix":
		DefinedType = api.DefinedType_PREFIX
	case "neigh", "neighbor", "nei", "Neighbor", "Neigh", "Nei":
		DefinedType = api.DefinedType_NEIGHBOR
	case "Community", "community":
		DefinedType = api.DefinedType_COMMUNITY
	case "ExtCommunity", "extCommunity", "extcommunity":
		DefinedType = api.DefinedType_EXT_COMMUNITY
	case "LargeCommunity", "largecommunity", "largeCommunity", "Largecommunity":
		DefinedType = api.DefinedType_LARGE_COMMUNITY
	case "AsPath", "asPath", "ASPath", "aspath":
		DefinedType = api.DefinedType_AS_PATH
	}
	Prefixes, err := gbh.MakePrefixDefinedSet(df.PrefixList)
	if err != nil {
		return 0, err
	}
	DefinedSet = api.DefinedSet{
		DefinedType: DefinedType,
		Name:        df.Name,
		List:        df.List,
		Prefixes:    Prefixes,
	}

	_, err = gbh.client.AddDefinedSet(ctx, &api.AddDefinedSetRequest{DefinedSet: &DefinedSet})
	return 0, err
}

// DelDefinedSets - Delete DefinedSet
func (gbh *GoBgpH) DelPolicyDefinedSets(Name string, DefinedTypeString string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var DefinedType api.DefinedType

	switch DefinedTypeString {
	case "prefix", "Prefix":
		DefinedType = api.DefinedType_PREFIX
	case "neigh", "neighbor", "nei", "Neighbor", "Neigh", "Nei":
		DefinedType = api.DefinedType_NEIGHBOR
	case "Community", "community":
		DefinedType = api.DefinedType_COMMUNITY
	case "ExtCommunity", "extCommunity", "extcommunity":
		DefinedType = api.DefinedType_EXT_COMMUNITY
	case "LargeCommunity", "largecommunity", "largeCommunity", "Largecommunity":
		DefinedType = api.DefinedType_LARGE_COMMUNITY
	case "AsPath", "asPath", "ASPath", "aspath":
		DefinedType = api.DefinedType_AS_PATH
	}
	// Make DefinedSet
	DefineSet := api.DefinedSet{
		DefinedType: DefinedType,
		Name:        Name,
	}

	_, err := gbh.client.DeleteDefinedSet(ctx, &api.DeleteDefinedSetRequest{
		DefinedSet: &DefineSet,
		All:        true,
	})
	return 0, err
}

// AddPolicyDefinitions - Add Policy with definitions
func (gbh *GoBgpH) AddPolicyDefinitions(name string, stmt []cmn.Statement) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	stmts := make([]*api.Statement, 0)

	// Common statement to goBGP statement
	for _, statement := range stmt {
		tmpStatement := &api.Statement{
			Name:       statement.Name,
			Conditions: &api.Conditions{},
			Actions:    &api.Actions{},
		}
		// Prefix and Neigh Condition add (any(), invert)
		if statement.Conditions.PrefixSet.PrefixSet != "" {
			var PrefixSet api.MatchSet
			PrefixSet.Name = statement.Conditions.PrefixSet.PrefixSet
			PrefixSet.Type = gbh.GetMatchSetType(statement.Conditions.PrefixSet.MatchSetOption)
			tmpStatement.Conditions.PrefixSet = &PrefixSet
		}
		if statement.Conditions.NeighborSet.NeighborSet != "" {
			var NeighborSet api.MatchSet
			NeighborSet.Name = statement.Conditions.NeighborSet.NeighborSet
			NeighborSet.Type = gbh.GetMatchSetType(statement.Conditions.NeighborSet.MatchSetOption)
			tmpStatement.Conditions.NeighborSet = &NeighborSet
		}
		// BGP condition
		if statement.Conditions.BGPConditions.AsPathSet.AsPathSet != "" {
			var AsPathSet api.MatchSet
			AsPathSet.Name = statement.Conditions.BGPConditions.AsPathSet.AsPathSet
			AsPathSet.Type = gbh.GetMatchSetType(statement.Conditions.BGPConditions.AsPathSet.MatchSetOptions)
			tmpStatement.Conditions.AsPathSet = &AsPathSet
		}
		if statement.Conditions.BGPConditions.CommunitySet.CommunitySet != "" {
			var CommunitySet api.MatchSet
			CommunitySet.Name = statement.Conditions.BGPConditions.CommunitySet.CommunitySet
			CommunitySet.Type = gbh.GetMatchSetType(statement.Conditions.BGPConditions.CommunitySet.MatchSetOptions)
			tmpStatement.Conditions.CommunitySet = &CommunitySet
		}
		if statement.Conditions.BGPConditions.ExtCommunitySet.CommunitySet != "" {
			var ExtCommunitySet api.MatchSet
			ExtCommunitySet.Name = statement.Conditions.BGPConditions.ExtCommunitySet.CommunitySet
			ExtCommunitySet.Type = gbh.GetMatchSetType(statement.Conditions.BGPConditions.ExtCommunitySet.MatchSetOptions)
			tmpStatement.Conditions.ExtCommunitySet = &ExtCommunitySet
		}
		if statement.Conditions.BGPConditions.LargeCommunitySet.CommunitySet != "" {
			var LargeCommunitySet api.MatchSet
			LargeCommunitySet.Name = statement.Conditions.BGPConditions.LargeCommunitySet.CommunitySet
			LargeCommunitySet.Type = gbh.GetMatchSetType(statement.Conditions.BGPConditions.LargeCommunitySet.MatchSetOptions)
			tmpStatement.Conditions.LargeCommunitySet = &LargeCommunitySet
		}
		if len(statement.Conditions.BGPConditions.AfiSafiIn) != 0 {
			afiSafisInList := make([]*api.Family, 0, len(statement.Conditions.BGPConditions.AfiSafiIn))
			for _, afisafi := range statement.Conditions.BGPConditions.AfiSafiIn {
				afi, safi := bgp.RouteFamilyToAfiSafi(bgp.AddressFamilyValueMap[afisafi])
				afiSafisInList = append(afiSafisInList, apiutil.ToApiFamily(afi, safi))
			}
			tmpStatement.Conditions.AfiSafiIn = afiSafisInList
		}
		if statement.Conditions.BGPConditions.AsPathLength.Operator != "" {
			var AsPathLength api.AsPathLength
			switch strings.ToLower(statement.Conditions.BGPConditions.AsPathLength.Operator) {
			case "eq":
				AsPathLength.Type = api.AsPathLength_EQ
			case "ge":
				AsPathLength.Type = api.AsPathLength_GE
			case "le":
				AsPathLength.Type = api.AsPathLength_LE
			}
			AsPathLength.Length = uint32(statement.Conditions.BGPConditions.AsPathLength.Value)
			tmpStatement.Conditions.AsPathLength = &AsPathLength
		}
		// From gobgp code
		type RpkiValidationResultType string
		const (
			RPKI_VALIDATION_RESULT_TYPE_NONE      RpkiValidationResultType = "none"
			RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND RpkiValidationResultType = "not-found"
			RPKI_VALIDATION_RESULT_TYPE_VALID     RpkiValidationResultType = "valid"
			RPKI_VALIDATION_RESULT_TYPE_INVALID   RpkiValidationResultType = "invalid"
		)

		var RpkiValidationResultTypeToIntMap = map[RpkiValidationResultType]int{
			RPKI_VALIDATION_RESULT_TYPE_NONE:      0,
			RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND: 1,
			RPKI_VALIDATION_RESULT_TYPE_VALID:     2,
			RPKI_VALIDATION_RESULT_TYPE_INVALID:   3,
		}
		if statement.Conditions.BGPConditions.Rpki != "" {
			switch strings.ToLower(statement.Conditions.BGPConditions.Rpki) {
			case "valid":
				tmpStatement.Conditions.RpkiResult = int32(RpkiValidationResultTypeToIntMap[RPKI_VALIDATION_RESULT_TYPE_VALID])
			case "invalid":
				tmpStatement.Conditions.RpkiResult = int32(RpkiValidationResultTypeToIntMap[RPKI_VALIDATION_RESULT_TYPE_INVALID])
			case "not-found":
				tmpStatement.Conditions.RpkiResult = int32(RpkiValidationResultTypeToIntMap[RPKI_VALIDATION_RESULT_TYPE_NOT_FOUND])
			}
		}
		if statement.Conditions.BGPConditions.RouteType != "" {
			switch strings.ToLower(statement.Conditions.BGPConditions.RouteType) {
			case "internal":
				tmpStatement.Conditions.RouteType = api.Conditions_ROUTE_TYPE_INTERNAL
			case "external":
				tmpStatement.Conditions.RouteType = api.Conditions_ROUTE_TYPE_EXTERNAL
			case "local":
				tmpStatement.Conditions.RouteType = api.Conditions_ROUTE_TYPE_LOCAL
			}
		}

		if len(statement.Conditions.BGPConditions.NextHopInList) != 0 {
			tmpStatement.Conditions.NextHopInList = statement.Conditions.BGPConditions.NextHopInList
		}

		// Action
		tmpStatement.Actions.RouteAction = gbh.GetRouteAction(statement.Actions.RouteDisposition)

		if statement.Actions.BGPActions.SetAsPathPrepend.ASN != "" {
			var AsPrepend api.AsPrependAction
			tmpASN, _ := strconv.Atoi(statement.Actions.BGPActions.SetAsPathPrepend.ASN)
			AsPrepend.Asn = uint32(tmpASN)
			AsPrepend.Repeat = uint32(statement.Actions.BGPActions.SetAsPathPrepend.RepeatN)
			tmpStatement.Actions.AsPrepend = &AsPrepend
		}

		if len(statement.Actions.BGPActions.SetCommunity.SetCommunityMethod) != 0 {
			var Community api.CommunityAction
			Community.Communities = statement.Actions.BGPActions.SetCommunity.SetCommunityMethod
			Community.Type = gbh.GetCommunityActionType(statement.Actions.BGPActions.SetCommunity.Options)
			tmpStatement.Actions.Community = &Community
		}
		if len(statement.Actions.BGPActions.SetExtCommunity.SetCommunityMethod) != 0 {
			var Community api.CommunityAction
			Community.Communities = statement.Actions.BGPActions.SetExtCommunity.SetCommunityMethod
			Community.Type = gbh.GetCommunityActionType(statement.Actions.BGPActions.SetExtCommunity.Options)
			tmpStatement.Actions.ExtCommunity = &Community
		}
		if len(statement.Actions.BGPActions.SetLargeCommunity.SetCommunityMethod) != 0 {
			var Community api.CommunityAction
			Community.Communities = statement.Actions.BGPActions.SetLargeCommunity.SetCommunityMethod
			Community.Type = gbh.GetCommunityActionType(statement.Actions.BGPActions.SetLargeCommunity.Options)
			tmpStatement.Actions.LargeCommunity = &Community
		}

		if statement.Actions.BGPActions.SetMed != "" {
			var Med api.MedAction
			med, err := strconv.ParseInt(statement.Actions.BGPActions.SetMed, 10, 32)
			if err != nil {
				return 0, err
			}
			Med.Value = med
			Med.Type = api.MedAction_REPLACE // Cause set-med
			tmpStatement.Actions.Med = &Med
		}
		if statement.Actions.BGPActions.SetLocalPerf != 0 {
			var LocalPref api.LocalPrefAction
			LocalPref.Value = uint32(statement.Actions.BGPActions.SetLocalPerf)
			tmpStatement.Actions.LocalPref = &LocalPref
		}
		if statement.Actions.BGPActions.SetNextHop != "" {
			var Nexthop api.NexthopAction
			Nexthop.Address = statement.Actions.BGPActions.SetNextHop
			tmpStatement.Actions.Nexthop = &Nexthop
		}

		stmts = append(stmts, tmpStatement)
	}
	p := &api.Policy{
		Name:       name,
		Statements: stmts,
	}

	_, err := gbh.client.AddPolicy(ctx,
		&api.AddPolicyRequest{
			Policy: p,
		})
	return 0, err
}

// DelPolicyDefinitions - Del Policy Definitions
func (gbh *GoBgpH) DelPolicyDefinitions(name string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	p := &api.Policy{
		Name: name,
	}

	_, err := gbh.client.DeletePolicy(ctx,
		&api.DeletePolicyRequest{
			Policy: p,
			All:    true,
		})
	return 0, err
}

// GetCommunityActionType - String to CommunityAction_Type
func (gbh *GoBgpH) GetCommunityActionType(name string) api.CommunityAction_Type {
	var ret api.CommunityAction_Type // add remove replace
	if name == "add" {
		ret = api.CommunityAction_ADD
	} else if name == "remove" {
		ret = api.CommunityAction_REMOVE
	} else {
		ret = api.CommunityAction_REPLACE
	}
	return ret
}

// GetCommunityActionType - String to CommunityAction_Type
func (gbh *GoBgpH) GetTypeCommunityAction(CommunityActionType api.CommunityAction_Type) string {
	var ret string // add remove replace
	if CommunityActionType == api.CommunityAction_ADD {
		ret = "add"
	} else if CommunityActionType == api.CommunityAction_REMOVE {
		ret = "remove"
	} else {
		ret = "replace"
	}
	return ret
}

// GetMatchSetType - String to MatchSet_Type
func (gbh *GoBgpH) GetMatchSetType(name string) api.MatchSet_Type {
	var ret api.MatchSet_Type
	if name == "any" {
		ret = api.MatchSet_ANY
	} else if name == "all" {
		ret = api.MatchSet_ALL
	} else if name == "invert" {
		ret = api.MatchSet_INVERT
	} else {
		ret = api.MatchSet_ANY
	}
	return ret
}

// GetTypeMatchSet - MatchSet_Type to String
func (gbh *GoBgpH) GetTypeMatchSet(matchSet api.MatchSet_Type) string {
	var ret string
	if matchSet == api.MatchSet_ANY {
		ret = "any"
	} else if matchSet == api.MatchSet_ALL {
		ret = "all"
	} else if matchSet == api.MatchSet_INVERT {
		ret = "invert"
	} else {
		ret = "any"
	}
	return ret
}

// GetRouteAction - String to RouteAction
func (gbh *GoBgpH) GetRouteAction(name string) api.RouteAction {
	var ret api.RouteAction
	if name == "accept-route" {
		ret = api.RouteAction_ACCEPT
	} else if name == "reject-route" {
		ret = api.RouteAction_REJECT
	} else {
		ret = api.RouteAction_NONE
	}
	return ret
}

// GetActionRoute - RouteAction to String
func (gbh *GoBgpH) GetActionRoute(route api.RouteAction) string {
	var ret string
	if route == api.RouteAction_ACCEPT {
		ret = "accept-route"
	} else if route == api.RouteAction_REJECT {
		ret = "reject-route"
	} else {
		ret = "none"
	}
	return ret
}

// BGPApplyPolicyToNeighbor - Routine to add BGP Policy to goBGP server
func (gbh *GoBgpH) BGPApplyPolicyToNeighbor(cmdType, neigh string, polType string, policies []string, routeAction string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var err error
	assign := &api.PolicyAssignment{
		Name: neigh,
	}

	switch strings.ToLower(polType) {
	case "import":
		assign.Direction = api.PolicyDirection_IMPORT
	case "export":
		assign.Direction = api.PolicyDirection_EXPORT
	}
	switch cmdType {
	case "add", "set":
		switch routeAction {
		case "accept":
			assign.DefaultAction = api.RouteAction_ACCEPT
		case "reject":
			assign.DefaultAction = api.RouteAction_REJECT
		}
	}
	ps := make([]*api.Policy, 0, len(policies))
	for _, name := range policies {
		ps = append(ps, &api.Policy{Name: name})
	}
	assign.Policies = ps
	switch cmdType {
	case "add":
		_, err = gbh.client.AddPolicyAssignment(ctx, &api.AddPolicyAssignmentRequest{
			Assignment: assign,
		})
	case "set":
		_, err = gbh.client.SetPolicyAssignment(ctx, &api.SetPolicyAssignmentRequest{
			Assignment: assign,
		})
	case "del":
		all := false
		if len(policies) == 0 {
			all = true
		}
		_, err = gbh.client.DeletePolicyAssignment(ctx, &api.DeletePolicyAssignmentRequest{
			Assignment: assign,
			All:        all,
		})
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// GetPolicy - Routine to apply global policy statement
func (gbh *GoBgpH) GetPolicy(name string) (*api.Policy, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	policies := make([]*api.Policy, 0)
	stream, err := gbh.client.ListPolicy(ctx, &api.ListPolicyRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		policies = append(policies, r.Policy)
	}

	return policies[0], nil
}

// addPolicy - Routine to apply global policy statement
func (gbh *GoBgpH) addPolicy(name string, stmt string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	stmts := make([]*api.Statement, 0, 1)
	stmts = append(stmts, &api.Statement{Name: stmt})
	p := &api.Policy{
		Name:       name,
		Statements: stmts,
	}

	_, err := gbh.client.AddPolicy(ctx,
		&api.AddPolicyRequest{
			Policy:                  p,
			ReferExistingStatements: true,
		})
	return 0, err
}

// addPolicy - Routine to apply global policy statement
func (gbh *GoBgpH) applyExportPolicy(remoteIP string, name string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	assign := &api.PolicyAssignment{Name: remoteIP}
	assign.Direction = api.PolicyDirection_EXPORT
	assign.DefaultAction = api.RouteAction_NONE
	ps := make([]*api.Policy, 0, 1)
	ps = append(ps, &api.Policy{Name: name})
	assign.Policies = ps
	_, err := gbh.client.AddPolicyAssignment(ctx,
		&api.AddPolicyAssignmentRequest{
			Assignment: assign,
		})

	return 0, err
}

// removePolicy - Routine to apply global policy statement
func (gbh *GoBgpH) removeExportPolicy(remoteIP string, name string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	assign := &api.PolicyAssignment{Name: remoteIP}
	assign.Direction = api.PolicyDirection_EXPORT
	assign.DefaultAction = api.RouteAction_NONE
	ps := make([]*api.Policy, 0, 1)
	ps = append(ps, &api.Policy{Name: name})
	assign.Policies = ps
	_, err := gbh.client.DeletePolicyAssignment(ctx,
		&api.DeletePolicyAssignmentRequest{
			Assignment: assign,
		})

	return 0, err
}

// resetSingleNeighAdj - Routine to reset a bgp neighbor
func (gbh *GoBgpH) resetSingleNeighAdj(remoteIP string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	var comm string
	soft := true
	dir := api.ResetPeerRequest_OUT
	_, err := gbh.client.ResetPeer(ctx, &api.ResetPeerRequest{
		Address:       remoteIP,
		Communication: comm,
		Soft:          soft,
		Direction:     dir,
	})

	tk.LogIt(tk.LogInfo, "[GoBGP] Soft reset neigh %s\n", remoteIP)
	return err
}

// BGPGlobalConfigAdd - Routine to add global config in goBGP server
func (gbh *GoBgpH) BGPGlobalConfigAdd(config cmn.GoBGPGlobalConfig) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	lalist := make([]string, 0, 1)
	lalist = append(lalist, "0.0.0.0")

	_, err := gbh.client.StartBgp(ctx, &api.StartBgpRequest{
		Global: &api.Global{
			Asn:             uint32(config.LocalAs),
			RouterId:        config.RouterID,
			ListenPort:      int32(config.ListenPort),
			ListenAddresses: lalist,
		},
	})

	if err != nil && !strings.Contains(err.Error(), "address already in use") {
		tk.LogIt(tk.LogError, "[GoBGP] Error to start BGP %s \n", err.Error())
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
	// Create the set-local-pref policy statement
	if _, err := gbh.createSetLocalPrefPolicy("set-localpref-export-gstmt", 10); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error creating set-localpref-export-gstmt %s\n", err.Error())
		return 0, err
	}
	// Create the global policy
	if _, err := gbh.addPolicy("set-llb-export-gpolicy", "set-med-export-gstmt"); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error creating set-med-export policy%s\n", err.Error())
		return 0, err
	}
	if _, err := gbh.addPolicy("set-llb-export-gpolicy", "set-localpref-export-gstmt"); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] Error creating set-localpref-export-gstmt policy%s\n", err.Error())
		return 0, err
	}
	// Apply the global policy
	//if _, err := gbh.applyExportPolicy("global", "set-llb-export-gpolicy"); err != nil {
	//	tk.LogIt(tk.LogError, "[GoBGP] Error applying set-llb-export policy%s\n", err.Error())
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

// goBGPLazyHouseKeeper - Periodic (lazy) house keeping operations
func (gbh *GoBgpH) goBGPLazyHouseKeeper() {
	if gbh.pMode {
		return
	}

	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	if err := gbh.AddCurrBgpRoutesToIPRoute(); err != nil {
		tk.LogIt(tk.LogError, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s\n", err.Error())
	}
}

// goBGPHouseKeeper - Periodic (faster) house keeping operations
func (gbh *GoBgpH) goBGPHouseKeeper() {
	gbh.mtx.Lock()
	defer gbh.mtx.Unlock()

	rsync := false

	if gbh.reSync || gbh.pMode {
		if err := gbh.AddCurrBgpRoutesToIPRoute(); err != nil {
			tk.LogIt(tk.LogError, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s\n", err.Error())
		}
		gbh.reSync = false
		rsync = true
	}

	if gbh.reqRst {
		if time.Duration(time.Since(gbh.resetTS).Seconds()) > time.Duration(4) {
			gbh.reqRst = false
			gbh.resetNeighAdj()
			if !rsync {
				gbh.AddCurrBgpRoutesToIPRoute()
			}
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
			gbh.goBGPLazyHouseKeeper()
		case <-gbh.fTicker.C:
			gbh.goBGPHouseKeeper()
		}
	}
}
