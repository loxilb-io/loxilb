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
	"net"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// error codes
const (
	NeighErrBase = iota - 4000
	NeighExistsErr
	NeighOifErr
	NeighNoEntErr
	NeighRangeErr
	NeighHostRtErr
	NeighMacErr
	NeighTunErr
)

// constants
const (
	NeighAts       = 20
	NeighRslvdAts  = 40
	MaxSysNeigh    = 3 * 1024
	MaxTunnelNeigh = 1024
)

// NeighKey - key of a neighbor entry
type NeighKey struct {
	NhString string
	Zone     string
}

// NeighAttr - attributes of a neighbor
type NeighAttr struct {
	OSLinkIndex  int
	OSState      int
	HardwareAddr net.HardwareAddr
}

// NhType - type of neighbor
type NhType uint8

// supported neighbor types
const (
	NhNormal NhType = 1 << iota
	NhTun
	NhRecursive
)

// NeighTunEp - tun-ep related to neighbor
type NeighTunEp struct {
	sIP      net.IP
	rIP      net.IP
	tunID    uint32
	tunType  DpTunT
	Mark     uint64
	Parent   *Neigh
	Inactive bool
	Sync     DpStatusT
}

// Neigh - a neighbor entry
type Neigh struct {
	Key      NeighKey
	Addr     net.IP
	Attr     NeighAttr
	Dummy    bool
	Inactive bool
	Resolved bool
	Mark     uint64
	RMark    uint64
	RecNh    *Neigh
	tFdb     *FdbEnt
	TunEps   []*NeighTunEp
	Type     NhType
	Sync     DpStatusT
	OifPort  *Port
	Ats      time.Time
	NhRtm    map[RtKey]*Rt
}

// NeighH - the context container
type NeighH struct {
	NeighMap map[NeighKey]*Neigh
	NeighID  *tk.Counter
	NeighTID *tk.Counter
	Zone     *Zone
}

// NeighInit - Initialize the neighbor subsystem
func NeighInit(zone *Zone) *NeighH {
	var nNh = new(NeighH)
	nNh.NeighMap = make(map[NeighKey]*Neigh)
	nNh.NeighID = tk.NewCounter(1, MaxSysNeigh)
	nNh.NeighTID = tk.NewCounter(MaxSysNeigh+1, MaxTunnelNeigh)
	nNh.Zone = zone

	return nNh
}

// Activate - Try to activate a neighbor
func (n *NeighH) Activate(ne *Neigh) {

	interval := NeighAts * time.Second
	if tk.IsNetIPv6(ne.Addr.String()) || ne.Dummy || ne.OifPort == nil {
		return
	}

	if ne.Resolved {
		//interval = NeighRslvdAts * time.Second
		return
	}

	if (time.Since(ne.Ats) < interval) || ne.OifPort.Name == "lo" {
		return
	}

	name := ne.OifPort.Name
	addr := ne.Addr
	var Sip net.IP
	ret := -1
	if ne.OifPort.IsIPinIPTunPort() {
		addr = ne.OifPort.HInfo.TunDst
		ret, Sip, name = n.Zone.L3.IfaSelectAny(addr, false)
		if ret != 0 {
			tk.LogIt(tk.LogDebug, "Failed to select l3-tun ifa select (%s:%s)\n", name, addr.String())
			return
		}
		goto doIT
	}

	ret, Sip, _ = n.Zone.L3.IfaSelect(name, addr, true)
	if ret != 0 {
		tk.LogIt(tk.LogDebug, "Failed to select l3 ifa select (%s:%s)\n", name, addr.String())
		return
	}

doIT:
	go tk.ArpPing(addr, Sip, name)

	ne.Ats = time.Now()
}

