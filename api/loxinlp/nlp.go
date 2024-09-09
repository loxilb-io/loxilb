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
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	opt "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
	nlp "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	IfOperUnknown uint8 = iota
	IfOperNotPresent
	IfOperDown
	IfOperLowerLayerDown
	IfOperTesting
	IfOperDormant
	IfOperUp
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
	IfTypeReal uint8 = iota
	IfTypeSubIntf
	IfTypeBond
	IfTypeBridge
	IfTypeVxlan
)

type Intf struct {
	dev            string
	state          bool
	configApplied  bool
	needRouteApply bool
}

type NlH struct {
	AddrUpdateCh
	LinkUpdateCh
	NeighUpdateCh
	RouteUpdateCh
	IMap      map[string]Intf
	BlackList string
	BLRgx     *regexp.Regexp
	WhiteList string
	WLRgx     *regexp.Regexp
}

var (
	skipIfRoute bool
	hooks       cmn.NetHookInterface
)

func NlpRegister(hook cmn.NetHookInterface) {
	hooks = hook
}

func NlpIsBlackListedIntf(name string, masterIdx int) bool {
	if nNl.WhiteList != "none" {
		filter := nNl.WLRgx.MatchString(name)
		return !filter
	}

	if name == "lo" {
		return true
	}
	filter := nNl.BLRgx.MatchString(name)
	return filter
}

func applyAllConfig(name string) bool {

	command := "loxicmd apply --per-intf " + name + " -c " + opt.Opts.ConfigPath + "/ipconfig/"
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
	dpath := opt.Opts.ConfigPath + "/lbconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
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
	dpath := opt.Opts.ConfigPath + "/sessionconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
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
	dpath := opt.Opts.ConfigPath + "/sessionulclconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
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

func applyFWConfig() bool {
	var resp struct {
		Attr []cmn.FwRuleMod `json:"fwAttr"`
	}
	dpath := opt.Opts.ConfigPath + "/FWconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, fw := range resp.Attr {
		hooks.NetFwRuleAdd(&fw)
	}
	return true
}

func applyEPConfig() bool {
	var resp struct {
		Attr []cmn.EndPointMod `json:"Attr"`
	}
	dpath := opt.Opts.ConfigPath + "/EPconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, ep := range resp.Attr {
		hooks.NetEpHostAdd(&ep)
	}
	return true
}

func ApplyBFDConfig() bool {
	var resp struct {
		Attr []cmn.BFDMod `json:"Attr"`
	}
	dpath := opt.Opts.ConfigPath + "/BFDconfig.txt"
	byteBuf, err := os.ReadFile(dpath)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	// Unmashal to Json
	if err := json.Unmarshal(byteBuf, &resp); err != nil {
		fmt.Printf("Error: Failed to unmarshal File: (%s)\n", err.Error())
		return false
	}
	for _, bfd := range resp.Attr {
		hooks.NetBFDAdd(&bfd)
	}
	return true
}

func applyRoutes(name string) {
	tk.LogIt(tk.LogDebug, "[NLP] Applying Route Config for %s \n", name)
	command := "loxicmd apply --per-intf " + name + " -r -c " + opt.Opts.ConfigPath + "/ipconfig/"
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
	dpath := opt.Opts.ConfigPath + "/ipconfig/"

	if _, err := os.Stat(dpath); errors.Is(err, os.ErrNotExist) {
		return
	}
	if add {
		if _, ok := nNl.IMap[name]; ok {
			configApplied = nNl.IMap[name].configApplied
			if !nNl.IMap[name].configApplied {
				tk.LogIt(tk.LogDebug, "[NLP] Applying Config for %s \n", name)
				if applyAllConfig(name) {
					configApplied = true
					tk.LogIt(tk.LogDebug, "[NLP] Applied Config for %s \n", name)
				} else {
					configApplied = false
					tk.LogIt(tk.LogError, "[NLP] Applied Config for %s - FAILED\n", name)
				}
				nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied, needRouteApply: false}
			} else if nNl.IMap[name].state != state {
				needRouteApply = nNl.IMap[name].needRouteApply
				if state && nNl.IMap[name].needRouteApply {
					applyRoutes(name)
					needRouteApply = false
				} else if !state {
					needRouteApply = true
					tk.LogIt(tk.LogDebug, "[NLP] Route Config for %s will be tried\n", name)
				}
				nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied, needRouteApply: needRouteApply}
			}
			tk.LogIt(tk.LogDebug, "[NLP] ConfigMap for %s : %v \n", name, nNl.IMap[name])
		} else {
			tk.LogIt(tk.LogDebug, "[NLP] Applying Config for %s \n", name)
			if applyAllConfig(name) {
				configApplied = true
				tk.LogIt(tk.LogDebug, "[NLP] Applied Config for %s \n", name)
			} else {
				configApplied = false
				tk.LogIt(tk.LogError, "[NLP] Applied Config for %s - FAILED\n", name)
			}
			nNl.IMap[name] = Intf{dev: name, state: state, configApplied: configApplied}
		}
	} else {
		delete(nNl.IMap, name)
	}
}

