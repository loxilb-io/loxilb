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
	"strings"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// error codes for vlan mod api
const (
	VlanBaseErr = iota - 2000
	VlanExistsErr
	VlaNotExistErr
	VlanRangeErr
	VlanAddBrpErr
	VlanMpExistErr
	VlanPortPhyErr
	VlanPortExistErr
	VlanPortTaggedErr
	VlanNoPortErr
	VlanPortCreateErr
	VlanZoneErr
)

// constant to declare maximum number of vlans
const (
	MaximumVlans = 4094
)

// vlanStat - statistics for vlan interface
type vlanStat struct {
	inBytes    uint64
	inPackets  uint64
	outBytes   uint64
	outPackets uint64
}

// Vlan - vlan interface info
type Vlan struct {
	VlanID        int
	Created       bool
	Name          string
	Zone          string
	NumTagPorts   int
	TaggedPorts   [MaxInterfaces]*Port
	NumUnTagPorts int
	UnTaggedPorts [MaxInterfaces]*Port
	Stat          vlanStat
}

// VlansH - vlan context handler
type VlansH struct {
	VlanMap [MaximumVlans]Vlan
	Zone    *Zone
}

// VlanInit - routine to initialize vlan context handler
func VlanInit(zone *Zone) *VlansH {
	var nV = new(VlansH)
	nV.Zone = zone
	return nV
}

// VlanValid - routine to validate vlanId
func VlanValid(vlanID int) bool {
	if vlanID > 0 && vlanID < MaximumVlans-1 {
		return true
	}
	return false
}

// VlanGet - Routine to get vlan bridge details
func (V *VlansH) VlanGet() ([]cmn.VlanGet, error) {
	ret := []cmn.VlanGet{}
	for k, v := range V.VlanMap {
		if v.Created {
			tmpVlan := cmn.VlanGet{
				Vid: k,
				Dev: v.Name,
			}
			if v.NumTagPorts != 0 {
				for _, port := range v.TaggedPorts {
					if port != nil {
						tmpSlave := cmn.VlanPortMod{
							Vid:    k,
							Dev:    port.Name,
							Tagged: true,
						}
						tmpVlan.Member = append(tmpVlan.Member, tmpSlave)
					}

				}
			}
			if v.NumUnTagPorts != 0 {
				for _, port := range v.UnTaggedPorts {
					if port != nil {
						tmpSlave := cmn.VlanPortMod{
							Vid:    k,
							Dev:    port.Name,
							Tagged: false,
						}
						tmpVlan.Member = append(tmpVlan.Member, tmpSlave)
					}

				}
			}
			tmpVlan.Stat.InBytes = v.Stat.inBytes
			tmpVlan.Stat.InPackets = v.Stat.inPackets
			tmpVlan.Stat.OutBytes = v.Stat.outBytes
			tmpVlan.Stat.OutPackets = v.Stat.outPackets

			ret = append(ret, tmpVlan)
		}
	}
	return ret, nil
}

// VlanAdd - routine to add vlan interface
func (V *VlansH) VlanAdd(vlanID int, name string, zone string, osid int, hwi PortHwInfo) (int, error) {
	if VlanValid(vlanID) == false {
		return VlanRangeErr, errors.New("Invalid VlanID")
	}

	if V.VlanMap[vlanID].Created == true {
		return VlanExistsErr, errors.New("Vlan already created")
	}

	_, err := mh.zn.ZoneBrAdd(name, zone)
	if err != nil {
		return VlanExistsErr, errors.New("Vlan zone err")
	}

	ret, err := V.Zone.Ports.PortAdd(name, osid, cmn.PortVlanBr, zone, hwi, PortLayer2Info{false, vlanID})
	if err != nil || ret != 0 {
		tk.LogIt(tk.LogError, "Vlan bridge interface not created %d\n", ret)
		mh.zn.ZoneBrDelete(name)
		return VlanAddBrpErr, errors.New("Can't add vlan bridge")
	}

	v := &V.VlanMap[vlanID]
	v.Name = name
	v.VlanID = vlanID
	v.Created = true
	v.Zone = zone

	tk.LogIt(tk.LogInfo, "vlan %d bd created\n", vlanID)

	return 0, nil
}

