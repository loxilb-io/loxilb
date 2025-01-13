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
	"errors"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// This file implements interface defined in cmn.NetHookInterface
// The implementation is thread-safe and can be called by multiple-clients at once

// NetAPIStruct - empty struct for anchoring client routines
type NetAPIStruct struct {
	BgpPeerMode bool
}

// NetAPIInit - Initialize a new instance of NetAPI
func NetAPIInit(bgpPeerMode bool) *NetAPIStruct {
	na := new(NetAPIStruct)
	na.BgpPeerMode = bgpPeerMode
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
func (na *NetAPIStruct) NetPortAdd(pm *cmn.PortMod) (int, error) {
	if na.BgpPeerMode {
		return PortBaseErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortAdd(pm.Dev, pm.LinkIndex, pm.Ptype, RootZone,
		PortHwInfo{pm.MacAddr, pm.Link, pm.State, pm.Mtu, pm.Master, pm.Real,
			uint32(pm.TunID), pm.TunSrc, pm.TunDst}, PortLayer2Info{false, 0})

	return ret, err
}

// NetPortDel - Delete port from loxinet
func (na *NetAPIStruct) NetPortDel(pm *cmn.PortMod) (int, error) {
	if na.BgpPeerMode {
		return PortBaseErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Ports.PortDel(pm.Dev, pm.Ptype)
	return ret, err
}

// NetVlanGet - Get Vlan Information of loxinet
func (na *NetAPIStruct) NetVlanGet() ([]cmn.VlanGet, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	ret, err := mh.zr.Vlans.VlanGet()
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// NetVlanAdd - Add vlan info to loxinet
func (na *NetAPIStruct) NetVlanAdd(vm *cmn.VlanMod) (int, error) {
	if na.BgpPeerMode {
		return VlanBaseErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanAdd(vm.Vid, vm.Dev, RootZone, vm.LinkIndex,
		PortHwInfo{vm.MacAddr, vm.Link, vm.State, vm.Mtu, "", "", vm.TunID, nil, nil})
	if ret == VlanExistsErr {
		ret = 0
		err = nil
	}

	return ret, err
}

// NetVlanDel - Delete vlan info from loxinet
func (na *NetAPIStruct) NetVlanDel(vm *cmn.VlanMod) (int, error) {
	if na.BgpPeerMode {
		return VlanBaseErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanDelete(vm.Vid)
	return ret, err
}

// NetVlanPortAdd - Add a port to vlan in loxinet
func (na *NetAPIStruct) NetVlanPortAdd(vm *cmn.VlanPortMod) (int, error) {
	if na.BgpPeerMode {
		return VlanPortCreateErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortAdd(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetVlanPortDel - Delete a port from vlan in loxinet
func (na *NetAPIStruct) NetVlanPortDel(vm *cmn.VlanPortMod) (int, error) {
	if na.BgpPeerMode {
		return VlanPortExistErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Vlans.VlanPortDelete(vm.Vid, vm.Dev, vm.Tagged)
	return ret, err
}

// NetAddrGet - Get an IPv4 Address info from loxinet
func (na *NetAPIStruct) NetAddrGet() ([]cmn.IPAddrGet, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret := mh.zr.L3.IfaGet()
	return ret, nil
}

// NetAddrAdd - Add an ipv4 address in loxinet
func (na *NetAPIStruct) NetAddrAdd(am *cmn.IPAddrMod) (int, error) {
	if na.BgpPeerMode {
		return L3AddrErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaAdd(am.Dev, am.IP)
	return ret, err
}

// NetAddrDel - Delete an ipv4 address in loxinet
func (na *NetAPIStruct) NetAddrDel(am *cmn.IPAddrMod) (int, error) {
	if na.BgpPeerMode {
		return L3AddrErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L3.IfaDelete(am.Dev, am.IP)
	return ret, err
}

// NetNeighGet - Get a neighbor in loxinet
func (na *NetAPIStruct) NetNeighGet() ([]cmn.NeighMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	ret, err := mh.zr.Nh.NeighGet()
	return ret, err
}

// NetNeighAdd - Add a neighbor in loxinet
func (na *NetAPIStruct) NetNeighAdd(nm *cmn.NeighMod) (int, error) {
	if na.BgpPeerMode {
		return NeighErrBase, errors.New("running in bgp only mode")
	}
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

// NetNeighDel - Delete a neighbor in loxinet
func (na *NetAPIStruct) NetNeighDel(nm *cmn.NeighMod) (int, error) {
	if na.BgpPeerMode {
		return NeighErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Nh.NeighDelete(nm.IP, RootZone, nm.LinkIndex)
	return ret, err
}

// NetFdbAdd - Add a forwarding database entry in loxinet
func (na *NetAPIStruct) NetFdbAdd(fm *cmn.FdbMod) (int, error) {
	if na.BgpPeerMode {
		return L2ErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	fdbAttr := FdbAttr{fm.Dev, fm.Dst, fm.Type}
	ret, err := mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
	return ret, err
}

// NetFdbDel - Delete a forwarding database entry in loxinet
func (na *NetAPIStruct) NetFdbDel(fm *cmn.FdbMod) (int, error) {
	if na.BgpPeerMode {
		return L2ErrBase, errors.New("running in bgp only mode")
	}

	fdbKey := FdbKey{fm.MacAddr, fm.BridgeID}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.L2.L2FdbDel(fdbKey)
	return ret, err
}

// NetRouteGet - Get Route info from loxinet
func (na *NetAPIStruct) NetRouteGet() ([]cmn.RouteGet, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret, _ := mh.zr.Rt.RouteGet()
	return ret, nil
}

// NetRouteAdd - Add a route in loxinet
func (na *NetAPIStruct) NetRouteAdd(rm *cmn.RouteMod) (int, error) {
	var ret int
	var err error

	if len(rm.GWs) <= 0 {
		return RtNhErr, errors.New("invalid gws")
	}
	if na.BgpPeerMode {
		return RtNhErr, errors.New("running in bgp only mode")
	}
	intfRt := false
	mlen, _ := rm.Dst.Mask.Size()
	if rm.GWs[0].Gw == nil {
		// This is an interface route
		if (tk.IsNetIPv4(rm.Dst.IP.String()) && mlen == 32) || (tk.IsNetIPv6(rm.Dst.IP.String()) && mlen == 128) {
			intfRt = true
			rm.GWs[0].Gw = rm.Dst.IP
		}
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ra := RtAttr{Protocol: rm.Protocol, OSFlags: rm.Flags, HostRoute: false, Ifi: rm.GWs[0].LinkIndex, IfRoute: intfRt}
	if rm.GWs[0].Gw != nil {
		var na []RtNhAttr
		for _, gw := range rm.GWs {
			na = append(na, RtNhAttr{gw.Gw, gw.LinkIndex})
		}
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, RootZone, ra, na)

	} else {
		ret, err = mh.zr.Rt.RtAdd(rm.Dst, RootZone, ra, nil)
	}
	return ret, err
}

// NetRouteDel - Delete a route in loxinet
func (na *NetAPIStruct) NetRouteDel(rm *cmn.RouteMod) (int, error) {
	if na.BgpPeerMode {
		return RtNhErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rt.RtDelete(rm.Dst, RootZone)
	return ret, err
}

// NetLbRuleAdd - Add a load-balancer rule in loxinet
func (na *NetAPIStruct) NetLbRuleAdd(lm *cmn.LbRuleMod) (int, error) {
	if na.BgpPeerMode {
		return RuleErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	var ips []string
	ret, err := mh.zr.Rules.AddLbRule(lm.Serv, lm.SecIPs[:], lm.SrcIPs[:], lm.Eps[:])
	if err == nil && lm.Serv.Bgp {
		if mh.bgp != nil {
			ips = append(ips, lm.Serv.ServIP)
			for _, ip := range lm.SecIPs {
				ips = append(ips, ip.SecIP)
			}
			mh.bgp.AddBGPRule(cmn.CIDefault, ips)
		} else {
			tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
		}
	}
	return ret, err
}

// NetLbRuleDel - Delete a load-balancer rule in loxinet
func (na *NetAPIStruct) NetLbRuleDel(lm *cmn.LbRuleMod) (int, error) {
	if na.BgpPeerMode {
		return RuleErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ips := mh.zr.Rules.GetLBRuleSecIPs(lm.Serv)
	ret, err := mh.zr.Rules.DeleteLbRule(lm.Serv)
	if lm.Serv.Bgp {
		if mh.bgp != nil {
			ips = append(ips, lm.Serv.ServIP)
			mh.bgp.DelBGPRule(cmn.CIDefault, ips)
		} else {
			tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
		}
	}
	return ret, err
}

// NetLbRuleGet - Get a load-balancer rule from loxinet
func (na *NetAPIStruct) NetLbRuleGet() ([]cmn.LbRuleMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	ret, err := mh.zr.Rules.GetLBRule()
	return ret, err
}

// NetCtInfoGet - Get connection track info from loxinet
func (na *NetAPIStruct) NetCtInfoGet() ([]cmn.CtInfo, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret := mh.dp.DpMapGetCt4()
	return ret, nil
}

// NetSessionAdd - Add a 3gpp user-session info in loxinet
func (na *NetAPIStruct) NetSessionAdd(sm *cmn.SessionMod) (int, error) {
	if na.BgpPeerMode {
		return SessErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessAdd(sm.Ident, sm.IP, sm.AnTun, sm.CnTun)
	return ret, err
}

// NetSessionDel - Delete a 3gpp user-session info in loxinet
func (na *NetAPIStruct) NetSessionDel(sm *cmn.SessionMod) (int, error) {
	if na.BgpPeerMode {
		return SessErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.SessDelete(sm.Ident)
	return ret, err
}

// NetSessionUlClAdd - Add a 3gpp ulcl-filter info in loxinet
func (na *NetAPIStruct) NetSessionUlClAdd(sr *cmn.SessionUlClMod) (int, error) {
	if na.BgpPeerMode {
		return SessErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClAddCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionUlClDel - Delete a 3gpp ulcl-filter info in loxinet
func (na *NetAPIStruct) NetSessionUlClDel(sr *cmn.SessionUlClMod) (int, error) {
	if na.BgpPeerMode {
		return SessErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Sess.UlClDeleteCls(sr.Ident, sr.Args)
	return ret, err
}

// NetSessionGet - Get 3gpp user-session info in loxinet
func (na *NetAPIStruct) NetSessionGet() ([]cmn.SessionMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessGet()
	return ret, err
}

// NetSessionUlClGet - Get 3gpp ulcl filter info from loxinet
func (na *NetAPIStruct) NetSessionUlClGet() ([]cmn.SessionUlClMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret, err := mh.zr.Sess.SessUlclGet()
	return ret, err
}

// NetPolicerGet - Get a policer in loxinet
func (na *NetAPIStruct) NetPolicerGet() ([]cmn.PolMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret, err := mh.zr.Pols.PolGetAll()
	return ret, err
}

// NetPolicerAdd - Add a policer in loxinet
func (na *NetAPIStruct) NetPolicerAdd(pm *cmn.PolMod) (int, error) {
	if na.BgpPeerMode {
		return PolInfoErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolAdd(pm.Ident, pm.Info, pm.Target)
	return ret, err
}

// NetPolicerDel - Delete a policer in loxinet
func (na *NetAPIStruct) NetPolicerDel(pm *cmn.PolMod) (int, error) {
	if na.BgpPeerMode {
		return PolInfoErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Pols.PolDelete(pm.Ident)
	return ret, err
}

// NetCIStateGet - Get current node cluster state
func (na *NetAPIStruct) NetCIStateGet() ([]cmn.HASMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	// There is no locking requirement for this operation
	ret, err := mh.has.CIStateGet()
	return ret, err
}

// NetCIStateMod - Modify cluster state
func (na *NetAPIStruct) NetCIStateMod(hm *cmn.HASMod) (int, error) {
	if na.BgpPeerMode {
		return CIErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	_, err := mh.has.CIStateUpdate(*hm)
	if err != nil {
		return -1, err
	}

	return 0, nil
}

// NetCIStateMod - Modify cluster state
func (na *NetAPIStruct) NetBFDGet() ([]cmn.BFDMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	return mh.has.CIBFDSessionGet()
}

// NetBFDAdd - Add BFD Session
func (na *NetAPIStruct) NetBFDAdd(bm *cmn.BFDMod) (int, error) {
	if na.BgpPeerMode {
		return CIErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	_, err := mh.has.CIBFDSessionAdd(*bm)
	if err != nil {
		return -1, err
	}

	return 0, nil
}

// NetBFDDel - Delete BFD Session
func (na *NetAPIStruct) NetBFDDel(bm *cmn.BFDMod) (int, error) {
	if na.BgpPeerMode {
		return CIErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	_, err := mh.has.CIBFDSessionDel(*bm)
	if err != nil {
		return -1, err
	}

	return 0, nil
}

// NetFwRuleAdd - Add a firewall rule in loxinet
func (na *NetAPIStruct) NetFwRuleAdd(fm *cmn.FwRuleMod) (int, error) {
	if na.BgpPeerMode {
		return RuleTupleErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.AddFwRule(fm.Rule, fm.Opts)
	return ret, err
}

// NetFwRuleDel - Delete a firewall rule in loxinet
func (na *NetAPIStruct) NetFwRuleDel(fm *cmn.FwRuleMod) (int, error) {
	if na.BgpPeerMode {
		return RuleTupleErr, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteFwRule(fm.Rule)
	return ret, err
}

// NetFwRuleGet - Get a firewall rule from loxinet
func (na *NetAPIStruct) NetFwRuleGet() ([]cmn.FwRuleMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	ret, err := mh.zr.Rules.GetFwRule()
	return ret, err
}

// NetEpHostAdd - Add a LB end-point in loxinet
func (na *NetAPIStruct) NetEpHostAdd(em *cmn.EndPointMod) (int, error) {
	if na.BgpPeerMode {
		return RuleErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	epArgs := epHostOpts{inActTryThr: em.InActTries, probeType: em.ProbeType,
		probeReq: em.ProbeReq, probeResp: em.ProbeResp,
		probeDuration: em.ProbeDuration, probePort: em.ProbePort,
	}
	ret, err := mh.zr.Rules.AddEPHost(true, em.HostName, em.Name, epArgs)
	return ret, err
}

// NetEpHostDel - Delete a LB end-point in loxinet
func (na *NetAPIStruct) NetEpHostDel(em *cmn.EndPointMod) (int, error) {
	if na.BgpPeerMode {
		return RuleErrBase, errors.New("running in bgp only mode")
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	ret, err := mh.zr.Rules.DeleteEPHost(true, em.Name, em.HostName, em.ProbeType, em.ProbePort)
	return ret, err
}

// NetEpHostGet - Get LB end-points from loxinet
func (na *NetAPIStruct) NetEpHostGet() ([]cmn.EndPointMod, error) {
	if na.BgpPeerMode {
		return nil, errors.New("running in bgp only mode")
	}
	ret, err := mh.zr.Rules.GetEpHosts()
	return ret, err
}

// NetParamSet - Set operational params of loxinet
func (na *NetAPIStruct) NetParamSet(param cmn.ParamMod) (int, error) {
	if na.BgpPeerMode {
		return 0, errors.New("running in bgp only mode")
	}
	ret, err := mh.ParamSet(param)
	return ret, err
}

// NetParamGet - Get operational params of loxinet
func (na *NetAPIStruct) NetParamGet(param *cmn.ParamMod) (int, error) {
	if na.BgpPeerMode {
		return 0, errors.New("running in bgp only mode")
	}
	ret, err := mh.ParamGet(param)
	return ret, err
}

// NetGoBGPNeighGet - Get bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPNeighGet() ([]cmn.GoBGPNeighGetMod, error) {
	if mh.bgp != nil {
		a, err := mh.bgp.BGPNeighGet("", false)
		if err != nil {
			return nil, err
		}
		return a, nil
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return nil, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPNeighAdd - Add bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPNeighAdd(param *cmn.GoBGPNeighMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.BGPNeighMod(true, param.Addr, param.RemoteAS, uint32(param.RemotePort), param.MultiHop)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPNeighDel - Del bgp neigh from gobgp
func (na *NetAPIStruct) NetGoBGPNeighDel(param *cmn.GoBGPNeighMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.BGPNeighMod(false, param.Addr, param.RemoteAS, uint32(param.RemotePort), param.MultiHop)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")
}

// NetGoBGPGCAdd - Add bgp global config
func (na *NetAPIStruct) NetGoBGPGCAdd(param *cmn.GoBGPGlobalConfig) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.BGPGlobalConfigAdd(*param)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetHandlePanic - Handle panics
func (na *NetAPIStruct) NetHandlePanic() {
	mh.dp.DpHooks.DpEbpfUnInit()
}

func (na *NetAPIStruct) NetGoBGPPolicyDefinedSetGet(name string, DefinedTypeString string) ([]cmn.GoBGPPolicyDefinedSetMod, error) {
	if mh.bgp != nil {
		a, err := mh.bgp.GetPolicyDefinedSet(name, DefinedTypeString)
		if err != nil {
			return nil, err
		}
		return a, nil
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return nil, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyPrefixAdd - Add Prefixset in bgp
func (na *NetAPIStruct) NetGoBGPPolicyDefinedSetAdd(param *cmn.GoBGPPolicyDefinedSetMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.AddPolicyDefinedSets(*param)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyPrefixAdd - Add Prefixset in bgp
func (na *NetAPIStruct) NetGoBGPPolicyDefinedSetDel(param *cmn.GoBGPPolicyDefinedSetMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.DelPolicyDefinedSets(param.Name, param.DefinedTypeString)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyDefinitionsGet - Add bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPPolicyDefinitionsGet() ([]cmn.GoBGPPolicyDefinitionsMod, error) {
	if mh.bgp != nil {
		a, err := mh.bgp.GetPolicyDefinitions()
		if err != nil {
			return nil, err
		}
		return a, nil
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return nil, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyNeighAdd - Add bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPPolicyDefinitionAdd(param *cmn.GoBGPPolicyDefinitionsMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.AddPolicyDefinitions(param.Name, param.Statement)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyNeighAdd - Add bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPPolicyDefinitionDel(param *cmn.GoBGPPolicyDefinitionsMod) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.DelPolicyDefinitions(param.Name)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyApplyAdd - Add bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPPolicyApplyAdd(param *cmn.GoBGPPolicyApply) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.BGPApplyPolicyToNeighbor("add", param.NeighIPAddress, param.PolicyType, param.Polices, param.RouteAction)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}

// NetGoBGPPolicyApplyDel - Del bgp neigh to gobgp
func (na *NetAPIStruct) NetGoBGPPolicyApplyDel(param *cmn.GoBGPPolicyApply) (int, error) {
	if mh.bgp != nil {
		return mh.bgp.BGPApplyPolicyToNeighbor("del", param.NeighIPAddress, param.PolicyType, param.Polices, param.RouteAction)
	}
	tk.LogIt(tk.LogDebug, "loxilb BGP mode is disabled \n")
	return 0, errors.New("loxilb BGP mode is disabled")

}