func AddFDBNoHook(macAddress, ifName string) int {
	var ret int
	MacAddress, err := net.ParseMAC(macAddress)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] MacAddress Parse %s Fail\n", macAddress)
		return -1
	}
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Port %s find Fail\n", ifName)
		return -1
	}

	// Make Neigh
	neigh := nlp.Neigh{
		Family:       syscall.AF_BRIDGE,
		HardwareAddr: MacAddress,
		LinkIndex:    IfName.Attrs().Index,
		State:        unix.NUD_PERMANENT,
		Flags:        unix.NTF_SELF,
	}
	err = nlp.NeighAppend(&neigh)
	if err != nil {
		fmt.Printf("err.Error(): %v\n", err.Error())
		tk.LogIt(tk.LogWarning, "[NLP] FDB added Fail\n")
		return -1
	}
	return ret
}

func DelFDBNoHook(macAddress, ifName string) int {
	var ret int
	MacAddress, err := net.ParseMAC(macAddress)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] MacAddress Parse %s Fail\n", macAddress)
		return -1
	}
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Port %s find Fail\n", ifName)
		return -1
	}

	// Make Neigh
	neigh := nlp.Neigh{
		Family:       syscall.AF_BRIDGE,
		HardwareAddr: MacAddress,
		LinkIndex:    IfName.Attrs().Index,
		State:        unix.NUD_PERMANENT,
		Flags:        unix.NTF_SELF,
	}
	err = nlp.NeighDel(&neigh)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] FDB delete Fail\n")
		return -1
	}
	return ret
}

func AddNeighNoHook(address, ifName, macAddress string) int {
	var ret int
	Address := net.ParseIP(address)

	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Port %s find Fail\n", ifName)
		return -1
	}
	MacAddress, err := net.ParseMAC(macAddress)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] MacAddress Parse %s Fail\n", macAddress)
		return -1
	}
	// Make Neigh
	neigh := nlp.Neigh{
		IP:           Address,
		HardwareAddr: MacAddress,
		LinkIndex:    IfName.Attrs().Index,
		State:        unix.NUD_PERMANENT,
	}

	err = nlp.NeighAdd(&neigh)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Neighbor added Fail\n")
		return -1
	}
	return ret
}

func DelNeighNoHook(address, ifName string) int {
	var ret int
	Address := net.ParseIP(address)
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		nList, err1 := nlp.NeighList(0, 0)
		if err1 != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Neighbor List get Failed\n")
			return -1
		}
		for _, n := range nList {
			if n.IP.String() == address {
				nlp.NeighDel(&n)
			}
		}
		return 0
	}

	// Make Neigh
	neigh := nlp.Neigh{
		IP:        Address,
		LinkIndex: IfName.Attrs().Index,
	}
	err = nlp.NeighDel(&neigh)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Neighbor delete Fail\n")
		return -1
	}
	return ret
}

func AddVLANNoHook(vlanid int) int {
	var ret int
	// Check Vlan interface has been added.
	// Vlan Name : vlan$vlanid (vlan10, vlan100...)
	VlanName := fmt.Sprintf("vlan%d", vlanid)
	_, err := nlp.LinkByName(VlanName)
	if err != nil {
		newBr := &nlp.Bridge{
			LinkAttrs: nlp.LinkAttrs{
				Name: VlanName,
				MTU:  9000, // Static value for VxLAN
			},
		}
		if err := nlp.LinkAdd(newBr); err != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge added Fail\n")
			ret = -1
		}
		nlp.LinkSetUp(newBr)
	}
	return ret
}

func DelVLANNoHook(vlanid int) int {
	var ret int
	VlanName := fmt.Sprintf("vlan%d", vlanid)
	vlanLink, err := nlp.LinkByName(VlanName)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge get Fail: %s\n", err.Error())
	}
	err = nlp.LinkSetDown(vlanLink)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge Link Down Fail: %s\n", err.Error())
	}
	err = nlp.LinkDel(vlanLink)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge delete Fail: %s\n", err.Error())
	}

	return ret
}

func AddVLANMemberNoHook(vlanid int, intfName string, tagged bool) int {
	var ret int
	var VlanDevName string
	// Check Vlan interface has been added.
	VlanBridgeName := fmt.Sprintf("vlan%d", vlanid)
	VlanLink, err := nlp.LinkByName(VlanBridgeName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge finding Fail\n")
		return 404
	}
	ParentInterface, err := nlp.LinkByName(intfName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Parent interface finding Fail\n")
		return 404
	}
	if tagged {
		VlanDevName = fmt.Sprintf("%s.%d", intfName, vlanid)
		VlanDev := &nlp.Vlan{
			LinkAttrs: nlp.LinkAttrs{
				Name:        VlanDevName,
				ParentIndex: ParentInterface.Attrs().Index,
			},
			VlanId: vlanid,
		}
		if err := nlp.LinkAdd(VlanDev); err != nil {
			tk.LogIt(tk.LogWarning, "failed to create VlanDev: [ %v ] with the error: %s", VlanDev, err)
			ret = -1
		}
	} else {
		VlanDevName = intfName
	}

	VlanDevNonPointer, _ := nlp.LinkByName(VlanDevName)
	nlp.LinkSetUp(VlanDevNonPointer)
	err = nlp.LinkSetMaster(VlanDevNonPointer, VlanLink)
	if err != nil {
		tk.LogIt(tk.LogWarning, "failed to master: [ %v ] with the error: %s", VlanDevNonPointer, err)
		ret = -1
	}

	return ret
}

