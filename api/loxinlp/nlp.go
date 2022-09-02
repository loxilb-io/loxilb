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
	"encoding/json"
	"errors"
	"fmt"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
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

const (
	IF_TYPE_REAL uint8 = iota
	IF_TYPE_SUBINTF
	IF_TYPE_BOND
	IF_TYPE_BRIGDE
	IF_TYPE_VXLAN
)

type Intf struct {
	dev            string
	itype          int
	state          bool
	configApplied  bool
	needRouteApply bool
}

type NlH struct {
	AddrUpdateCh
	LinkUpdateCh
	NeighUpdateCh
	RouteUpdateCh
	IMap map[string]Intf
}

var hooks cmn.NetHookInterface

func NlpRegister(hook cmn.NetHookInterface) {
	hooks = hook
}

func applyAllConfig(name string) bool {
	command := "loxicmd apply --per-intf " + name + " -c /opt/loxilb/ipconfig/"
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		return false
	}
	fmt.Printf("%v\n", string(output))
	return true
}

func applyLoadBalancerConfig() bool {
	var resp struct {
		Attr []cmn.LbRuleMod `json:"lbAttr"`
	}
	byteBuf, err := ioutil.ReadFile("/opt/loxilb/lbconfig.txt")
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, lb := range resp.Attr {
		hooks.NetLbRuleAdd(&lb)
	}
	return true
}

func applySessionConfig() bool {
	var resp struct {
		Attr []cmn.SessionMod `json:"sessionAttr"`
	}
	byteBuf, err := ioutil.ReadFile("/opt/loxilb/sessionconfig.txt")
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, session := range resp.Attr {
		hooks.NetSessionAdd(&session)
	}
	return true
}

func applyUlClConfig() bool {
	var resp struct {
		Attr []cmn.SessionUlClMod `json:"ulclAttr"`
	}
	byteBuf, err := ioutil.ReadFile("/opt/loxilb/sessionulclconfig.txt")
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, ulcl := range resp.Attr {
		hooks.NetSessionUlClAdd(&ulcl)
	}
	return true
}

func applyRoutes(name string) {
	tk.LogIt(tk.LOG_DEBUG, "[NLP] Applying Route Config for %s \n", name)
	command := "loxicmd apply --per-intf " + name + " -r -c /opt/loxilb/ipconfig/"
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%v\n", string(output))
}

