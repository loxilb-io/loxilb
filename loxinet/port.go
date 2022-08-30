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
	"io"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"strings"
)

const (
	PORT_BASE_ERR = iota - 1000
	PORT_EXISTS_ERR
	PORT_NOTEXIST_ERR
	PORT_NOMASTER_ERR
	PORT_COUNTER_ERR
	PORT_MAP_ERR
	PORT_ZONE_ERR
	PORT_NOREALDEV_ERR
	PORT_PROPEXISTS_ERR
	PORT_PROPNOT_EXISTS_ERR
)

const (
	MAX_BOND_IFS = 8
	MAX_PHY_IFS  = 128
	MAX_IFS      = 512
)

const (
	REAL_PORT_VB = 3800
	BOND_VB      = 4000
)

type PortEvent uint

const (
	PORT_EV_DOWN PortEvent = 1 << iota
	PORT_EV_LOWER_DOWN
	PORT_EV_DELETE
)

type PortEventIntf interface {
	PortNotifier(name string, osId int, evType PortEvent)
}

type PortStatsInfo struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxError   uint64
	TxError   uint64
}

type PortHwInfo struct {
	MacAddr [6]byte
	Link    bool
	State   bool
	Mtu     int
	Master  string
	Real    string
	TunId   uint32
}

type PortLayer3Info struct {
	Routed     bool
	Ipv4_addrs []string
	Ipv6_addrs []string
}

type PortSwInfo struct {
	OsId       int
	PortType   int
	PortProp   cmn.PortProp
	PortPolNum int
	PortActive bool
	PortReal   *Port
	PortOvl    *Port
	BpfLoaded  bool
}

type PortLayer2Info struct {
	IsPvid bool
	Vid    int
}

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

type PortsH struct {
	portImap   []*Port
	portSmap   map[string]*Port
	portOmap   map[int]*Port
	portNotifs []PortEventIntf
	portHwMark *tk.Counter
	bondHwMark *tk.Counter
}

func PortInit() *PortsH {
	var nllp = new(PortsH)
	nllp.portImap = make([]*Port, MAX_IFS)
	nllp.portSmap = make(map[string]*Port)
	nllp.portOmap = make(map[int]*Port)
	nllp.portHwMark = tk.NewCounter(1, MAX_IFS)
	nllp.bondHwMark = tk.NewCounter(1, MAX_BOND_IFS)
	return nllp
}

func (P *PortsH) PortGetSlaves(master string) (int, []*Port) {
	var slaves []*Port

	for _, p := range P.portSmap {
		if p.HInfo.Master == master {
			slaves = append(slaves, p)
		}
	}

	return 0, slaves
}

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