func DelVLANMemberNoHook(vlanid int, intfName string, tagged bool) int {
	var ret int
	var VlanDevName string
	VlanName := fmt.Sprintf("vlan%d", vlanid)
	_, err := nlp.LinkByName(VlanName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Vlan Bridge finding Fail\n")
		return 404
	}
	_, err = nlp.LinkByName(intfName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Parent interface finding Fail\n")
		return 404
	}
	if tagged {
		VlanDevName = fmt.Sprintf("%s.%d", intfName, vlanid)
	} else {
		VlanDevName = intfName
	}
	VlanDevNonPointer, err := nlp.LinkByName(VlanDevName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] Vlan interface finding Fail\n")
		return 404
	}
	err = nlp.LinkSetNoMaster(VlanDevNonPointer)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] No master fail \n")
	}
	if tagged {
		nlp.LinkDel(VlanDevNonPointer)
	}
	return ret
}

func AddVxLANBridgeNoHook(vxlanid int, epIntfName string) int {
	var ret int
	// Check Vlan interface has been added.
	VxlanBridgeName := fmt.Sprintf("vxlan%d", vxlanid)
	_, err := nlp.LinkByName(VxlanBridgeName)
	if err != nil {

		EndpointInterface, err := nlp.LinkByName(epIntfName)
		if err != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Endpoint interface finding Fail\n")
			return 404
		}
		LocalIPs, err := nlp.AddrList(EndpointInterface, nlp.FAMILY_V4)
		if err != nil || len(LocalIPs) == 0 {
			tk.LogIt(tk.LogWarning, "[NLP] Endpoint interface dosen't have Local IP address\n")
			return 403
		}
		VxlanDev := &nlp.Vxlan{
			LinkAttrs: nlp.LinkAttrs{
				Name: VxlanBridgeName,
				MTU:  9000, // Static Value for Vxlan in loxiLB
			},
			SrcAddr:      LocalIPs[0].IP,
			VtepDevIndex: EndpointInterface.Attrs().Index,
			VxlanId:      vxlanid,
			Port:         4789, // VxLAN default port
		}
		if err := nlp.LinkAdd(VxlanDev); err != nil {
			tk.LogIt(tk.LogWarning, "failed to create VxlanDev: [ %v ] with the error: %s", VxlanDev, err)
			ret = -1
		}
		time.Sleep(1 * time.Second)
		VxlanDevNonPointer, err := nlp.LinkByName(VxlanBridgeName)
		if err != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Vxlan Interface create fail: %s\n", err.Error())
			return -1
		}
		nlp.LinkSetUp(VxlanDevNonPointer)

	} else {
		tk.LogIt(tk.LogWarning, "[NLP] Vxlan Bridge Already exists\n")
		return 409
	}

	return ret
}

func DelVxLANNoHook(vxlanid int) int {
	var ret int
	VxlanName := fmt.Sprintf("vxlan%d", vxlanid)
	vxlanLink, err := nlp.LinkByName(VxlanName)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vxlan Bridge get Fail:%s\n", err.Error())
	}
	err = nlp.LinkSetDown(vxlanLink)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vxlan Bridge Link Down Fail:%s\n", err.Error())
	}
	err = nlp.LinkDel(vxlanLink)
	if err != nil {
		ret = -1
		tk.LogIt(tk.LogWarning, "[NLP] Vxlan Bridge delete Fail:%s\n", err.Error())
	}

	return ret
}

func GetVxLANPeerNoHook() (map[int][]string, error) {
	ret := map[int][]string{}
	links, err := nlp.LinkList()
	if err != nil {
		tk.LogIt(tk.LogError, "[NLP] Error in getting device info(%v)\n", err)
		return nil, err
	}

	for _, link := range links {
		if link.Type() == "vxlan" {
			neighs, err := nlp.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
			if err != nil {
				tk.LogIt(tk.LogError, "[NLP] Error getting neighbors list %v for intf %s\n",
					err, link.Attrs().Name)
				return nil, err
			}
			for _, neigh := range neighs {
				if neigh.IP != nil {
					ret[link.Attrs().Index] = append(ret[link.Attrs().Index], neigh.IP.String())
				}
			}
		}
	}
	return ret, nil
}

func GetFDBNoHook() ([]map[string]string, error) {
	ret := []map[string]string{}
	links, err := nlp.LinkList()
	if err != nil {
		tk.LogIt(tk.LogError, "[NLP] Error in getting device info(%v)\n", err)
		return nil, err
	}

	for _, link := range links {
		if link.Attrs().MasterIndex > 0 {
			fdbs, err := nlp.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
			if err != nil {
				tk.LogIt(tk.LogError, "[NLP] Error getting fdb list %v for intf %s\n",
					err, link.Attrs().Name)
				return nil, err
			}
			for _, fdb := range fdbs {
				tmpRet := map[string]string{}
				tmpRet["macAddress"] = fdb.HardwareAddr.String()
				tmpRet["dev"] = link.Attrs().Name
				ret = append(ret, tmpRet)
			}
		}
	}
	return ret, nil
}

func AddVxLANPeerNoHook(vxlanid int, PeerIP string) int {
	var ret int
	MacAddress, _ := net.ParseMAC("00:00:00:00:00:00")
	ifName := fmt.Sprintf("vxlan%d", vxlanid)
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] VxLAN %s find Fail\n", ifName)
		return -1
	}
	peerIP := net.ParseIP(PeerIP)
	// Make Peer
	Peer := nlp.Neigh{
		IP:           peerIP,
		Family:       syscall.AF_BRIDGE,
		HardwareAddr: MacAddress,
		LinkIndex:    IfName.Attrs().Index,
		State:        unix.NUD_PERMANENT,
		Flags:        unix.NTF_SELF,
	}
	err = nlp.NeighAppend(&Peer)
	if err != nil {
		fmt.Printf("err.Error(): %v\n", err.Error())
		tk.LogIt(tk.LogWarning, "[NLP] VxLAN Peer added Fail\n")
		return -1
	}
	return ret
}