func applyConfigMap(name string, state bool, add bool) {
	var configApplied bool
	var needRouteApply bool
	if _, err := os.Stat("/opt/loxilb/ipconfig/"); errors.Is(err, os.ErrNotExist) {
		return
	}
	if add {
		if _, ok := nNl.IMap[name]; ok {
			configApplied = nNl.IMap[name].configApplied
			if !nNl.IMap[name].configApplied {
				tk.LogIt(tk.LOG_DEBUG, "[NLP] Applying Config for %s \n", name)
				if applyAllConfig(name) == true {
					configApplied = true
					tk.LogIt(tk.LOG_DEBUG, "[NLP] Applied Config for %s \n", name)
				} else {
					configApplied = false
					tk.LogIt(tk.LOG_ERROR, "[NLP] Applied Config for %s - FAILED\n", name)
				}
				nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied, needRouteApply: false}
			} else if nNl.IMap[name].state != state {
				needRouteApply = nNl.IMap[name].needRouteApply
				if state && nNl.IMap[name].needRouteApply {
					applyRoutes(name)
					needRouteApply = false
				} else if !state {
					needRouteApply = true
					tk.LogIt(tk.LOG_DEBUG, "[NLP] Route Config for %s will be tried\n", name)
				}
				nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied, needRouteApply: needRouteApply}
			}
			tk.LogIt(tk.LOG_DEBUG, "[NLP] ConfigMap for %s : %v \n", name, nNl.IMap[name])
		} else {
			tk.LogIt(tk.LOG_DEBUG, "[NLP] Applying Config for %s \n", name)
			if applyAllConfig(name) == true {
				configApplied = true
				tk.LogIt(tk.LOG_DEBUG, "[NLP] Applied Config for %s \n", name)
			} else {
				configApplied = false
				tk.LogIt(tk.LOG_ERROR, "[NLP] Applied Config for %s - FAILED\n", name)
			}
			nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied}
		}
	} else {
		if _, ok := nNl.IMap[name]; ok {
			delete(nNl.IMap, name)
		}
	}
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
	state := uint8(attrs.OperState) != nlp.OperDown
	if add {
		mod = "ADD"
	} else {
		mod = "DELETE"
	}
	tk.LogIt(tk.LOG_DEBUG, "[NLP] %s Device %v mac(%v) attrs(%v) info recvd\n", mod, name, ifMac, attrs)

	if _, ok := link.(*nlp.Bridge); ok {

		vid, _ = strconv.Atoi(strings.Join(re.FindAllString(name, -1), " "))
		if add {
			ret, err = hooks.NetVlanAdd(&cmn.VlanMod{Vid: vid, Dev: name, LinkIndex: idx,
				MacAddr: ifMac, Link: linkState, State: state, Mtu: mtu, TunId: 0})
		} else {
			ret, err = hooks.NetVlanDel(&cmn.VlanMod{Vid: vid})
		}

		if err != nil {
			tk.LogIt(tk.LOG_INFO, "[NLP] Bridge %v, %d, %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] Bridge %v, %d, %v, %v, %v %s [OK]\n", name, vid, ifMac, state, mtu, mod)
		}

		if (add && (err != nil)) || !add {
			applyConfigMap(name, state, add)
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
				tk.LogIt(tk.LOG_ERROR, "[NLP] TVlan Port %v, v(%v), %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
				fmt.Println(err)
			} else {
				tk.LogIt(tk.LOG_INFO, "[NLP] TVlan Port %v, v(%v), %v, %v, %v %s OK\n", name, vid, ifMac, state, mtu, mod)
			}

		}
		applyConfigMap(name, state, add)
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
		tk.LogIt(tk.LOG_INFO, "[NLP] Port %v, uif %v %s\n", name, real, mod)
	}

	if add {
		ret, err = hooks.NetPortAdd(&cmn.PortMod{Dev: name, LinkIndex: idx, Ptype: pType, MacAddr: ifMac,
			Link: linkState, State: state, Mtu: mtu, Master: master, Real: real,
			TunId: tunId})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] Port %v, %v, %v, %v add failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] Port %v, %v, %v, %v add [OK]\n", name, ifMac, state, mtu)
		}
		applyConfigMap(name, state, add)
	} else if attrs.MasterIndex == 0 {
		ret, err = hooks.NetPortDel(&cmn.PortMod{Dev: name, Ptype: pType})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] Port %v, %v, %v, %v delete failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] Port %v, %v, %v, %v delete [OK]\n", name, ifMac, state, mtu)
		}

		applyConfigMap(name, state, add)
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
			tk.LogIt(tk.LOG_ERROR, "[NLP] Vlan(%v) Port %v, %v, %v, %v %s failed\n", vid, name, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] Vlan(%v) Port %v, %v, %v, %v %s [OK]\n", vid, name, ifMac, state, mtu, mod)
		}
		if (add && (err != nil)) || !add {
			applyConfigMap(name, state, add)
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
		tk.LogIt(tk.LOG_ERROR, "[NLP] IPv4 Address %v Port %v failed %v\n", ipStr, name, err)
		ret = -1
	} else {
		tk.LogIt(tk.LOG_INFO, "[NLP] IPv4 Address %v Port %v added\n", ipStr, name)
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
			tk.LogIt(tk.LOG_ERROR, "[NLP] NH %v mac %v dev %v add failed %v\n", neigh.IP.String(), mac,
				name, err)

		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] NH %v mac %v dev %v added\n", neigh.IP.String(), mac, name)
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
				tk.LogIt(tk.LOG_ERROR, "[NLP] L2fdb %v brId %v dst %v dev %v IGNORED\n", mac[:], brId, dst, name)
				return 0
			}
		} else {
			dst = net.ParseIP("0.0.0.0")
			ftype = cmn.FDB_VLAN
		}

		ret, err = hooks.NetFdbAdd(&cmn.FdbMod{MacAddr: mac, BridgeId: brId, Dev: name, Dst: dst,
			Type: ftype})
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] L2fdb %v brId %v dst %v dev %v add failed\n", mac[:], brId, dst, name)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] L2fdb %v brId %v dst %v dev %v added\n", mac[:], brId, dst, name)
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
			tk.LogIt(tk.LOG_ERROR, "[NLP] NH  %v %v del failed\n", neigh.IP.String(), name)
			ret = -1
		} else {
			tk.LogIt(tk.LOG_ERROR, "[NLP] NH %v %v deleted\n", neigh.IP.String(), name)
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
			tk.LogIt(tk.LOG_ERROR, "[NLP] L2fdb %v brId %v dst %s dev %v delete failed %v\n", mac[:], brId, dst, name, err)
			ret = -1
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] L2fdb %v brId %v dst %s dev %v deleted\n", mac[:], brId, dst, name)
		}
	}
	return ret
}

