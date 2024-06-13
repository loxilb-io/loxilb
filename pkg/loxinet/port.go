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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"io"
	"net"
	"strings"
)

// error codes
const (
	PortBaseErr = iota - 1000
	PortExistsErr
	PortNotExistErr
	PortNoMasterErr
	PortCounterErr
	PortMapErr
	PortZoneErr
	PortNoRealDevErr
	PortPropExistsErr
	PortPropNotExistsErr
)

// constants
const (
	MaxBondInterfaces = 8
	MaxRealInterfaces = 128
	MaxInterfaces     = 512
	MaxWgInterfaces   = 8
	MaxVtiInterfaces  = 8
	RealPortIDB       = 3800
	BondIDB           = 4000
	WgIDB             = 4010
	VtIDB             = 4020
)

// PortEvent - port event type
type PortEvent uint

// port events bitmask
const (
	PortEvDown PortEvent = 1 << iota
	PortEvLowerDown
	PortEvDelete
)

// PortEventIntf - interface for getting notifications
type PortEventIntf interface {
	PortNotifier(name string, osID int, evType PortEvent)
}

// PortStatsInfo - per interface statistics information
// Note that this is not snmp compliant stats
type PortStatsInfo struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxError   uint64
	TxError   uint64
}

// PortHwInfo - hardware specific information of an interface
type PortHwInfo struct {
	MacAddr [6]byte
	Link    bool
	State   bool
	Mtu     int
	Master  string
	Real    string
	TunID   uint32
	TunSrc  net.IP
	TunDst  net.IP
}

// PortLayer3Info - layer3 information related to an interface
type PortLayer3Info struct {
	Routed    bool
	Ipv4Addrs []string
	Ipv6Addrs []string
}

// PortSwInfo - software specific information for interface maintenance
type PortSwInfo struct {
	OsID       int
	PortType   int
	PortProp   cmn.PortProp
	PortPolNum int
	PortMirNum int
	PortActive bool
	PortReal   *Port
	PortOvl    *Port
	SessMark   uint64
	BpfLoaded  bool
}

// PortLayer2Info - layer2 information related to an interface
type PortLayer2Info struct {
	IsPvid bool
	Vid    int
}

// Port - holds all information related to an interface
type Port struct {
	Name   string
	PortNo int
	Zone   string
	SInfo  PortSwInfo
	HInfo  PortHwInfo
	Stats  PortStatsInfo
	L3     PortLayer3Info
	L2     PortLayer2Info
	Sync   DpStatusT
}

// PortsH - the port context container
type PortsH struct {
	portImap   []*Port
	portSmap   map[string]*Port
	portOmap   map[int]*Port
	portNotifs []PortEventIntf
	portMark   *tk.Counter
	bondMark   *tk.Counter
	wGMark     *tk.Counter
	vtiMark    *tk.Counter
}

// PortInit - Initialize the port subsystem
func PortInit() *PortsH {
	var nllp = new(PortsH)
	nllp.portImap = make([]*Port, MaxInterfaces)
	nllp.portSmap = make(map[string]*Port)
	nllp.portOmap = make(map[int]*Port)
	nllp.portMark = tk.NewCounter(1, MaxInterfaces)
	nllp.bondMark = tk.NewCounter(1, MaxBondInterfaces)
	nllp.wGMark = tk.NewCounter(1, MaxWgInterfaces)
	nllp.vtiMark = tk.NewCounter(1, MaxVtiInterfaces)
	return nllp
}

// PortGetSlaves - get any slaves related to the given master interface
func (P *PortsH) PortGetSlaves(master string) (int, []*Port) {
	var slaves []*Port

	for _, p := range P.portSmap {
		if p.HInfo.Master == master {
			slaves = append(slaves, p)
		}
	}

	return 0, slaves
}

// PortHasTunSlaves - get any tunnel slaves related to the given master interface
func (P *PortsH) PortHasTunSlaves(master string, ptype int) (bool, []*Port) {
	var slaves []*Port

	for _, p := range P.portSmap {
		if p.HInfo.Master == master &&
			p.SInfo.PortType&ptype == ptype {
			slaves = append(slaves, p)
		}
	}

	if len(slaves) > 0 {
		return true, slaves
	}
	return false, nil
}