// NeighAddTunEP - Add tun-ep to a neighbor
func (n *NeighH) NeighAddTunEP(ne *Neigh, rIP net.IP, sIP net.IP, tunID uint32, tunType DpTunT, sync bool) (int, *NeighTunEp) {
	// FIXME - Need to be able to support multiple overlays with same entry
	port := ne.OifPort
	if port == nil || (port.SInfo.PortOvl == nil && tunType != DpTunIPIP) {
		return -1, nil
	}

	for _, tep := range ne.TunEps {
		if tep.rIP.Equal(rIP) &&
			tep.tunID == tunID &&
			tep.tunType == tunType {
			return 0, tep
		}
	}
	if sIP == nil {
		e := 0
		e, sIP, _ = n.Zone.L3.IfaSelect(port.Name, rIP, false)
		if e != 0 {
			tk.LogIt(tk.LogError, "%s:ifa select error\n", port.Name)
			return -1, nil
		}
	}

	tep := new(NeighTunEp)
	tep.rIP = rIP
	tep.sIP = sIP
	tep.tunID = tunID
	tep.tunType = tunType

	idx, err := n.NeighTID.GetCounter()
	if err != nil {
		return -1, nil
	}
	tep.Mark = idx
	tep.Parent = ne

	ne.TunEps = append(ne.TunEps, tep)

	ne.Type |= NhTun

	if sync {
		tep.DP(DpCreate)
	}

	tk.LogIt(tk.LogDebug, "neigh tunep added - %s:%s (%d)\n", sIP.String(), rIP.String(), tunID)

	return 0, tep
}

// NeighRemoveTunEP - remove tun-ep from a neighbor
func (ne *Neigh) NeighRemoveTunEP(i int) []*NeighTunEp {
	copy(ne.TunEps[i:], ne.TunEps[i+1:])
	return ne.TunEps[:len(ne.TunEps)-1]
}

// NeighDelAllTunEP - delete all tun-eps from a neighbor
func (n *NeighH) NeighDelAllTunEP(ne *Neigh) int {
	i := 0
	for _, tep := range ne.TunEps {
		tep.DP(DpRemove)
		n.NeighTID.PutCounter(tep.Mark)
		tep.Inactive = true
		ne.NeighRemoveTunEP(i)
		i++
	}
	return 0
}

// NeighRecursiveResolve - try to resolve recursive neighbors
// Recursive neighbors are the ones which have the following association :
// nh -> tunfdb -> rt -> tun-nh (Wow)
func (n *NeighH) NeighRecursiveResolve(ne *Neigh) bool {
	chg := false
	zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")

	attr := ne.Attr
	port := ne.OifPort

	if port == nil {
		return chg
	}

	if bytes.Equal(attr.HardwareAddr, zeroHwAddr) == true {
		ne.Resolved = false
	} else {
		ne.Resolved = true
	}

	if ne.tFdb != nil &&
		(ne.tFdb.inActive || ne.tFdb.unReach) {
		ne.Resolved = false
		ne.Type &= ^NhRecursive
		ne.tFdb = nil
		ne.RMark = 0
	}

	if ne.Resolved == true {

		if port.IsIPinIPTunPort() {
			err, pDstNet, tDat := n.Zone.Rt.Trie4.FindTrie(port.HInfo.TunDst.String())
			if err == 0 && pDstNet != nil {
				switch rtn := tDat.(type) {
				case *Neigh:
					if rtn == nil {
						ne.Resolved = false
						ne.RMark = 0
						return false
					}
				default:
					ne.Resolved = false
					ne.RMark = 0
					return false
				}
				if nh, ok := tDat.(*Neigh); ok && !nh.Inactive {
					rt := n.Zone.Rt.RtFind(*pDstNet, n.Zone.Name)
					if rt == nil {
						ne.Resolved = false
						ne.RMark = 0
						return false
					}
					if ne.RMark == 0 || ne.RecNh == nil || ne.RecNh != nh {
						tk.LogIt(tk.LogDebug, "IPTun-NH for %s:%s\n", port.HInfo.TunDst.String(), nh.Key.NhString)
						ret, tep := n.NeighAddTunEP(nh, port.HInfo.TunDst, port.HInfo.TunSrc, port.HInfo.TunID, DpTunIPIP, true)
						if ret == 0 {
							rt.RtDepObjs = append(rt.RtDepObjs, nh)
							ne.RMark = tep.Mark
							ne.Resolved = true
							ne.RecNh = nh
							ne.Type |= NhRecursive
						}
						return true
					}
				}
			}
			return false
		}

		mac := [6]uint8{attr.HardwareAddr[0],
			attr.HardwareAddr[1],
			attr.HardwareAddr[2],
			attr.HardwareAddr[3],
			attr.HardwareAddr[4],
			attr.HardwareAddr[5]}
		key := FdbKey{mac, port.L2.Vid}

		if f := n.Zone.L2.L2FdbFind(key); f == nil {
			hasTun, _ := n.Zone.Ports.PortHasTunSlaves(port.Name, cmn.PortVxlanSif)
			if hasTun {
				ne.tFdb = nil
				ne.Resolved = false
				ne.RMark = 0
			}
		} else {
			if f.FdbAttr.FdbType == cmn.FdbTun {
				if f.unReach || f.FdbTun.ep == nil {
					ne.Resolved = false
					ne.RMark = 0
				} else {
					if ne.tFdb != f {
						ne.tFdb = f
						ne.RMark = f.FdbTun.ep.Mark
						chg = true
					} else if ne.RMark != f.FdbTun.ep.Mark {
						ne.RMark = f.FdbTun.ep.Mark
						chg = true
					}
					ne.Type |= NhRecursive
				}
			}
		}
	}
	return chg
}

