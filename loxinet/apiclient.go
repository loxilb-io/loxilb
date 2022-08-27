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

// Initialize a new instance of NetApi
func NetApiInit() *NetApiStruct {
	na := new(NetApiStruct)
	return na
}

// Get Port Information of loxinet
func (*NetApiStruct) NetPortGet() ([]cmn.PortDump, error) {
	ret, err := mh.zr.Ports.PortsToGet()
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Add a port in loxinet
func (*NetApiStruct) NetPortAdd(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortAdd(pm.Dev, pm.LinkIndex, pm.Ptype, ROOT_ZONE,
		PortHwInfo{pm.MacAddr, pm.Link, pm.State, pm.Mtu, pm.Master, pm.Real,
			uint32(pm.TunId)}, PortLayer2Info{false, 0})

	return ret, err
}

// Delete port from loxinet
func (*NetApiStruct) NetPortDel(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortDel(pm.Dev, pm.Ptype)
	return ret, err
}

// Add vlan info to loxinet
func (*NetApiStruct) NetVlanAdd(vm *cmn.VlanMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanAdd(vm.Vid, vm.Dev, ROOT_ZONE, vm.LinkIndex,
		PortHwInfo{vm.MacAddr, vm.Link, vm.State, vm.Mtu, "", "", vm.TunId})
	if ret == VLAN_EXISTS_ERR {
		ret = 0
	}

	return ret, err
}

// Delete vlan info from loxinet
func (*NetApiStruct) NetVlanDel(vm *cmn.VlanMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanDelete(vm.Vid)
	return ret, err
}

// Add a port to vlan in loxinet
func (*NetApiStruct) NetVlanPortAdd(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortAdd(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// Delete a port from vlan in loxinet
func (*NetApiStruct) NetVlanPortDel(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortDelete(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// Add an ipv4 address in loxinet
func (*NetApiStruct) NetIpv4AddrAdd(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaAdd(am.Dev, am.Ip)
	return ret, err
}

// Delete an ipv4 address in loxinet
func (*NetApiStruct) NetIpv4AddrDel(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaDelete(am.Dev, am.Ip)
	return ret, err
}

// Add a ipv4 neighbor in loxinet
func (*NetApiStruct) NetNeighv4Add(nm *cmn.Neighv4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighAdd(nm.Ip, ROOT_ZONE, NeighAttr{nm.LinkIndex, nm.State, nm.HardwareAddr})
	if err != nil {
		if ret != NEIGH_EXISTS_ERR {
			return ret, err
		}
	}

	return 0, nil
}

// Delete a ipv4 neighbor in loxinet
func (*NetApiStruct) NetNeighv4Del(nm *cmn.Neighv4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighDelete(nm.Ip, ROOT_ZONE)
	return ret, err
}

// Add a forwarding database entry in loxinet
func (*NetApiStruct) NetFdbAdd(fm *cmn.FdbMod) (int, error) {
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeId}
	fdbAttr := FdbAttr{fm.Dev, fm.Dst, fm.Type}

	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	return ret, err
}

// Delete a forwarding database entry in loxinet
func (*NetApiStruct) NetFdbDel(fm *cmn.FdbMod) (int, error) {
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeId}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbDel(fdbKey)
	return ret, err
}

// Add an ipv4 route in loxinet
func (*NetApiStruct) NetRoutev4Add(rm *cmn.Routev4Mod) (int, error) {
	var ret int
	var err error

	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ra := RtAttr{rm.Protocol, rm.Flags, false}
	if rm.Gw != nil {
		na := []RtNhAttr{{rm.Gw, rm.LinkIndex}}
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, ROOT_ZONE, ra, na)
	} else {
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, ROOT_ZONE, ra, nil)
	}

	return ret, err
}

// Delete an ipv4 route in loxinet
func (*NetApiStruct) NetRoutev4Del(rm *cmn.Routev4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rt.RtDelete(rm.Dst, ROOT_ZONE)
	return ret, err
}

// Add a load-balancer rule in loxinet
func (*NetApiStruct) NetLbRuleAdd(lm *cmn.LbRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.AddNatLbRule(lm.Serv, lm.Eps[:])
	if err == nil && lm.Serv.Bgp {
		if mh.bgp != nil {
			mh.bgp.AddBGPRule(lm.Serv.ServIP)
		} else {
			tk.LogIt(tk.LOG_DEBUG, "loxilb BGP mode is disable \n")
		}
	}
	return ret, err
}

// Delete a load-balancer rule in loxinet
func (*NetApiStruct) NetLbRuleDel(lm *cmn.LbRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteNatLbRule(lm.Serv)
	if lm.Serv.Bgp {
		if mh.bgp != nil {
			mh.bgp.DelBGPRule(lm.Serv.ServIP)
		} else {
			tk.LogIt(tk.LOG_DEBUG, "loxilb BGP mode is disable \n")
		}
	}
	return ret, err
}

// Get a load-balancer rule from loxinet
func (*NetApiStruct) NetLbRuleGet() ([]cmn.LbRuleMod, error) {
	ret, err := mh.zr.Rules.GetNatLbRule()
	return ret, err
}

// Get connection track info from loxinet
func (*NetApiStruct) NetCtInfoGet() ([]cmn.CtInfo, error) {
	// There is no locking requirement for this operation
	ret := mh.dp.DpMapGetCt4()
	return ret, nil
}

// Add a 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionAdd(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessAdd(sm.Ident, sm.Ip, sm.AnTun, sm.CnTun)
	return ret, err
}

// Delete a 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionDel(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessDelete(sm.Ident)
	return ret, err
}

// Add a 3gpp ulcl-filter info in loxinet
func (*NetApiStruct) NetSessionUlClAdd(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClAddCls(sr.Ident, sr.Args)
	return ret, err
}

// Delete a 3gpp ulcl-filter info in loxinet
func (*NetApiStruct) NetSessionUlClDel(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClDeleteCls(sr.Ident, sr.Args)
	return ret, err
}

// Get 3gpp user-session info in loxinet
func (*NetApiStruct) NetSessionGet() ([]cmn.SessionMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessGet()
	return ret, err
}

// Get 3gpp ulcl filter info from loxinet
func (*NetApiStruct) NetSessionUlClGet() ([]cmn.SessionUlClMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessUlclGet()
	return ret, err
}
