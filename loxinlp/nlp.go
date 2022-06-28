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
package loxinlp

import (
	"fmt"
	cmn "loxilb/common"
	tk "loxilb/loxilib"
	"net"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	nlp "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	IF_OPER_UNKNOWN uint8 = iota
	IF_OPER_NOTPRESENT
	IF_OPER_DOWN
	IF_OPER_LOWERLAYERDOWN
	IF_OPER_TESTING
	IF_OPER_DORMANT
	IF_OPER_UP
)

type AddrUpdateCh struct {
	FromAUCh   chan nlp.AddrUpdate
	FromAUDone chan struct{}
}
type LinkUpdateCh struct {
	FromLUCh   chan nlp.LinkUpdate
	FromLUDone chan struct{}
}
type NeighUpdateCh struct {
	FromNUCh   chan nlp.NeighUpdate
	FromNUDone chan struct{}
}
type RouteUpdateCh struct {
	FromRUCh   chan nlp.RouteUpdate
	FromRUDone chan struct{}
}

type NlH struct {
	AddrUpdateCh
	LinkUpdateCh
	NeighUpdateCh
	RouteUpdateCh
}

var hooks cmn.NetHookInterface

func NlpRegister(hook cmn.NetHookInterface) {
	hooks = hook
}

func ModLink(link nlp.Link, add bool) int {
	var ifMac [6]byte
	var ret int
	var err error
	var mod string
	var vid int
	var brLink nlp.Link
	re := regexp.MustCompile("[0-9]+")

	attrs := link.Attrs()
	name := attrs.Name
	idx := attrs.Index

	if len(attrs.HardwareAddr) > 0 {
		copy(ifMac[:], attrs.HardwareAddr[:6])
	}

	mtu := attrs.MTU
	linkState := attrs.Flags&net.FlagUp == 1
	state := uint8(attrs.OperState) == IF_OPER_UP
	if add {
		mod = "add"
	} else {
		mod = "delete"
	}
	tk.LogIt(tk.LOG_DEBUG, "%s Device %v mac(%v) info recvd\n", mod, name, ifMac)

	if _, ok := link.(*nlp.Bridge); ok {

		vid, _ = strconv.Atoi(strings.Join(re.FindAllString(name, -1), " "))
		if add {
			ret, err = hooks.NetVlanAdd(&cmn.VlanMod{Vid: vid, Dev: name, LinkIndex: idx,
				MacAddr: ifMac, Link: linkState, State: state, Mtu: mtu, TunId: 0})
		} else {
			ret, err = hooks.NetVlanDel(&cmn.VlanMod{Vid: vid})
		}

		if err != nil {
			tk.LogIt(tk.LOG_INFO, "Bridge %v, %d, %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "Bridge %v, %d, %v, %v, %v %s [OK]\n", name, vid, ifMac, state, mtu, mod)
		}
	}

	/* Get bridge detail */
	if attrs.MasterIndex > 0 {
		brLink, err = nlp.LinkByIndex(attrs.MasterIndex)
		if err != nil {
			fmt.Println(err)
			return -1
		}
		vid, _ = strconv.Atoi(strings.Join(re.FindAllString(brLink.Attrs().Name, -1), " "))
	}

	/* Tagged Vlan port */
	if strings.Contains(name, ".") {
		/* Currently, Sub-interfaces can only be part of bridges */
		if attrs.MasterIndex > 0 {
			pname := strings.Split(name, ".")
			if add {
				ret, err = hooks.NetVlanPortAdd(&cmn.VlanPortMod{Vid: vid, Dev: pname[0], Tagged: true})
			} else {
				ret, err = hooks.NetVlanPortDel(&cmn.VlanPortMod{Vid: vid, Dev: pname[0], Tagged: true})
			}
			if err != nil {
				tk.LogIt(tk.LOG_ERROR, "TVlan Port %v, v(%v), %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
				fmt.Println(err)
			} else {
				tk.LogIt(tk.LOG_INFO, "TVlan Port %v, v(%v), %v, %v, %v %s OK\n", name, vid, ifMac, state, mtu, mod)
			}

		}
		return ret
	}

	/* Physical port/ Bond/ VxLAN */
	master := ""
	real := ""
	pType := cmn.PORT_REAL
	tunId := 0
	if vxlan, ok := link.(*nlp.Vxlan); ok {
		pType = cmn.PORT_VXLANBR
		tunId = vxlan.VxlanId
		uif, err := nlp.LinkByIndex(vxlan.VtepDevIndex)
		if err != nil {
			fmt.Println(err)
			return -1
		}
		real = uif.Attrs().Name
		tk.LogIt(tk.LOG_INFO, "Port %v, uif %v %s\n", name, real, mod)
	}

	if add {

		ret, err = hooks.NetPortAdd(&cmn.PortMod{Dev: name, LinkIndex: idx, Ptype: pType, MacAddr: ifMac,
			Link: linkState, State: state, Mtu: mtu, Master: master, Real: real,
			TunId: tunId})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Port %v, %v, %v, %v add failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "Port %v, %v, %v, %v add [OK]\n", name, ifMac, state, mtu)
		}
	} else if attrs.MasterIndex == 0 {
		ret, err = hooks.NetPortDel(&cmn.PortMod{Dev: name, Ptype: pType})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Port %v, %v, %v, %v delete failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "Port %v, %v, %v, %v delete [OK]\n", name, ifMac, state, mtu)
		}
		return ret
	}

	/* Untagged vlan ports */
	if attrs.MasterIndex > 0 {
		if add {
			ret, err = hooks.NetVlanPortAdd(&cmn.VlanPortMod{Vid: vid, Dev: name, Tagged: false})
		} else {
			ret, err = hooks.NetVlanPortDel(&cmn.VlanPortMod{Vid: vid, Dev: name, Tagged: false})
		}
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Vlan Port %v, %v, %v, %v %s failed\n", name, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "Vlan Port %v, %v, %v, %v %s [OK]\n", name, ifMac, state, mtu, mod)
		}
	}
	return ret
}

