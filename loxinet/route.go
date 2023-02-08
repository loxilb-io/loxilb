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

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// error codes
const (
	RtErrBase = iota - 5000
	RtExistsErr
	RtNhErr
	RtNoEntErr
	RtRangeErr
	RtModErr
	RtTrieAddErr
	RtTrieDelErr
)

// rt type constants
const (
	RtTypeInd  = 0x1
	RtTypeDyn  = 0x2
	RtTypeSelf = 0x4
	RtTypeHost = 0x8
)

// constants
const (
	MaxSysRoutes = (32 + 8) * 1024 //32k Ipv4 + 8k Ipv6
)

// RtKey - key for a rt entry
type RtKey struct {
	RtCidr string
	Zone   string
}

// RtAttr - extra attribs for a rt entry
type RtAttr struct {
	Protocol  int
	OSFlags   int
	HostRoute bool
	Ifi       int
}

// RtNhAttr - neighbor attribs for a rt entry
type RtNhAttr struct {
	NhAddr    net.IP
	LinkIndex int
}

// RtStat - statistics of a rt entry
type RtStat struct {
	Packets uint64
	Bytes   uint64
}

// RtDepObj - an empty interface to hold any object dependent on rt entry
type RtDepObj interface {
}

// Rt - the rt entry
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

// RtH - context container
type RtH struct {
	RtMap  map[RtKey]*Rt
	Trie4  *tk.TrieRoot
	Trie6  *tk.TrieRoot
	Zone   *Zone
	HwMark *tk.Counter
}

// RtInit - Initialize the route subsystem
func RtInit(zone *Zone) *RtH {
	var nRt = new(RtH)
	nRt.RtMap = make(map[RtKey]*Rt)
	nRt.Trie4 = tk.TrieInit(false)
	nRt.Trie6 = tk.TrieInit(false)
	nRt.Zone = zone
	nRt.HwMark = tk.NewCounter(1, MaxSysRoutes)
	return nRt
}

// TrieNodeWalker - tlpm package interface implementation
func (r *RtH) TrieNodeWalker(b string) {
	fmt.Printf("%s\n", b)
}

// TrieData2String - tlpm package interface implementation
func (r *RtH) TrieData2String(d tk.TrieData) string {

	if nh, ok := d.(*Neigh); ok {
		return fmt.Sprintf("%s", nh.Key.NhString)
	}

	return ""
}

// RtFind - Find a route matching given IPNet in a zone
func (r *RtH) RtFind(Dst net.IPNet, Zone string) *Rt {
	key := RtKey{Dst.String(), Zone}
	rt, found := r.RtMap[key]

	if found == true {
		return rt
	}
	return nil
}

// RouteGet - tlpm package interface implementation
func (r *RtH) RouteGet() ([]cmn.RouteGet, error) {
	var ret []cmn.RouteGet
	for rk, r2 := range r.RtMap {
		var tmpRt cmn.RouteGet
		tmpRt.Dst = rk.RtCidr
		tmpRt.Flags = GetFlagToString(r2.TFlags)
		if len(r2.NhAttr) != 0 {
			// TODO : Current multiple gw not showing. So I added as a static code
			tmpRt.Gw = r2.NhAttr[0].NhAddr.String()
		}
		tmpRt.HardwareMark = r2.HwMark
		tmpRt.Protocol = r2.Attr.Protocol
		tmpRt.Statistic.Bytes = int(r2.Stat.Bytes)
		tmpRt.Statistic.Packets = int(r2.Stat.Packets)
		tmpRt.Sync = cmn.DpStatusT(r2.Sync)
		ret = append(ret, tmpRt)
	}
	return ret, nil
}

// GetFlagToString - Stringify route flags
func GetFlagToString(flag int) string {
	var ret string
	if flag&RtTypeInd != 0 {
		ret += "Ind "
	}
	if flag&RtTypeDyn != 0 {
		ret += "Dyn "
	}
	if flag&RtTypeSelf != 0 {
		ret += "Self "
	}
	if flag&RtTypeHost != 0 {
		ret += "Host "
	}

	return ret
}

