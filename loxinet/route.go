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
	"net"
	tk "github.com/loxilb-io/loxilib"
)

const (
	RT_ERR_BASE = iota - 5000
	RT_EXISTS_ERR
	RT_NH_ERR
	RT_NOENT_ERR
	RT_RANGE_ERR
	RT_MOD_ERR
	RT_TRIE_ADD_ERR
	RT_TRIE_DEL_ERR
)

const (
	RT_TYPE_IND  = 0x1
	RT_TYPE_DYN  = 0x2
	RT_TYPE_SELF = 0x4
	RT_TYPE_HOST = 0x8
)

const (
	MAX_ROUTES = 32 * 1024
)

type RtKey struct {
	RtCidr string
	Zone   string
}

type RtAttr struct {
	Protocol  int
	OSFlags   int
	HostRoute bool
}

type RtNhAttr struct {
	NhAddr    net.IP
	LinkIndex int
}

type RtStat struct {
	Packets uint64
	Bytes   uint64
}

type RtDepObj interface {
}

type Rt struct {
	Key       RtKey
	Addr      net.IP
	Attr      RtAttr
	TFlags    int
	Dead      bool
	Sync      DpStatusT
	ZoneNum   int
	HwMark    int
	Stat      RtStat
	NhAttr    []RtNhAttr
	NextHops  []*Neigh
	RtDepObjs []RtDepObj
}

type RtH struct {
	RtMap  map[RtKey]*Rt
	Trie4  *tk.TrieRoot
	Zone   *Zone
	HwMark *tk.Counter
}

func RtInit(zone *Zone) *RtH {
	var nRt = new(RtH)
	nRt.RtMap = make(map[RtKey]*Rt)
	nRt.Trie4 = tk.TrieInit(false)
	nRt.Zone = zone
	nRt.HwMark = tk.NewCounter(1, MAX_ROUTES)
	return nRt
}

func (r *RtH) TrieNodeWalker(b string) {
	fmt.Printf("%s\n", b)
}

func (r *RtH) TrieData2String(d tk.TrieData) string {

	if nh, ok := d.(*Neigh); ok {
		return fmt.Sprintf("%s", nh.Key.NhString)
	}

	return ""
}

// Find a route matching given IPNet in a zone
func (r *RtH) RtFind(Dst net.IPNet, Zone string) *Rt {
	key := RtKey{Dst.String(), Zone}
	rt, found := r.RtMap[key]

	if found == true {
		return rt
	}
	return nil
}