// Add a port to loxinet realm
func (P *PortsH) PortAdd(name string, osid int, ptype int, zone string,
	hwi PortHwInfo, l2i PortLayer2Info) (int, error) {

	if _, err := mh.zn.ZonePortIsValid(name, zone); err != nil {
		tk.LogIt(tk.LOG_ERROR, "port add - %s no such zone\n", name)
		return PORT_ZONE_ERR, errors.New("no-zone error")
	}

	zn, _ := mh.zn.Zonefind(zone)
	if zn == nil {
		tk.LogIt(tk.LOG_ERROR, "port add - %s no such zone\n", name)
		return PORT_ZONE_ERR, errors.New("no-zone error")
	}

	if P.portSmap[name] != nil {
		p := P.portSmap[name]
		if bytes.Equal(hwi.MacAddr[:], p.HInfo.MacAddr[:]) == false {
			p.HInfo.MacAddr = hwi.MacAddr
			p.DP(DP_CREATE)
		}
		if p.SInfo.PortType == cmn.PORT_REAL {
			if ptype == cmn.PORT_VLANSIF &&
				l2i.IsPvid == true {
				p.HInfo.Master = hwi.Master
				p.SInfo.PortType |= ptype
				if p.L2 != l2i {
					p.DP(DP_REMOVE)

					p.L2 = l2i
					p.DP(DP_CREATE)
					tk.LogIt(tk.LOG_DEBUG, "port add - %s vinfo updated\n", name)
					return 0, nil
				}
			}
			if ptype == cmn.PORT_BONDSIF {
				master := P.portSmap[hwi.Master]
				if master == nil {
					tk.LogIt(tk.LOG_ERROR, "port add - %s no master(%s)\n", name, hwi.Master)
					return PORT_NOMASTER_ERR, errors.New("no-master error")
				}
				p.DP(DP_REMOVE)

				p.SInfo.PortType |= ptype
				p.HInfo.Master = hwi.Master
				p.L2.IsPvid = true
				p.L2.Vid = master.PortNo + BOND_VB

				p.DP(DP_CREATE)
				return 0, nil
			}

		} else if p.SInfo.PortType == cmn.PORT_BOND {
			if ptype == cmn.PORT_VLANSIF &&
				l2i.IsPvid == true {
				if p.L2 != l2i {

					p.DP(DP_REMOVE)

					p.L2 = l2i

					p.SInfo.PortType |= ptype
					p.DP(DP_CREATE)
					return 0, nil
				}
			}
		}
		if p.SInfo.PortType == cmn.PORT_VXLANBR {
			if ptype == cmn.PORT_VLANSIF &&
				l2i.IsPvid == true {
				p.HInfo.Master = hwi.Master
				p.SInfo.PortType |= ptype
				p.DP(DP_REMOVE)
				p.L2 = l2i
				p.DP(DP_CREATE)
				tk.LogIt(tk.LOG_DEBUG, "port add - %s vxinfo updated\n", name)
				return 0, nil
			}
		}
		tk.LogIt(tk.LOG_ERROR, "port add - %s exists\n", name)
		return PORT_EXISTS_ERR, errors.New("port exists")
	}

	var rid int
	var err error

	if ptype == cmn.PORT_BOND {
		rid, err = P.bondHwMark.GetCounter()
	} else {
		rid, err = P.portHwMark.GetCounter()
	}
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "port add - %s hwmark error\n", name)
		return PORT_COUNTER_ERR, err
	}

	var rp *Port = nil
	if hwi.Real != "" {
		rp = P.portSmap[hwi.Real]
		if rp == nil {
			tk.LogIt(tk.LOG_ERROR, "port add - %s no real-port(%s)\n", name, hwi.Real)
			return PORT_NOREALDEV_ERR, errors.New("no-realport error")
		}
	} else if ptype == cmn.PORT_VXLANBR {
		tk.LogIt(tk.LOG_ERROR, "port add - %s real-port needed\n", name)
		return PORT_NOREALDEV_ERR, errors.New("need-realdev error")
	}

	p := new(Port)
	p.Name = name
	p.Zone = zone
	p.HInfo = hwi
	p.PortNo = rid
	p.SInfo.PortActive = true
	p.SInfo.OsId = osid
	p.SInfo.PortType = ptype
	p.SInfo.PortReal = rp

	vMac := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	switch ptype {
	case cmn.PORT_REAL:
		p.L2.IsPvid = true
		p.L2.Vid = rid + REAL_PORT_VB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0})
	case cmn.PORT_BOND:
		p.L2.IsPvid = true
		p.L2.Vid = rid + BOND_VB

		/* We create an vlan BD to keep things in sync */
		vstr := fmt.Sprintf("vlan%d", p.L2.Vid)
		zn.Vlans.VlanAdd(p.L2.Vid, vstr, zone, -1,
			PortHwInfo{vMac, true, true, 9000, "", "", 0})
	case cmn.PORT_VXLANBR:
		if p.SInfo.PortReal != nil {
			p.SInfo.PortReal.SInfo.PortOvl = p
			p.SInfo.PortReal.SInfo.PortType |= cmn.PORT_VXLANSIF
			p.SInfo.PortReal.HInfo.Master = p.Name
		}
		p.L2.IsPvid = true
		p.L2.Vid = int(p.HInfo.TunId)
	default:
		tk.LogIt(tk.LOG_DEBUG, "port add - %s isPvid %v\n", name, p.L2.IsPvid)
		p.L2 = l2i
	}

	P.portSmap[name] = p
	P.portImap[rid] = p
	P.portOmap[osid] = p

	mh.zn.ZonePortAdd(name, zone)
	p.DP(DP_CREATE)

	tk.LogIt(tk.LOG_DEBUG, "port added - %s:%d\n", name, p.PortNo)

	return 0, nil
}