// NeighGet - Get neigh entries in Neighv4Mod slice
func (n *NeighH) NeighGet() ([]cmn.NeighMod, error) {
	var ret []cmn.NeighMod
	for _, n2 := range n.NeighMap {
		var tmpNeigh cmn.NeighMod
		tmpNeigh.HardwareAddr = n2.Attr.HardwareAddr
		tmpNeigh.IP = n2.Addr
		tmpNeigh.State = int(n2.Sync)
		tmpNeigh.LinkIndex = n2.OifPort.SInfo.OsID
		ret = append(ret, tmpNeigh)
	}
	return ret, nil
}

// NeighAdd - add a neigh entry
func (n *NeighH) NeighAdd(Addr net.IP, Zone string, Attr NeighAttr) (int, error) {
	var idx uint64
	var err error
	key := NeighKey{Addr.String(), Zone}
	zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")
	ne, found := n.NeighMap[key]

	add2Map := !found

	port := n.Zone.Ports.PortFindByOSID(Attr.OSLinkIndex)
	if port == nil {
		tk.LogIt(tk.LogError, "neigh add - %s:%s no oport\n", Addr.String(), Zone)
		if !found {
			n.NeighMap[key] = &Neigh{Key: key, Dummy: true, Addr: Addr, Attr: Attr, Inactive: true, NhRtm: make(map[RtKey]*Rt)}
		} else {
			ne.Dummy = true
			ne.OifPort = nil
		}
		return NeighOifErr, errors.New("nh-oif error")
	}

	if ne != nil && ne.Dummy {
		found = false
	}

	// Special case to handle IpinIP VTIs
	if port.IsL3TunPort() {
		Attr.HardwareAddr, _ = net.ParseMAC("00:11:22:33:44:55")
	}

	var mask net.IPMask
	if tk.IsNetIPv4(Addr.String()) {
		mask = net.CIDRMask(32, 32)
	} else {
		mask = net.CIDRMask(128, 128)
	}
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	ra := RtAttr{0, 0, true, Attr.OSLinkIndex, false}
	na := []RtNhAttr{{Addr, Attr.OSLinkIndex}}

	if found {
		ne.Inactive = false
		ne.Dummy = false
		if bytes.Equal(Attr.HardwareAddr, zeroHwAddr) {
			ne.Resolved = false
		} else {
			if !bytes.Equal(Attr.HardwareAddr, ne.Attr.HardwareAddr) || !ne.Resolved {
				ne.Attr.HardwareAddr = Attr.HardwareAddr
				ne.Resolved = true
				n.NeighRecursiveResolve(ne)
				tk.LogIt(tk.LogDebug, "nh update - %s:%s (%v)\n", Addr.String(), Zone, ne.Resolved)
				ne.DP(DpCreate)
				goto NhExist
			}
		}
		tk.LogIt(-1, "nh add - %s:%s exists\n", Addr.String(), Zone)
		return NeighExistsErr, errors.New("nh exists")
	}

	if ne == nil {
		ne = new(Neigh)
		ne.Key = key
	}

	if ne.Mark == 0 {
		idx, err = n.NeighID.GetCounter()
		if err != nil {
			tk.LogIt(tk.LogError, "neigh add - %s:%s no marks\n", Addr.String(), Zone)
			return NeighRangeErr, errors.New("nh-hwm error")
		}
		ne.Mark = idx
	}

	ne.Dummy = false
	ne.Addr = Addr
	ne.Attr = Attr
	ne.OifPort = port
	ne.Type |= NhNormal
	if ne.NhRtm == nil {
		ne.NhRtm = make(map[RtKey]*Rt)
	}
	ne.Inactive = false
	n.NeighRecursiveResolve(ne)

	if add2Map {
		n.NeighMap[ne.Key] = ne
	}
	ne.DP(DpCreate)

NhExist:

	// Add a host specific to this neighbor
	ec, err := n.Zone.Rt.RtAdd(ipnet, Zone, ra, na)
	if err != nil && ec != RtExistsErr {
		n.NeighDelete(Addr, Zone)
		tk.LogIt(tk.LogError, "neigh add - %s:%s host-rt fail(%s)\n", Addr.String(), Zone, err)
		return NeighHostRtErr, errors.New("nh-hostrt error")
	}

	//Add a related L2 Pair entry if needed
	if port.IsSlavePort() == false && port.IsLeafPort() == true && ne.Resolved {
		var fdbAddr [6]byte
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}

		fdbKey := FdbKey{fdbAddr, port.L2.Vid}
		fdbAttr := FdbAttr{port.Name, net.ParseIP("0.0.0.0"), cmn.FdbPhy}

		code, err := n.Zone.L2.L2FdbAdd(fdbKey, fdbAttr)
		if err != nil && code != L2SameFdbErr {
			n.Zone.Rt.RtDeleteHost(ipnet, Zone)
			n.NeighDelete(Addr, Zone)
			tk.LogIt(tk.LogError, "neigh add - %s:%s mac fail\n", Addr.String(), Zone)
			return NeighMacErr, errors.New("nh-mac error")
		}
	}

	n.Activate(ne)

	tk.LogIt(tk.LogDebug, "neigh added - %s:%s (%v)\n", Addr.String(), Zone, ne.Mark)

	return 0, nil
}

