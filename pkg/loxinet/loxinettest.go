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
	"fmt"
	"net"
	"testing"

	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
)

// TestLoxinet - Go unit test entry point
func TestLoxinet(t *testing.T) {

	opts.Opts.NoNlp = true
	opts.Opts.NoAPI = true
	opts.Opts.CPUProfile = "none"
	opts.Opts.Prometheus = false
	opts.Opts.K8sAPI = "none"
	opts.Opts.ClusterNodes = "none"

	fmt.Printf("LoxiLB Net Unit-Test \n")
	loxiNetInit()

	ifmac := [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6}
	_, err := mh.zr.Ports.PortAdd("hs0", 12, cmn.PortReal, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil},
		PortLayer2Info{false, 10})

	if err != nil {
		t.Errorf("failed to add port %s:%s", "hs0", err)
	}

	p := mh.zr.Ports.PortFindByName("hs0")
	if p == nil {
		t.Errorf("failed to add port %s", "hs0")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x7}
	_, err = mh.zr.Ports.PortAdd("bond1", 15, cmn.PortBond, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil},
		PortLayer2Info{false, 10})

	if err != nil {
		t.Errorf("failed to add port %s", "bond1")
	}

	p = mh.zr.Ports.PortFindByName("bond1")
	if p == nil {
		t.Errorf("failed to add port %s", "bond1")
	}

	_, err = mh.zr.Ports.PortAdd("hs1", 15, cmn.PortReal, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil},
		PortLayer2Info{false, 10})
	if err != nil {
		t.Errorf("failed to add port hs1")
	}

	_, err = mh.zr.Ports.PortAdd("hs1", 15, cmn.PortBondSif, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "bond1", "", 0, nil, nil},
		PortLayer2Info{false, 10})
	if err != nil {
		t.Errorf("failed to add port hs1 to bond1")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x8}
	_, err = mh.zr.Ports.PortAdd("hs2", 100, cmn.PortReal, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil},
		PortLayer2Info{false, 10})
	if err != nil {
		t.Errorf("failed to add port %s", "hs2")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x8}
	_, err = mh.zr.Ports.PortAdd("hs4", 400, cmn.PortReal, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil},
		PortLayer2Info{false, 10})
	if err != nil {
		t.Errorf("failed to add port %s", "hs4")
	}

	ifmac = [6]byte{0xde, 0xdc, 0x1f, 0x62, 0x60, 0x55}
	_, err = mh.zr.Ports.PortAdd("vxlan4", 20, cmn.PortVxlanBr, RootZone,
		PortHwInfo{ifmac, true, true, 1500, "", "hs4", 4, nil, nil},
		PortLayer2Info{false, 0})
	if err != nil {
		t.Errorf("failed to add port %s", "vxlan4")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0xa}
	_, err = mh.zr.Vlans.VlanAdd(100, "vlan100", RootZone, 124,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil})
	if err != nil {
		t.Errorf("failed to add port %s", "vlan100")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0xa}
	_, err = mh.zr.Vlans.VlanAdd(4, "vlan4", RootZone, 126,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil})
	if err != nil {
		t.Errorf("failed to add port %s", "vlan4")
	}

	_, err = mh.zr.Vlans.VlanPortAdd(4, "vxlan4", false)
	if err != nil {
		t.Errorf("failed to add port %s to vlan %d\n", "vxlan4", 4)
	}

	p = mh.zr.Ports.PortFindByName("vlan100")
	if p == nil {
		t.Errorf("failed to add port %s", "vlan100")
	}

	_, err = mh.zr.Vlans.VlanPortAdd(100, "hs0", false)
	if err != nil {
		t.Errorf("failed to add port %s to vlan %d", "hs0", 100)
	}

	_, err = mh.zr.Vlans.VlanPortAdd(100, "hs0", true)
	if err != nil {
		t.Errorf("failed to add tagged port %s to vlan %d", "hs0", 100)
	}

	_, err = mh.zr.L3.IfaAdd("vlan100", "21.21.21.1/24")
	if err != nil {
		t.Errorf("failed to add l3 ifa to vlan%d", 100)
	}

	_, err = mh.zr.L3.IfaAdd("hs0", "11.11.11.1/32")
	if err != nil {
		t.Errorf("failed to add l3 ifa to hs0")
	}
	fmt.Printf("#### Interface List ####\n")
	mh.zr.Ports.Ports2String(&mh)
	fmt.Printf("#### IFA List ####\n")
	mh.zr.L3.Ifas2String(&mh)

	_, err = mh.zr.L3.IfaDelete("vlan100", "21.21.21.1/24")
	if err != nil {
		t.Errorf("failed to delete l3 ifa from vlan100")
	}

	fmt.Printf("#### IFA List ####\n")
	mh.zr.L3.Ifas2String(&mh)

	fmt.Printf("#### Vlan List ####\n")
	mh.zr.Vlans.Vlans2String(&mh)

	_, err = mh.zr.Vlans.VlanPortDelete(100, "hs0", false)
	if err != nil {
		t.Errorf("failed to delete hs0 from from vlan100")
	}
	_, err = mh.zr.Vlans.VlanPortDelete(100, "hs0", true)
	if err != nil {
		t.Errorf("failed to delete tagged hs0 from from vlan100")
	}
	_, err = mh.zr.Vlans.VlanDelete(100)
	if err != nil {
		t.Errorf("failed to delete vlan100")
	}

	ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0xa}
	_, err = mh.zr.Vlans.VlanAdd(100, "vlan100", RootZone, 124,
		PortHwInfo{ifmac, true, true, 1500, "", "", 0, nil, nil})
	if err != nil {
		t.Errorf("failed to add port %s", "vlan100")
	}

	fdbKey := FdbKey{[6]byte{0x05, 0x04, 0x03, 0x3, 0x1, 0x0}, 100}
	fdbAttr := FdbAttr{"hs0", net.ParseIP("0.0.0.0"), cmn.FdbVlan}

	_, err = mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	if err != nil {
		t.Errorf("failed to add fdb hs0:vlan100")
	}
	_, err = mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	if err == nil {
		t.Errorf("added duplicate fdb vlan100")
	}

	fdbKey1 := FdbKey{[6]byte{0xb, 0xa, 0x9, 0x8, 0x7, 0x6}, 100}
	fdbAttr1 := FdbAttr{"hs2", net.ParseIP("0.0.0.0"), cmn.FdbVlan}

	_, err = mh.zr.L2.L2FdbAdd(fdbKey1, fdbAttr1)
	if err != nil {
		t.Errorf("failed to add fdb hs2:vlan100")
	}

	_, err = mh.zr.L2.L2FdbDel(fdbKey1)
	if err != nil {
		t.Errorf("failed to del fdb hs2:vlan100")
	}

	_, err = mh.zr.L2.L2FdbDel(fdbKey1)
	if err == nil {
		t.Errorf("deleted non-existing fdb hs2:vlan100")
	}

	hwmac, _ := net.ParseMAC("00:00:00:00:00:01")
	_, err = mh.zr.Nh.NeighAdd(net.IPv4(8, 8, 8, 8), "default", NeighAttr{12, 1, hwmac})
	if err != nil {
		t.Errorf("NHAdd fail 8.8.8.8")
	}

	hwmac1, _ := net.ParseMAC("00:00:00:00:00:00")
	_, err = mh.zr.Nh.NeighAdd(net.IPv4(10, 10, 10, 10), "default", NeighAttr{12, 1, hwmac1})
	if err != nil {
		t.Errorf("NHAdd fail 10.10.10.10")
	}

	route := net.IPv4(1, 1, 1, 1)
	mask := net.CIDRMask(24, 32)
	route = route.Mask(mask)
	ipnet := net.IPNet{IP: route, Mask: mask}
	ra := RtAttr{0, 0, false, -1, false}
	na := []RtNhAttr{{net.IPv4(8, 8, 8, 8), 12}}
	_, err = mh.zr.Rt.RtAdd(ipnet, "default", ra, na)
	if err != nil {
		t.Errorf("NHAdd fail 1.1.1.1/24 via 8.8.8.8")
	}

	_, err = mh.zr.Nh.NeighDelete(net.IPv4(8, 8, 8, 8), "default")
	if err != nil {
		t.Errorf("NHAdd fail 8.8.8.8")
	}

	_, err = mh.zr.Rt.RtDelete(ipnet, "default")
	if err != nil {
		t.Errorf("RouteDel fail 1.1.1.1/24 via 8.8.8.8")
	}

	_, err = mh.zr.L3.IfaAdd("hs4", "4.4.4.254/24")
	if err != nil {
		t.Errorf("fail to add 4.4.4.4/24 ifa to hs4")
	}

	hwmac, _ = net.ParseMAC("46:17:8e:50:3c:e5")
	_, err = mh.zr.Nh.NeighAdd(net.IPv4(4, 4, 4, 1), RootZone, NeighAttr{400, 1, hwmac})
	if err != nil {
		t.Errorf("NHAdd fail 4.4.4.1\n")
	}

	fdbKey = FdbKey{[6]byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf}, 4}
	fdbAttr = FdbAttr{"vxlan4", net.ParseIP("4.4.4.1"), cmn.FdbTun}
	_, err = mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	if err != nil {
		t.Errorf("tun FDB add fail 4.4.4.1 %s\n", err)
	}

	_, err = mh.zr.L3.IfaAdd("vlan4", "44.44.44.254/24")
	if err != nil {
		t.Errorf("add ifa fail 44.44.44.254 to vlan4\n")
	}

	hwmac1, _ = net.ParseMAC("0a:0b:0c:0d:0e:0f")
	_, err = mh.zr.Nh.NeighAdd(net.IPv4(44, 44, 44, 1), RootZone,
		NeighAttr{126, 1, hwmac1})
	if err != nil {
		t.Errorf("add neighbor fail 44.44.44.1 to vlan4\n")
	}

	lbServ := cmn.LbServiceArg{ServIP: "10.10.10.1", ServPort: 2020, Proto: "tcp", Sel: cmn.LbSelRr}
	lbEps := []cmn.LbEndPointArg{
		{
			EpIP:   "32.32.32.1",
			EpPort: 5001,
			Weight: 1,
		},
		{
			EpIP:   "32.32.32.1",
			EpPort: 5001,
			Weight: 2,
		},
	}
	_, err = mh.zr.Rules.AddNatLbRule(lbServ, nil, lbEps[:])
	if err != nil {
		t.Errorf("failed to add nat lb rule for 10.10.10.1\n")
	}

	_, err = mh.zr.Rules.DeleteNatLbRule(lbServ)
	if err != nil {
		t.Errorf("failed to delete nat lb rule for 10.10.10.1\n")
	}

	// Session information
	anTun := cmn.SessTun{TeID: 1, Addr: net.IP{172, 17, 1, 231}} // An TeID, gNBIP
	cnTun := cmn.SessTun{TeID: 1, Addr: net.IP{172, 17, 1, 50}}  // Cn TeID, MyIP

	_, err = mh.zr.Sess.SessAdd("user1", net.IP{100, 64, 50, 1}, anTun, cnTun)
	if err != nil {
		t.Errorf("Failed to add session for user1\n")
	}

	// Add ULCL classifier
	_, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr: net.IP{8, 8, 8, 8}, Qfi: 11})
	if err != nil {
		t.Errorf("Failed to ulcl-cls session for user1 - 8.8.8.8\n")
	}

	_, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr: net.IP{9, 9, 9, 9}, Qfi: 1})
	if err != nil {
		t.Errorf("Failed to ulcl-cls session for user1 - 9.9.9.9\n")
	}

	fmt.Printf("#### User-Session ####\n")
	mh.zr.Sess.USess2String(&mh)

	_, err = mh.zr.Sess.UlClDeleteCls("user1", cmn.UlClArg{Addr: net.IP{9, 9, 9, 9}, Qfi: 1})
	if err != nil {
		t.Errorf("Failed to delete ulcl-cls session for user1 - 9.9.9.9\n")
	}

	_, err = mh.zr.Sess.UlClDeleteCls("user1", cmn.UlClArg{Addr: net.IP{8, 8, 8, 8}, Qfi: 11})
	if err != nil {
		t.Errorf("Failed to delete ulcl-cls session for user1 - 8.8.8.8\n")
	}

	_, err = mh.zr.Sess.SessDelete("user1")
	if err != nil {
		t.Errorf("Failed to delete session for user1\n")
	}

	pInfo := cmn.PolInfo{PolType: 0, ColorAware: false, CommittedInfoRate: 100, PeakInfoRate: 100}
	polObj := cmn.PolObj{PolObjName: "hs0", AttachMent: cmn.PolAttachPort}
	_, err = mh.zr.Pols.PolAdd("pol-hs0", pInfo, polObj)
	if err != nil {
		t.Errorf("Failed to add policer pol-hs0\n")
	}

	_, err = mh.zr.Pols.PolDelete("pol-hs0")
	if err != nil {
		t.Errorf("Failed to delete policer pol-hs0\n")
	}

	mInfo := cmn.MirrInfo{MirrType: cmn.MirrTypeSpan, MirrPort: "hs0"}
	mObj := cmn.MirrObj{MirrObjName: "hs1", AttachMent: cmn.MirrAttachPort}
	_, err = mh.zr.Mirrs.MirrAdd("mirr-1", mInfo, mObj)
	if err != nil {
		t.Errorf("Failed to add mirror mirr-1\n")
	}

	_, err = mh.zr.Mirrs.MirrDelete("mirr-1")
	if err != nil {
		t.Errorf("Failed to delete mirror mirr-1\n")
	}

	fwR := cmn.FwRuleArg{SrcIP: "192.168.1.2/24", DstIP: "192.168.2.1/24", Pref: 100}
	fwOpts := cmn.FwOptArg{Drop: true}
	_, err = mh.zr.Rules.AddFwRule(fwR, fwOpts)
	if err != nil {
		t.Errorf("Failed to add fw-1\n")
	}

	fwR1 := cmn.FwRuleArg{SrcIP: "192.169.1.2/24", DstIP: "192.169.2.1/24", Pref: 200}
	fwOpts = cmn.FwOptArg{Drop: true}
	_, err = mh.zr.Rules.AddFwRule(fwR1, fwOpts)
	if err != nil {
		t.Errorf("Failed to add fw-2\n")
	}

	_, err = mh.zr.Rules.AddFwRule(fwR1, fwOpts)
	if err == nil {
		t.Errorf("Allowed to add duplicate fw-2\n")
	}

	fwR2 := cmn.FwRuleArg{SrcIP: "0.0.0.0/0", DstIP: "31.31.31.1/24", Pref: 200}
	fwOpts = cmn.FwOptArg{Allow: true}
	_, err = mh.zr.Rules.AddFwRule(fwR2, fwOpts)
	if err != nil {
		t.Errorf("Failed to add fw-3\n")
	}

	_, err = mh.zr.Rules.DeleteFwRule(fwR)
	if err != nil {
		t.Errorf("Failed to del fw-1\n")
	}

	_, err = mh.zr.Rules.DeleteFwRule(fwR1)
	if err != nil {
		t.Errorf("Failed to del fw-2\n")
	}

	_, err = mh.zr.Rules.DeleteFwRule(fwR2)
	if err != nil {
		t.Errorf("Failed to del fw-3\n")
	}

	fmt.Printf("#### Route-List ####\n")
	mh.zr.Rt.Rts2String(&mh)

	fmt.Printf("#### NH-List ####\n")
	mh.zr.Nh.Neighs2String(&mh)

	fmt.Printf("#### Trie-List1 ####\n")
	mh.zr.Rt.Trie4.Trie2String(mh.zr.Rt)

}