// Add a route to a zone
func (r *RtH) RtAdd(Dst net.IPNet, Zone string, Ra RtAttr, Na []RtNhAttr) (int, error) {
	key := RtKey{Dst.String(), Zone}
	nhLen := len(Na)

	if nhLen > 1 {
		tk.LogIt(tk.LOG_ERROR, "rt add - %s:%s ecmp not supported\n", Dst.String(), Zone)
		return RT_NH_ERR, errors.New("ecmp-rt error not supported")
	}

	rt, found := r.RtMap[key]
	if found == true {
		rtMod := false
		if len(rt.NhAttr) != nhLen {
			rtMod = true
		} else {
			for i := 0; i < nhLen; i++ {
				// FIXME - Need to sort before comparing
				if Na[i].NhAddr.Equal(rt.NhAttr[i].NhAddr) == false {
					rtMod = false
					break
				}
			}
		}

		if rtMod == true {
			ret, _ := r.RtDelete(Dst, Zone)
			if ret != 0 {
				tk.LogIt(tk.LOG_ERROR, "rt add - %s:%s del failed on mod\n", Dst.String(), Zone)
				return RT_MOD_ERR, errors.New("rt mod error")
			} else {
				return r.RtAdd(Dst, Zone, Ra, Na)
			}
		}

		tk.LogIt(tk.LOG_ERROR, "rt add - %s:%s exists\n", Dst.String(), Zone)
		return RT_EXISTS_ERR, errors.New("rt exists")
	}

	rt = new(Rt)
	rt.Key = key
	rt.Attr = Ra
	rt.NhAttr = Na
	rt.ZoneNum = r.Zone.ZoneNum

	newNhs := make([]*Neigh, 0)

	if len(Na) != 0 {
		rt.TFlags |= RT_TYPE_IND

		if Ra.HostRoute == true {
			rt.TFlags |= RT_TYPE_HOST
		}

		hwmac, _ := net.ParseMAC("00:00:00:00:00:00")

		for i := 0; i < len(Na); i++ {
			nh, _ := r.Zone.Nh.NeighFind(Na[i].NhAddr, Zone)
			if nh == nil {
				// If this is a host route then neighbor has to exist
				// Usually host route addition is triggered by neigh add
				if Ra.HostRoute == true {
					tk.LogIt(tk.LOG_ERROR, "rt add host - %s:%s no neigh\n", Dst.String(), Zone)
					return RT_NH_ERR, errors.New("rt-neigh host error")
				}

				r.Zone.Nh.NeighAdd(Na[i].NhAddr, Zone, NeighAttr{Na[i].LinkIndex, 0, hwmac})
				nh, _ = r.Zone.Nh.NeighFind(Na[i].NhAddr, Zone)
				if nh == nil {
					tk.LogIt(tk.LOG_ERROR, "rt add - %s:%s no neigh\n", Dst.String(), Zone)
					return RT_NH_ERR, errors.New("rt-neigh error")
				}
				newNhs = append(newNhs, nh)
			}
			rt.NextHops = append(rt.NextHops, nh)
		}

	} else {
		rt.TFlags |= RT_TYPE_SELF
	}

	var tret int
	if len(rt.NextHops) > 0 {
		tret = r.Trie4.AddTrie(Dst.String(), rt.NextHops[0])
	} else {
		tret = r.Trie4.AddTrie(Dst.String(), nil)
	}
	if tret != 0 {
		// Delete any neigbors created here
		for i := 0; i < len(newNhs); i++ {
			r.Zone.Nh.NeighDelete(newNhs[i].Addr, Zone)
		}
		tk.LogIt(tk.LOG_ERROR, "rt add - %s:%s lpm add fail\n", Dst.String(), Zone)
		return RT_TRIE_ADD_ERR, errors.New("RT Trie Err")
	}

	// If we cant allocate HwMark, we don't care
	rt.HwMark, _ = r.HwMark.GetCounter()

	r.RtMap[rt.Key] = rt

	// Pair this route with appropriate neighbor
	//if rt.TFlags & RT_TYPE_HOST != RT_TYPE_HOST {
	for i := 0; i < len(rt.NextHops); i++ {
		r.Zone.Nh.NeighPairRt(rt.NextHops[i], rt)
	}
	//}

	rt.DP(DP_CREATE)

	tk.LogIt(tk.LOG_DEBUG, "rt added - %s:%s\n", Dst.String(), Zone)

	return 0, nil
}

func (rt *Rt) RtClearDeps() {
	for _, obj := range rt.RtDepObjs {
		if f, ok := obj.(*FdbEnt); ok {
			f.FdbTun.rt = nil
			f.FdbTun.nh = nil
			f.unReach = true
		}
	}
}

func (rt *Rt) RtRemoveDepObj(i int) []RtDepObj {
	copy(rt.RtDepObjs[i:], rt.RtDepObjs[i+1:])
	return rt.RtDepObjs[:len(rt.RtDepObjs)-1]
}

// Delete a route from a zone
func (r *RtH) RtDelete(Dst net.IPNet, Zone string) (int, error) {
	key := RtKey{Dst.String(), Zone}

	rt, found := r.RtMap[key]
	if found == false {
		tk.LogIt(tk.LOG_ERROR, "rt delete - %s:%s not found\n", Dst.String(), Zone)
		return RT_NOENT_ERR, errors.New("no such route")
	}

	// Take care of any dependencies on this route object
	rt.RtClearDeps()

	// UnPair route from related neighbor
	//if rt.TFlags & RT_TYPE_HOST != RT_TYPE_HOST {
	for _, nh := range rt.NextHops {
		r.Zone.Nh.NeighUnPairRt(nh, rt)
	}
	//}

	tret := r.Trie4.DelTrie(Dst.String())
	if tret != 0 {
		tk.LogIt(tk.LOG_ERROR, "rt delete - %s:%s lpm not found\n", Dst.String(), Zone)
		return RT_TRIE_DEL_ERR, errors.New("rt-lpm delete error")
	}

	delete(r.RtMap, rt.Key)
	defer r.HwMark.PutCounter(rt.HwMark)

	rt.DP(DP_REMOVE)

	tk.LogIt(tk.LOG_DEBUG, "rt deleted - %s:%s\n", Dst.String(), Zone)

	return 0, nil
}

