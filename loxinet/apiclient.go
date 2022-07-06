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
    cmn "loxilb/common"
)

type NetApiStruct struct {
}

func NetApiInit() *NetApiStruct {
    na := new(NetApiStruct)
    return na
}

func (*NetApiStruct) NetPortAdd(pm *cmn.PortMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Ports.PortAdd(pm.Dev, pm.LinkIndex, pm.Ptype, ROOT_ZONE,
        PortHwInfo{pm.MacAddr, pm.Link, pm.State, pm.Mtu, pm.Master, pm.Real,
            uint32(pm.TunId)}, PortLayer2Info{false, 0})
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetPortDel(pm *cmn.PortMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Ports.PortDel(pm.Dev, pm.Ptype)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetVlanAdd(vm *cmn.VlanMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Vlans.VlanAdd(vm.Vid, vm.Dev, ROOT_ZONE, vm.LinkIndex,
        PortHwInfo{vm.MacAddr, vm.Link, vm.State, vm.Mtu, "", "", vm.TunId})
    if ret == VLAN_EXISTS_ERR {
        ret = 0
    }
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetVlanDel(vm *cmn.VlanMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Vlans.VlanDelete(vm.Vid)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetVlanPortAdd(vm *cmn.VlanPortMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Vlans.VlanPortAdd(vm.Vid, vm.Dev, vm.Tagged)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetVlanPortDel(vm *cmn.VlanPortMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Vlans.VlanPortDelete(vm.Vid, vm.Dev, vm.Tagged)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetIpv4AddrAdd(am *cmn.Ipv4AddrMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.L3.IfaAdd(am.Dev, am.Ip)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetIpv4AddrDel(am *cmn.Ipv4AddrMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.L3.IfaDelete(am.Dev, am.Ip)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetNeighv4Add(nm *cmn.Neighv4Mod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Nh.NeighAdd(nm.Ip, ROOT_ZONE, NeighAttr{nm.LinkIndex, nm.State, nm.HardwareAddr})
    if err != nil {
        if ret != NEIGH_EXISTS_ERR {
            return ret, err
        }
    }
    mh.mtx.Unlock()
    return 0, nil
}

func (*NetApiStruct) NetNeighv4Del(nm *cmn.Neighv4Mod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Nh.NeighDelete(nm.Ip, ROOT_ZONE)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetFdbAdd(fm *cmn.FdbMod) (int, error) {
    fdbKey := FdbKey{fm.MacAddr, fm.BridgeId}
    fdbAttr := FdbAttr{fm.Dev, fm.Dst, fm.Type}
    mh.mtx.Lock()
    ret, err := mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetFdbDel(fm *cmn.FdbMod) (int, error) {
    fdbKey := FdbKey{fm.MacAddr, fm.BridgeId}
    mh.mtx.Lock()
    ret, err := mh.zr.L2.L2FdbDel(fdbKey)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetRoutev4Add(rm *cmn.Routev4Mod) (int, error) {
    mh.mtx.Lock()
    ra := RtAttr{rm.Protocol, rm.Flags, false}
    na := []RtNhAttr{{rm.Gw, rm.LinkIndex}}
    ret, err := mh.zr.Rt.RtAdd(rm.Dst, ROOT_ZONE, ra, na)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetRoutev4Del(rm *cmn.Routev4Mod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Rt.RtDelete(rm.Dst, ROOT_ZONE)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetLbRuleAdd(lm *cmn.LbRuleMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Rules.AddNatLbRule(lm.Serv, lm.Eps[:])
    if err != nil {
        AddBGPRule(lm.Serv.ServIP)
    }
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetLbRuleDel(lm *cmn.LbRuleMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Rules.DeleteNatLbRule(lm.Serv)
    DelBGPRule(lm.Serv.ServIP)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetLbRuleGet() ([]cmn.LbRuleMod, error) {
    ret, err := mh.zr.Rules.GetNatLbRule()
    return ret, err
}

func (*NetApiStruct) NetCtInfoGet() ([]cmn.CtInfo, error) {
    // There is no locking requirement for this operation
    ret := mh.dp.DpMapGetCt4()
    return ret, nil
}

func (*NetApiStruct) NetSessionAdd(sm *cmn.SessionMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Sess.SessAdd(sm.Ident, sm.Ip, sm.AnTun, sm.CnTun)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetSessionDel(sm *cmn.SessionMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Sess.SessDelete(sm.Ident)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetSessionUlClAdd(sr *cmn.SessionUlClMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Sess.UlClAddCls(sr.Ident, sr.Args)
    mh.mtx.Unlock()
    return ret, err
}

func (*NetApiStruct) NetSessionUlClDel(sr *cmn.SessionUlClMod) (int, error) {
    mh.mtx.Lock()
    ret, err := mh.zr.Sess.UlClDeleteCls(sr.Ident, sr.Args)
    mh.mtx.Unlock()
    return ret, err
}
