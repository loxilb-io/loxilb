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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"time"
)

const (
	L2_ERR_BASE = iota - 3000
	L2_SAMEFDB_ERR
	L2_OIF_ERR
	L2_NOFDB_ERR
	L2_VXATTR_ERR
)

const (
	FDB_GTS = 10
)

type FdbKey struct {
	MacAddr  [6]byte
	BridgeId int
}

type FdbAttr struct {
	Oif     string
	Dst     net.IP
	FdbType int
}

type FdbTunAttr struct {
	rt *Rt
	nh *Neigh
	ep *NeighTunEp
}

type FdbStat struct {
	Packets uint64
	Bytes   uint64
}

type FdbEnt struct {
	FdbKey   FdbKey
	FdbAttr  FdbAttr
	FdbTun   FdbTunAttr
	Port     *Port
	itime    time.Time
	stime    time.Time
	unReach  bool
	inActive bool
	Sync     DpStatusT
}

type L2H struct {
	FdbMap map[FdbKey]*FdbEnt
	Zone   *Zone
}

func L2Init(z *Zone) *L2H {
	var nL2 = new(L2H)
	nL2.FdbMap = make(map[FdbKey]*FdbEnt)
	nL2.Zone = z
	z.Ports.PortNotifierRegister(nL2)

	return nL2
}

func l2FdbAttrEqual(a1 *FdbAttr, a2 *FdbAttr) bool {
	if a1.FdbType == a2.FdbType &&
		a1.Oif == a2.Oif &&
		a1.Dst.Equal(a2.Dst) {
		return true
	}
	return false
}

func l2FdbAttrCopy(dst *FdbAttr, src *FdbAttr) {
	dst.FdbType = src.FdbType
	dst.Oif = src.Oif
	dst.Dst = src.Dst
}

// For TunFDB, try to associate with appropriate neighbor
func (f *FdbEnt) L2FdbResolveNh() (bool, int, error) {
	p := f.Port
	attr := f.FdbAttr
	unRch := false

	if p == nil {
		return true, L2_VXATTR_ERR, errors.New("fdb port error")
	}

	zone, _ := mh.zn.Zonefind(p.Zone)
	if zone == nil {
		return true, L2_VXATTR_ERR, errors.New("fdb zone error")
	}

	if p.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
		if attr.FdbType != cmn.FDB_TUN {
			return true, L2_VXATTR_ERR, errors.New("fdb attr error")
		}

		if attr.Dst.To4() == nil {
			return true, L2_VXATTR_ERR, errors.New("fdb v6 dst unsupported")
		}

		tk.LogIt(tk.LOG_DEBUG, "fdb tun rt lookup %s\n", attr.Dst.String())
		// Check if the end-point is reachable
		err, pDstNet, tDat := zone.Rt.Trie4.FindTrie(attr.Dst.String())
		if err == 0 && pDstNet != nil {
			switch rtn := tDat.(type) {
			case *Neigh:
				if rtn == nil {
					return true, -1, errors.New("no neigh found")
				}
			default:
				return true, -1, errors.New("no neigh found")
			}
			if nh, ok := tDat.(*Neigh); ok && !nh.Inactive {
				rt := zone.Rt.RtFind(*pDstNet, zone.Name)
				if rt == nil {
					unRch = true
					tk.LogIt(tk.LOG_DEBUG, "fdb tun rtlookup %s no-rt\n", attr.Dst.String())
				} else {
					ret, tep := zone.Nh.NeighAddTunEP(nh, attr.Dst, p.HInfo.TunId, DP_TUN_VXLAN, true)
					if ret == 0 {
						rt.RtDepObjs = append(rt.RtDepObjs, f)
						f.FdbTun.rt = rt
						f.FdbTun.nh = nh
						f.FdbTun.ep = tep
						unRch = false
					}
				}
			}
		} else {
			unRch = true
			tk.LogIt(tk.LOG_DEBUG, "fdb tun rtlookup %s no trie-ent\n", attr.Dst.String())
		}
	}
	if unRch {
		tk.LogIt(tk.LOG_DEBUG, "fdb tun rtlookup %s unreachable\n", attr.Dst.String())
	}
	return unRch, 0, nil
}

func (l2 *L2H) L2FdbFind(key FdbKey) *FdbEnt {
	fdb, found := l2.FdbMap[key]

	if found == true {
		return fdb
	}

	return nil
}

// Add a l2 forwarding entry
func (l2 *L2H) L2FdbAdd(key FdbKey, attr FdbAttr) (int, error) {

	p := l2.Zone.Ports.PortFindByName(attr.Oif)
	if p == nil || !p.SInfo.PortActive {
		tk.LogIt(tk.LOG_DEBUG, "fdb port not found %s\n", attr.Oif)
		return L2_OIF_ERR, errors.New("no such port")
	}

	fdb, found := l2.FdbMap[key]

	if found == true {
		// Check if it is a modify
		if l2FdbAttrEqual(&attr, &fdb.FdbAttr) {
			tk.LogIt(tk.LOG_DEBUG, "fdb ent exists, %v", key)
			return L2_SAMEFDB_ERR, errors.New("same fdb")
		}
		// Handle modify by deleting and reinstalling
		l2.L2FdbDel(key)
	}

	// Need to double check vlan associations are valid ??
	nfdb := new(FdbEnt)
	nfdb.FdbKey = key
	l2FdbAttrCopy(&nfdb.FdbAttr, &attr)
	nfdb.Port = p
	nfdb.itime = time.Now()
	nfdb.stime = time.Now()

	if p.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
		unRch, ret, err := nfdb.L2FdbResolveNh()
		if err != nil {
			tk.LogIt(tk.LOG_DEBUG, "tun-fdb ent resolve error, %v", key)
			return ret, err
		}
		nfdb.unReach = unRch
	}

	l2.FdbMap[nfdb.FdbKey] = nfdb

	nfdb.DP(DP_CREATE)

	tk.LogIt(tk.LOG_DEBUG, "added fdb ent, %v", key)

	return 0, nil
}