// Delete a port from loxinet realm
func (P *PortsH) PortDel(name string, ptype int) (int, error) {
	if P.portSmap[name] == nil {
		tk.LogIt(tk.LOG_ERROR, "port delete - %s no such port\n", name)
		return PORT_NOTEXIST_ERR, errors.New("no-port error")
	}

	p := P.portSmap[name]

	// If phy port was access vlan, it is converted to normal phy port
	// If it has a trunk vlan association, we will have a subinterface
	if (p.SInfo.PortType&(cmn.PORT_REAL|cmn.PORT_VLANSIF) == (cmn.PORT_REAL | cmn.PORT_VLANSIF)) &&
		ptype == cmn.PORT_VLANSIF {
		p.DP(DP_REMOVE)

		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PORT_VLANSIF
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + REAL_PORT_VB
		p.DP(DP_CREATE)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PORT_VXLANBR|cmn.PORT_VLANSIF) == (cmn.PORT_VXLANBR | cmn.PORT_VLANSIF)) &&
		ptype == cmn.PORT_VXLANBR {
		p.DP(DP_REMOVE)

		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PORT_VLANSIF
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = int(p.HInfo.TunId)
		p.DP(DP_CREATE)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PORT_BOND|cmn.PORT_VLANSIF) == (cmn.PORT_BOND | cmn.PORT_VLANSIF)) &&
		ptype == cmn.PORT_VLANSIF {
		p.DP(DP_REMOVE)
		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PORT_VLANSIF
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + BOND_VB
		p.DP(DP_CREATE)
		return 0, nil
	}

	if (p.SInfo.PortType&(cmn.PORT_REAL|cmn.PORT_BONDSIF) == (cmn.PORT_REAL | cmn.PORT_BONDSIF)) &&
		ptype == cmn.PORT_BONDSIF {
		p.DP(DP_REMOVE)
		p.SInfo.PortType = p.SInfo.PortType & ^cmn.PORT_BONDSIF
		p.HInfo.Master = ""
		p.L2.IsPvid = true
		p.L2.Vid = p.PortNo + REAL_PORT_VB
		p.DP(DP_CREATE)
		return 0, nil
	}

	rid := P.portSmap[name].PortNo

	if P.portImap[rid] == nil {
		tk.LogIt(tk.LOG_ERROR, "port delete - %s no such num\n", name)
		return PORT_MAP_ERR, errors.New("no-portimap error")
	}

	if P.portOmap[P.portSmap[name].SInfo.OsId] == nil {
		tk.LogIt(tk.LOG_ERROR, "port delete - %s no such osid\n", name)
		return PORT_MAP_ERR, errors.New("no-portomap error")
	}

	p.DP(DP_REMOVE)

	switch p.SInfo.PortType {
	case cmn.PORT_VXLANBR:
		if p.SInfo.PortReal != nil {
			p.SInfo.PortReal.SInfo.PortOvl = nil
		}
	case cmn.PORT_REAL:
	case cmn.PORT_BOND:
		zone := mh.zn.GetPortZone(p.Name)
		if zone != nil {
			zone.Vlans.VlanDelete(p.L2.Vid)
		}
		break
	}

	p.SInfo.PortReal = nil
	p.SInfo.PortActive = false
	mh.zn.ZonePortDelete(name)

	tk.LogIt(tk.LOG_DEBUG, "port deleted - %s:%d\n", name, p.PortNo)

	delete(P.portOmap, p.SInfo.OsId)
	delete(P.portSmap, name)
	P.portImap[rid] = nil

	return 0, nil
}