// VlanDelete - routine to delete vlan interface
func (V *VlansH) VlanDelete(vlanID int) (int, error) {
	if VlanValid(vlanID) == false {
		return VlanRangeErr, errors.New("Invalid VlanID")
	}

	if V.VlanMap[vlanID].Created == false {
		return VlaNotExistErr, errors.New("Vlan not yet created")
	}

	if V.VlanMap[vlanID].NumTagPorts != 0 ||
		V.VlanMap[vlanID].NumUnTagPorts != 0 {
		return VlanMpExistErr, errors.New("Vlan has ports configured")
	}

	v := &V.VlanMap[vlanID]
	mh.zn.ZoneBrDelete(v.Name)

	V.Zone.Ports.PortDel(v.Name, cmn.PortVlanBr)
	v.DP(DpStatsClr)

	v.Name = ""
	v.VlanID = 0
	v.Created = false
	v.Zone = ""

	tk.LogIt(tk.LogInfo, "vlan %d bd deleted\n", vlanID)
	return 0, nil
}

// VlanPortAdd - routine to add a port membership to vlan
func (V *VlansH) VlanPortAdd(vlanID int, portName string, tagged bool) (int, error) {
	if VlanValid(vlanID) == false {
		return VlanRangeErr, errors.New("Invalid VlanID")
	}

	if V.VlanMap[vlanID].Created == false {
		// FIXME : Do we create implicitly here
		tk.LogIt(tk.LogError, "Vlan not created\n")
		return VlaNotExistErr, errors.New("Vlan not created")
	}

	v := &V.VlanMap[vlanID]
	p := V.Zone.Ports.PortFindByName(portName)
	if p == nil {
		tk.LogIt(tk.LogError, "Phy port not created %s\n", portName)
		return VlanPortPhyErr, errors.New("Phy port not created")
	}

	if tagged {
		var membPortName string
		osID := 4000 + (vlanID * MaxRealInterfaces) + p.PortNo
		membPortName = fmt.Sprintf("%s.%d", portName, vlanID)

		if p.SInfo.PortType&cmn.PortVxlanBr == cmn.PortVxlanBr {
			return VlanPortTaggedErr, errors.New("vxlan can not be tagged")
		}

		if v.TaggedPorts[p.PortNo] != nil {
			return VlanPortExistErr, errors.New("vlan tag port exists")
		}

		hInfo := p.HInfo
		hInfo.Real = p.Name
		hInfo.Master = v.Name
		if e, _ := V.Zone.Ports.PortAdd(membPortName, osID, cmn.PortVlanSif, v.Zone,
			hInfo, PortLayer2Info{false, vlanID}); e == 0 {
			tp := V.Zone.Ports.PortFindByName(membPortName)
			if tp == nil {
				return VlanPortCreateErr, errors.New("vlan tag port not created")
			}
			v.TaggedPorts[p.PortNo] = tp
			v.NumTagPorts++
		} else {
			return VlanPortCreateErr, errors.New("vlan tag port create failed in DP")
		}
	} else {
		if v.UnTaggedPorts[p.PortNo] != nil {
			return VlanPortExistErr, errors.New("vlan untag port exists")
		}
		hInfo := p.HInfo
		hInfo.Master = v.Name
		if e, _ := V.Zone.Ports.PortAdd(portName, p.SInfo.OsID, cmn.PortVlanSif, v.Zone,
			hInfo, PortLayer2Info{true, vlanID}); e == 0 {
			v.UnTaggedPorts[p.PortNo] = p
			v.NumUnTagPorts++
		} else {
			return VlanPortCreateErr, errors.New("vlan untag port create failed in DP")
		}
	}

	return 0, nil
}