func DelVxLANPeerNoHook(vxlanid int, PeerIP string) int {
	var ret int
	MacAddress, _ := net.ParseMAC("00:00:00:00:00:00")
	ifName := fmt.Sprintf("vxlan%d", vxlanid)
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] VxLAN %s find Fail\n", ifName)
		return -1
	}
	peerIP := net.ParseIP(PeerIP)
	// Make Peer
	Peer := nlp.Neigh{
		IP:           peerIP,
		Family:       syscall.AF_BRIDGE,
		HardwareAddr: MacAddress,
		LinkIndex:    IfName.Attrs().Index,
		State:        unix.NUD_PERMANENT,
		Flags:        unix.NTF_SELF,
	}

	err = nlp.NeighDel(&Peer)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] VxLAN Peer delete Fail\n")
		return -1
	}
	return ret
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
	tk.LogIt(tk.LogDebug, "[NLP] %s Device %v mac(%v) attrs(%v) info recvd\n", mod, name, ifMac, attrs)

	if _, ok := link.(*nlp.Bridge); ok {

		vid, _ = strconv.Atoi(strings.Join(re.FindAllString(name, -1), " "))
		// Dirty hack to support docker0 bridge
		if vid == 0 {
			if name == "docker0" {
				vid = 4090
			} else if name == "cni0" {
				vid = 4091
			}
		}
		if add {
			ret, err = hooks.NetVlanAdd(&cmn.VlanMod{Vid: vid, Dev: name, LinkIndex: idx,
				MacAddr: ifMac, Link: linkState, State: state, Mtu: mtu, TunID: 0})
		} else {
			ret, err = hooks.NetVlanDel(&cmn.VlanMod{Vid: vid})
		}

		if err != nil {
			tk.LogIt(tk.LogInfo, "[NLP] Bridge %v, %d, %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Bridge %v, %d, %v, %v, %v %s [OK]\n", name, vid, ifMac, state, mtu, mod)
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
		// Dirty hack to support docker bridge
		if vid == 0 {
			if brLink.Attrs().Name == "docker0" {
				vid = 4090
			} else if brLink.Attrs().Name == "cni0" {
				vid = 4091
			}
		}
	}

	master := ""

	if attrs.MasterIndex > 0 {
		/* Tagged Vlan port */
		if strings.Contains(name, ".") {
			/* Currently, Sub-interfaces can only be part of bridges */
			pname := strings.Split(name, ".")
			if add {
				ret, err = hooks.NetVlanPortAdd(&cmn.VlanPortMod{Vid: vid, Dev: pname[0], Tagged: true})
			} else {
				ret, err = hooks.NetVlanPortDel(&cmn.VlanPortMod{Vid: vid, Dev: pname[0], Tagged: true})
			}
			if err != nil {
				tk.LogIt(tk.LogError, "[NLP] TVlan Port %v, v(%v), %v, %v, %v %s failed\n", name, vid, ifMac, state, mtu, mod)
				fmt.Println(err)
			} else {
				tk.LogIt(tk.LogInfo, "[NLP] TVlan Port %v, v(%v), %v, %v, %v %s OK\n", name, vid, ifMac, state, mtu, mod)
			}
			applyConfigMap(name, state, add)
			return ret
		} else {
			mif, err := nlp.LinkByIndex(attrs.MasterIndex)
			if err != nil {
				fmt.Println(err)
				return -1
			} else {
				if _, ok := mif.(*nlp.Bond); ok {
					master = mif.Attrs().Name
				}
			}
		}
	}

	/* Physical port/ Bond/ VxLAN */

	real := ""
	pType := cmn.PortReal
	tunId := 0
	tunSrc := net.IPv4zero
	tunDst := net.IPv4zero

	if strings.Contains(name, "ipsec") || strings.Contains(name, "vti") {
		pType = cmn.PortVti
	} else if strings.Contains(name, "wg") {
		pType = cmn.PortWg
	}

	if vxlan, ok := link.(*nlp.Vxlan); ok {
		pType = cmn.PortVxlanBr
		tunId = vxlan.VxlanId
		uif, err := nlp.LinkByIndex(vxlan.VtepDevIndex)
		if err != nil {
			fmt.Println(err)
			return -1
		}
		real = uif.Attrs().Name
		tk.LogIt(tk.LogInfo, "[NLP] Port %v, uif %v %s\n", name, real, mod)
	} else if _, ok := link.(*nlp.Bond); ok {
		pType = cmn.PortBond
		tk.LogIt(tk.LogInfo, "[NLP] Bond %v, %s\n", name, mod)
	} else if iptun, ok := link.(*nlp.Iptun); ok {
		pType = cmn.PortIPTun
		if iptun.Remote == nil || iptun.Local == nil {
			return -1
		}

		if iptun.Remote.IsUnspecified() || iptun.Local.IsUnspecified() {
			return -1
		}
		tunId = 1 // Just needed internally
		tunDst = iptun.Remote
		tunSrc = iptun.Local
		tk.LogIt(tk.LogInfo, "[NLP] IPTun %v (%s:%s), %s\n", name, tunSrc.String(), tunDst.String(), mod)
	} else if vtiTun, ok := link.(*nlp.Vti); ok {
		pType = cmn.PortVti
		if vtiTun.Remote == nil || vtiTun.Local == nil {
			return -1
		}

		if vtiTun.Remote.IsUnspecified() || vtiTun.Local.IsUnspecified() {
			return -1
		}
		tunId = int(vtiTun.OKey)
		tunDst = vtiTun.Remote
		tunSrc = vtiTun.Local
		tk.LogIt(tk.LogInfo, "[NLP] VTITun %v (%s:%s), %s\n", name, tunSrc.String(), tunDst.String(), mod)
	} else if master != "" {
		pType = cmn.PortBondSif
	}

	if add {
		ret, err = hooks.NetPortAdd(&cmn.PortMod{Dev: name, LinkIndex: idx, Ptype: pType, MacAddr: ifMac,
			Link: linkState, State: state, Mtu: mtu, Master: master, Real: real,
			TunID: tunId, TunDst: tunDst, TunSrc: tunSrc})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] Port %v, %v, %v, %v add failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Port %v, %v, %v, %v add [OK]\n", name, ifMac, state, mtu)
		}
		applyConfigMap(name, state, add)
	} else if attrs.MasterIndex == 0 {
		ret, err = hooks.NetPortDel(&cmn.PortMod{Dev: name, Ptype: pType})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] Port %v, %v, %v, %v delete failed\n", name, ifMac, state, mtu)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Port %v, %v, %v, %v delete [OK]\n", name, ifMac, state, mtu)
		}

		applyConfigMap(name, state, add)
		return ret
	}

	/* Untagged vlan ports */
	if attrs.MasterIndex > 0 && master == "" {
		if add {
			ret, err = hooks.NetVlanPortAdd(&cmn.VlanPortMod{Vid: vid, Dev: name, Tagged: false})
		} else {
			ret, err = hooks.NetVlanPortDel(&cmn.VlanPortMod{Vid: vid, Dev: name, Tagged: false})
		}
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] Vlan(%v) Port %v, %v, %v, %v %s failed\n", vid, name, ifMac, state, mtu, mod)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Vlan(%v) Port %v, %v, %v, %v %s [OK]\n", vid, name, ifMac, state, mtu, mod)
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

	ret, err := hooks.NetAddrAdd(&cmn.IPAddrMod{Dev: name, IP: ipStr})
	if err != nil {
		tk.LogIt(tk.LogError, "[NLP] IPv4 Address %v Port %v failed %v\n", ipStr, name, err)
		ret = -1
	} else {
		tk.LogIt(tk.LogInfo, "[NLP] IPv4 Address %v Port %v added\n", ipStr, name)
	}
	return ret
}