func AddRoute(route nlp.Route) int {
	var ipNet net.IPNet
	if route.Dst == nil {
		r := net.IPv4(0, 0, 0, 0)
		m := net.CIDRMask(0, 32)
		r = r.Mask(m)
		ipNet = net.IPNet{IP: r, Mask: m}
	} else {
		ipNet = *route.Dst
	}
	ret, err := hooks.NetRoutev4Add(&cmn.Routev4Mod{Protocol: int(route.Protocol), Flags: route.Flags,
		Gw: route.Gw, LinkIndex: route.LinkIndex, Dst: ipNet})
	if err != nil {
		if route.Gw != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s via %s add failed-%s\n", ipNet.String(),
				route.Gw.String(), err)
		} else {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s add failed-%s\n", ipNet.String(), err)
		}
	} else {
		if route.Gw != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s via %s added\n", ipNet.String(),
				route.Gw.String())
		} else {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s added\n", ipNet.String())
		}
	}

	return ret
}

func DelRoute(route nlp.Route) int {
	var ret int
	var ipNet net.IPNet
	if route.Dst == nil {
		r := net.IPv4(0, 0, 0, 0)
		m := net.CIDRMask(0, 32)
		r = r.Mask(m)
		ipNet = net.IPNet{IP: r, Mask: m}
	} else {
		ipNet = *route.Dst
	}
	ret, err := hooks.NetRoutev4Del(&cmn.Routev4Mod{Dst: ipNet})
	if err != nil {
		if route.Gw != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s via %s delete failed-%s\n", ipNet.String(),
				route.Gw.String(), err)
		} else {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s delete failed-%s\n", ipNet.String(), err)
		}
	} else {
		if route.Gw != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s via %s deleted\n", ipNet.String(),
				route.Gw.String())
		} else {
			tk.LogIt(tk.LOG_ERROR, "[NLP] RT  %s deleted\n", ipNet.String())
		}
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
			tk.LogIt(tk.LOG_INFO, "[NLP] IPv4 Address %v Port %v add failed\n", m.LinkAddress.String(), name)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] IPv4 Address %v Port %v added\n", m.LinkAddress.String(), name)
		}

	} else {
		_, err := hooks.NetIpv4AddrDel(&cmn.Ipv4AddrMod{Dev: name, Ip: m.LinkAddress.String()})
		if err != nil {
			tk.LogIt(tk.LOG_INFO, "[NLP] IPv4 Address %v Port %v delete failed\n", m.LinkAddress.String(), name)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LOG_INFO, "[NLP] IPv4 Address %v Port %v deleted\n", m.LinkAddress.String(), name)
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

	if m.Type == syscall.RTM_NEWROUTE {
		ret = AddRoute(m.Route)
	} else {
		ret = DelRoute(m.Route)
	}

	return ret
}

func LUWorker(ch chan nlp.LinkUpdate, f chan struct{}) {

	for n := 0; n < cmn.LU_WORKQ_LEN; n++ {
		select {
		case m := <-ch:
			LUWorkSingle(m)
		default:
			continue
		}
	}
}

func AUWorker(ch chan nlp.AddrUpdate, f chan struct{}) {

	for n := 0; n < cmn.AU_WORKQ_LEN; n++ {
		select {
		case m := <-ch:
			AUWorkSingle(m)
		default:
			continue
		}
	}

}

func NUWorker(ch chan nlp.NeighUpdate, f chan struct{}) {

	for n := 0; n < cmn.NU_WORKQ_LEN; n++ {
		select {
		case m := <-ch:
			NUWorkSingle(m)
		default:
			continue
		}
	}
}

func RUWorker(ch chan nlp.RouteUpdate, f chan struct{}) {

	for n := 0; n < cmn.RU_WORKQ_LEN; n++ {
		select {
		case m := <-ch:
			RUWorkSingle(m)
		default:
			continue
		}
	}
}