func AddAddr(addr nlp.Addr, link nlp.Link) int {
	var ret int

	attrs := link.Attrs()
	name := attrs.Name
	ipStr := (addr.IPNet).String()

	ret, err := hooks.NetIpv4AddrAdd(&cmn.Ipv4AddrMod{Dev: name, Ip: ipStr})
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "IPv4 Address %v Port %v failed %v\n", ipStr, name, err)
		ret = -1
	} else {
		tk.LogIt(tk.LOG_INFO, "IPv4 Address %v Port %v added\n", ipStr, name)
	}
	return ret
}

func AddNeigh(neigh nlp.Neigh, link nlp.Link) int {
	var ret int
	var brId int
	var mac [6]byte
	var brMac [6]byte
	var err error
	var dst net.IP
	var ftype int

	re := regexp.MustCompile("[0-9]+")
	attrs := link.Attrs()
	name := attrs.Name

	if len(neigh.HardwareAddr) == 0 {
		return -1
	}
	copy(mac[:], neigh.HardwareAddr[:6])

	if neigh.Family == unix.AF_INET {
		ret, err = hooks.NetNeighv4Add(&cmn.Neighv4Mod{Ip: neigh.IP, LinkIndex: neigh.LinkIndex,
			State:        neigh.State,
			HardwareAddr: neigh.HardwareAddr})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "NH %v mac %v dev %v add failed %v\n", neigh.IP.String(), mac,
				name, err)

		} else {
			tk.LogIt(tk.LOG_INFO, "NH %v mac %v dev %v added\n", neigh.IP.String(), mac, name)
		}
	} else if neigh.Family == unix.AF_BRIDGE {

		if len(neigh.HardwareAddr) == 0 {
			return -1
		}
		copy(mac[:], neigh.HardwareAddr[:6])

		if neigh.Vlan == 1 {
			/*FDB comes with vlan 1 also */
			return 0
		}

		if mac[0]&0x01 == 1 || mac[0] == 0 {
			/* Multicast MAC or ZERO address --- IGNORED */
			return 0
		}

		if neigh.MasterIndex > 0 {
			brLink, err := nlp.LinkByIndex(neigh.MasterIndex)
			if err != nil {
				fmt.Println(err)
				return -1
			}

			copy(brMac[:], brLink.Attrs().HardwareAddr[:6])
			if mac == brMac {
				/*Same as bridge mac --- IGNORED */
				return 0
			}
			brId, _ = strconv.Atoi(strings.Join(re.FindAllString(brLink.Attrs().Name, -1), " "))
		}

		if vxlan, ok := link.(*nlp.Vxlan); ok {
			/* Interested in only VxLAN FDB */
			if len(neigh.IP) > 0 && (neigh.MasterIndex == 0) {
				dst = neigh.IP
				brId = vxlan.VxlanId
				ftype = cmn.FDB_TUN
			} else {
				return 0
			}
		} else {
			dst = net.ParseIP("0.0.0.0")
			ftype = cmn.FDB_VLAN
		}

		ret, err = hooks.NetFdbAdd(&cmn.FdbMod{MacAddr: mac, BridgeId: brId, Dev: name, Dst: dst,
			Type: ftype})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "L2fdb %v brId %v dst %v dev %v add failed\n", mac[:], brId, dst, name)
		} else {
			tk.LogIt(tk.LOG_INFO, "L2fdb %v brId %v dst %v dev %v added\n", mac[:], brId, dst, name)
		}
	}

	return ret

}

