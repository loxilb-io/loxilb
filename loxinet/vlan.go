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
    "fmt"
    cmn "loxilb/common"
    tk "loxilb/loxilib"
    "strings"
)

const (
    VLAN_BASE_ERR = iota - 2000
    VLAN_EXISTS_ERR
    VLAN_NOTEXIST_ERR
    VLAN_RANGE_ERR
    VLAN_ADDBRP_ERR
    VLAN_MPEXIST_ERR
    VLAN_PORTPHY_ERR
    VLAN_PORTEXIST_ERR
    VLAN_PORT_TAGGED_ERR
    VLAN_NOPORT_ERR
    VLAN_PORTCREATE_ERR
    VLAN_ZONE_ERR
)

const (
    MAX_VLANS = 4094
)

type vlanStat struct {
    inBytes    uint64
    inPackets  uint64
    outBytes   uint64
    outPackets uint64
}

type Vlan struct {
    VlanID        int
    Created       bool
    Name          string
    Zone          string
    NumTagPorts   int
    TaggedPorts   [MAX_IFS]*Port
    NumUnTagPorts int
    UnTaggedPorts [MAX_IFS]*Port
    Stat          vlanStat
}

type VlansH struct {
    VlanMap [MAX_VLANS]Vlan
    Zone    *Zone
}

func VlanInit(zone *Zone) *VlansH {
    var nV = new(VlansH)
    nV.Zone = zone
    return nV
}

func VlanValid(vlanId int) bool {
    if vlanId > 0 && vlanId < MAX_VLANS-1 {
        return true
    }
    return false
}

func (V *VlansH) VlanAdd(vlanID int, name string, zone string, osid int, hwi PortHwInfo) (int, error) {
    if VlanValid(vlanID) == false {
        return VLAN_RANGE_ERR, errors.New("Invalid VlanID")
    }

    if V.VlanMap[vlanID].Created == true {
        return VLAN_EXISTS_ERR, errors.New("Vlan already created")
    }

    _, err := mh.zn.ZoneBrAdd(name, zone)
    if err != nil {
        return VLAN_EXISTS_ERR, errors.New("Vlan zone err")
    }

    ret, err := V.Zone.Ports.PortAdd(name, osid, cmn.PORT_VLANBR, zone, hwi, PortLayer2Info{false, vlanID})
    if err != nil || ret != 0 {
        tk.LogIt(tk.LOG_ERROR, "Vlan bridge interface not created %d\n", ret)
        mh.zn.ZoneBrDelete(name)
        return VLAN_ADDBRP_ERR, errors.New("Can't add vlan bridge")
    }

    v := &V.VlanMap[vlanID]
    v.Name = name
    v.VlanID = vlanID
    v.Created = true
    v.Zone = zone

    tk.LogIt(tk.LOG_INFO, "vlan %d bd created\n", vlanID)

    return 0, nil
}

func (V *VlansH) VlanDelete(vlanID int) (int, error) {
    if VlanValid(vlanID) == false {
        return VLAN_RANGE_ERR, errors.New("Invalid VlanID")
    }

    if V.VlanMap[vlanID].Created == false {
        return VLAN_NOTEXIST_ERR, errors.New("Vlan not yet created")
    }

    if V.VlanMap[vlanID].NumTagPorts != 0 ||
        V.VlanMap[vlanID].NumUnTagPorts != 0 {
        return VLAN_MPEXIST_ERR, errors.New("Vlan has ports configured")
    }

    v := &V.VlanMap[vlanID]
    mh.zn.ZoneBrDelete(v.Name)

    V.Zone.Ports.PortDel(v.Name, cmn.PORT_VLANBR)
    v.DP(DP_STATS_CLR)

    v.Name = ""
    v.VlanID = 0
    v.Created = false
    v.Zone = ""

    tk.LogIt(tk.LOG_INFO, "vlan %d bd deleted\n", vlanID)
    return 0, nil
}

func (V *VlansH) VlanPortAdd(vlanID int, portName string, tagged bool) (int, error) {
    if VlanValid(vlanID) == false {
        return VLAN_RANGE_ERR, errors.New("Invalid VlanID")
    }

    if V.VlanMap[vlanID].Created == false {
        // FIXME : Do we create implicitly here
        tk.LogIt(tk.LOG_ERROR, "Vlan not created\n")
        return VLAN_NOTEXIST_ERR, errors.New("Vlan not created")
    }

    v := &V.VlanMap[vlanID]
    p := V.Zone.Ports.PortFindByName(portName)
    if p == nil {
        tk.LogIt(tk.LOG_ERROR, "Phy port not created %s\n", portName)
        return VLAN_PORTPHY_ERR, errors.New("Phy port not created")
    }

    if tagged {
        var membPortName string
        osID := 4000 + (vlanID * MAX_PHY_IFS) + p.PortNo
        membPortName = fmt.Sprintf("%s.%d", portName, vlanID)

        if p.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
            return VLAN_PORT_TAGGED_ERR, errors.New("vxlan can not be tagged")
        }

        if v.TaggedPorts[p.PortNo] != nil {
            return VLAN_PORTEXIST_ERR, errors.New("vlan tag port exists")
        }

        hInfo := p.HInfo
        hInfo.Real = p.Name
        hInfo.Master = v.Name
        if e, _ := V.Zone.Ports.PortAdd(membPortName, osID, cmn.PORT_VLANSIF, v.Zone,
            hInfo, PortLayer2Info{false, vlanID}); e == 0 {
            tp := V.Zone.Ports.PortFindByName(membPortName)
            if tp == nil {
                return VLAN_PORTCREATE_ERR, errors.New("vlan tag port not created")
            }
            v.TaggedPorts[p.PortNo] = tp
            v.NumTagPorts++
        } else {
            return VLAN_PORTCREATE_ERR, errors.New("vlan tag port create failed in DP")
        }
    } else {
        if v.UnTaggedPorts[p.PortNo] != nil {
            return VLAN_PORTEXIST_ERR, errors.New("vlan untag port exists")
        }
        hInfo := p.HInfo
        hInfo.Master = v.Name
        if e, _ := V.Zone.Ports.PortAdd(portName, p.SInfo.OsId, cmn.PORT_VLANSIF, v.Zone,
            hInfo, PortLayer2Info{true, vlanID}); e == 0 {
            v.UnTaggedPorts[p.PortNo] = p
            v.NumUnTagPorts++
        } else {
            return VLAN_PORTCREATE_ERR, errors.New("vlan untag port create failed in DP")
        }
    }

    return 0, nil
}

