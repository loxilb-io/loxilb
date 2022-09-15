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
	NeighAts       = 10
	MaxV4Neigh     = 2048
	MaxV6Neigh     = 1024
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
	HwMark   int
	Parent   *Neigh
	Inactive bool
	Sync     DpStatusT
}

// Neigh - a neighbor entry
type Neigh struct {
	Key      NeighKey
	Addr     net.IP
	Attr     NeighAttr
	Inactive bool
	Resolved bool
	HwMark   int
	RHwMark  int
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
	Neigh6ID *tk.Counter
	NeighTID *tk.Counter
	Zone     *Zone
}

// NeighInit - Initialize the neighbor subsystem
func NeighInit(zone *Zone) *NeighH {
	var nNh = new(NeighH)
	nNh.NeighMap = make(map[NeighKey]*Neigh)
	nNh.NeighID = tk.NewCounter(1, MaxV4Neigh)
	nNh.NeighTID = tk.NewCounter(MaxV4Neigh+1, MaxTunnelNeigh)
	nNh.Neigh6ID = tk.NewCounter(1, MaxV6Neigh)
	nNh.Zone = zone

	return nNh
}

// Activate - Try to activate a neighbor
func (n *NeighH) Activate(ne *Neigh) {

	if ne.Resolved {
		return
	}

	if time.Now().Sub(ne.Ats) < NeighAts || ne.OifPort.Name == "lo" {
		return
	}

	ret, Sip := n.Zone.L3.IfaSelect(ne.OifPort.Name, ne.Addr)
	if ret != 0 {
		fmt.Printf("Failed to select l3 ifa select")
	}

	tk.ArpPing(ne.Addr, Sip, ne.OifPort.Name)

	ne.Ats = time.Now()
}

// NeighAddTunEP - Add tun-ep to a neighbor
func (n *NeighH) NeighAddTunEP(ne *Neigh, rIP net.IP, tunID uint32, tunType DpTunT, sync bool) (int, *NeighTunEp) {
	// FIXME - Need to be able to support multiple overlays with same entry
	port := ne.OifPort
	if port == nil || port.SInfo.PortOvl == nil {
		return -1, nil
	}

	for _, tep := range ne.TunEps {
		if tep.rIP.Equal(rIP) &&
			tep.tunID == tunID &&
			tep.tunType == tunType {
			return 0, tep
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

	idx, err := n.NeighTID.GetCounter()
	if err != nil {
		return -1, nil
	}
	tep.HwMark = idx
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
	var i int = 0
	for _, tep := range ne.TunEps {
		tep.DP(DpRemove)
		n.NeighTID.PutCounter(tep.HwMark)
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
		ne.RHwMark = 0
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
			hasTun, _ := n.Zone.Ports.PortHasTunSlaves(port.Name, cmn.PortVxlanSif)
			if hasTun {
				ne.tFdb = nil
				ne.Resolved = false
				ne.RHwMark = 0
			}
		} else {
			if f.FdbAttr.FdbType == cmn.FdbTun {
				if f.unReach  || f.FdbTun.ep == nil {
					ne.Resolved = false
					ne.RHwMark = 0
				} else {
					if ne.tFdb != f {
						ne.tFdb = f
						ne.RHwMark = f.FdbTun.ep.HwMark
						chg = true
					} else if ne.RHwMark != f.FdbTun.ep.HwMark {
						ne.RHwMark = f.FdbTun.ep.HwMark
						chg = true
					}
					ne.Type |= NhRecursive
				}
			}
		}
	}
	return chg
}

// NeighAdd - add a neigh entry
func (n *NeighH) NeighAdd(Addr net.IP, Zone string, Attr NeighAttr) (int, error) {
	var idx int
	var err error
	key := NeighKey{Addr.String(), Zone}
	zeroHwAddr, _ := net.ParseMAC("00:00:00:00:00:00")
	ne, found := n.NeighMap[key]

	port := n.Zone.Ports.PortFindByOSID(Attr.OSLinkIndex)
	if port == nil {
		tk.LogIt(tk.LogError, "neigh add - %s:%s no oport\n", Addr.String(), Zone)
		return NeighOifErr, errors.New("nh-oif error")
	}

	mask := net.CIDRMask(32, 32)
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	ra := RtAttr{0, 0, true}
	na := []RtNhAttr{{Addr, Attr.OSLinkIndex}}

	if found == true {
		ne.Inactive = false
		if bytes.Equal(Attr.HardwareAddr, zeroHwAddr) == true {
			ne.Resolved = false
		} else {
			if bytes.Equal(Attr.HardwareAddr, ne.Attr.HardwareAddr) == false ||
				ne.Resolved == false {
				ne.Attr.HardwareAddr = Attr.HardwareAddr
				ne.Resolved = true
				n.NeighRecursiveResolve(ne)
				tk.LogIt(tk.LogDebug, "nh update - %s:%s (%v)\n", Addr.String(), Zone, ne.Resolved)
				ne.DP(DpCreate)
				goto NhExist
			}
		}
		tk.LogIt(tk.LogError, "nh add - %s:%s exists\n", Addr.String(), Zone)
		return NeighExistsErr, errors.New("nh exists")
	}

	if Addr.To4() == nil {
		idx, err = n.Neigh6ID.GetCounter()
		if err != nil {
			tk.LogIt(tk.LogError, "neigh6 add - %s:%s no hwmarks\n", Addr.String(), Zone)
			return NeighRangeErr, errors.New("nh6-hwm error")
		}
	} else {
		idx, err = n.NeighID.GetCounter()
		if err != nil {
			tk.LogIt(tk.LogError, "neigh add - %s:%s no hwmarks\n", Addr.String(), Zone)
			return NeighRangeErr, errors.New("nh-hwm error")
		}
	}

	ne = new(Neigh)

	ne.Key = key
	ne.Addr = Addr
	ne.Attr = Attr
	ne.OifPort = port
	ne.HwMark = idx
	ne.Type |= NhNormal
	ne.NhRtm = make(map[RtKey]*Rt)
	ne.Inactive = false

	n.NeighRecursiveResolve(ne)
	n.NeighMap[ne.Key] = ne
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
	if port.SInfo.PortType&(cmn.PortVlanSif|cmn.PortBondSif) == 0 &&
		port.SInfo.PortType&(cmn.PortReal|cmn.PortBond) != 0 &&
		ne.Resolved {
		var fdbAddr [6]byte
		var vid int
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}
		if port.SInfo.PortType&cmn.PortReal != 0 {
			vid = port.PortNo + RealPortVb
		} else {
			vid = port.PortNo + BondVb
		}

		fdbKey := FdbKey{fdbAddr, vid}
		fdbAttr := FdbAttr{port.Name, net.ParseIP("0.0.0.0"), cmn.FdbPhy}

		_, err = n.Zone.L2.L2FdbAdd(fdbKey, fdbAttr)
		if err != nil {
			n.Zone.Rt.RtDelete(ipnet, Zone)
			n.NeighDelete(Addr, Zone)
			tk.LogIt(tk.LogError, "neigh add - %s:%s mac fail\n", Addr.String(), Zone)
			return NeighMacErr, errors.New("nh-mac error")
		}
	}

	n.Activate(ne)

	tk.LogIt(tk.LogDebug, "neigh added - %s:%s (%v)\n", Addr.String(), Zone, ne.HwMark)

	return 0, nil
}