func AddAddrNoHook(address, ifName string) int {
	var ret int
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		_, err := hooks.NetAddrAdd(&cmn.IPAddrMod{Dev: ifName, IP: address})
		if err != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Hook IPv4 Address %v Port %v Add Fail\n", address, ifName)
			return -1
		}
		return 0
	}
	Address, err := nlp.ParseAddr(address)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] IPv4 Address %s Parse Fail\n", address)
		return -1
	}
	err = nlp.AddrAdd(IfName, Address)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] IPv4 Address %v Port %v added Fail\n", address, ifName)
		return -1
	}
	return ret
}

func DelAddrNoHook(address, ifName string) int {
	var ret int
	IfName, err := nlp.LinkByName(ifName)
	if err != nil {
		_, err := hooks.NetAddrDel(&cmn.IPAddrMod{Dev: ifName, IP: address})
		if err != nil {
			tk.LogIt(tk.LogWarning, "[NLP] Hook IPv4 Address %v Port %v delete Fail\n", address, ifName)
			return -1
		}
		return 0
	}
	Address, err := nlp.ParseAddr(address)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] IPv4 Address %s Parse Fail\n", address)
		return -1
	}
	err = nlp.AddrDel(IfName, Address)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[NLP] IPv4 Address %v Port %v delete Fail\n", address, ifName)
		return -1
	}
	return ret
}