// VlanPortDelete - routine to delete a port membership from vlan
func (V *VlansH) VlanPortDelete(vlanID int, portName string, tagged bool) (int, error) {
	if VlanValid(vlanID) == false {
		return VlanRangeErr, errors.New("Invalid VlanID")
	}

	if V.VlanMap[vlanID].Created == false {
		// FIXME : Do we create implicitly here ??
		return VlaNotExistErr, errors.New("Vlan not created")
	}

	v := &V.VlanMap[vlanID]
	p := V.Zone.Ports.PortFindByName(portName)
	if p == nil {
		return VlanPortPhyErr, errors.New("Phy port not created")
	}

	if tagged {
		tp := v.TaggedPorts[p.PortNo]
		if tp == nil {
			return VlanNoPortErr, errors.New("No such tag port")
		}
		var membPortName string
		membPortName = fmt.Sprintf("%s.%d", portName, vlanID)
		V.Zone.Ports.PortDel(membPortName, cmn.PortVlanSif)
		v.TaggedPorts[p.PortNo] = nil
		v.NumTagPorts--
	} else {
		V.Zone.Ports.PortDel(portName, cmn.PortVlanSif)
		v.UnTaggedPorts[p.PortNo] = nil
		v.NumUnTagPorts--
	}

	return 0, nil
}

// VlanDestructAll - routine to delete all vlan interfaces
func (V *VlansH) VlanDestructAll() {

	for i := 0; i < MaximumVlans; i++ {

		v := V.VlanMap[i]
		if v.Created == true {
			vp := V.Zone.Ports.PortFindByName(v.Name)
			if vp == nil {
				continue
			}

			for p := 0; p < MaxInterfaces; p++ {
				mp := v.TaggedPorts[p]
				if mp != nil {
					V.VlanPortDelete(i, mp.Name, true)
				}
			}

			for p := 0; p < MaxInterfaces; p++ {
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

// Vlans2String - routine to convert vlan information to string
func (V *VlansH) Vlans2String(it IterIntf) error {
	var s string
	for i := 0; i < MaximumVlans; i++ {
		s = ""
		v := V.VlanMap[i]
		if v.Created == true {
			vp := V.Zone.Ports.PortFindByName(v.Name)
			if vp != nil {
				s += fmt.Sprintf("%-10s: ", vp.Name)
			} else {
				tk.LogIt(tk.LogError, "VLan %s not found\n", v.Name)
				continue
			}

			s += fmt.Sprintf("Tagged-   ")
			for p := 0; p < MaxInterfaces; p++ {
				mp := v.TaggedPorts[p]
				if mp != nil {
					s += fmt.Sprintf("%s,", mp.Name)
				}
			}

			ts := strings.TrimSuffix(s, ",")
			s = ts

			s += fmt.Sprintf("\n%22s", "UnTagged- ")
			for p := 0; p < MaxInterfaces; p++ {
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

// VlansSync - routine to sync vlan information with DP
func (V *VlansH) VlansSync() {
	for i := 0; i < MaximumVlans; i++ {
		v := &V.VlanMap[i]
		if v.Created == true {
			if v.Stat.inPackets != 0 || v.Stat.outPackets != 0 {
				fmt.Printf("BD stats %d : in %v:%v out %v:%v\n",
					i, v.Stat.inPackets, v.Stat.inBytes,
					v.Stat.outPackets, v.Stat.outBytes)
			}
			v.DP(DpStatsGet)
		}
	}
}

// VlansTicker - ticker routine to sync all vlan information with datapath
func (V *VlansH) VlansTicker() {
	V.VlansSync()
}

// DP - routine to sync vlan information with datapath
func (v *Vlan) DP(work DpWorkT) int {

	if work == DpStatsGet {
		iStat := new(StatDpWorkQ)
		iStat.Work = work
		iStat.Mark = uint32(v.VlanID)
		iStat.Name = MapNameRxBD
		iStat.Bytes = &v.Stat.inBytes
		iStat.Packets = &v.Stat.inPackets
		mh.dp.ToDpCh <- iStat

		oStat := new(StatDpWorkQ)
		oStat.Work = work
		oStat.Mark = uint32(v.VlanID)
		oStat.Name = MapNameTxBD
		oStat.Bytes = &v.Stat.outBytes
		oStat.Packets = &v.Stat.outPackets
		mh.dp.ToDpCh <- oStat

		return 0
	} else if work == DpStatsClr {
		cStat := new(StatDpWorkQ)
		cStat.Work = work
		cStat.Mark = uint32(v.VlanID)
		cStat.Name = MapNameBD

		mh.dp.ToDpCh <- cStat

		return 0
	}

	return -1
}