func DelNeigh(neigh nlp.Neigh, link nlp.Link) int {
	var ret int
	var mac [6]byte
	var brMac [6]byte
	var brId int
	var err error
	var dst net.IP

	re := regexp.MustCompile("[0-9]+")
	attrs := link.Attrs()
	name := attrs.Name

	if neigh.Family == unix.AF_INET {
		ret, err = hooks.NetNeighv4Del(&cmn.Neighv4Mod{Ip: neigh.IP})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "NH  %v %v del failed\n", neigh.IP.String(), name)
			ret = -1
		} else {
			tk.LogIt(tk.LOG_ERROR, "NH %v %v deleted\n", neigh.IP.String(), name)
		}
	} else {

		if neigh.Vlan == 1 {
			/*FDB comes with vlan 1 also */
			return 0
		}
		if len(neigh.HardwareAddr) == 0 {
			return -1
		}

		copy(mac[:], neigh.HardwareAddr[:6])
		if mac[0]&0x01 == 1 || mac[0] == 0 {
			/* Multicast MAC or ZERO address --- IGNORED */
			return 0
		}

		if neigh.MasterIndex > 0 {
			brLink, err := nlp.LinkByIndex(neigh.MasterIndex)
			if err != nil {
				fmt.Println(err)
				return -1
			}

			copy(brMac[:], brLink.Attrs().HardwareAddr[:6])
			if mac == brMac {
				/*Same as bridge mac --- IGNORED */
				return 0
			}
			brId, _ = strconv.Atoi(strings.Join(re.FindAllString(brLink.Attrs().Name, -1), " "))
		}

		if vxlan, ok := link.(*nlp.Vxlan); ok {
			/* Interested in only VxLAN FDB */
			if len(neigh.IP) > 0 && (neigh.MasterIndex == 0) {
				dst = neigh.IP
				brId = vxlan.VxlanId
			} else {
				return 0
			}
		} else {
			dst = net.ParseIP("0.0.0.0")
		}

		ret, err = hooks.NetFdbDel(&cmn.FdbMod{MacAddr: mac, BridgeId: brId})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "L2fdb %v brId %v dst %s dev %v delete failed %v\n", mac[:], brId, dst, name, err)
			ret = -1
		} else {
			tk.LogIt(tk.LOG_INFO, "L2fdb %v brId %v dst %s dev %v deleted\n", mac[:], brId, dst, name)
		}
	}
	return ret
}