func (V *VlansH) VlanPortDelete(vlanID int, portName string, tagged bool) (int, error) {
    if VlanValid(vlanID) == false {
        return VLAN_RANGE_ERR, errors.New("Invalid VlanID")
    }

    if V.VlanMap[vlanID].Created == false {
        // FIXME : Do we create implicitly here ??
        return VLAN_NOTEXIST_ERR, errors.New("Vlan not created")
    }

    v := &V.VlanMap[vlanID]
    p := V.Zone.Ports.PortFindByName(portName)
    if p == nil {
        return VLAN_PORTPHY_ERR, errors.New("Phy port not created")
    }

    if tagged {
        tp := v.TaggedPorts[p.PortNo]
        if tp == nil {
            return VLAN_NOPORT_ERR, errors.New("No such tag port")
        }
        var membPortName string
        membPortName = fmt.Sprintf("%s.%d", portName, vlanID)
        V.Zone.Ports.PortDel(membPortName, cmn.PORT_VLANSIF)
        v.TaggedPorts[p.PortNo] = nil
        v.NumTagPorts--
    } else {
        V.Zone.Ports.PortDel(portName, cmn.PORT_VLANSIF)
        v.UnTaggedPorts[p.PortNo] = nil
        v.NumUnTagPorts--
    }

    return 0, nil
}

func (V *VlansH) VlanDestructAll() {

    for i := 0; i < MAX_VLANS; i++ {

        v := V.VlanMap[i]
        if v.Created == true {
            vp := V.Zone.Ports.PortFindByName(v.Name)
            if vp == nil {
                continue
            }

            for p := 0; p < MAX_IFS; p++ {
                mp := v.TaggedPorts[p]
                if mp != nil {
                    V.VlanPortDelete(i, mp.Name, true)
                }
            }

            for p := 0; p < MAX_IFS; p++ {
                mp := v.UnTaggedPorts[p]
                if mp != nil {
                    V.VlanPortDelete(i, mp.Name, false)
                }
            }
            V.VlanDelete(i)
        }
    }
    return
}

func (V *VlansH) Vlans2String(it IterIntf) error {
    var s string
    for i := 0; i < MAX_VLANS; i++ {
        s = ""
        v := V.VlanMap[i]
        if v.Created == true {
            vp := V.Zone.Ports.PortFindByName(v.Name)
            if vp != nil {
                s += fmt.Sprintf("%-10s: ", vp.Name)
            } else {
                tk.LogIt(tk.LOG_ERROR, "VLan %s not found\n", v.Name)
                continue
            }

            s += fmt.Sprintf("Tagged-   ")
            for p := 0; p < MAX_IFS; p++ {
                mp := v.TaggedPorts[p]
                if mp != nil {
                    s += fmt.Sprintf("%s,", mp.Name)
                }
            }

            ts := strings.TrimSuffix(s, ",")
            s = ts

            s += fmt.Sprintf("\n%22s", "UnTagged- ")
            for p := 0; p < MAX_IFS; p++ {
                mp := v.UnTaggedPorts[p]
                if mp != nil {
                    s += fmt.Sprintf("%s,", mp.Name)
                }
            }
            uts := strings.TrimSuffix(s, ",")
            s = uts
            s += fmt.Sprintf("\n")
            it.NodeWalker(s)
        }
    }
    return nil
}

func (V *VlansH) VlansSync() {
    for i := 0; i < MAX_VLANS; i++ {
        v := &V.VlanMap[i]
        if v.Created == true {
            if v.Stat.inPackets != 0 || v.Stat.outPackets != 0 {
                fmt.Printf("BD stats %d : in %v:%v out %v:%v\n",
                    i, v.Stat.inPackets, v.Stat.inBytes,
                    v.Stat.outPackets, v.Stat.outBytes)
            }
            v.DP(DP_STATS_GET)
        }
    }
}

func (V *VlansH) VlansTicker() {
    V.VlansSync()
}

func (v *Vlan) DP(work DpWorkT) int {

    if work == DP_STATS_GET {
        iStat := new(StatDpWorkQ)
        iStat.Work = work
        iStat.HwMark = uint32(v.VlanID)
        iStat.Name = "RXBD"
        iStat.Bytes = &v.Stat.inBytes
        iStat.Packets = &v.Stat.inPackets
        mh.dp.ToDpCh <- iStat

        oStat := new(StatDpWorkQ)
        oStat.Work = work
        oStat.HwMark = uint32(v.VlanID)
        oStat.Name = "TXBD"
        oStat.Bytes = &v.Stat.outBytes
        oStat.Packets = &v.Stat.outPackets
        mh.dp.ToDpCh <- oStat

        return 0
    } else if work == DP_STATS_CLR {
        cStat := new(StatDpWorkQ)
        cStat.Work = work
        cStat.HwMark = uint32(v.VlanID)
        cStat.Name = "BD"

        mh.dp.ToDpCh <- cStat

        return 0
    }

    return -1
}
