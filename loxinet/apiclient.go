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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// This file implements interface defined in cmn.NetHookInterface
// The implementation is thread-safe and can be called by multiple-clients at once

type NetApiStruct struct {
}

// NetApiInit - Initialize a new instance of NetApi
func NetApiInit() *NetApiStruct {
	na := new(NetApiStruct)
	return na
}

// NetMirrorAdd - Add a mirror in loxinet
func (*NetApiStruct) NetMirrorAdd(mm *cmn.MirrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Mirrs.MirrAdd(mm.Ident, mm.Info, mm.Target)
	return ret, err
}

// NetMirrorDel - Delete a mirror in loxinet
func (*NetApiStruct) NetMirrorDel(mm *cmn.MirrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Mirrs.MirrDelete(mm.Ident)
	return ret, err
}

// NetPortGet - Get Port Information of loxinet
func (*NetApiStruct) NetPortGet() ([]cmn.PortDump, error) {
	ret, err := mh.zr.Ports.PortsToGet()
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// NetPortAdd - Add a port in loxinet
func (*NetApiStruct) NetPortAdd(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortAdd(pm.Dev, pm.LinkIndex, pm.Ptype, RootZone,
		PortHwInfo{pm.MacAddr, pm.Link, pm.State, pm.Mtu, pm.Master, pm.Real,
			uint32(pm.TunID)}, PortLayer2Info{false, 0})

	return ret, err
}

// NetPortDel - Delete port from loxinet
func (*NetApiStruct) NetPortDel(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortDel(pm.Dev, pm.Ptype)
	return ret, err
}

// NetVlanAdd - Add vlan info to loxinet
func (*NetApiStruct) NetVlanAdd(vm *cmn.VlanMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanAdd(vm.Vid, vm.Dev, RootZone, vm.LinkIndex,
		PortHwInfo{vm.MacAddr, vm.Link, vm.State, vm.Mtu, "", "", vm.TunID})
	if ret == VlanExistsErr {
		ret = 0
	}

	return ret, err
}

// NetVlanDel - Delete vlan info from loxinet
func (*NetApiStruct) NetVlanDel(vm *cmn.VlanMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanDelete(vm.Vid)
	return ret, err
}

// NetVlanPortAdd - Add a port to vlan in loxinet
func (*NetApiStruct) NetVlanPortAdd(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortAdd(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetVlanPortDel - Delete a port from vlan in loxinet
func (*NetApiStruct) NetVlanPortDel(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortDelete(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetIpv4AddrAdd - Add an ipv4 address in loxinet
func (*NetApiStruct) NetIpv4AddrAdd(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaAdd(am.Dev, am.IP)
	return ret, err
}

// NetIpv4AddrDel - Delete an ipv4 address in loxinet
func (*NetApiStruct) NetIpv4AddrDel(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaDelete(am.Dev, am.IP)
	return ret, err
}

// NetNeighv4Add - Add a ipv4 neighbor in loxinet
func (*NetApiStruct) NetNeighv4Add(nm *cmn.Neighv4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighAdd(nm.IP, RootZone, NeighAttr{nm.LinkIndex, nm.State, nm.HardwareAddr})
	if err != nil {
		if ret != NeighExistsErr {
			return ret, err
		}
	}

	return 0, nil
}

// NetNeighv4Del - Delete a ipv4 neighbor in loxinet
func (*NetApiStruct) NetNeighv4Del(nm *cmn.Neighv4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighDelete(nm.IP, RootZone)
	return ret, err
}

// NetFdbAdd - Add a forwarding database entry in loxinet
func (*NetApiStruct) NetFdbAdd(fm *cmn.FdbMod) (int, error) {
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	fdbAttr := FdbAttr{fm.Dev, fm.Dst, fm.Type}

	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	return ret, err
}

// NetFdbDel - Delete a forwarding database entry in loxinet
func (*NetApiStruct) NetFdbDel(fm *cmn.FdbMod) (int, error) {
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbDel(fdbKey)
	return ret, err
}

// NetRoutev4Add - Add an ipv4 route in loxinet
func (*NetApiStruct) NetRoutev4Add(rm *cmn.Routev4Mod) (int, error) {
	var ret int
	var err error

	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ra := RtAttr{rm.Protocol, rm.Flags, false}
	if rm.Gw != nil {
		na := []RtNhAttr{{rm.Gw, rm.LinkIndex}}
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, RootZone, ra, na)
	} else {
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, RootZone, ra, nil)
	}

	return ret, err
}

// NetRoutev4Del - Delete an ipv4 route in loxinet
func (*NetApiStruct) NetRoutev4Del(rm *cmn.Routev4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rt.RtDelete(rm.Dst, RootZone)
	return ret, err
}

// NetLbRuleAdd - Add a load-balancer rule in loxinet
func (*NetApiStruct) NetLbRuleAdd(lm *cmn.LbRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.AddNatLbRule(lm.Serv, lm.Eps[:])
	if err == nil && lm.Serv.Bgp {
		if mh.bgp != nil {
			mh.bgp.AddBGPRule(lm.Serv.ServIP)
		} else {
			tk.LogIt(tk.LogDebug, "loxilb BGP mode is disable \n")
		}
	}
	return ret, err
}

// NetLbRuleDel - Delete a load-balancer rule in loxinet
func (*NetApiStruct) NetLbRuleDel(lm *cmn.LbRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteNatLbRule(lm.Serv)
	if lm.Serv.Bgp {
		if mh.bgp != nil {
			mh.bgp.DelBGPRule(lm.Serv.ServIP)
		} else {
			tk.LogIt(tk.LogDebug, "loxilb BGP mode is disable \n")
		}
	}
	return ret, err
}

// NetLbRuleGet - Get a load-balancer rule from loxinet
func (*NetApiStruct) NetLbRuleGet() ([]cmn.LbRuleMod, error) {
	ret, err := mh.zr.Rules.GetNatLbRule()
	return ret, err
}

// NetCtInfoGet - Get connection track info from loxinet
func (*NetApiStruct) NetCtInfoGet() ([]cmn.CtInfo, error) {
	// There is no locking requirement for this operation
	ret := mh.dp.DpMapGetCt4()
	return ret, nil
}

// NetSessionAdd - Add a 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionAdd(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessAdd(sm.Ident, sm.IP, sm.AnTun, sm.CnTun)
	return ret, err
}

// NetSessionDel - Delete a 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionDel(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessDelete(sm.Ident)
	return ret, err
}

// NetSessionUlClAdd - Add a 3gpp ulcl-filter info in loxinet
func (*NetApiStruct) NetSessionUlClAdd(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClAddCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionUlClDel - Delete a 3gpp ulcl-filter info in loxinet
func (*NetApiStruct) NetSessionUlClDel(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClDeleteCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionGet - Get 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionGet() ([]cmn.SessionMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessGet()
	return ret, err
}

// NetSessionUlClGet - Get 3gpp ulcl filter info from loxinet
func (*NetApiStruct) NetSessionUlClGet() ([]cmn.SessionUlClMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessUlclGet()
	return ret, err
}

// NetPolicerAdd - Add a policer in loxinet
func (*NetApiStruct) NetPolicerAdd(pm *cmn.PolMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolAdd(pm.Ident, pm.Info, pm.Target)
	return ret, err
}

// NetPolicerDel - Delete a policer in loxinet
func (*NetApiStruct) NetPolicerDel(pm *cmn.PolMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolDelete(pm.Ident)
	return ret, err
}