func AddRoute(route nlp.Route) int {

	ret, err := hooks.NetRoutev4Add(&cmn.Routev4Mod{Protocol: int(route.Protocol), Flags: route.Flags,
		Gw: route.Gw, LinkIndex: route.LinkIndex, Dst: route.Dst})
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "RT add failed-%s\n", err)
	} else {
		tk.LogIt(tk.LOG_DEBUG, "RT %s via %s added\n", route.Dst.String(), route.Gw.String())
	}
	return ret
}

func DelRoute(route nlp.Route) int {
	var ret int
	ret, err := hooks.NetRoutev4Del(&cmn.Routev4Mod{Dst: route.Dst})
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "RT del failed-%s\n", err)
	} else {
		tk.LogIt(tk.LOG_DEBUG, "RT %s via %s deleted\n", route.Dst.String(), route.Gw.String())
	}
	return ret
}

func LUWorkSingle(m nlp.LinkUpdate) int {
	var ret int
	ret = ModLink(m.Link, m.Header.Type == syscall.RTM_NEWLINK)
	return ret
}

func AUWorkSingle(m nlp.AddrUpdate) int {
	var ret int
	link, err := nlp.LinkByIndex(m.LinkIndex)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	attrs := link.Attrs()
	name := attrs.Name
	if m.NewAddr {
		_, err := hooks.NetIpv4AddrAdd(&cmn.Ipv4AddrMod{Dev: name, Ip: m.LinkAddress.String()})
		if err != nil {
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "IPv4 Address %v Port %v added\n", m.LinkAddress.String(), name)
		}

	} else {

		_, err := hooks.NetIpv4AddrDel(&cmn.Ipv4AddrMod{Dev: name, Ip: m.LinkAddress.String()})
		if err != nil {
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "IPv4 Address %v Port %v deleted\n", m.LinkAddress.String(), name)
		}
	}

	return ret
}

func NUWorkSingle(m nlp.NeighUpdate) int {
	var ret int

	link, err := nlp.LinkByIndex(m.LinkIndex)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	add := m.Type == syscall.RTM_NEWNEIGH

	if add {
		ret = AddNeigh(m.Neigh, link)
	} else {
		ret = DelNeigh(m.Neigh, link)
	}

	return ret
}

func RUWorkSingle(m nlp.RouteUpdate) int {
	var ret int
	if m.Dst != nil {
		if m.Type == syscall.RTM_NEWROUTE {
			ret = AddRoute(m.Route)
		} else {
			ret = DelRoute(m.Route)
		}
	} else {
		fmt.Println("RT mod missing IP")
	}
	return ret
}

