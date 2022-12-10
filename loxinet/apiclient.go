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

// NetAPIStruct - empty struct for anchoring client routines
type NetAPIStruct struct {
}

// NetAPIInit - Initialize a new instance of NetAPI
func NetAPIInit() *NetAPIStruct {
	na := new(NetAPIStruct)
	return na
}

// NetMirrorGet - Get a mirror in loxinet
func (*NetAPIStruct) NetMirrorGet() ([]cmn.MirrGetMod, error) {
	// There is no locking requirement for this operation
	ret, _ := mh.zr.Mirrs.MirrGet()
	return ret, nil

}

// NetMirrorAdd - Add a mirror in loxinet
func (*NetAPIStruct) NetMirrorAdd(mm *cmn.MirrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Mirrs.MirrAdd(mm.Ident, mm.Info, mm.Target)
	return ret, err
}

// NetMirrorDel - Delete a mirror in loxinet
func (*NetAPIStruct) NetMirrorDel(mm *cmn.MirrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Mirrs.MirrDelete(mm.Ident)
	return ret, err
}

// NetPortGet - Get Port Information of loxinet
func (*NetAPIStruct) NetPortGet() ([]cmn.PortDump, error) {
	ret, err := mh.zr.Ports.PortsToGet()
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// NetPortAdd - Add a port in loxinet
func (*NetAPIStruct) NetPortAdd(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortAdd(pm.Dev, pm.LinkIndex, pm.Ptype, RootZone,
		PortHwInfo{pm.MacAddr, pm.Link, pm.State, pm.Mtu, pm.Master, pm.Real,
			uint32(pm.TunID)}, PortLayer2Info{false, 0})

	return ret, err
}

// NetPortDel - Delete port from loxinet
func (*NetAPIStruct) NetPortDel(pm *cmn.PortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortDel(pm.Dev, pm.Ptype)
	return ret, err
}

// NetVlanGet - Get Vlan Information of loxinet
func (*NetAPIStruct) NetVlanGet() ([]cmn.VlanGet, error) {
	ret, err := mh.zr.Vlans.VlanGet()
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// NetVlanAdd - Add vlan info to loxinet
func (*NetAPIStruct) NetVlanAdd(vm *cmn.VlanMod) (int, error) {
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
func (*NetAPIStruct) NetVlanDel(vm *cmn.VlanMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanDelete(vm.Vid)
	return ret, err
}

// NetVlanPortAdd - Add a port to vlan in loxinet
func (*NetAPIStruct) NetVlanPortAdd(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortAdd(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetVlanPortDel - Delete a port from vlan in loxinet
func (*NetAPIStruct) NetVlanPortDel(vm *cmn.VlanPortMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortDelete(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetIpv4AddrGet - Get an IPv4 Address info from loxinet
func (*NetAPIStruct) NetIpv4AddrGet() ([]cmn.Ipv4AddrGet, error) {
	// There is no locking requirement for this operation
	ret := mh.zr.L3.IfaGet()
	return ret, nil
}

// NetIpv4AddrAdd - Add an ipv4 address in loxinet
func (*NetAPIStruct) NetIpv4AddrAdd(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaAdd(am.Dev, am.IP)
	return ret, err
}

// NetIpv4AddrDel - Delete an ipv4 address in loxinet
func (*NetAPIStruct) NetIpv4AddrDel(am *cmn.Ipv4AddrMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaDelete(am.Dev, am.IP)
	return ret, err
}

// NetNeighv4Get - Get a ipv4 neighbor in loxinet
func (*NetAPIStruct) NetNeighv4Get() ([]cmn.Neighv4Mod, error) {
	ret, err := mh.zr.Nh.NeighGet()
	return ret, err
}

// NetNeighv4Add - Add a ipv4 neighbor in loxinet
func (*NetAPIStruct) NetNeighv4Add(nm *cmn.Neighv4Mod) (int, error) {
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
func (*NetAPIStruct) NetNeighv4Del(nm *cmn.Neighv4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighDelete(nm.IP, RootZone)
	return ret, err
}

// NetFdbAdd - Add a forwarding database entry in loxinet
func (*NetAPIStruct) NetFdbAdd(fm *cmn.FdbMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	fdbAttr := FdbAttr{fm.Dev, fm.Dst, fm.Type}
	ret, err := mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	return ret, err
}

// NetFdbDel - Delete a forwarding database entry in loxinet
func (*NetAPIStruct) NetFdbDel(fm *cmn.FdbMod) (int, error) {
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbDel(fdbKey)
	return ret, err
}

// NetRoutev4Get - Get Route info from loxinet
func (*NetAPIStruct) NetRoutev4Get() ([]cmn.Routev4Get, error) {
	// There is no locking requirement for this operation
	ret, _ := mh.zr.Rt.RouteGet()
	return ret, nil
}

// NetRoutev4Add - Add an ipv4 route in loxinet
func (*NetAPIStruct) NetRoutev4Add(rm *cmn.Routev4Mod) (int, error) {
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
func (*NetAPIStruct) NetRoutev4Del(rm *cmn.Routev4Mod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rt.RtDelete(rm.Dst, RootZone)
	return ret, err
}

// NetLbRuleAdd - Add a load-balancer rule in loxinet
func (*NetAPIStruct) NetLbRuleAdd(lm *cmn.LbRuleMod) (int, error) {
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
func (*NetAPIStruct) NetLbRuleDel(lm *cmn.LbRuleMod) (int, error) {
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
func (*NetAPIStruct) NetLbRuleGet() ([]cmn.LbRuleMod, error) {
	ret, err := mh.zr.Rules.GetNatLbRule()
	return ret, err
}

// NetCtInfoGet - Get connection track info from loxinet
func (*NetAPIStruct) NetCtInfoGet() ([]cmn.CtInfo, error) {
	// There is no locking requirement for this operation
	ret := mh.dp.DpMapGetCt4()
	return ret, nil
}

// NetSessionAdd - Add a 3gpp user-session info in loxinet
func (*NetAPIStruct) NetSessionAdd(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessAdd(sm.Ident, sm.IP, sm.AnTun, sm.CnTun)
	return ret, err
}

// NetSessionDel - Delete a 3gpp user-session info in loxinet
func (*NetAPIStruct) NetSessionDel(sm *cmn.SessionMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessDelete(sm.Ident)
	return ret, err
}

// NetSessionUlClAdd - Add a 3gpp ulcl-filter info in loxinet
func (*NetAPIStruct) NetSessionUlClAdd(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClAddCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionUlClDel - Delete a 3gpp ulcl-filter info in loxinet
func (*NetAPIStruct) NetSessionUlClDel(sr *cmn.SessionUlClMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClDeleteCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionGet - Get 3gpp user-session info in loxinet
func (*NetAPIStruct) NetSessionGet() ([]cmn.SessionMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessGet()
	return ret, err
}

// NetSessionUlClGet - Get 3gpp ulcl filter info from loxinet
func (*NetAPIStruct) NetSessionUlClGet() ([]cmn.SessionUlClMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessUlclGet()
	return ret, err
}

// NetPolicerGet - Get a policer in loxinet
func (*NetAPIStruct) NetPolicerGet() ([]cmn.PolMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.zr.Pols.PolGetAll()
	return ret, err
}

// NetPolicerAdd - Add a policer in loxinet
func (*NetAPIStruct) NetPolicerAdd(pm *cmn.PolMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolAdd(pm.Ident, pm.Info, pm.Target)
	return ret, err
}

// NetPolicerDel - Delete a policer in loxinet
func (*NetAPIStruct) NetPolicerDel(pm *cmn.PolMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolDelete(pm.Ident)
	return ret, err
}

// NetCIStateGet - Get current node cluster state
func (*NetAPIStruct) NetCIStateGet() ([]cmn.HASMod, error) {
	// There is no locking requirement for this operation
	ret, err := mh.has.CIStateGet()
	return ret, err
}

// NetCIStateMod - Modify cluster state
func (*NetAPIStruct) NetCIStateMod(hm *cmn.HASMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	state, err := mh.has.CIStateUpdate(*hm)
	if err != nil {
		return -1, err
	}

	if mh.bgp != nil {
		mh.bgp.UpdateCIState(state)
	}
	return 0, nil
}

// NetFwRuleAdd - Add a firewall rule in loxinet
func (*NetAPIStruct) NetFwRuleAdd(fm *cmn.FwRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.AddFwRule(fm.Rule, fm.Opts)
	return ret, err
}

// NetFwRuleDel - Delete a firewall rule in loxinet
func (*NetAPIStruct) NetFwRuleDel(fm *cmn.FwRuleMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteFwRule(fm.Rule)
	return ret, err
}

// NetFwRuleGet - Get a firewall rule from loxinet
func (*NetAPIStruct) NetFwRuleGet() ([]cmn.FwRuleMod, error) {
	ret, err := mh.zr.Rules.GetFwRule()
	return ret, err
}

// NetEpHostAdd - Add a LB end-point in loxinet
func (*NetAPIStruct) NetEpHostAdd(em *cmn.EndPointMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	epArgs := epHostOpts{inActTryThr: em.InActTries, probeType: em.ProbeType,
		probeReq: em.ProbeReq, probeResp: em.ProbeResp,
		probeDuration: em.ProbeDuration, probePort: em.ProbePort,
	}

	ret, err := mh.zr.Rules.AddEpHost(true, em.Name, em.Desc, epArgs)
	return ret, err
}

// NetEpHostDel - Delete a LB end-point in loxinet
func (*NetAPIStruct) NetEpHostDel(fm *cmn.EndPointMod) (int, error) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteEpHost(true, fm.Name)
	return ret, err
}

// NetEpHostGet - Get LB end-points from loxinet
func (*NetAPIStruct) NetEpHostGet() ([]cmn.EndPointMod, error) {
	ret, err := mh.zr.Rules.GetEpHosts()
	return ret, err
}