// NeighDelete - delete a neigh entry
func (n *NeighH) NeighDelete(Addr net.IP, Zone string) (int, error) {
	key := NeighKey{Addr.String(), Zone}

	ne, found := n.NeighMap[key]
	if found == false {
		tk.LogIt(tk.LogError, "neigh delete - %s:%s doesnt exist\n", Addr.String(), Zone)
		return NeighNoEntErr, errors.New("no-nh error")
	}

	// Delete related L2 Pair entry if needed
	port := ne.OifPort
	if port != nil &&
		port.SInfo.PortType&(cmn.PortVlanSif|cmn.PortBondSif) == 0 &&
		port.SInfo.PortType&(cmn.PortReal|cmn.PortBond) != 0 &&
		ne.Resolved {
		var fdbAddr [6]byte
		var vid int
		for i := 0; i < 6; i++ {
			fdbAddr[i] = uint8(ne.Attr.HardwareAddr[i])
		}
		if port.SInfo.PortType&cmn.PortReal != 0 {
			vid = port.PortNo + RealPortVb
		} else {
			vid = port.PortNo + BondVb
		}

		fdbKey := FdbKey{fdbAddr, vid}
		n.Zone.L2.L2FdbDel(fdbKey)
	}

	// Delete the host specific to this NH
	mask := net.CIDRMask(32, 32)
	ipnet := net.IPNet{IP: Addr, Mask: mask}
	_, err := n.Zone.Rt.RtDelete(ipnet, Zone)
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

	if ne.Addr.To4() == nil {
		n.Neigh6ID.PutCounter(ne.HwMark)
	} else {
		n.Neigh6ID.PutCounter(ne.HwMark)
	}

	ne.tFdb = nil
	ne.HwMark = -1
	ne.OifPort = nil
	ne.Inactive = true
	ne.Resolved = false

	delete(n.NeighMap, ne.Key)

	tk.LogIt(tk.LogDebug, "neigh deleted - %s:%s\n", Addr.String(), Zone)

	return 0, nil
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

	return ne, -1
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
	if found == false {
		return -1
	}

	delete(ne.NhRtm, rt.Key)
	if len(ne.NhRtm) < 1 && ne.Inactive == true {
		// Safely remove
		tk.LogIt(tk.LogDebug, "neigh rt unpair - %s->%s\n", rt.Key.RtCidr, ne.Key.NhString)
		n.NeighDelete(ne.Addr, ne.Key.Zone)
		ne.DP(DpRemove)
	}

	return 0
}

// Neigh2String - stringify a neighbor
func Neigh2String(ne *Neigh, it IterIntf) {

	nhBuf := fmt.Sprintf("%16s: %s (R:%v) Oif %8s HwMark %d",
		ne.Key.NhString, ne.Attr.HardwareAddr.String(),
		ne.Resolved, ne.OifPort.Name, ne.HwMark)

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
			if ne.OifPort.Name == name {
				n.NeighDelete(net.ParseIP(ne.Key.NhString), ne.Key.Zone)
			}
		}
	}
	return
}

// NeighTicker - a per neighbor ticker sub-routine
func (n *NeighH) NeighTicker(ne *Neigh) {
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
			neighWq.NNextHopNum = ne.RHwMark
		} else {
			neighWq.Resolved = false
		}
		neighWq.NextHopNum = ne.HwMark
	} else {
		neighWq.NextHopNum = ne.HwMark
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
	neighWq.NextHopNum = tep.HwMark
	neighWq.Resolved = ne.Resolved
	neighWq.RIP = tep.rIP
	neighWq.SIP = tep.sIP
	neighWq.TunNh = true
	neighWq.TunID = tep.tunID

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