// PortAdd - add a port to loxinet realm
func (P *PortsH) PortAdd(name string, osid int, ptype int, zone string,
	hwi PortHwInfo, l2i PortLayer2Info) (int, error) {

	if _, err := mh.zn.ZonePortIsValid(name, zone); err != nil {
		tk.LogIt(tk.LogError, "port add - %s no such zone\n", name)
		return PortZoneErr, errors.New("no-zone error")
	}

	zn, _ := mh.zn.Zonefind(zone)
	if zn == nil {
		tk.LogIt(tk.LogError, "port add - %s no such zone\n", name)
		return PortZoneErr, errors.New("no-zone error")
	}

	if P.portSmap[name] != nil {
		p := P.portSmap[name]
		p.HInfo.Link = hwi.Link
		p.HInfo.State = hwi.State
		p.HInfo.Mtu = hwi.Mtu
		if !p.IsL3TunPort() && bytes.Equal(hwi.MacAddr[:], p.HInfo.MacAddr[:]) == false {
			p.HInfo.MacAddr = hwi.MacAddr
			p.DP(DpCreate)
		}
		if p.SInfo.PortType == cmn.PortReal {

			if ptype == cmn.PortVlanSif {
				p.HInfo.Master = hwi.Master
				p.SInfo.PortType |= ptype
				if p.L2 != l2i {
					var rp *Port = nil
					lds := p.SInfo.BpfLoaded
					if hwi.Real != "" {
						rp = P.portSmap[hwi.Real]
						if rp == nil {
							tk.LogIt(tk.LogError, "port add - %s no real-port(%s) sif\n", name, hwi.Real)
							return PortNoRealDevErr, errors.New("no-realport sif error")
						}
					} else {
						p.SInfo.BpfLoaded = false
					}
					p.DP(DpRemove)

					p.L2 = l2i
					p.SInfo.PortReal = rp
					p.SInfo.BpfLoaded = lds
					p.DP(DpCreate)
					tk.LogIt(tk.LogDebug, "port add - %s vinfo updated\n", name)
					return 0, nil
				}
			}
			if ptype == cmn.PortBondSif {
				master := P.portSmap[hwi.Master]
				if master == nil {
					tk.LogIt(tk.LogError, "port add - %s no master(%s)\n", name, hwi.Master)
					return PortNoMasterErr, errors.New("no-master error")
				}
				p.DP(DpRemove)
				p.SInfo.PortType |= ptype
				p.HInfo.Master = hwi.Master
				p.L2.IsPvid = true
				p.L2.Vid = master.PortNo + BondIDB

				//p.DP(DpCreate)
				return 0, nil
			}

		} else if p.SInfo.PortType == cmn.PortBond {
			if ptype == cmn.PortVlanSif &&
				l2i.IsPvid == true {
				if p.L2 != l2i {

					p.DP(DpRemove)

					p.L2 = l2i

					p.SInfo.PortType |= ptype
					p.DP(DpCreate)
					return 0, nil
				}
			}
		}
		if p.SInfo.PortType == cmn.PortVxlanBr {
			if ptype == cmn.PortVlanSif &&
				l2i.IsPvid == true {
				p.HInfo.Master = hwi.Master
				p.SInfo.PortType |= ptype
				p.DP(DpRemove)
				p.L2 = l2i
				p.DP(DpCreate)
				tk.LogIt(tk.LogDebug, "port add - %s vxinfo updated\n", name)
				return 0, nil
			}
		}
		if p.SInfo.PortType&(cmn.PortReal|cmn.PortBondSif) == (cmn.PortReal | cmn.PortBondSif) {
			if ptype == cmn.PortReal {
				p.L2.IsPvid = true
				p.L2.Vid = p.PortNo + RealPortIDB
				p.SInfo.PortType &= ^cmn.PortBondSif
				p.HInfo.Master = ""
				p.DP(DpCreate)
				return 0, nil
			}
		}
		tk.LogIt(tk.LogError, "port add - %s exists\n", name)
		return PortExistsErr, errors.New("port exists")
	}

	var rid uint64
	var err error

	if ptype == cmn.PortBond {
		rid, err = P.bondMark.GetCounter()
	} else if ptype == cmn.PortWg {
		rid, err = P.wGMark.GetCounter()
	} else if ptype == cmn.PortVti {
		rid, err = P.vtiMark.GetCounter()
	} else {
		rid, err = P.portMark.GetCounter()
	}
	if err != nil {
		tk.LogIt(tk.LogError, "port add - %s mark error\n", name)
		return PortCounterErr, err
	}

	var rp *Port = nil
	if hwi.Real != "" {
		rp = P.portSmap[hwi.Real]
		if rp == nil {
			tk.LogIt(tk.LogError, "port add - %s no real-port(%s)\n", name, hwi.Real)
			return PortNoRealDevErr, errors.New("no-realport error")
		}
	} else if ptype == cmn.PortVxlanBr {
		tk.LogIt(tk.LogError, "port add - %s real-port needed\n", name)
		return PortNoRealDevErr, errors.New("need-realdev error")
	}

	p := new(Port)
	p.Name = name
	p.Zone = zone
	p.HInfo = hwi
	p.PortNo = int(rid)
	p.SInfo.PortActive = true
	p.SInfo.OsID = osid
	p.SInfo.PortType = ptype
	p.SInfo.PortReal = rp

	vMac := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	switch ptype {
	case cmn.PortReal:
		p.L2.IsPvid = true
		p.L2.Vid = int(rid) + RealPortIDB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0, nil, nil})
	case cmn.PortBond:
		p.L2.IsPvid = true
		p.L2.Vid = int(rid) + BondIDB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0, nil, nil})
	case cmn.PortWg:
		p.L2.IsPvid = true
		p.L2.Vid = int(rid) + WgIDB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0, nil, nil})
	case cmn.PortVti:
		p.L2.IsPvid = true
		p.L2.Vid = int(rid) + VtIDB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0, nil, nil})
	case cmn.PortVxlanBr:
		if p.SInfo.PortReal != nil {
			p.SInfo.PortReal.SInfo.PortOvl = p
			p.SInfo.PortReal.SInfo.PortType |= cmn.PortVxlanSif
			p.SInfo.PortReal.HInfo.Master = p.Name
		}
		p.L2.IsPvid = true
		p.L2.Vid = int(p.HInfo.TunID)
	case cmn.PortIPTun:
		p.SInfo.SessMark, err = zn.Sess.Mark.GetCounter()
		if err != nil {
			tk.LogIt(tk.LogError, "port add - %s sess-alloc fail\n", name)
			p.SInfo.SessMark = 0
		}
		p.L2 = l2i
	default:
		tk.LogIt(tk.LogDebug, "port add - %s isPvid %v\n", name, p.L2.IsPvid)
		p.L2 = l2i
	}

	P.portSmap[name] = p
	P.portImap[rid] = p
	if osid > 0 {
		P.portOmap[osid] = p
	}

	mh.zn.ZonePortAdd(name, zone)
	p.DP(DpCreate)

	tk.LogIt(tk.LogDebug, "port added - %s:%d OSID %d\n", name, p.PortNo, osid)

	return 0, nil
}

