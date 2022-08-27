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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

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

type goBgpEvent struct {
	EventType goBgpEventType
	Src       string
	Data      api.Path
	conn      *grpc.ClientConn
}

const (
	BGPConnected goBgpState = iota
	BGPDisconnected
)

type GoBgpH struct {
	eventCh chan goBgpEvent
	host    string
	conn    *grpc.ClientConn
	client  api.GobgpApiClient
	mtx     sync.RWMutex
	state   goBgpState
	rules   map[string]bool
	noNlp   bool
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

func (gbh *GoBgpH) makeMonitorRouteArgs(p *api.Path, showIdentifier bgp.BGPAddPathMode) []interface{} {
	pathStr := make([]interface{}, 0)

	// Title
	title := "ADDROUTE"
	if p.IsWithdraw {
		title = "DELROUTE"
	}
	pathStr = append(pathStr, title)

	// NLRI
	// If Add-Path required, append Path Identifier.
	nlri, _ := apiutil.GetNativeNlri(p)
	if showIdentifier != bgp.BGP_ADD_PATH_NONE {
		pathStr = append(pathStr, p.GetIdentifier())
	}
	pathStr = append(pathStr, nlri)

	attrs, _ := apiutil.GetNativePathAttributes(p)
	// Next Hop
	nexthop := "fictitious"
	if n := gbh.getNextHopFromPathAttributes(attrs); n != nil {
		nexthop = n.String()
	}
	pathStr = append(pathStr, nexthop)

	// AS_PATH
	aspathstr := func() string {
		for _, attr := range attrs {
			switch a := attr.(type) {
			case *bgp.PathAttributeAsPath:
				return bgp.AsPathString(a)
			}
		}
		return ""
	}()
	pathStr = append(pathStr, aspathstr)

	// Path Attributes
	pathStr = append(pathStr, gbh.getPathAttributeString(nlri, attrs))

	return pathStr
}

func (gbh *GoBgpH) processRouteSingle(p *api.Path, showIdentifier bgp.BGPAddPathMode) {
	//pathStr := make([]interface{}, 1)

	pathStr := gbh.makeMonitorRouteArgs(p, showIdentifier)

	format := time.Now().UTC().Format(time.RFC3339)
	if showIdentifier == bgp.BGP_ADD_PATH_NONE {
		format += " [%s] %s via %s aspath [%s] attrs %s\n"
	} else {
		format += " [%s] %d:%s via %s aspath [%s] attrs %s\n"
	}

	tk.LogIt(tk.LOG_INFO, format, pathStr...)

	if err := gbh.syncRoute(p, showIdentifier); err != nil {
		tk.LogIt(tk.LOG_ERROR, " failed to"+format, pathStr...)
	}
}

func (gbh *GoBgpH) syncRoute(p *api.Path, showIdentifier bgp.BGPAddPathMode) error {
	if gbh.noNlp {
		return nil
	}

	// NLRI have destination CIDR info
	nlri, err := apiutil.GetNativeNlri(p)
	if err != nil {
		return err
	}
	_, dstIPN, err := net.ParseCIDR(nlri.String())
	if err != nil {
		return err
	}

	// NextHop
	attrs, err := apiutil.GetNativePathAttributes(p)
	if err != nil {
		return err
	}
	nexthop := gbh.getNextHopFromPathAttributes(attrs)

	// Make netlink route and add
	route := &nlp.Route{
		Dst: dstIPN,
		Gw:  nexthop,
	}

	if p.GetIsWithdraw() {
		tk.LogIt(tk.LOG_DEBUG, "[GoBGP] ip route delete %s via %s", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteDel(route); err != nil {
			tk.LogIt(tk.LOG_ERROR, "[GoBGP] failed to ip route delete. err: %s", err.Error())
			return err
		}
	} else {
		tk.LogIt(tk.LOG_DEBUG, "[GoBGP] ip route add %s via %s", route.Dst.String(), route.Gw.String())
		if err := nlp.RouteAdd(route); err != nil {
			tk.LogIt(tk.LOG_ERROR, "[GoBGP] failed to ip route add. err: %s", err.Error())
			return err
		}
	}

	return nil
}

func (gbh *GoBgpH) processRoute(pathList []*api.Path) {

	for _, p := range pathList {
		gbh.eventCh <- goBgpEvent{
			EventType: bgpRtRecvd,
			Src:       "",
			Data:      *p,
			conn:      &grpc.ClientConn{},
		}
	}
}

func (gbh *GoBgpH) GetRoutes(client api.GobgpApiClient) int {

	processRoutes := func(recver interface {
		Recv() (*api.WatchEventResponse, error)
	}) {
		for {
			r, err := recver.Recv()
			if err == io.EOF {

			} else if err != nil {
				tk.LogIt(tk.LOG_CRITICAL, "processRoutes failed : %v\n", err)

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
		tk.LogIt(tk.LOG_ERROR, "%v", err)
		return -1

	}
	processRoutes(routes)
	return 0
}

func (gbh *GoBgpH) AdvertiseRoute(rtPrefix string, pLen int, nh string) int {

	// add routes
	//tk.LogIt(tk.LOG_DEBUG, "\n\n\n Advertising Route : %v via %v\n\n\n", rtPrefix, nh)
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

	attrs := []*apb.Any{a1, a2}

	_, err := gbh.client.AddPath(context.Background(), &api.AddPathRequest{
		Path: &api.Path{
			Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
			Nlri:   nlri,
			Pattrs: attrs,
		},
	})

	if err != nil {
		tk.LogIt(tk.LOG_CRITICAL, "Advertised Route add failed: %v\n", err)
		return -1
	}
	return 0
}

func (gbh *GoBgpH) DelAdvertiseRoute(rtPrefix string, pLen int, nh string) int {

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

	attrs := []*apb.Any{a1, a2}

	_, err := gbh.client.DeletePath(context.Background(), &api.DeletePathRequest{
		Path: &api.Path{
			Family: &api.Family{Afi: api.Family_AFI_IP, Safi: api.Family_SAFI_UNICAST},
			Nlri:   nlri,
			Pattrs: attrs,
		},
	})

	if err != nil {
		tk.LogIt(tk.LOG_CRITICAL, "Advertised Route del failed: %v", err)
		return -1
	}
	return 0
}

func GoBgpInit() *GoBgpH {
	//gbh = new(GoBgpH)
	gbh := new(GoBgpH)

	gbh.eventCh = make(chan goBgpEvent, cmn.RU_WORKQ_LEN)
	gbh.host = "127.0.0.1:50051"
	gbh.rules = make(map[string]bool)
	gbh.state = BGPDisconnected
	go gbh.goBgpSpawn()
	go gbh.goBgpConnect(gbh.host)
	go gbh.goBgpMonitor()
	return gbh
}

func (gbh *GoBgpH) goBgpSpawn() {
	if _, err := os.Stat("/opt/loxilb/gobgp_loxilb.yaml"); errors.Is(err, os.ErrNotExist) {
		return
	}
	for {
		command := "gobgpd -t yaml -f /opt/loxilb/gobgp_loxilb.yaml"
		cmd := exec.Command("bash", "-c", command)
		err := cmd.Run()
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func (gbh *GoBgpH) goBgpConnect(host string) {
	for {
		gbh.mtx.Lock()
		conn, err := grpc.DialContext(context.TODO(), host, grpc.WithInsecure())
		if err != nil {
			tk.LogIt(tk.LOG_NOTICE, "BGP session %s not alive. Will Retry!\n", gbh.host)
			gbh.mtx.Unlock()
			time.Sleep(2000 * time.Millisecond)
		} else {
			gbh.client = api.NewGobgpApiClient(conn)
			gbh.mtx.Unlock()
			for {
				gbh.mtx.Lock()
				r, err := gbh.client.GetBgp(context.TODO(), &api.GetBgpRequest{})
				if err != nil {

					tk.LogIt(tk.LOG_NOTICE, "BGP session %s not ready. Will Retry!\n", gbh.host)
					gbh.mtx.Unlock()
					time.Sleep(2000 * time.Millisecond)
					continue
				}
				tk.LogIt(tk.LOG_NOTICE, "BGP session %s ready! Attributes[%v]\n", gbh.host, r)

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

func (gbh *GoBgpH) AddBGPRule(Ip string) {
	gbh.mtx.Lock()
	if !gbh.rules[Ip] {
		gbh.rules[Ip] = true
	}
	if gbh.state == BGPConnected {
		gbh.AdvertiseRoute(Ip, 32, "0.0.0.0")
	}

	gbh.mtx.Unlock()
}

func (gbh *GoBgpH) DelBGPRule(ip string) {
	gbh.mtx.Lock()
	if gbh.rules[ip] {
		gbh.rules[ip] = false
	}

	if gbh.state == BGPConnected {
		gbh.DelAdvertiseRoute(ip, 32, "0.0.0.0")
	}
	gbh.mtx.Unlock()
}

func (gbh *GoBgpH) AddCurrentBgpRoutesToIpRoute() error {
	ipv4UC := &api.Family{
		Afi:  api.Family_AFI_IP,
		Safi: api.Family_SAFI_UNICAST,
	}

	stream, err := gbh.client.ListPath(context.TODO(), &api.ListPathRequest{
		TableType: api.TableType_GLOBAL,
		Family:    ipv4UC,
	})
	if err != nil {
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
			return fmt.Errorf("%s is invalid prefix", r.GetPrefix())
		}

		var nlpRoute *nlp.Route
		for _, p := range r.Paths {
			if !p.Best {
				continue
			}

			nexthopIP := net.ParseIP(p.GetNeighborIp())
			if nexthopIP == nil {
				tk.LogIt(tk.LOG_DEBUG, "prefix %s neighbor_ip %s is invalid", r.GetPrefix(), p.GetNeighborIp())
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
		tk.LogIt(tk.LOG_DEBUG, "[GoBGP] ip route add %s via %s", dstIPN.String(), nlpRoute.Gw.String())
		nlp.RouteAdd(nlpRoute)
	}

	return nil
}

func (gbh *GoBgpH) getGoBgpRoutes() {
	gbh.mtx.Lock()

	for ip, valid := range gbh.rules {
		tk.LogIt(tk.LOG_DEBUG, "[GoBGP] connected BGP rules ip %s is valied(%v)", ip, valid)
		if valid {
			gbh.AdvertiseRoute(ip, 32, "0.0.0.0")
		}
	}
	if err := gbh.AddCurrentBgpRoutesToIpRoute(); err != nil {
		tk.LogIt(tk.LOG_ERROR, "[GoBGP] AddCurrentBgpRoutesToIpRoute() return err: %s", err.Error())
	}
	gbh.mtx.Unlock()

	go gbh.GetRoutes(gbh.client)
}

func (gbh *GoBgpH) processBgpEvent(e goBgpEvent) {

	switch e.EventType {
	case bgpDisconnected:
		tk.LogIt(tk.LOG_NOTICE, "******************* BGP %s disconnected *******************\n", gbh.host)
		gbh.conn.Close()
		gbh.conn = nil
		gbh.state = BGPDisconnected
		go gbh.goBgpConnect(gbh.host)
	case bgpConnected:
		tk.LogIt(tk.LOG_NOTICE, "******************* BGP %s connected *******************\n", gbh.host)
		gbh.conn = e.conn
		gbh.state = BGPConnected
		gbh.getGoBgpRoutes()
	case bgpRtRecvd:
		gbh.processRouteSingle(&e.Data, bgp.BGP_ADD_PATH_NONE)
	}
}

func (gbh *GoBgpH) goBgpMonitor() {
	tk.LogIt(tk.LOG_NOTICE, "\n\n\n\nBGP Monitor start *******************\n")
	for {
		for n := 0; n < cmn.RU_WORKQ_LEN; n++ {
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
