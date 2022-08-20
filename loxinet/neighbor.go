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
	"errors"
	"fmt"
	cmn "loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
)

const (
	NEIGH_ERR_BASE = iota - 4000
	NEIGH_EXISTS_ERR
	NEIGH_OIF_ERR
	NEIGH_NOENT_ERR
	NEIGH_RANGE_ERR
	NEIGH_HOSTRT_ERR
	NEIGH_MAC_ERR
	NEIGH_TUN_ERR
)

const (
	MAX_V4NEIGH  = 2048
	MAX_V6NEIGH  = 1024
	MAX_TUNNEIGH = 1024
)

type NeighKey struct {
	NhString string
	Zone     string
}

type NeighAttr struct {
	OSLinkIndex  int
	OSState      int
	HardwareAddr net.HardwareAddr
}

type NhType uint8

const (
	NH_NORMAL NhType = 1 << iota
	NH_TUN
	NH_RECURSIVE
)

type NeighTunEp struct {
	sIP      net.IP
	rIP      net.IP
	tunID    uint32
	tunType  DpTunT
	HwMark   int
	Parent   *Neigh
	Inactive bool
	Sync     DpStatusT
}

type Neigh struct {
	Key      NeighKey
	Addr     net.IP
	Attr     NeighAttr
	Inactive bool
	Resolved bool
	HwMark   int
	tFdb     *FdbEnt
	TunEps   []*NeighTunEp
	Type     NhType
	Sync     DpStatusT
	OifPort  *Port
	NhRtm    map[RtKey]*Rt
}

type NeighH struct {
	NeighMap map[NeighKey]*Neigh
	NeighId  *tk.Counter
	Neigh6Id *tk.Counter
	NeighTid *tk.Counter
	Zone     *Zone
}

func NeighInit(zone *Zone) *NeighH {
	var nNh = new(NeighH)
	nNh.NeighMap = make(map[NeighKey]*Neigh)
	nNh.NeighId = tk.NewCounter(1, MAX_V4NEIGH)
	nNh.NeighTid = tk.NewCounter(MAX_V4NEIGH+1, MAX_TUNNEIGH)
	nNh.Neigh6Id = tk.NewCounter(1, MAX_V6NEIGH)
	nNh.Zone = zone

	return nNh
}

func (n *NeighH) NeighAddTunEP(ne *Neigh, rIP net.IP,
	tunID uint32, tunType DpTunT,
	sync bool) (int, *NeighTunEp) {
	// FIXME - Need to be able to support multiple overlays with same entry
	port := ne.OifPort
	if port == nil || port.SInfo.PortOvl == nil {
		return -1, nil
	}

	for _, tep := range ne.TunEps {
		if tep.rIP.Equal(rIP) &&
			tep.tunID == tunID &&
			tep.tunType == tunType {
			return -1, nil
		}
	}
	e, sIP := n.Zone.L3.IfaSelect(port.Name, rIP)
	if e != 0 {
		return -1, nil
	}

	tep := new(NeighTunEp)
	tep.rIP = rIP
	tep.sIP = sIP
	tep.tunID = tunID
	tep.tunType = tunType

	idx, err := n.NeighTid.GetCounter()
	if err != nil {
		return -1, nil
	}
	tep.HwMark = idx
	tep.Parent = ne

	ne.TunEps = append(ne.TunEps, tep)

	ne.Type |= NH_TUN

	if sync {
		tep.DP(DP_CREATE)
	}

	return 0, tep
}

func (ne *Neigh) NeighRemoveTunEP(i int) []*NeighTunEp {
	copy(ne.TunEps[i:], ne.TunEps[i+1:])
	return ne.TunEps[:len(ne.TunEps)-1]
}

func (n *NeighH) NeighDelTunEP(ne *Neigh, rIP net.IP,
	tunID uint32, tunType DpTunT,
	sync bool) int {

	var i int = 0
	for _, tep := range ne.TunEps {
		if tep.rIP.Equal(rIP) &&
			tep.tunID == tunID &&
			tep.tunType == tunType {

			if sync {
				tep.DP(DP_REMOVE)
			}

			n.NeighTid.PutCounter(tep.HwMark)
			ne.NeighRemoveTunEP(i)
			return 0
		}
		i++
	}

	return -1
}