func NLWorker(nNl *NlH) {
	for { /* Single thread for reading all NL msgs in below order */
		LUWorker(nNl.FromLUCh, nNl.FromLUDone)
		AUWorker(nNl.FromAUCh, nNl.FromAUDone)
		NUWorker(nNl.FromNUCh, nNl.FromNUDone)
		RUWorker(nNl.FromRUCh, nNl.FromRUDone)
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

func NlpGet(ch chan bool) int {
	var ret int
	tk.LogIt(tk.LOG_INFO, "[NLP] Getting device info\n")

	GetBridges()

	links, err := nlp.LinkList()
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "[NLP] Error in getting device info(%v)\n", err)
		ret = -1
	}

	for _, link := range links {
		ret = ModLink(link, true)

		if ret == -1 {
			continue
		}

		/* Get FDBs */
		_, ok := link.(*nlp.Vxlan)
		if link.Attrs().MasterIndex > 0 || ok {
			neighs, err := nlp.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
			if err != nil {
				tk.LogIt(tk.LOG_ERROR, "[NLP] Error getting neighbors list %v for intf %s\n",
					err, link.Attrs().Name)
			}

			if len(neighs) == 0 {
				tk.LogIt(tk.LOG_DEBUG, "[NLP] No FDBs found for intf %s\n", link.Attrs().Name)
			} else {
				for _, neigh := range neighs {
					AddNeigh(neigh, link)
				}
			}
		}

		addrs, err := nlp.AddrList(link, nlp.FAMILY_V4)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] Error getting address list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(addrs) == 0 {
			tk.LogIt(tk.LOG_DEBUG, "[NLP] No addresses found for intf %s\n", link.Attrs().Name)
		} else {
			for _, addr := range addrs {
				AddAddr(addr, link)
			}
		}

		neighs, err := nlp.NeighList(link.Attrs().Index, nlp.FAMILY_ALL)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] Error getting neighbors list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(neighs) == 0 {
			tk.LogIt(tk.LOG_DEBUG, "[NLP] No neighbors found for intf %s\n", link.Attrs().Name)
		} else {
			for _, neigh := range neighs {
				AddNeigh(neigh, link)
			}
		}

		/* Get Routes */
		routes, err := nlp.RouteList(link, nlp.FAMILY_V4)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "[NLP] Error getting route list %v\n", err)
		}

		if len(routes) == 0 {
			tk.LogIt(tk.LOG_DEBUG, "[NLP] No STATIC routes found for intf %s\n", link.Attrs().Name)
		} else {
			for _, route := range routes {
				AddRoute(route)
			}
		}
	}
	tk.LogIt(tk.LOG_INFO, "[NLP] nlp get done\n")
	ch <- true
	return ret
}

var nNl *NlH

func LbSessionGet(done bool) int {

	if done {
		tk.LogIt(tk.LOG_INFO, "[NLP] LbSessionGet Start\n")
		if _, err := os.Stat("/opt/loxilb/lbconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LOG_INFO, "[NLP] No load balancer config file : %s \n", err.Error())
			}
		} else {
			applyLoadBalancerConfig()
		}

		tk.LogIt(tk.LOG_INFO, "[NLP] LoadBalancer done\n")
		if _, err := os.Stat("/opt/loxilb/sessionconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LOG_INFO, "[NLP] No Session config file : %s \n", err.Error())
			}
		} else {
			applySessionConfig()
		}

		tk.LogIt(tk.LOG_INFO, "[NLP] Session done\n")
		if _, err := os.Stat("/opt/loxilb/sessionulclconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LOG_INFO, "[NLP] No UlCl config file : %s \n", err.Error())
			}
		} else {
			applyUlClConfig()
		}

		tk.LogIt(tk.LOG_INFO, "[NLP] Session UlCl done\n")
		tk.LogIt(tk.LOG_INFO, "[NLP] LbSessionGet done\n")
	}

	return 0
}

func NlpInit() *NlH {

	nNl = new(NlH)

	nNl.FromAUCh = make(chan nlp.AddrUpdate, cmn.AU_WORKQ_LEN)
	nNl.FromLUCh = make(chan nlp.LinkUpdate, cmn.LU_WORKQ_LEN)
	nNl.FromNUCh = make(chan nlp.NeighUpdate, cmn.NU_WORKQ_LEN)
	nNl.FromRUCh = make(chan nlp.RouteUpdate, cmn.RU_WORKQ_LEN)
	nNl.FromAUDone = make(chan struct{})
	nNl.FromLUDone = make(chan struct{})
	nNl.FromNUDone = make(chan struct{})
	nNl.FromRUDone = make(chan struct{})
	nNl.IMap = make(map[string]Intf)

	checkInit := make(chan bool)
	go NlpGet(checkInit)
	done := <-checkInit
	go LbSessionGet(done)

	err := nlp.LinkSubscribe(nNl.FromLUCh, nNl.FromAUDone)
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "%v", err)
	} else {
		tk.LogIt(tk.LOG_INFO, "[NLP] Link msgs subscribed\n")
	}
	err = nlp.AddrSubscribe(nNl.FromAUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "[NLP] Addr msgs subscribed\n")
	}
	err = nlp.NeighSubscribe(nNl.FromNUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "[NLP] Neigh msgs subscribed\n")
	}
	err = nlp.RouteSubscribe(nNl.FromRUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LOG_INFO, "[NLP] Route msgs subscribed\n")
	}

	go NLWorker(nNl)
	tk.LogIt(tk.LOG_INFO, "[NLP] NLP Subscription done\n")
	return nNl
}