// PortDel - delete a port from loxinet realm
func (P *PortsH) PortDel(name string, ptype int) (int, error) {
	if P.portSmap[name] == nil {
		tk.LogIt(tk.LogError, "port delete - %s no such port\n", name)
		return PortNotExistErr, errors.New("no-port error")
	}

	p := P.portSmap[name]

	// If phy port was access vlan, it is converted to normal phy port
	// If it has a trunk vlan association, we will have a subinterface
	if (p.SInfo.PortType&(cmn.PortReal|cmn.PortVlanSif) == (cmn.PortReal | cmn.PortVlanSif)) &&
		ptype == cmn.PortVlanSif {
		p.DP(DpRemove)

		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PortVlanSif
		p.SInfo.PortReal = nil
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + RealPortIDB
		p.DP(DpCreate)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PortVxlanBr|cmn.PortVlanSif) == (cmn.PortVxlanBr | cmn.PortVlanSif)) &&
		ptype == cmn.PortVxlanBr {
		p.DP(DpRemove)

		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PortVlanSif
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = int(p.HInfo.TunID)
		p.DP(DpCreate)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PortBond|cmn.PortVlanSif) == (cmn.PortBond | cmn.PortVlanSif)) &&
		ptype == cmn.PortVlanSif {
		p.DP(DpRemove)
		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PortVlanSif
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + BondIDB
		p.DP(DpCreate)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PortReal|cmn.PortBondSif) == (cmn.PortReal | cmn.PortBondSif)) &&
		ptype == cmn.PortBondSif {
		p.DP(DpRemove)
		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PortBondSif
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + RealPortIDB
		p.DP(DpCreate)
		return 0, nil
	}

	rid := P.portSmap[name].PortNo

	if P.portImap[rid] == nil {
		tk.LogIt(tk.LogError, "port delete - %s no such num\n", name)
		return PortMapErr, errors.New("no-portimap error")
	}

	if P.portOmap[P.portSmap[name].SInfo.OsID] == nil {
		tk.LogIt(tk.LogError, "port delete - %s no such osid\n", name)
		return PortMapErr, errors.New("no-portomap error")
	}

	p.DP(DpRemove)
	zone := mh.zn.GetPortZone(p.Name)

	switch p.SInfo.PortType {
	case cmn.PortVxlanBr:
		if p.SInfo.PortReal != nil {
			p.SInfo.PortReal.SInfo.PortOvl = nil
		}
	case cmn.PortReal:
	case cmn.PortBond:
	case cmn.PortWg:
	case cmn.PortVti:
		if zone != nil {
			zone.Vlans.VlanDelete(p.L2.Vid)
		}
	case cmn.PortIPTun:
		zone.Sess.Mark.PutCounter(p.SInfo.SessMark)
	}

	p.SInfo.PortReal = nil
	p.SInfo.PortActive = false
	mh.zn.ZonePortDelete(name)

	tk.LogIt(tk.LogDebug, "port deleted - %s:%d\n", name, p.PortNo)

	delete(P.portOmap, p.SInfo.OsID)
	delete(P.portSmap, name)
	P.portImap[rid] = nil

	if zone != nil {
		zone.Rt.RtDeleteByPort(p.Name)
		zone.Nh.NeighDeleteByPort(p.Name)
		zone.L3.IfaDeleteAll(p.Name)
	}

	return 0, nil
}