// Update port properties given an existing port
func (P *PortsH) PortUpdateProp(name string, prop cmn.PortProp, zone string, updt bool, propVal int) (int, error) {

	var allDevs []*Port

	if _, err := mh.zn.ZonePortIsValid(name, zone); err != nil {
		return PORT_ZONE_ERR, errors.New("no-zone error")
	}

	zn, _ := mh.zn.Zonefind(zone)
	if zn == nil {
		return PORT_ZONE_ERR, errors.New("no-zone error")
	}

	p := P.portSmap[name]

	if p == nil {
		tk.LogIt(tk.LOG_ERROR, "port updt - %s doesnt exist\n", name)
		return PORT_NOTEXIST_ERR, errors.New("no-port error")
	}

	if updt {
		if p.SInfo.PortProp&prop == prop {
			tk.LogIt(tk.LOG_ERROR, "port updt - %s prop exists\n", name)
			return PORT_PROPEXISTS_ERR, errors.New("prop-exists error")
		}
	} else {
		if p.SInfo.PortProp&prop != prop {
			tk.LogIt(tk.LOG_ERROR, "port updt - %s prop doesnt exists\n", name)
			return PORT_PROPNOT_EXISTS_ERR, errors.New("prop-noexist error")
		}
	}

	allDevs = append(allDevs, p)
	for _, pe := range P.portSmap {
		if p != pe && pe.SInfo.PortReal == p &&
			pe.SInfo.PortType&cmn.PORT_VLANSIF == cmn.PORT_VLANSIF &&
			pe.SInfo.PortType&cmn.PORT_VXLANBR != cmn.PORT_VXLANBR {
			allDevs = append(allDevs, pe)
		}
	}

	for _, pe := range allDevs {
		if updt {
			pe.SInfo.PortProp |= prop
			if prop & cmn.PORT_PROP_POL == cmn.PORT_PROP_POL {
				pe.SInfo.PortPolNum = propVal
			}
		} else {
			if prop & cmn.PORT_PROP_POL == cmn.PORT_PROP_POL {
				pe.SInfo.PortPolNum = 0
			}
			pe.SInfo.PortProp ^= prop
		}
		tk.LogIt(tk.LOG_DEBUG, "port updt - %s:%v\n", name, prop)
		pe.DP(DP_CREATE)
	}

	return 0, nil
}

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