func GetLinkNameByIndex(index int) (string, error) {
	brLink, err := nlp.LinkByIndex(index)
	if err != nil {
		return "", err
	}
	return brLink.Attrs().Name, nil
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

	if neigh.Family == unix.AF_INET ||
		neigh.Family == unix.AF_INET6 {
		ret, err = hooks.NetNeighAdd(&cmn.NeighMod{IP: neigh.IP, LinkIndex: neigh.LinkIndex,
			State:        neigh.State,
			HardwareAddr: neigh.HardwareAddr})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] NH %v mac %v dev %v add failed %v\n", neigh.IP.String(), mac,
				name, err)

		} /*else {
			tk.LogIt(tk.LogInfo, "[NLP] NH %v mac %v dev %v added\n", neigh.IP.String(), mac, name)
		} */
	} else if neigh.Family == unix.AF_BRIDGE {
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
				ftype = cmn.FdbTun
			} else {
				tk.LogIt(tk.LogInfo, "[NLP] L2fdb %v brId %v dst %v dev %v IGNORED\n", mac[:], brId, dst, name)
				return 0
			}
		} else {
			dst = net.ParseIP("0.0.0.0")
			ftype = cmn.FdbVlan
		}

		ret, err = hooks.NetFdbAdd(&cmn.FdbMod{MacAddr: mac, BridgeID: brId, Dev: name, Dst: dst,
			Type: ftype})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] L2fdb %v brId %v dst %v dev %v add failed\n", mac[:], brId, dst, name)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] L2fdb %v brId %v dst %v dev %v added\n", mac[:], brId, dst, name)
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

	if neigh.Family == unix.AF_INET ||
		neigh.Family == unix.AF_INET6 {

		ret, err = hooks.NetNeighDel(&cmn.NeighMod{IP: neigh.IP})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] NH  %v %v del failed\n", neigh.IP.String(), name)
			ret = -1
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] NH %v %v deleted\n", neigh.IP.String(), name)
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

			if len(brLink.Attrs().HardwareAddr) != 6 {
				brMac = [6]byte{0, 0, 0, 0, 0, 0}
			} else {
				copy(brMac[:], brLink.Attrs().HardwareAddr[:6])
			}

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

		ret, err = hooks.NetFdbDel(&cmn.FdbMod{MacAddr: mac, BridgeID: brId})
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] L2fdb %v brId %v dst %s dev %v delete failed %v\n", mac[:], brId, dst, name, err)
			ret = -1
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] L2fdb %v brId %v dst %s dev %v deleted\n", mac[:], brId, dst, name)
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

	var gws []cmn.GWInfo

	if len(route.MultiPath) <= 0 {
		gw := cmn.GWInfo{Gw: route.Gw, LinkIndex: route.LinkIndex}
		gws = append(gws, gw)
	} else {
		for i := range route.MultiPath {
			gws = append(gws, cmn.GWInfo{Gw: route.MultiPath[i].Gw, LinkIndex: route.MultiPath[i].LinkIndex})
		}
	}

	ret, err := hooks.NetRouteAdd(&cmn.RouteMod{Protocol: int(route.Protocol), Flags: route.Flags, Dst: ipNet, GWs: gws})
	if err != nil {
		if route.Gw != nil {
			tk.LogIt(tk.LogError, "[NLP] RT  %s via %s proto %d add failed-%s\n", ipNet.String(),
				route.Gw.String(), route.Protocol, err)
		} else {
			tk.LogIt(tk.LogError, "[NLP] RT  %s add failed-%s\n", ipNet.String(), err)
		}
	} else {
		if route.Gw != nil {
			tk.LogIt(tk.LogInfo, "[NLP] RT  %s via %s added\n", ipNet.String(),
				route.Gw.String())
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] RT  %s added\n", ipNet.String())
		}
	}

	return ret
}

func AddRouteNoHook(DestinationIPNet, gateway, proto string) int {
	var ret int
	var route nlp.Route
	_, Ipnet, err := net.ParseCIDR(DestinationIPNet)
	if err != nil {
		return -1
	}
	Gw := net.ParseIP(gateway)
	route.Dst = Ipnet
	route.Gw = Gw

	if proto == "static" {
		route.Protocol = 4 // 4 means Proto Static.
	}
	err = nlp.RouteAdd(&route)
	if err != nil {
		return -1
	}
	return ret
}

func DelRouteNoHook(DestinationIPNet string) int {
	var ret int
	var route nlp.Route
	_, Ipnet, err := net.ParseCIDR(DestinationIPNet)
	if err != nil {
		return -1
	}
	route.Dst = Ipnet
	err = nlp.RouteDel(&route)
	if err != nil {
		return -1
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
	ret, err := hooks.NetRouteDel(&cmn.RouteMod{Dst: ipNet})
	if err != nil {
		if route.Gw != nil {
			tk.LogIt(tk.LogError, "[NLP] RT  %s via %s delete failed-%s\n", ipNet.String(),
				route.Gw.String(), err)
		} else {
			tk.LogIt(tk.LogError, "[NLP] RT  %s delete failed-%s\n", ipNet.String(), err)
		}
	} else {
		if route.Gw != nil {
			tk.LogIt(tk.LogInfo, "[NLP] RT  %s via %s deleted\n", ipNet.String(),
				route.Gw.String())
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] RT  %s deleted\n", ipNet.String())
		}
	}
	return ret
}

func LUWorkSingle(m nlp.LinkUpdate) int {
	var ret int

	if NlpIsBlackListedIntf(m.Link.Attrs().Name, m.Link.Attrs().MasterIndex) {
		return -1
	}

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

	//if iSBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
	//	return -1
	//}

	attrs := link.Attrs()
	name := attrs.Name
	if m.NewAddr {
		_, err := hooks.NetAddrAdd(&cmn.IPAddrMod{Dev: name, IP: m.LinkAddress.String()})
		if err != nil {
			tk.LogIt(tk.LogInfo, "[NLP] Address %v Port %v add failed\n", m.LinkAddress.String(), name)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Address %v Port %v added\n", m.LinkAddress.String(), name)
		}

	} else {
		_, err := hooks.NetAddrDel(&cmn.IPAddrMod{Dev: name, IP: m.LinkAddress.String()})
		if err != nil {
			tk.LogIt(tk.LogInfo, "[NLP] Address %v Port %v delete failed\n", m.LinkAddress.String(), name)
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Address %v Port %v deleted\n", m.LinkAddress.String(), name)
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

	if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
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

	if len(m.MultiPath) <= 0 {
		link, err := nlp.LinkByIndex(m.LinkIndex)
		if err != nil {
			tk.LogIt(tk.LogError, "RUWorkSingle: link find error %s\n", err)
			return -1
		}

		if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
			return -1
		}
	} else {
		for _, path := range m.MultiPath {
			link, err := nlp.LinkByIndex(path.LinkIndex)
			if err != nil {
				tk.LogIt(tk.LogError, "RUWorkSingle: link find error %s\n", err)
				return -1
			}

			if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
				return -1
			}
		}
	}

	if skipIfRoute {
		if m.Route.Scope.String() == "link" && tk.IsNetIPv4(m.Dst.IP.String()) {
			return -1
		}
	}

	if m.Type == syscall.RTM_NEWROUTE {
		ret = AddRoute(m.Route)
	} else {
		ret = DelRoute(m.Route)
	}

	return ret
}