// PortUpdateProp - update port properties given an existing port
func (P *PortsH) PortUpdateProp(name string, prop cmn.PortProp, zone string, updt bool, propVal int) (int, error) {

	var allDevs []*Port

	if _, err := mh.zn.ZonePortIsValid(name, zone); err != nil {
		return PortZoneErr, errors.New("no-zone error")
	}

	zn, _ := mh.zn.Zonefind(zone)
	if zn == nil {
		return PortZoneErr, errors.New("no-zone error")
	}

	p := P.portSmap[name]

	if p == nil {
		tk.LogIt(tk.LogError, "port updt - %s doesnt exist\n", name)
		return PortNotExistErr, errors.New("no-port error")
	}

	if updt {
		if p.SInfo.PortProp&prop == prop {
			tk.LogIt(tk.LogError, "port updt - %s prop exists\n", name)
			return PortPropExistsErr, errors.New("prop-exists error")
		}
	} else {
		if p.SInfo.PortProp&prop != prop {
			tk.LogIt(tk.LogError, "port updt - %s prop doesnt exists\n", name)
			return PortPropNotExistsErr, errors.New("prop-noexist error")
		}
	}

	allDevs = append(allDevs, p)
	for _, pe := range P.portSmap {
		if p != pe && pe.SInfo.PortReal == p &&
			pe.SInfo.PortType&cmn.PortVlanSif == cmn.PortVlanSif &&
			pe.SInfo.PortType&cmn.PortVxlanBr != cmn.PortVxlanBr {
			allDevs = append(allDevs, pe)
		}
	}

	for _, pe := range allDevs {
		if updt {
			pe.SInfo.PortProp |= prop
			if prop&cmn.PortPropPol == cmn.PortPropPol {
				pe.SInfo.PortPolNum = propVal
			} else if prop&cmn.PortPropSpan == cmn.PortPropSpan {
				pe.SInfo.PortMirNum = propVal
			}
		} else {
			if prop&cmn.PortPropPol == cmn.PortPropPol {
				pe.SInfo.PortPolNum = 0
			} else if prop&cmn.PortPropSpan == cmn.PortPropSpan {
				pe.SInfo.PortMirNum = 0
			}
			pe.SInfo.PortProp ^= prop
		}
		tk.LogIt(tk.LogDebug, "port updt - %s:%v(%d)\n", name, prop, propVal)
		pe.DP(DpCreate)
	}

	return 0, nil
}

// Ports2Json - dump ports in loxinet realm to json format
func (P *PortsH) Ports2Json(w io.Writer) error {

	for _, e := range P.portSmap {
		var buf bytes.Buffer
		js, err := json.Marshal(e)
		if err != nil {
			return err
		}
		//_, err = w.Write(js)
		json.Indent(&buf, js, "", "\t")

		_, err = w.Write(buf.Bytes())
	}

	return nil
}