// NeighDelete - delete a neigh entry
func (n *NeighH) NeighDelete(Addr net.IP, Zone string) (int, error) {
	key := NeighKey{Addr.String(), Zone}

	ne, found := n.NeighMap[key]
	if !found {
		tk.LogIt(tk.LogError, "neigh delete - %s:%s doesnt exist\n", Addr.String(), Zone)
		return NeighNoEntErr, errors.New("no-nh error")
	}

	if ne != nil && ne.Dummy {
		if len(ne.NhRtm) > 0 {
			ne.Resolved = false
			ne.Inactive = true
			zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")
			ne.Attr.HardwareAddr = zeroHwAddr
			return 0, nil
		}
		delete(n.NeighMap, ne.Key)
		return 0, nil
	}

	var mask net.IPMask
	if tk.IsNetIPv4(Addr.String()) {
		mask = net.CIDRMask(32, 32)
	} else {
		mask = net.CIDRMask(128, 128)
	}

	// Delete related L2 Pair entry if needed
	port := ne.OifPort
	if port != nil && port.IsSlavePort() == false && port.IsLeafPort() == true && ne.Resolved {
		var fdbAddr [6]byte
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}

		fdbKey := FdbKey{fdbAddr, port.L2.Vid}
		n.Zone.L2.L2FdbDel(fdbKey)
	}

	// Delete the host specific to this NH
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	_, err := n.Zone.Rt.RtDeleteHost(ipnet, Zone)
	if err != nil {
		tk.LogIt(tk.LogError, "neigh delete - %s:%s host-rt fail\n", Addr.String(), Zone)
		/*return NeighHostRtErr, errors.New("nh-hostrt error" + err.Error())*/
	}

	ne.DP(DpRemove)

	if len(ne.NhRtm) > 0 {
		ne.Resolved = false
		ne.Inactive = true
		zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")
		ne.Attr.HardwareAddr = zeroHwAddr
		// This is a potentially non-optimal situation and we try to recover if possible
		n.Activate(ne)
		tk.LogIt(tk.LogDebug, "neigh deactivated - %s:%s\n", Addr.String(), Zone)
		return 0, nil
	}

	n.NeighDelAllTunEP(ne)
	n.NeighID.PutCounter(ne.Mark)

	ne.tFdb = nil
	ne.Mark = ^uint64(0)
	ne.OifPort = nil
	ne.Inactive = true
	ne.Resolved = false

	delete(n.NeighMap, ne.Key)

	tk.LogIt(tk.LogDebug, "neigh deleted - %s:%s\n", Addr.String(), Zone)

	return 0, nil
}