// Delete a l2 forwarding entry
func (l2 *L2H) L2FdbDel(key FdbKey) (int, error) {

	fdb, found := l2.FdbMap[key]
	if found == false {
		tk.LogIt(tk.LOG_DEBUG, "fdb ent not found, %v", key)
		return L2_NOFDB_ERR, errors.New("no such fdb")
	}

	if fdb.Port.SInfo.PortType == cmn.PORT_VXLANBR {
		// Remove route dependencies if any
		n := 0
		if fdb.FdbTun.rt != nil {
			rt := fdb.FdbTun.rt
			for _, obj := range rt.RtDepObjs {
				if f, ok := obj.(*FdbEnt); ok {
					if f == fdb {
						rt.RtDepObjs = rt.RtRemoveDepObj(n)
						break
					}
				}
				n++
			}
		}

		fdb.FdbTun.rt = nil
		fdb.FdbTun.nh = nil
		fdb.FdbTun.ep = nil
	}

	fdb.DP(DP_REMOVE)

	fdb.inActive = true

	delete(l2.FdbMap, fdb.FdbKey)

	tk.LogIt(tk.LOG_DEBUG, "deleted fdb ent, %v", key)

	return 0, nil
}

func (l2 *L2H) FdbTicker(f *FdbEnt) {
	if time.Now().Sub(f.stime) > FDB_GTS {
		// This scans for inconsistencies in a fdb
		// 1. Do garbage cleaning if underlying oif or vlan is not valid anymore
		// 2. If FDB is a TunFDB, we need to make sure NH is reachable
		if f.Port.SInfo.PortActive == false {
			l2.L2FdbDel(f.FdbKey)
		} else if f.unReach == true {
			tk.LogIt(tk.LOG_DEBUG, "unrch scan - %v", f)
			unRch, _, _ := f.L2FdbResolveNh()
			if f.unReach != unRch {
				f.unReach = unRch
				f.DP(DP_CREATE)
			}
		}
		f.stime = time.Now()
	}
}

func (l2 *L2H) FdbsTicker() {
	n := 1
	for _, e := range l2.FdbMap {
		l2.FdbTicker(e)
		n++
	}
	return
}

func (l2 *L2H) PortNotifier(name string, osID int, evType PortEvent) {
	if evType&PORT_EV_DOWN|PORT_EV_DELETE|PORT_EV_LOWER_DOWN != 0 {
		for _, f := range l2.FdbMap {
			if f.FdbAttr.Oif == name {
				l2.L2FdbDel(f.FdbKey)
			}
		}
	}
	return
}

func fdb2String(f *FdbEnt, it IterIntf, n *int) {
	var s string
	s = fmt.Sprintf("FdbEnt%-3d : ether %02x:%02x:%02x:%02x:%02x:%02x,br %d :: Oif %s\n",
		*n, f.FdbKey.MacAddr[0], f.FdbKey.MacAddr[1], f.FdbKey.MacAddr[2],
		f.FdbKey.MacAddr[3], f.FdbKey.MacAddr[4], f.FdbKey.MacAddr[5],
		f.FdbKey.BridgeId, f.FdbAttr.Oif)
	it.NodeWalker(s)
}

func (l2 *L2H) Fdbs2String(it IterIntf) error {
	n := 1
	for _, e := range l2.FdbMap {
		fdb2String(e, it, &n)
		n++
	}
	return nil
}

func (l2 *L2H) L2DestructAll() {
	for _, f := range l2.FdbMap {
		l2.L2FdbDel(f.FdbKey)
	}
	return
}

// Sync state of L2 entities to data-path
func (f *FdbEnt) DP(work DpWorkT) int {

	if work == DP_CREATE && f.unReach == true {
		return 0
	}

	l2Wq := new(L2AddrDpWorkQ)
	l2Wq.Work = work
	l2Wq.Status = &f.Sync
	if f.Port.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
		l2Wq.Tun = DP_TUN_VXLAN
	}

	if f.FdbTun.nh != nil {
		l2Wq.NhNum = f.FdbTun.nh.HwMark
	}

	for i := 0; i < 6; i++ {
		l2Wq.l2Addr[i] = uint8(f.FdbKey.MacAddr[i])
	}
	l2Wq.PortNum = f.Port.PortNo
	l2Wq.BD = f.Port.L2.Vid
	if f.Port.L2.IsPvid {
		l2Wq.Tagged = 0
	} else {
		l2Wq.Tagged = 1
		l2Wq.PortNum = f.Port.SInfo.PortReal.PortNo
	}
	mh.dp.ToDpCh <- l2Wq

	if l2Wq.Tun == DP_TUN_VXLAN {
		rmWq := new(RouterMacDpWorkQ)
		rmWq.Work = work
		rmWq.Status = nil

		if f.Port.SInfo.PortReal == nil ||
			f.FdbTun.ep == nil {
			return -1
		}

		up := f.Port.SInfo.PortReal

		for i := 0; i < 6; i++ {
			rmWq.l2Addr[i] = uint8(f.FdbKey.MacAddr[i])
		}
		rmWq.PortNum = up.PortNo
		rmWq.TunId = f.Port.HInfo.TunId
		rmWq.TunType = DP_TUN_VXLAN
		rmWq.BD = f.Port.L2.Vid
		rmWq.NhNum = f.FdbTun.ep.HwMark
		mh.dp.ToDpCh <- rmWq
	}

	return 0
}