func LUWorker(ch chan nlp.LinkUpdate, f chan struct{}) {
	for {
		for n := 0; n < cmn.LU_WORKQ_LEN; n++ {
			select {
			case m := <-ch:
				LUWorkSingle(m)
			case <-f:
				return
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func AUWorker(ch chan nlp.AddrUpdate, f chan struct{}) {
	for {
		for n := 0; n < cmn.AU_WORKQ_LEN; n++ {
			select {
			case m := <-ch:
				AUWorkSingle(m)
			case <-f:
				return
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func NUWorker(ch chan nlp.NeighUpdate, f chan struct{}) {
	for {
		for n := 0; n < cmn.NU_WORKQ_LEN; n++ {
			select {
			case m := <-ch:
				NUWorkSingle(m)
			case <-f:
				return
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func RUWorker(ch chan nlp.RouteUpdate, f chan struct{}) {
	for {
		for n := 0; n < cmn.RU_WORKQ_LEN; n++ {
			select {
			case m := <-ch:
				RUWorkSingle(m)
			case <-f:
				return
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func GetBridges() {
	links, err := nlp.LinkList()
	if err != nil {
		return
	}
	for _, link := range links {
		switch link.(type) {
		case *nlp.Bridge:
			{
				ModLink(link, true)
			}
		}
	}
}

func NlpGet() int {
	var ret int
	tk.LogIt(tk.LOG_INFO, "Getting device info\n")

	GetBridges()

	links, err := nlp.LinkList()
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "Error in getting device info(%v)\n", err)
		ret = -1
	}

	for _, link := range links {
		ret = ModLink(link, true)

		if ret == -1 {
			continue
		}

		/* Get FDBs */
		if link.Attrs().MasterIndex > 0 {
			neighs, err := nlp.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
			if err != nil {
				tk.LogIt(tk.LOG_ERROR, "Error getting neighbors list %v for intf %s\n",
					err, link.Attrs().Name)
			}

			if len(neighs) == 0 {
				tk.LogIt(tk.LOG_DEBUG, "No FDBs found for intf %s\n", link.Attrs().Name)
			} else {
				for _, neigh := range neighs {
					AddNeigh(neigh, link)
				}
			}
		}

		addrs, err := nlp.AddrList(link, nlp.FAMILY_V4)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Error getting address list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(addrs) == 0 {
			tk.LogIt(tk.LOG_DEBUG, "No addresses found for intf %s\n", link.Attrs().Name)
		} else {
			for _, addr := range addrs {
				AddAddr(addr, link)
			}
		}

		neighs, err := nlp.NeighList(link.Attrs().Index, nlp.FAMILY_ALL)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Error getting neighbors list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(neighs) == 0 {
			tk.LogIt(tk.LOG_DEBUG, "No neighbors found for intf %s\n", link.Attrs().Name)
		} else {
			for _, neigh := range neighs {
				AddNeigh(neigh, link)
			}
		}
	}

	/* Get Routes */
	rFilter := nlp.Route{Protocol: syscall.RTPROT_STATIC}
	routes, err := nlp.RouteListFiltered(nlp.FAMILY_V4, &rFilter, nlp.RT_FILTER_PROTOCOL)

	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "Error getting route list %v\n", err)
	}

	if len(routes) == 0 {
		tk.LogIt(tk.LOG_DEBUG, "No routes found in the system!\n")
	} else {
		for _, route := range routes {
			AddRoute(route)
		}
	}

	tk.LogIt(tk.LOG_INFO, "nlp get done\n")
	return ret
}

func NlpInit() *NlH {

	nNl := new(NlH)

	nNl.FromAUCh = make(chan nlp.AddrUpdate, cmn.AU_WORKQ_LEN)
	nNl.FromLUCh = make(chan nlp.LinkUpdate, cmn.LU_WORKQ_LEN)
	nNl.FromNUCh = make(chan nlp.NeighUpdate, cmn.NU_WORKQ_LEN)
	nNl.FromRUCh = make(chan nlp.RouteUpdate, cmn.RU_WORKQ_LEN)
	nNl.FromAUDone = make(chan struct{})
	nNl.FromLUDone = make(chan struct{})
	nNl.FromNUDone = make(chan struct{})
	nNl.FromRUDone = make(chan struct{})

	go NlpGet()

	err := nlp.LinkSubscribe(nNl.FromLUCh, nNl.FromAUDone)
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "%v", err)
	} else {
		tk.LogIt(tk.LOG_INFO, "Link msgs subscribed\n")
	}
	err = nlp.AddrSubscribe(nNl.FromAUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "Addr msgs subscribed\n")
	}
	err = nlp.NeighSubscribe(nNl.FromNUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "Neigh msgs subscribed\n")
	}
	err = nlp.RouteSubscribe(nNl.FromRUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "Route msgs subscribed\n")
	}
	go LUWorker(nNl.FromLUCh, nNl.FromLUDone)
	go AUWorker(nNl.FromAUCh, nNl.FromAUDone)
	go NUWorker(nNl.FromNUCh, nNl.FromNUDone)
	go RUWorker(nNl.FromRUCh, nNl.FromRUDone)

	tk.LogIt(tk.LOG_INFO, "NLP Subscription done\n")
	return nNl
}