// NeighDeleteByPort - Routine to delete all the neigh on this port
func (n *NeighH) NeighDeleteByPort(port string) {
	for _, ne := range n.NeighMap {
		if ne.OifPort != nil && ne.OifPort.Name == port {
			n.NeighDelete(ne.Addr, ne.Key.Zone)
		}
	}
}

// NeighFind - Find a neighbor entry
func (n *NeighH) NeighFind(Addr net.IP, Zone string) (*Neigh, int) {
	key := NeighKey{Addr.String(), Zone}

	ne, found := n.NeighMap[key]
	if found == false {
		return nil, -1
	}

	if ne != nil && ne.Inactive {
		return nil, -1
	}

	return ne, 0
}

// NeighPairRt - Associate a route with the given neighbor
func (n *NeighH) NeighPairRt(ne *Neigh, rt *Rt) int {
	_, found := ne.NhRtm[rt.Key]
	if found == true {
		return 1
	}

	ne.NhRtm[rt.Key] = rt

	tk.LogIt(tk.LogDebug, "neigh rtpair - %s->%s\n", rt.Key.RtCidr, ne.Key.NhString)

	return 0
}

// NeighUnPairRt - De-Associate a route from the given neighbor
func (n *NeighH) NeighUnPairRt(ne *Neigh, rt *Rt) int {

	_, found := ne.NhRtm[rt.Key]
	if !found {
		return -1
	}

	delete(ne.NhRtm, rt.Key)
	if len(ne.NhRtm) < 1 && ne.Inactive {
		// Safely remove
		tk.LogIt(tk.LogDebug, "neigh rt unpair - %s->%s\n", rt.Key.RtCidr, ne.Key.NhString)
		n.NeighDelete(ne.Addr, ne.Key.Zone)
		ne.DP(DpRemove)
	}

	return 0
}

// Neigh2String - stringify a neighbor
func Neigh2String(ne *Neigh, it IterIntf) {

	nhBuf := fmt.Sprintf("%16s: %s (R:%v) Oif %8s Mark %d",
		ne.Key.NhString, ne.Attr.HardwareAddr.String(),
		ne.Resolved, ne.OifPort.Name, ne.Mark)

	it.NodeWalker(nhBuf)
}

// Neighs2String - stringify all neighbors
func (n *NeighH) Neighs2String(it IterIntf) error {
	for _, n := range n.NeighMap {
		Neigh2String(n, it)
	}
	return nil
}

// PortNotifier - implementation of PortEventIntf interface
func (n *NeighH) PortNotifier(name string, osID int, evType PortEvent) {
	if evType&PortEvDown|PortEvDelete|PortEvLowerDown != 0 {
		for _, ne := range n.NeighMap {
			if ne.OifPort != nil && ne.OifPort.Name == name {
				n.NeighDelete(net.ParseIP(ne.Key.NhString), ne.Key.Zone)
			}
		}
	}
	return
}