func (P *PortsH) PortsToGet() ([]cmn.PortDump, error) {
	var ret []cmn.PortDump

	for _, ports := range P.portSmap {
		zn, _ := mh.zn.Zonefind(ports.Zone)
		if zn == nil {
			tk.LogIt(tk.LOG_ERROR, "port-zone is not active")
			continue
		}
		routed := false
		var addr4 []string
		addr4 = append(addr4, zn.L3.IfObjMkString(ports.Name))
		if len(addr4) > 0 {
			if addr4[0] != "" {
				routed = true
			}
		}

		ret = append(ret, cmn.PortDump{
			Name:   ports.Name,
			PortNo: ports.PortNo,
			Zone:   ports.Zone,
			SInfo: cmn.PortSwInfo{
				OsId:       ports.SInfo.OsId,
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
				TunId:      ports.HInfo.TunId,
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
				Ipv4_addrs: addr4,
				Routed:     routed,
				Ipv6_addrs: ports.L3.Ipv6_addrs,
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
	if e.SInfo.PortType&cmn.PORT_REAL == cmn.PORT_REAL {
		pStr += "phy,"
	}
	if e.SInfo.PortType&cmn.PORT_VLANSIF == cmn.PORT_VLANSIF {
		pStr += "vlan-sif,"
	}
	if e.SInfo.PortType&cmn.PORT_VLANBR == cmn.PORT_VLANBR {
		pStr += "vlan,"
	}
	if e.SInfo.PortType&cmn.PORT_BONDSIF == cmn.PORT_BONDSIF {
		pStr += "bond-sif,"
	}
	if e.SInfo.PortType&cmn.PORT_BONDSIF == cmn.PORT_BOND {
		pStr += "bond,"
	}
	if e.SInfo.PortType&cmn.PORT_VXLANSIF == cmn.PORT_VXLANSIF {
		pStr += "vxlan-sif,"
	}
	if e.SInfo.PortProp&cmn.PORT_PROP_UPP == cmn.PORT_PROP_UPP {
		pStr += "upp,"
	}
	if e.SInfo.PortProp&cmn.PORT_PROP_POL == cmn.PORT_PROP_POL {
		pol := fmt.Sprintf("pol%d,", e.SInfo.PortPolNum)
		pStr += pol
	}
	if e.SInfo.PortProp&cmn.PORT_PROP_SPAN == cmn.PORT_PROP_SPAN {
		pStr += "span,"
	}
	if e.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
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

func (P *PortsH) Ports2String(it IterIntf) error {
	for _, e := range P.portSmap {
		port2String(e, it)
	}
	return nil
}

func (P *PortsH) PortFindByName(name string) (p *Port) {
	p, _ = P.portSmap[name]
	return p
}

func (P *PortsH) PortFindByOSId(osId int) (p *Port) {
	p, _ = P.portOmap[osId]
	return p
}

func (P *PortsH) PortL2AddrMatch(name string, mp *Port) bool {
	p := P.PortFindByName(name)
	if p != nil {
		if p.HInfo.MacAddr == mp.HInfo.MacAddr {
			return true
		}
	}
	return false
}

func (P *PortsH) PortNotifierRegister(notifier PortEventIntf) {
	P.portNotifs = append(P.portNotifs, notifier)
}

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
				ev = PORT_EV_DOWN
				portMod = true
			}
		}

		if portMod {
			for _, notif := range P.portNotifs {
				notif.PortNotifier(port.Name, port.SInfo.OsId, ev)
			}
		}

	}
}

func (P *PortsH) PortDestructAll() {
	var realDevs []*Port
	var bSlaves []*Port
	var bridges []*Port
	var bondSlaves []*Port
	var bonds []*Port
	var tunSlaves []*Port
	var tunnels []*Port

	for _, p := range P.portSmap {

		if p.SInfo.PortType&cmn.PORT_REAL == cmn.PORT_REAL {
			realDevs = append(realDevs, p)
		}
		if p.SInfo.PortType&cmn.PORT_VLANSIF == cmn.PORT_VLANSIF {
			bSlaves = append(bSlaves, p)
		}
		if p.SInfo.PortType&cmn.PORT_VLANBR == cmn.PORT_VLANBR {
			bridges = append(bridges, p)
		}
		if p.SInfo.PortType&cmn.PORT_BONDSIF == cmn.PORT_BONDSIF {
			bondSlaves = append(bondSlaves, p)
		}
		if p.SInfo.PortType&cmn.PORT_BONDSIF == cmn.PORT_BOND {
			bonds = append(bonds, p)
		}
		if p.SInfo.PortType&cmn.PORT_VXLANSIF == cmn.PORT_VXLANSIF {
			tunSlaves = append(tunSlaves, p)
		}
		if p.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
			tunnels = append(tunnels, p)
		}
	}

	for _, p := range tunSlaves {
		P.PortDel(p.Name, cmn.PORT_VXLANSIF)
	}

	for _, p := range bSlaves {
		P.PortDel(p.Name, cmn.PORT_VLANSIF)
	}

	for _, p := range bondSlaves {
		P.PortDel(p.Name, cmn.PORT_BONDSIF)
	}

	for _, p := range bonds {
		P.PortDel(p.Name, cmn.PORT_BOND)
	}

	for _, p := range bridges {
		P.PortDel(p.Name, cmn.PORT_VLANBR)
	}

	for _, p := range tunnels {
		P.PortDel(p.Name, cmn.PORT_VXLANBR)
	}

	for _, p := range realDevs {
		P.PortDel(p.Name, cmn.PORT_REAL)
	}
}