func Rt2String(rt *Rt) string {
	var tStr string
	if rt.TFlags&RT_TYPE_DYN == RT_TYPE_DYN {
		tStr += fmt.Sprintf("Dyn")
	} else {
		tStr += fmt.Sprintf("Static")
	}
	if rt.TFlags&RT_TYPE_IND == RT_TYPE_IND {
		tStr += fmt.Sprintf(",In")
	} else {
		tStr += fmt.Sprintf(",Dr")
	}
	if rt.TFlags&RT_TYPE_SELF == RT_TYPE_SELF {
		tStr += fmt.Sprintf(",Self")
	}
	if rt.TFlags&RT_TYPE_HOST == RT_TYPE_HOST {
		tStr += fmt.Sprintf(",Host")
	}
	if rt.HwMark > 0 {
		tStr += fmt.Sprintf(" HwMark %d", rt.HwMark)
	}

	var rtBuf string
	if len(rt.NhAttr) > 0 {
		rtBuf = fmt.Sprintf("%16s via %12s : %s,%sZn",
							rt.Key.RtCidr, rt.NhAttr[0].NhAddr.String(), tStr, rt.Key.Zone)
	} else {
		rtBuf = fmt.Sprintf("%16s %s,%sZn", rt.Key.RtCidr, tStr, rt.Key.Zone)
	}

	return rtBuf
}

func (r *RtH) Rts2String(it IterIntf) error {
	for _, r := range r.RtMap {
		rtBuf := Rt2String(r)
		it.NodeWalker(rtBuf)
	}
	return nil
}

func (r *RtH) RtDestructAll() {
	for _, rt := range r.RtMap {
		_, dst, err := net.ParseCIDR(rt.Key.RtCidr)
		if err == nil {
			r.RtDelete(*dst, rt.Key.Zone)
		}
	}
	return
}

func (r *RtH) RoutesSync() {
	for _, rt := range r.RtMap {
		if rt.Stat.Packets != 0 {
			rts := Rt2String(rt)
			fmt.Printf("%s: pc %v bc %v\n", rts, rt.Stat.Packets, rt.Stat.Bytes)
		}
		rt.DP(DP_STATS_GET)
	}
}

func (r *RtH) RoutesTicker() {
	r.RoutesSync()
}

func (rt *Rt) RtGetNhHwMark() int {
	if len(rt.NextHops) > 0 {
		return rt.NextHops[0].HwMark
	} else {
		return -1
	}
}

// Sync state of route entities to data-path
func (rt *Rt) DP(work DpWorkT) int {

	_, rtNet, err := net.ParseCIDR(rt.Key.RtCidr)

	if err != nil {
		return -1
	}

	if work == DP_STATS_GET {
		nStat := new(StatDpWorkQ)
		nStat.Work = work
		nStat.HwMark = uint32(rt.HwMark)
		nStat.Name = "RT4"
		nStat.Bytes = &rt.Stat.Bytes
		nStat.Packets = &rt.Stat.Packets

		mh.dp.ToDpCh <- nStat
		return 0
	}

	rtWq := new(RouteDpWorkQ)
	rtWq.Work = work
	rtWq.Status = &rt.Sync
	rtWq.ZoneNum = rt.ZoneNum
	rtWq.Dst = *rtNet
	rtWq.RtType = rt.TFlags
	rtWq.RtHwMark = rt.HwMark
	rtWq.NHwMark = rt.RtGetNhHwMark()

	mh.dp.ToDpCh <- rtWq

	return 0
}