func LUWorker(ch chan nlp.LinkUpdate, f chan struct{}) {

	for n := 0; n < 32; n++ {
		select {
		case m := <-ch:
			LUWorkSingle(m)
		default:
			continue
		}
	}
}

func AUWorker(ch chan nlp.AddrUpdate, f chan struct{}) {

	for n := 0; n < 64; n++ {
		select {
		case m := <-ch:
			AUWorkSingle(m)
		default:
			continue
		}
	}

}

func NUWorker(ch chan nlp.NeighUpdate, f chan struct{}) {

	for n := 0; n < 128; n++ {
		select {
		case m := <-ch:
			NUWorkSingle(m)
		default:
			continue
		}
	}
}

func RUWorker(ch chan nlp.RouteUpdate, f chan struct{}) {

	for n := 0; n < 128; n++ {
		select {
		case m := <-ch:
			RUWorkSingle(m)
		default:
			continue
		}
	}
}

func NLWorker(nNl *NlH, bgpPeerMode bool, ch chan bool, wch chan bool) {
	ch <- true
	<-wch

	defer func() {
		if e := recover(); e != nil {
			tk.LogIt(tk.LogCritical, "%s: %s\n", e, debug.Stack())
		}
		hooks.NetHandlePanic()
		os.Exit(1)
	}()

	if bgpPeerMode {
		for { /* Single thread for reading route NL msgs in below order */
			RUWorker(nNl.FromRUCh, nNl.FromRUDone)
			time.Sleep(1000 * time.Millisecond)
		}
	} else {
		for { /* Single thread for reading all NL msgs in below order */
			LUWorker(nNl.FromLUCh, nNl.FromLUDone)
			AUWorker(nNl.FromAUCh, nNl.FromAUDone)
			NUWorker(nNl.FromNUCh, nNl.FromNUDone)
			RUWorker(nNl.FromRUCh, nNl.FromRUDone)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func GetBridges() {
	links, err := nlp.LinkList()
	if err != nil {
		return
	}
	for _, link := range links {
		if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
			continue
		}
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
	tk.LogIt(tk.LogInfo, "[NLP] Getting device info\n")

	GetBridges()

	links, err := nlp.LinkList()
	if err != nil {
		tk.LogIt(tk.LogError, "[NLP] Error in getting device info(%v)\n", err)
		ret = -1
	}

	for _, link := range links {

		if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
			continue
		}

		ret = ModLink(link, true)
		if ret == -1 {
			continue
		}
	}

	for _, link := range links {

		if NlpIsBlackListedIntf(link.Attrs().Name, link.Attrs().MasterIndex) {
			// Need addresss to work with
			addrs, err := nlp.AddrList(link, nlp.FAMILY_ALL)
			if err != nil {
				tk.LogIt(tk.LogError, "[NLP] Error getting address list %v for intf %s\n",
					err, link.Attrs().Name)
			}

			if len(addrs) == 0 {
				tk.LogIt(tk.LogDebug, "[NLP] No addresses found for intf %s\n", link.Attrs().Name)
			} else {
				for _, addr := range addrs {
					AddAddr(addr, link)
				}
			}
			continue
		}

		/* Get FDBs */
		_, ok := link.(*nlp.Vxlan)
		if link.Attrs().MasterIndex > 0 || ok {
			neighs, err := nlp.NeighList(link.Attrs().Index, unix.AF_BRIDGE)
			if err != nil {
				tk.LogIt(tk.LogError, "[NLP] Error getting neighbors list %v for intf %s\n",
					err, link.Attrs().Name)
			}

			if len(neighs) == 0 {
				tk.LogIt(tk.LogDebug, "[NLP] No FDBs found for intf %s\n", link.Attrs().Name)
			} else {
				for _, neigh := range neighs {
					AddNeigh(neigh, link)
				}
			}
		}

		addrs, err := nlp.AddrList(link, nlp.FAMILY_ALL)
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] Error getting address list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(addrs) == 0 {
			tk.LogIt(tk.LogDebug, "[NLP] No addresses found for intf %s\n", link.Attrs().Name)
		} else {
			for _, addr := range addrs {
				AddAddr(addr, link)
			}
		}

		neighs, err := nlp.NeighList(link.Attrs().Index, nlp.FAMILY_ALL)
		if err != nil {
			tk.LogIt(tk.LogError, "[NLP] Error getting neighbors list %v for intf %s\n",
				err, link.Attrs().Name)
		}

		if len(neighs) == 0 {
			tk.LogIt(tk.LogDebug, "[NLP] No neighbors found for intf %s\n", link.Attrs().Name)
		} else {
			for _, neigh := range neighs {
				AddNeigh(neigh, link)
			}
		}
	}

	/* Get Routes */
	routes, err := nlp.RouteList(nil, nlp.FAMILY_ALL)
	if err != nil {
		tk.LogIt(tk.LogError, "[NLP] Error getting route list %v\n", err)
	}

	if len(routes) == 0 {
		tk.LogIt(tk.LogDebug, "[NLP] No STATIC routes found\n")
	} else {
		for _, route := range routes {
			var m nlp.RouteUpdate
			if skipIfRoute {
				if route.Scope.String() == "link" && tk.IsNetIPv4(route.Dst.IP.String()) {
					continue
				}
			}
			m.Type = syscall.RTM_NEWROUTE
			m.Route = route

			RUWorkSingle(m)
		}
	}
	tk.LogIt(tk.LogInfo, "[NLP] nlp get done\n")
	ch <- true
	return ret
}

var nNl *NlH

func LbSessionGet(done bool) int {

	if done {

		if _, err := os.Stat(opt.Opts.ConfigPath + "/EPconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LogInfo, "[NLP] No EndPoint config file : %s \n", err.Error())
			}
		} else {
			applyEPConfig()
		}
		tk.LogIt(tk.LogInfo, "[NLP] EndPoint done\n")

		if _, err := os.Stat(opt.Opts.ConfigPath + "/lbconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LogInfo, "[NLP] No load balancer config file : %s \n", err.Error())
			}
		} else {
			applyLoadBalancerConfig()
		}

		tk.LogIt(tk.LogInfo, "[NLP] LoadBalancer done\n")
		if _, err := os.Stat(opt.Opts.ConfigPath + "/sessionconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LogInfo, "[NLP] No Session config file : %s \n", err.Error())
			}
		} else {
			applySessionConfig()
		}

		tk.LogIt(tk.LogInfo, "[NLP] Session done\n")
		if _, err := os.Stat(opt.Opts.ConfigPath + "/sessionulclconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LogInfo, "[NLP] No UlCl config file : %s \n", err.Error())
			}
		} else {
			applyUlClConfig()
		}

		tk.LogIt(tk.LogInfo, "[NLP] Session UlCl done\n")
		if _, err := os.Stat(opt.Opts.ConfigPath + "/FWconfig.txt"); errors.Is(err, os.ErrNotExist) {
			if err != nil {
				tk.LogIt(tk.LogInfo, "[NLP] No Firewall config file : %s \n", err.Error())
			}
		} else {
			applyFWConfig()
		}
		tk.LogIt(tk.LogInfo, "[NLP] Firewall done\n")

		tk.LogIt(tk.LogInfo, "[NLP] LbSessionGet done\n")
	}

	return 0
}