// PortsToGet - dump ports in loxinet realm to api format
func (P *PortsH) PortsToGet() ([]cmn.PortDump, error) {
	var ret []cmn.PortDump

	for _, ports := range P.portSmap {
		zn, _ := mh.zn.Zonefind(ports.Zone)
		if zn == nil {
			tk.LogIt(tk.LogError, "port-zone is not active")
			continue
		}

		ifis := tk.IfiStat{Ifs: [tk.MaxSidx]uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
		opCode := tk.NetGetIfiStats(ports.Name, &ifis)
		if opCode == 0 {
			ports.Stats.RxBytes = ifis.Ifs[tk.RxBytes]
			ports.Stats.RxPackets = ifis.Ifs[tk.RxPkts]
			ports.Stats.RxError = ifis.Ifs[tk.RxErrors]
			ports.Stats.TxBytes = ifis.Ifs[tk.TxBytes]
			ports.Stats.TxPackets = ifis.Ifs[tk.TxPkts]
			ports.Stats.TxError = ifis.Ifs[tk.TxErrors]
		}

		routed := false
		var addr4 []string
		addr4 = append(addr4, zn.L3.IfObjMkString(ports.Name, true))
		if len(addr4) > 0 {
			if addr4[0] != "" {
				routed = true
			}
		}

		var addr6 []string
		addr6 = append(addr6, zn.L3.IfObjMkString(ports.Name, false))
		if len(addr6) > 0 {
			if addr6[0] != "" {
				routed = true
			}
		}

		ret = append(ret, cmn.PortDump{
			Name:   ports.Name,
			PortNo: ports.PortNo,
			Zone:   ports.Zone,
			SInfo: cmn.PortSwInfo{
				OsID:       ports.SInfo.OsID,
				PortType:   ports.SInfo.PortType,
				PortActive: ports.SInfo.PortActive,
				//PortReal:   ports.SInfo.PortReal,
				//PortOvl:    ports.SInfo.PortOvl,
				BpfLoaded: ports.SInfo.BpfLoaded,
			},
			HInfo: cmn.PortHwInfo{
				MacAddr:    ports.HInfo.MacAddr,
				MacAddrStr: fmt.Sprint(net.HardwareAddr(ports.HInfo.MacAddr[:])),
				Link:       ports.HInfo.Link,
				State:      ports.HInfo.State,
				Mtu:        ports.HInfo.Mtu,
				Master:     ports.HInfo.Master,
				Real:       ports.HInfo.Real,
				TunID:      ports.HInfo.TunID,
			},
			Stats: cmn.PortStatsInfo{
				RxBytes:   ports.Stats.RxBytes,
				TxBytes:   ports.Stats.TxBytes,
				RxPackets: ports.Stats.RxPackets,
				TxPackets: ports.Stats.TxPackets,
				RxError:   ports.Stats.RxError,
				TxError:   ports.Stats.TxError,
			},
			L3: cmn.PortLayer3Info{
				//Routed:     ports.L3.Routed,
				//Ipv4_addrs: ports.L3.Ipv4_addrs,
				//Ipv6Addrs: ports.L3.Ipv6Addrs,
				Ipv4Addrs: addr4,
				Routed:    routed,
				Ipv6Addrs: addr6,
			},
			L2: cmn.PortLayer2Info{
				IsPvid: ports.L2.IsPvid,
				Vid:    ports.L2.Vid,
			},
			Sync: cmn.DpStatusT(ports.Sync),
		})

	}
	return ret, nil
}

func port2String(e *Port, it IterIntf) {
	var s string
	var pStr string
	//var portStr string;
	if e.HInfo.State {
		pStr = "UP"
	} else {
		pStr = "DOWN"
	}

	if e.HInfo.Link {
		pStr += ",RUNNING"
	}

	s += fmt.Sprintf("%-10s: <%s> mtu %d %s\n",
		e.Name, pStr, e.HInfo.Mtu, e.Zone)

	pStr = ""
	if e.SInfo.PortType&cmn.PortReal == cmn.PortReal {
		pStr += "phy,"
	}
	if e.SInfo.PortType&cmn.PortVlanSif == cmn.PortVlanSif {
		pStr += "vlan-sif,"
	}
	if e.SInfo.PortType&cmn.PortVlanBr == cmn.PortVlanBr {
		pStr += "vlan,"
	}
	if e.SInfo.PortType&cmn.PortBondSif == cmn.PortBondSif {
		pStr += "bond-sif,"
	}
	if e.SInfo.PortType&cmn.PortBond == cmn.PortBond {
		pStr += "bond,"
	}
	if e.SInfo.PortType&cmn.PortVxlanSif == cmn.PortVxlanSif {
		pStr += "vxlan-sif,"
	}
	if e.SInfo.PortType&cmn.PortVti == cmn.PortVti {
		pStr += "vti,"
	}
	if e.SInfo.PortType&cmn.PortWg == cmn.PortWg {
		pStr += "wg,"
	}
	if e.SInfo.PortProp&cmn.PortPropUpp == cmn.PortPropUpp {
		pStr += "upp,"
	}
	if e.SInfo.PortProp&cmn.PortPropPol == cmn.PortPropPol {
		pol := fmt.Sprintf("pol%d,", e.SInfo.PortPolNum)
		pStr += pol
	}
	if e.SInfo.PortProp&cmn.PortPropSpan == cmn.PortPropSpan {
		pol := fmt.Sprintf("mirr%d,", e.SInfo.PortMirNum)
		pStr += pol
	}
	if e.SInfo.PortType&cmn.PortVxlanBr == cmn.PortVxlanBr {
		pStr += "vxlan"
		if e.SInfo.PortReal != nil {
			pStr += fmt.Sprintf("(%s)", e.SInfo.PortReal.Name)
		}
	}

	nStr := strings.TrimSuffix(pStr, ",")
	s += fmt.Sprintf("%-10s  ether %02x:%02x:%02x:%02x:%02x:%02x  %s\n",
		"", e.HInfo.MacAddr[0], e.HInfo.MacAddr[1], e.HInfo.MacAddr[2],
		e.HInfo.MacAddr[3], e.HInfo.MacAddr[4], e.HInfo.MacAddr[5], nStr)
	it.NodeWalker(s)
}

// Ports2String - dump ports in loxinet realm to string format
func (P *PortsH) Ports2String(it IterIntf) error {
	for _, e := range P.portSmap {
		port2String(e, it)
	}
	return nil
}

// PortFindByName - find a port in loxinet realm given port name
func (P *PortsH) PortFindByName(name string) (p *Port) {
	p, _ = P.portSmap[name]
	return p
}

// PortFindByOSID - find a port in loxinet realm given os identifier
func (P *PortsH) PortFindByOSID(osID int) (p *Port) {
	p, _ = P.portOmap[osID]
	return p
}

// PortL2AddrMatch - check if port of given name has the same hw-mac address
// as the port contained in the given pointer
func (P *PortsH) PortL2AddrMatch(name string, mp *Port) bool {
	p := P.PortFindByName(name)
	if p != nil {
		if p.HInfo.MacAddr == mp.HInfo.MacAddr {
			return true
		}
	}
	return false
}

// PortNotifierRegister - register an interface implementation of type PortEventIntf
func (P *PortsH) PortNotifierRegister(notifier PortEventIntf) {
	P.portNotifs = append(P.portNotifs, notifier)
}

// PortTicker - a ticker routine for ports
func (P *PortsH) PortTicker() {
	var ev PortEvent
	var portMod = false

	for _, port := range P.portSmap {
		portMod = false

		// TODO - This is not very efficient since internally
		// it will get all OS interfaces each time
		osIntf, err := net.InterfaceByName(port.Name)
		if err == nil {
			// Delete Port - TODO
			continue
		}

		// TODO - check link status also ??
		// Currently golang's net package does not extract it
		if !port.HInfo.State {
			if osIntf.Flags&net.FlagUp != 0 {
				port.HInfo.State = true
				ev = 0
				portMod = true
			}
		} else {
			if osIntf.Flags&net.FlagUp == 0 {
				port.HInfo.State = false
				ev = PortEvDown
				portMod = true
			}
		}

		if portMod {
			for _, notif := range P.portNotifs {
				notif.PortNotifier(port.Name, port.SInfo.OsID, ev)
			}
		}

	}
}

// PortDestructAll - destroy all ports in loxinet realm
func (P *PortsH) PortDestructAll() {
	var realDevs []*Port
	var bSlaves []*Port
	var bridges []*Port
	var bondSlaves []*Port
	var bonds []*Port
	var tunSlaves []*Port
	var tunnels []*Port

	for _, p := range P.portSmap {

		if p.SInfo.PortType&cmn.PortReal == cmn.PortReal {
			realDevs = append(realDevs, p)
		}
		if p.SInfo.PortType&cmn.PortVlanSif == cmn.PortVlanSif {
			bSlaves = append(bSlaves, p)
		}
		if p.SInfo.PortType&cmn.PortVlanBr == cmn.PortVlanBr {
			bridges = append(bridges, p)
		}
		if p.SInfo.PortType&cmn.PortBondSif == cmn.PortBondSif {
			bondSlaves = append(bondSlaves, p)
		}
		if p.SInfo.PortType&cmn.PortBond == cmn.PortBond {
			bonds = append(bonds, p)
		}
		if p.SInfo.PortType&cmn.PortVxlanSif == cmn.PortVxlanSif {
			tunSlaves = append(tunSlaves, p)
		}
		if p.SInfo.PortType&cmn.PortVxlanBr == cmn.PortVxlanBr {
			tunnels = append(tunnels, p)
		}
	}

	for _, p := range tunSlaves {
		P.PortDel(p.Name, cmn.PortVxlanSif)
	}

	for _, p := range bSlaves {
		P.PortDel(p.Name, cmn.PortVlanSif)
	}

	for _, p := range bondSlaves {
		P.PortDel(p.Name, cmn.PortBondSif)
	}

	for _, p := range bonds {
		P.PortDel(p.Name, cmn.PortBond)
	}

	for _, p := range bridges {
		P.PortDel(p.Name, cmn.PortVlanBr)
	}

	for _, p := range tunnels {
		P.PortDel(p.Name, cmn.PortVxlanBr)
	}

	for _, p := range realDevs {
		P.PortDel(p.Name, cmn.PortReal)
	}
}

// DP - sync state of port entities in loxinet realm to data-path
func (p *Port) DP(work DpWorkT) int {

	zn, zoneNum := mh.zn.Zonefind(p.Zone)
	if zoneNum < 0 {
		return -1
	}

	// If it is a IP-in-IP tunnel, we add a session entry
	// to decapsulate the tunnel
	if p.SInfo.PortType == cmn.PortIPTun {
		ipts := new(UlClDpWorkQ)
		ipts.Work = work
		ipts.MDip = p.HInfo.TunSrc
		ipts.MSip = p.HInfo.TunDst
		ipts.mTeID = 0
		ipts.Zone = zoneNum
		ipts.Mark = int(p.SInfo.SessMark)
		ipts.Type = DpTunIPIP
		ipts.Qfi = 0
		ipts.TTeID = 0

		mh.dp.ToDpCh <- ipts
		//DpWorkSingle(mh.dp, ipts)
		return 0
	}

	// When a vxlan interface is created
	if p.SInfo.PortType == cmn.PortVxlanBr {
		// Do nothing
		return 0
	}

	// When a vxlan interface becomes slave of a bridge
	if p.SInfo.PortType&(cmn.PortVxlanBr|cmn.PortVlanSif) == (cmn.PortVxlanBr | cmn.PortVlanSif) {
		rmWq := new(RouterMacDpWorkQ)
		rmWq.Work = work
		rmWq.Status = nil

		if p.SInfo.PortReal == nil {
			return -1
		}

		up := p.SInfo.PortReal

		for i := 0; i < 6; i++ {
			rmWq.L2Addr[i] = uint8(up.HInfo.MacAddr[i])
		}
		rmWq.PortNum = up.PortNo
		rmWq.TunID = p.HInfo.TunID
		rmWq.TunType = DpTunVxlan
		rmWq.BD = p.L2.Vid

		mh.dp.ToDpCh <- rmWq
		//DpWorkSingle(mh.dp, rmWq)

		return 0
	}

	// When bond subinterface e.g bond1.100 is created
	if p.SInfo.PortType == cmn.PortVlanSif && p.SInfo.PortReal != nil &&
		p.SInfo.PortReal.SInfo.PortType&cmn.PortBond == cmn.PortBond {

		pWq := new(PortDpWorkQ)

		pWq.Work = work
		pWq.PortNum = p.SInfo.PortReal.PortNo
		pWq.OsPortNum = p.SInfo.PortReal.SInfo.OsID
		pWq.IngVlan = p.L2.Vid
		pWq.SetBD = p.L2.Vid
		pWq.SetZoneNum = zoneNum
		mh.dp.ToDpCh <- pWq
		//DpWorkSingle(mh.dp, pWq)

		return 0
	}

	// When bond becomes a vlan-port e.g bond1 ==> vlan200
	if p.SInfo.PortType&(cmn.PortBond|cmn.PortVlanSif) == (cmn.PortBond | cmn.PortVlanSif) {
		_, slaves := zn.Ports.PortGetSlaves(p.Name)
		for _, sp := range slaves {
			pWq := new(PortDpWorkQ)
			pWq.Work = work
			pWq.OsPortNum = sp.SInfo.OsID
			pWq.PortNum = sp.PortNo
			pWq.IngVlan = 0
			pWq.SetBD = p.L2.Vid
			pWq.SetZoneNum = zoneNum
			pWq.Prop = p.SInfo.PortProp
			pWq.SetPol = p.SInfo.PortPolNum
			pWq.SetMirr = p.SInfo.PortMirNum

			mh.dp.ToDpCh <- pWq
			//DpWorkSingle(mh.dp, pWq)
		}
		return 0
	}

	if (p.SInfo.PortType&(cmn.PortReal|cmn.PortBond|cmn.PortVti|cmn.PortWg) == 0) &&
		(p.SInfo.PortReal == nil || p.SInfo.PortReal.SInfo.PortType&cmn.PortReal != cmn.PortReal) {
		return 0
	}

	pWq := new(PortDpWorkQ)

	pWq.Work = work

	if p.SInfo.PortReal != nil {
		pWq.OsPortNum = p.SInfo.PortReal.SInfo.OsID
		pWq.PortNum = p.SInfo.PortReal.PortNo
	} else {
		pWq.OsPortNum = p.SInfo.OsID
		pWq.PortNum = p.PortNo
	}

	if p.L2.IsPvid {
		pWq.IngVlan = 0
	} else {
		pWq.IngVlan = p.L2.Vid
	}

	pWq.SetBD = p.L2.Vid
	_, pWq.SetZoneNum = mh.zn.Zonefind(p.Zone)
	pWq.Prop = p.SInfo.PortProp
	pWq.SetPol = p.SInfo.PortPolNum
	pWq.SetMirr = p.SInfo.PortMirNum

	if pWq.SetZoneNum < 0 {
		return -1
	}

	if (work == DpCreate || work == DpRemove) && (p.IsLeafPort() == true && p.L2.IsPvid == true) {
		if work == DpCreate {
			if p.SInfo.BpfLoaded == false {
				pWq.LoadEbpf = p.Name
				p.SInfo.BpfLoaded = true
			} else {
				pWq.LoadEbpf = ""
			}
			if strings.Contains(p.Name, "cali") {
				rmWq := new(RouterMacDpWorkQ)
				rmWq.Work = work

				for i := 0; i < 6; i++ {
					rmWq.L2Addr[i] = uint8(p.HInfo.MacAddr[i])
				}
				rmWq.Status = &p.Sync
				rmWq.PortNum = p.PortNo
				//DpWorkSingle(mh.dp, rmWq)
				mh.dp.ToDpCh <- rmWq
			}
		} else if work == DpRemove {
			if p.SInfo.BpfLoaded == true {
				pWq.LoadEbpf = p.Name
				p.SInfo.BpfLoaded = false
			}

			if strings.Contains(p.Name, "cali") {
				zn, _ := mh.zn.Zonefind(p.Zone)
				if zn != nil {
					match := false
					for _, pe := range zn.Ports.portSmap {
						if pe != nil && pe.Name != p.Name {
							if pe.HInfo.MacAddr == p.HInfo.MacAddr {
								match = true
								break
							}
						}
					}
					if !match {
						rmWq := new(RouterMacDpWorkQ)
						rmWq.Work = work

						for i := 0; i < 6; i++ {
							rmWq.L2Addr[i] = uint8(p.HInfo.MacAddr[i])
						}
						rmWq.Status = &p.Sync
						rmWq.PortNum = p.PortNo
						//DpWorkSingle(mh.dp, rmWq)
						mh.dp.ToDpCh <- rmWq
					}
				}
			}
		}
	} else {
		pWq.LoadEbpf = ""
	}

	// TODO - Need to unload eBPF when port properties change
	mh.dp.ToDpCh <- pWq
	//DpWorkSingle(mh.dp, pWq)

	return 0
}

// IsLeafPort - check if the port is a leaf port (eBPF hooks need to
// attached to such ports)
func (p *Port) IsLeafPort() bool {
	if p.SInfo.PortType&(cmn.PortReal|cmn.PortBond|cmn.PortVti|cmn.PortWg) != 0 {
		return true
	}
	return false
}

// IsSlavePort - check if the port is slave of another port
func (p *Port) IsSlavePort() bool {
	if p.SInfo.PortType&(cmn.PortVlanSif|cmn.PortBondSif) == 0 {
		return false
	}
	return true
}

// IsL3TunPort - check if the port is of L3Tun type
func (p *Port) IsL3TunPort() bool {
	if p.SInfo.PortType&(cmn.PortVti|cmn.PortWg|cmn.PortIPTun) != 0 {
		return true
	}
	return false
}

// IsIPinIPTunPort - check if the port is of IPinIPTun type
func (p *Port) IsIPinIPTunPort() bool {
	if p.SInfo.PortType&(cmn.PortIPTun) != 0 {
		return true
	}
	return false
}