func (n *NeighH) NeighDelAllTunEP(ne *Neigh) int {
	var i int = 0
	for _, tep := range ne.TunEps {
		tep.DP(DP_REMOVE)
		n.NeighTid.PutCounter(tep.HwMark)
		tep.Inactive = true
		ne.NeighRemoveTunEP(i)
		i++
	}
	return 0
}

func (n *NeighH) NeighRecursiveResolve(ne *Neigh) {
	zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")

	attr := ne.Attr
	port := ne.OifPort

	if port == nil {
		return
	}

	if bytes.Equal(attr.HardwareAddr, zeroHwAddr) == true {
		ne.Resolved = false
	} else {
		ne.Resolved = true
	}

	if ne.tFdb != nil &&
		(ne.tFdb.inActive || ne.tFdb.unReach) {
		ne.Resolved = false
		ne.Type &= ^NH_RECURSIVE
		ne.tFdb = nil
	}

	if ne.Resolved == true {
		mac := [6]uint8{attr.HardwareAddr[0],
			attr.HardwareAddr[1],
			attr.HardwareAddr[2],
			attr.HardwareAddr[3],
			attr.HardwareAddr[4],
			attr.HardwareAddr[5]}
		key := FdbKey{mac, port.L2.Vid}

		if f := n.Zone.L2.L2FdbFind(key); f == nil {
			has_tun, _ := n.Zone.Ports.PortHasTunSlaves(port.Name, cmn.PORT_VXLANSIF)
			if has_tun {
				ne.tFdb = nil
				ne.Resolved = false
			}
		} else {
			if f.FdbAttr.FdbType == cmn.FDB_TUN {
				if f.unReach {
					ne.Resolved = false
				} else {
					ne.tFdb = f
					ne.Type |= NH_RECURSIVE
				}
			}
		}
	}
	return
}

func (n *NeighH) NeighAdd(Addr net.IP, Zone string, Attr NeighAttr) (int, error) {
	key := NeighKey{Addr.String(), Zone}
	zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")
	ne, found := n.NeighMap[key]
	if found == true {
		if bytes.Equal(Attr.HardwareAddr, zeroHwAddr) == true {
			ne.Resolved = false
		} else {
			if bytes.Equal(Attr.HardwareAddr, ne.Attr.HardwareAddr) == false {
				ne.Attr.HardwareAddr = Attr.HardwareAddr
				ne.Resolved = true
				n.NeighRecursiveResolve(ne)
				ne.DP(DP_CREATE)
			}
		}
		return NEIGH_EXISTS_ERR, errors.New("NH Exists")
	}

	port := n.Zone.Ports.PortFindByOSId(Attr.OSLinkIndex)
	if port == nil {
		return NEIGH_OIF_ERR, errors.New("NH Oif Error")
	}

	var idx int
	var err error
	if Addr.To4() == nil {
		idx, err = n.Neigh6Id.GetCounter()
		if err != nil {
			return NEIGH_RANGE_ERR, errors.New("NH6 Range Error")
		}
	} else {
		idx, err = n.NeighId.GetCounter()
		if err != nil {
			return NEIGH_RANGE_ERR, errors.New("NH Range Error")
		}
	}

	ne = new(Neigh)

	ne.Key = key
	ne.Attr = Attr
	ne.OifPort = port
	ne.HwMark = idx
	ne.Type |= NH_NORMAL
	ne.NhRtm = make(map[RtKey]*Rt)

	n.NeighRecursiveResolve(ne)

	n.NeighMap[ne.Key] = ne
	ne.DP(DP_CREATE)

	// Add a host route specific to this NH
	mask := net.CIDRMask(32, 32)
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	ra := RtAttr{0, 0, true}
	na := []RtNhAttr{{Addr, Attr.OSLinkIndex}}
	_, err = n.Zone.Rt.RtAdd(ipnet, Zone, ra, na)
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "Host RT add failed-%s\n", err)
		n.NeighDelete(Addr, Zone)
		return NEIGH_HOSTRT_ERR, errors.New("NH host Rt error")
	}

	//Add a FDB entry if needed
	if port.HInfo.Master == "" &&
		port.SInfo.PortType&(cmn.PORT_REAL|cmn.PORT_BOND) != 0 &&
		ne.Resolved {
		var fdbAddr [6]byte
		var vid int
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}
		if port.SInfo.PortType&cmn.PORT_REAL != 0 {
			vid = port.PortNo + REAL_PORT_VB
		} else {
			vid = port.PortNo + BOND_VB
		}

		fdbKey := FdbKey{fdbAddr, vid}
		fdbAttr := FdbAttr{port.Name, net.ParseIP("0.0.0.0"), cmn.FDB_PHY}

		_, err = n.Zone.L2.L2FdbAdd(fdbKey, fdbAttr)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "Neigh MAC add failed-%s\n", err)
			n.Zone.Rt.RtDelete(ipnet, Zone)
			n.NeighDelete(Addr, Zone)
			return NEIGH_MAC_ERR, errors.New("NH mac error")
		}
	}

	tk.LogIt(tk.LOG_DEBUG, "neigh added %s\n", Addr.String())

	return 0, nil
}