func NlpInit(bgpPeerMode bool, blackList, whitelist string, ipvsCompat bool) *NlH {

	nNl = new(NlH)

	nNl.BlackList = blackList
	nNl.BLRgx = regexp.MustCompile(blackList)
	nNl.WhiteList = whitelist
	nNl.WLRgx = regexp.MustCompile(whitelist)
	checkInit := make(chan bool)
	waitInit := make(chan bool)

	if bgpPeerMode {
		nNl.FromRUCh = make(chan nlp.RouteUpdate, cmn.RuWorkQLen)
		err := nlp.RouteSubscribe(nNl.FromRUCh, nNl.FromRUDone)
		if err != nil {
			fmt.Println(err)
		} else {
			tk.LogIt(tk.LogInfo, "[NLP] Route msgs subscribed\n")
		}

		go NLWorker(nNl, bgpPeerMode, checkInit, waitInit)
		<-checkInit
		return nNl
	}

	nNl.FromAUCh = make(chan nlp.AddrUpdate, cmn.AuWorkqLen)
	nNl.FromLUCh = make(chan nlp.LinkUpdate, cmn.LuWorkQLen)
	nNl.FromNUCh = make(chan nlp.NeighUpdate, cmn.NuWorkQLen)
	nNl.FromRUCh = make(chan nlp.RouteUpdate, cmn.RuWorkQLen)
	nNl.FromAUDone = make(chan struct{})
	nNl.FromLUDone = make(chan struct{})
	nNl.FromNUDone = make(chan struct{})
	nNl.FromRUCh = make(chan nlp.RouteUpdate, cmn.RuWorkQLen)
	nNl.IMap = make(map[string]Intf)

	go NLWorker(nNl, bgpPeerMode, checkInit, waitInit)
	<-checkInit

	err := nlp.LinkSubscribe(nNl.FromLUCh, nNl.FromLUDone)
	if err != nil {
		tk.LogIt(tk.LogError, "%v", err)
	} else {
		tk.LogIt(tk.LogInfo, "[NLP] Link msgs subscribed\n")
	}
	err = nlp.AddrSubscribe(nNl.FromAUCh, nNl.FromAUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LogInfo, "[NLP] Addr msgs subscribed\n")
	}
	err = nlp.NeighSubscribe(nNl.FromNUCh, nNl.FromNUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LogInfo, "[NLP] Neigh msgs subscribed\n")
	}
	err = nlp.RouteSubscribe(nNl.FromRUCh, nNl.FromRUDone)
	if err != nil {
		fmt.Println(err)
	} else {
		tk.LogIt(tk.LogInfo, "[NLP] Route msgs subscribed\n")
	}

	tk.LogIt(tk.LogInfo, "[NLP] NLP Subscription done\n")

	go NlpGet(checkInit)
	done := <-checkInit
	waitInit <- true

	go LbSessionGet(done)

	if ipvsCompat {
		IPVSInit()
	}

	return nNl
}