// RtAdd - Add a route
func (r *RtH) RtAdd(Dst net.IPNet, Zone string, Ra RtAttr, Na []RtNhAttr) (int, error) {
	key := RtKey{Dst.String(), Zone}
	nhLen := len(Na)

	if nhLen > 1 {
		tk.LogIt(tk.LogError, "rt add - %s:%s ecmp not supported\n", Dst.String(), Zone)
		return RtNhErr, errors.New("ecmp-rt error not supported")
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
				tk.LogIt(tk.LogError, "rt add - %s:%s del failed on mod\n", Dst.String(), Zone)
				return RtModErr, errors.New("rt mod error")
			}
			return r.RtAdd(Dst, Zone, Ra, Na)
		}

		tk.LogIt(tk.LogError, "rt add - %s:%s exists\n", Dst.String(), Zone)
		return RtExistsErr, errors.New("rt exists")
	}

	rt = new(Rt)
	rt.Key = key
	rt.Attr = Ra
	rt.NhAttr = Na
	rt.ZoneNum = r.Zone.ZoneNum

	newNhs := make([]*Neigh, 0)

	if len(Na) != 0 {
		rt.TFlags |= RtTypeInd

		if Ra.HostRoute == true {
			rt.TFlags |= RtTypeHost
		}

		hwmac, _ := net.ParseMAC("00:00:00:00:00:00")

		for i := 0; i < len(Na); i++ {
			nh, _ := r.Zone.Nh.NeighFind(Na[i].NhAddr, Zone)
			if nh == nil {

				// If this is a host route then neighbor has to exist
				// Usually host route addition is triggered by neigh add
				if Ra.HostRoute == true {
					tk.LogIt(tk.LogError, "rt add host - %s:%s no neigh\n", Dst.String(), Zone)
					return RtNhErr, errors.New("rt-neigh host error")
				}

				r.Zone.Nh.NeighAdd(Na[i].NhAddr, Zone, NeighAttr{Na[i].LinkIndex, 0, hwmac})
				nh, _ = r.Zone.Nh.NeighFind(Na[i].NhAddr, Zone)
				if nh == nil {
					tk.LogIt(tk.LogError, "rt add - %s:%s no neigh\n", Dst.String(), Zone)
					return RtNhErr, errors.New("rt-neigh error")
				}
				newNhs = append(newNhs, nh)
			}
			rt.NextHops = append(rt.NextHops, nh)
		}

	} else {
		rt.TFlags |= RtTypeSelf
	}

	var tret int
	var tR *tk.TrieRoot
	if tk.IsNetIPv4(Dst.IP.String()) {
		tR = r.Trie4
	} else {
		tR = r.Trie6
	}
	if len(rt.NextHops) > 0 {
		tret = tR.AddTrie(Dst.String(), rt.NextHops[0])
	} else {
		tret = tR.AddTrie(Dst.String(), &rt.Attr.Ifi)
	}
	if tret != 0 {
		// Delete any neigbors created here
		for i := 0; i < len(newNhs); i++ {
			r.Zone.Nh.NeighDelete(newNhs[i].Addr, Zone)
		}
		tk.LogIt(tk.LogError, "rt add - %s:%s lpm add fail\n", Dst.String(), Zone)
		return RtTrieAddErr, errors.New("RT Trie Err")
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

	rt.DP(DpCreate)

	tk.LogIt(tk.LogDebug, "rt added - %s:%s\n", Dst.String(), Zone)

	return 0, nil
}

func (rt *Rt) rtClearDeps() {
	for _, obj := range rt.RtDepObjs {
		if f, ok := obj.(*FdbEnt); ok {
			f.FdbTun.rt = nil
			f.FdbTun.nh = nil
			f.unReach = true
		} else if ne, ok := obj.(*Neigh); ok {
			ne.Type &= ^NhRecursive
			ne.RHwMark = 0
			ne.Resolved = false
		}
	}
}

func (rt *Rt) rtRemoveDepObj(i int) []RtDepObj {
	copy(rt.RtDepObjs[i:], rt.RtDepObjs[i+1:])
	return rt.RtDepObjs[:len(rt.RtDepObjs)-1]
}