// NeighTicker - a per neighbor ticker sub-routine
func (n *NeighH) NeighTicker(ne *Neigh) {

	if ne.Dummy {
		zone, _ := mh.zn.Zonefind(ne.Key.Zone)
		if zone == nil {
			delete(n.NeighMap, ne.Key)
			return
		}

		_, err := zone.Nh.NeighAdd(net.ParseIP(ne.Key.NhString), ne.Key.Zone, ne.Attr)
		if err == nil {
			tk.LogIt(tk.LogInfo, "nh defer added - %s:%s\n", ne.Key.NhString, ne.Key.Zone)
		}

	}
	n.Activate(ne)
	if n.NeighRecursiveResolve(ne) {
		ne.DP(DpCreate)
	}
}

// NeighsTicker - neighbor subsystem ticker sub-routine
func (n *NeighH) NeighsTicker() {
	i := 1
	for _, ne := range n.NeighMap {
		n.NeighTicker(ne)
		i++
	}
	return
}

// NeighDestructAll - destroy all neighbors
func (n *NeighH) NeighDestructAll() {
	for _, ne := range n.NeighMap {
		addr := net.ParseIP(ne.Key.NhString)
		n.NeighDelete(addr, ne.Key.NhString)
	}
	return
}

// DP - sync state of neighbor entity to data-path
func (ne *Neigh) DP(work DpWorkT) int {

	if ne.Dummy {
		return 0
	}
	//if nh.Resolved == false && work == DP_CREATE {
	//	return -1
	//}

	neighWq := new(NextHopDpWorkQ)
	neighWq.Work = work
	neighWq.Status = &ne.Sync
	neighWq.Resolved = ne.Resolved
	neighWq.NNextHopNum = 0
	if ne.Type&NhRecursive == NhRecursive {
		f := ne.tFdb
		if f != nil && f.FdbTun.ep != nil {
			neighWq.NNextHopNum = int(ne.RMark)
		} else if ne.OifPort != nil && ne.OifPort.IsL3TunPort() {
			neighWq.NNextHopNum = int(ne.RMark)
		} else {
			neighWq.Resolved = false
		}
		neighWq.NextHopNum = int(ne.Mark)
	} else {
		neighWq.NextHopNum = int(ne.Mark)
	}

	for i := 0; i < 6; i++ {
		neighWq.DstAddr[i] = uint8(ne.Attr.HardwareAddr[i])
	}

	if ne.OifPort != nil {
		for i := 0; i < 6; i++ {
			neighWq.SrcAddr[i] = uint8(ne.OifPort.HInfo.MacAddr[i])
		}
		neighWq.BD = ne.OifPort.L2.Vid

	}

	mh.dp.ToDpCh <- neighWq

	return 0
}

// DP - sync state of neighbor tunnel endpoint entity to data-path
func (tep *NeighTunEp) DP(work DpWorkT) int {

	ne := tep.Parent

	if ne == nil {
		return -1
	}

	neighWq := new(NextHopDpWorkQ)
	neighWq.Work = work
	neighWq.Status = &tep.Sync
	neighWq.NextHopNum = int(tep.Mark)
	neighWq.Resolved = ne.Resolved
	neighWq.RIP = tep.rIP
	neighWq.SIP = tep.sIP
	neighWq.TunNh = true
	neighWq.TunID = tep.tunID
	neighWq.TunType = tep.tunType

	if tep.tunID != 0 || tep.tunType == DpTunIPIP {
		for i := 0; i < 6; i++ {
			neighWq.DstAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}

		if ne.OifPort != nil {
			for i := 0; i < 6; i++ {
				neighWq.SrcAddr[i] = uint8(ne.OifPort.HInfo.MacAddr[i])
			}
			neighWq.BD = ne.OifPort.L2.Vid

		}
	}

	mh.dp.ToDpCh <- neighWq

	return 0
}