// Sync state of port entities in loxinet realm to data-path
func (p *Port) DP(work DpWorkT) int {

	zn, zoneNum := mh.zn.Zonefind(p.Zone)
	if zoneNum < 0 {
		return -1
	}

	// When a vxlan interface is created
	if p.SInfo.PortType == cmn.PORT_VXLANBR {
		// Do nothing
		return 0
	}

	// When a vxlan interface becomes slave of a bridge
	if p.SInfo.PortType&(cmn.PORT_VXLANBR|cmn.PORT_VLANSIF) == (cmn.PORT_VXLANBR | cmn.PORT_VLANSIF) {
		rmWq := new(RouterMacDpWorkQ)
		rmWq.Work = work
		rmWq.Status = nil

		if p.SInfo.PortReal == nil {
			return -1
		}

		up := p.SInfo.PortReal

		for i := 0; i < 6; i++ {
			rmWq.l2Addr[i] = uint8(up.HInfo.MacAddr[i])
		}
		rmWq.PortNum = up.PortNo
		rmWq.TunId = p.HInfo.TunId
		rmWq.TunType = DP_TUN_VXLAN
		rmWq.BD = p.L2.Vid

		mh.dp.ToDpCh <- rmWq

		return 0
	}

	// When bond subinterface e.g bond1.100 is created
	if p.SInfo.PortType == cmn.PORT_VLANSIF && p.SInfo.PortReal != nil &&
		p.SInfo.PortReal.SInfo.PortType&cmn.PORT_BOND == cmn.PORT_BOND {

		pWq := new(PortDpWorkQ)

		pWq.Work = work
		pWq.PortNum = p.SInfo.PortReal.PortNo
		pWq.OsPortNum = p.SInfo.PortReal.SInfo.OsId
		pWq.IngVlan = p.L2.Vid
		pWq.SetBD = p.L2.Vid
		pWq.SetZoneNum = zoneNum
		mh.dp.ToDpCh <- pWq

		return 0
	}

	// When bond becomes a vlan-port e.g bond1 ==> vlan200
	if p.SInfo.PortType&(cmn.PORT_BOND|cmn.PORT_VLANSIF) == (cmn.PORT_BOND | cmn.PORT_VLANSIF) {
		_, slaves := zn.Ports.PortGetSlaves(p.Name)
		for _, sp := range slaves {
			pWq := new(PortDpWorkQ)
			pWq.Work = work
			pWq.OsPortNum = sp.SInfo.OsId
			pWq.PortNum = sp.PortNo
			pWq.IngVlan = 0
			pWq.SetBD = p.L2.Vid
			pWq.SetZoneNum = zoneNum
			pWq.Prop = p.SInfo.PortProp
			pWq.SetPol = p.SInfo.PortPolNum

			mh.dp.ToDpCh <- pWq
		}
		return 0
	}

	if (p.SInfo.PortType&cmn.PORT_REAL != cmn.PORT_REAL) &&
		(p.SInfo.PortReal == nil || p.SInfo.PortReal.SInfo.PortType&cmn.PORT_REAL != cmn.PORT_REAL) {
		return 0
	}

	pWq := new(PortDpWorkQ)

	pWq.Work = work

	if p.SInfo.PortReal != nil {
		pWq.OsPortNum = p.SInfo.PortReal.SInfo.OsId
		pWq.PortNum = p.SInfo.PortReal.PortNo
	} else {
		pWq.OsPortNum = p.SInfo.OsId
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

	if pWq.SetZoneNum < 0 {
		return -1
	}

	if (work == DP_CREATE || work == DP_REMOVE) &&
		p.SInfo.PortType&cmn.PORT_REAL == cmn.PORT_REAL ||
		p.SInfo.PortType&cmn.PORT_BOND == cmn.PORT_BOND {
		if p.SInfo.BpfLoaded == false {
			pWq.LoadEbpf = p.Name
			p.SInfo.BpfLoaded = true
		} else {
			pWq.LoadEbpf = ""
		}
	} else {
		pWq.LoadEbpf = ""
	}

	// TODO - Need to unload eBPF when port properties change
	mh.dp.ToDpCh <- pWq

	return 0
}