func (n *NeighH) NeighDelete(Addr net.IP, Ns string) (int, error) {
	key := NeighKey{Addr.String(), Ns}

	ne, found := n.NeighMap[key]
	if found == false {
		return NEIGH_NOENT_ERR, errors.New("No such NH")
	}

	n.NeighDelAllTunEP(ne)

	//Delete related MAC entry if needed
	port := ne.OifPort
	if port != nil &&
		port.HInfo.Master == "" &&
		port.SInfo.PortType&(cmn.PORT_REAL|cmn.PORT_BOND) != 0 &&
		ne.Resolved {
		var fdbAddr [6]byte
		var vid int
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}
		if port.SInfo.PortType&cmn.PORT_REAL != 0 {
			vid = port.PortNo + REAL_PORT_VB
		} else {
			vid = port.PortNo + BOND_VB
		}

		fdbKey := FdbKey{fdbAddr, vid}
		n.Zone.L2.L2FdbDel(fdbKey)
	}

	// Delete the host route specific to this NH
	mask := net.CIDRMask(32, 32)
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	_, err := n.Zone.Rt.RtDelete(ipnet, Ns)
	if err != nil {
		return NEIGH_HOSTRT_ERR, errors.New("NH host Rt delete error:" + err.Error())
	}

	if len(ne.NhRtm) == 0 {
		ne.DP(DP_REMOVE)
	}

	if ne.Addr.To4() == nil {
		n.Neigh6Id.PutCounter(ne.HwMark)
	} else {
		n.Neigh6Id.PutCounter(ne.HwMark)
	}

	ne.tFdb = nil
	ne.HwMark = -1
	ne.OifPort = nil
	ne.Inactive = true
	ne.Resolved = false

	tk.LogIt(tk.LOG_DEBUG, "neigh delete %s\n", Addr.String())

	delete(n.NeighMap, ne.Key)

	return 0, nil
}

func (n *NeighH) NeighFind(Addr net.IP, Ns string) (*Neigh, int) {
	key := NeighKey{Addr.String(), Ns}

	ne, found := n.NeighMap[key]
	if found == false {
		return nil, -1
	}

	if ne != nil && ne.Inactive {
		return nil, -1
	}

	return ne, -1
}

func (n *NeighH) NeighPairRt(ne *Neigh, rt *Rt) int {
	_, found := ne.NhRtm[rt.Key]
	if found == true {
		return 1
	}

	ne.NhRtm[rt.Key] = rt

	fmt.Printf("Pairing RT %s via %s\n", rt.Key.RtCidr, ne.Key.NhString)

	return 0
}