// RtDelete - Delete a route
func (r *RtH) RtDelete(Dst net.IPNet, Zone string) (int, error) {
	key := RtKey{Dst.String(), Zone}

	rt, found := r.RtMap[key]
	if found == false {
		tk.LogIt(tk.LogError, "rt delete - %s:%s not found\n", Dst.String(), Zone)
		return RtNoEntErr, errors.New("no such route")
	}

	// Take care of any dependencies on this route object
	rt.rtClearDeps()

	// UnPair route from related neighbor
	//if rt.TFlags & RT_TYPE_HOST != RT_TYPE_HOST {
	for _, nh := range rt.NextHops {
		r.Zone.Nh.NeighUnPairRt(nh, rt)
	}
	//}

	var tR *tk.TrieRoot
	if tk.IsNetIPv4(Dst.IP.String()) {
		tR = r.Trie4
	} else {
		tR = r.Trie6
	}
	tret := tR.DelTrie(Dst.String())
	if tret != 0 {
		tk.LogIt(tk.LogError, "rt delete - %s:%s lpm not found\n", Dst.String(), Zone)
		return RtTrieDelErr, errors.New("rt-lpm delete error")
	}

	delete(r.RtMap, rt.Key)
	defer r.HwMark.PutCounter(rt.HwMark)

	rt.DP(DpRemove)

	tk.LogIt(tk.LogDebug, "rt deleted - %s:%s\n", Dst.String(), Zone)

	return 0, nil
}

// RtDeleteByPort - Delete a route which has specified port association
func (r *RtH) RtDeleteByPort(port string) (int, error) {
	for _, rte := range r.RtMap {
		if rte.Attr.HostRoute {
			continue
		}
		_, dst, err := net.ParseCIDR(rte.Key.RtCidr)
		if err != nil {
			continue
		}
		for _, nh := range rte.NextHops {
			if nh.OifPort.Name == port {
				r.RtDelete(*dst, r.Zone.Name)
			}
		}
	}
	return 0, nil
}

// Rt2String - stringify the rt entry
func Rt2String(rt *Rt) string {
	var tStr string
	if rt.TFlags&RtTypeDyn == RtTypeDyn {
		tStr += fmt.Sprintf("Dyn")
	} else {
		tStr += fmt.Sprintf("Static")
	}
	if rt.TFlags&RtTypeInd == RtTypeInd {
		tStr += fmt.Sprintf(",In")
	} else {
		tStr += fmt.Sprintf(",Dr")
	}
	if rt.TFlags&RtTypeSelf == RtTypeSelf {
		tStr += fmt.Sprintf(",Self")
	}
	if rt.TFlags&RtTypeHost == RtTypeHost {
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

// Rts2String - Format rt entries to a string
func (r *RtH) Rts2String(it IterIntf) error {
	for _, r := range r.RtMap {
		rtBuf := Rt2String(r)
		it.NodeWalker(rtBuf)
	}
	return nil
}

// RtDestructAll - Destroy all rt entries
func (r *RtH) RtDestructAll() {
	for _, rt := range r.RtMap {
		_, dst, err := net.ParseCIDR(rt.Key.RtCidr)
		if err == nil {
			r.RtDelete(*dst, rt.Key.Zone)
		}
	}
	return
}

// RoutesSync - grab statistics for a rt entry
func (r *RtH) RoutesSync() {
	for _, rt := range r.RtMap {
		if rt.Stat.Packets != 0 {
			rts := Rt2String(rt)
			fmt.Printf("%s: pc %v bc %v\n", rts, rt.Stat.Packets, rt.Stat.Bytes)
		}
		rt.DP(DpStatsGet)
	}
}

// RoutesTicker - a ticker for this subsystem
func (r *RtH) RoutesTicker() {
	r.RoutesSync()
}

// RtGetNhHwMark - get the rt-entry's neighbor identifier
func (rt *Rt) RtGetNhHwMark() int {
	if len(rt.NextHops) > 0 {
		return rt.NextHops[0].HwMark
	}
	return -1
}

// DP - Sync state of route entities to data-path
func (rt *Rt) DP(work DpWorkT) int {

	_, rtNet, err := net.ParseCIDR(rt.Key.RtCidr)

	if err != nil {
		return -1
	}

	if work == DpStatsGet {
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