func (n *NeighH) NeighUnPairRt(ne *Neigh, rt *Rt) int {

	_, found := ne.NhRtm[rt.Key]
	if found == false {
		return -1
	}

	delete(ne.NhRtm, rt.Key)
	if len(ne.NhRtm) == 0 && ne.Inactive == true {
		// Safely remove
		tk.LogIt(tk.LOG_DEBUG, "UnPair RT %s via %s\n", rt.Key.RtCidr, ne.Key.NhString)
		ne.DP(DP_REMOVE)
	}

	return 0
}

func Neigh2String(ne *Neigh, it IterIntf) {

	nhBuf := fmt.Sprintf("%16s: %s (R:%v) Oif %8s HwMark %d",
		ne.Key.NhString, ne.Attr.HardwareAddr.String(),
		ne.Resolved, ne.OifPort.Name, ne.HwMark)

	it.NodeWalker(nhBuf)
}

func (n *NeighH) Neighs2String(it IterIntf) error {
	for _, n := range n.NeighMap {
		Neigh2String(n, it)
	}
	return nil
}

func (n *NeighH) PortNotifier(name string, osID int, evType PortEvent) {
	if evType&PORT_EV_DOWN|PORT_EV_DELETE|PORT_EV_LOWER_DOWN != 0 {
		for _, ne := range n.NeighMap {
			if ne.OifPort.Name == name {
				n.NeighDelete(net.ParseIP(ne.Key.NhString), ne.Key.Zone)
			}
		}
	}
	return
}

func (n *NeighH) NeighTicker(ne *Neigh) {
	n.NeighRecursiveResolve(ne)
}

func (n *NeighH) NeighsTicker() {
	i := 1
	for _, ne := range n.NeighMap {
		n.NeighTicker(ne)
		i++
	}
	return
}

func (n *NeighH) NeighhDestructAll() {
	for _, ne := range n.NeighMap {
		addr := net.ParseIP(ne.Key.NhString)
		n.NeighDelete(addr, ne.Key.NhString)
	}
	return
}

func (ne *Neigh) DP(work DpWorkT) int {

	//if nh.Resolved == false && work == DP_CREATE {
	//	return -1
	//}

	neighWq := new(NextHopDpWorkQ)
	neighWq.Work = work
	neighWq.Status = &ne.Sync
	neighWq.resolved = ne.Resolved
	neighWq.nNextHopNum = 0
	if ne.Type&NH_RECURSIVE == NH_RECURSIVE {
		f := ne.tFdb
		if f != nil && f.FdbTun.ep != nil {
			neighWq.nNextHopNum = f.FdbTun.ep.HwMark
		} else {
			neighWq.resolved = false
		}
		neighWq.nextHopNum = ne.HwMark
	} else {
		neighWq.nextHopNum = ne.HwMark
	}

	for i := 0; i < 6; i++ {
		neighWq.dstAddr[i] = uint8(ne.Attr.HardwareAddr[i])
	}

	if ne.OifPort != nil {
		for i := 0; i < 6; i++ {
			neighWq.srcAddr[i] = uint8(ne.OifPort.HInfo.MacAddr[i])
		}
		neighWq.BD = ne.OifPort.L2.Vid

	}

	mh.dp.ToDpCh <- neighWq

	return 0
}

func (tep *NeighTunEp) DP(work DpWorkT) int {

	ne := tep.Parent

	if ne == nil {
		return -1
	}

	neighWq := new(NextHopDpWorkQ)
	neighWq.Work = work
	neighWq.Status = &tep.Sync
	neighWq.nextHopNum = tep.HwMark
	neighWq.resolved = ne.Resolved
	neighWq.rIP = tep.rIP
	neighWq.sIP = tep.sIP
	neighWq.tunNh = true
	neighWq.tunID = tep.tunID

	for i := 0; i < 6; i++ {
		neighWq.dstAddr[i] = uint8(ne.Attr.HardwareAddr[i])
	}

	if ne.OifPort != nil {
		for i := 0; i < 6; i++ {
			neighWq.srcAddr[i] = uint8(ne.OifPort.HInfo.MacAddr[i])
		}
		neighWq.BD = ne.OifPort.L2.Vid

	}

	mh.dp.ToDpCh <- neighWq

	return 0
}
