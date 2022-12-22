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
	"fmt"
	"net"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// man names constants
const (
	MapNameCt4  = "CT4"
	MapNameCt6  = "CT6"
	MapNameNat4 = "NAT4"
	MapNameBD   = "BD"
	MapNameRxBD = "RXBD"
	MapNameTxBD = "TXBD"
	MapNameRt4  = "RT4"
	MapNameULCL = "ULCL"
	MapNameIpol = "IPOL"
	MapNameFw4  = "FW4"
)

// error codes
const (
	DpErrBase = iota - L3ErrBase - 1000
	DpWqUnkErr
)

// DpWorkT - type of requested work
type DpWorkT uint8

// dp work codes
const (
	DpCreate DpWorkT = iota + 1
	DpRemove
	DpChange
	DpStatsGet
	DpStatsClr
	DpMapGet
)

// DpStatusT - status of a dp work
type DpStatusT uint8

// dp work status codes
const (
	DpCreateErr DpStatusT = iota + 1
	DpRemoveErr
	DpChangeErr
	DpUknownErr
	DpInProgressErr
)

// maximum dp work queue lengths
const (
	DpWorkQLen = 1024
)

// MirrDpWorkQ - work queue entry for mirror operation
type MirrDpWorkQ struct {
	Work      DpWorkT
	Name      string
	HwMark    int
	MiPortNum int
	MiBD      int
	Status    *DpStatusT
}

// PortDpWorkQ - work queue entry for port operation
type PortDpWorkQ struct {
	Work       DpWorkT
	Status     *DpStatusT
	OsPortNum  int
	PortNum    int
	IngVlan    int
	SetBD      int
	SetZoneNum int
	Prop       cmn.PortProp
	SetMirr    int
	SetPol     int
	LoadEbpf   string
}

// L2AddrDpWorkQ - work queue entry for l2 address operation
type L2AddrDpWorkQ struct {
	Work    DpWorkT
	Status  *DpStatusT
	L2Addr  [6]uint8
	Tun     DpTunT
	NhNum   int
	PortNum int
	BD      int
	Tagged  int
}

// DpTunT - type of a dp tunnel
type DpTunT uint8

// tunnel type constants
const (
	DpTunVxlan DpTunT = iota + 1
	DpTunGre
	DpTunGtp
	DpTunStt
)

// RouterMacDpWorkQ - work queue entry for rt-mac operation
type RouterMacDpWorkQ struct {
	Work    DpWorkT
	Status  *DpStatusT
	L2Addr  [6]uint8
	PortNum int
	BD      int
	TunID   uint32
	TunType DpTunT
	NhNum   int
}

// NextHopDpWorkQ - work queue entry for nexthop operation
type NextHopDpWorkQ struct {
	Work        DpWorkT
	Status      *DpStatusT
	TunNh       bool
	TunID       uint32
	RIP         net.IP
	SIP         net.IP
	NNextHopNum int
	NextHopNum  int
	Resolved    bool
	DstAddr     [6]uint8
	SrcAddr     [6]uint8
	BD          int
}

// RouteDpWorkQ - work queue entry for rt operation
type RouteDpWorkQ struct {
	Work     DpWorkT
	Status   *DpStatusT
	ZoneNum  int
	Dst      net.IPNet
	RtType   int
	RtHwMark int
	NHwMark  int
}

// StatDpWorkQ - work queue entry for stat operation
type StatDpWorkQ struct {
	Work        DpWorkT
	Name        string
	HwMark      uint32
	Packets     *uint64
	Bytes       *uint64
	DropPackets *uint64
}

// TableDpWorkQ - work queue entry for map related operation
type TableDpWorkQ struct {
	Work DpWorkT
	Name string
}

// PolDpWorkQ - work queue entry for policer related operation
type PolDpWorkQ struct {
	Work   DpWorkT
	Name   string
	HwMark int
	Cir    uint64
	Pir    uint64
	Cbs    uint64
	Ebs    uint64
	Color  bool
	Srt    bool
	Status *DpStatusT
}

// FwOpT - type of firewall operation
type FwOpT uint8

// Fw type constants
const (
	DpFwDrop FwOpT = iota + 1
	DpFwFwd
	DpFwRdr
	DpFwTrap
)

// FwDpWorkQ - work queue entry for fw related operation
type FwDpWorkQ struct {
	Work     DpWorkT
	Status   *DpStatusT
	ZoneNum  int
	SrcIP    net.IPNet
	DstIP    net.IPNet
	L4SrcMin uint16
	L4SrcMax uint16
	L4DstMin uint16
	L4DstMax uint16
	Port     uint16
	Pref     uint16
	Proto    uint8
	HwMark   int
	FwType   FwOpT
	FwVal1   uint16
	FwVal2   uint32
}

// NatT - type of NAT
type NatT uint8

// nat type constants
const (
	DpSnat NatT = iota + 1
	DpDnat
	DpHsnat
	DpHdnat
	DpFullNat
)

// NatSel - type of nat end-point selection algorithm
type NatSel uint8

// nat selection algorithm constants
const (
	EpRR NatSel = iota + 1
	EpHash
	EpPrio
)

// NatEP - a nat end-point
type NatEP struct {
	XIP      net.IP
	RIP      net.IP
	XPort    uint16
	Weight   uint8
	InActive bool
}

// NatDpWorkQ - work queue entry for nat related operation
type NatDpWorkQ struct {
	Work      DpWorkT
	Status    *DpStatusT
	ZoneNum   int
	ServiceIP net.IP
	L4Port    uint16
	BlockNum  uint16
	DsrMode   bool
	Proto     uint8
	HwMark    int
	NatType   NatT
	EpSel     NatSel
	InActTo   uint64
	endPoints []NatEP
}

// DpCtInfo - representation of a datapath conntrack information
type DpCtInfo struct {
	DIP     net.IP
	SIP     net.IP
	Dport   uint16
	Sport   uint16
	Proto   string
	CState  string
	CAct    string
	Packets uint64
	Bytes   uint64
}

// UlClDpWorkQ - work queue entry for ul-cl filter related operation
type UlClDpWorkQ struct {
	Work   DpWorkT
	Status *DpStatusT
	MDip   net.IP
	MSip   net.IP
	mTeID  uint32
	Zone   int
	Qfi    uint8
	HwMark int
	TDip   net.IP
	TSip   net.IP
	TTeID  uint32
}

// Key - outputs a key string for given DpCtInfo pointer
func (ct *DpCtInfo) Key() string {
	str := fmt.Sprintf("%s%s%d%d%s", ct.DIP.String(), ct.SIP.String(), ct.Dport, ct.Sport, ct.Proto)
	return str
}

// DpRetT - an empty interface to represent immediate operation result
type DpRetT interface {
}

// DpHookInterface - represents a go interface which should be implemented to
// integrate with loxinet realm
type DpHookInterface interface {
	DpMirrAdd(*MirrDpWorkQ) int
	DpMirrDel(*MirrDpWorkQ) int
	DpPolAdd(*PolDpWorkQ) int
	DpPolDel(*PolDpWorkQ) int
	DpPortPropAdd(*PortDpWorkQ) int
	DpPortPropDel(*PortDpWorkQ) int
	DpL2AddrAdd(*L2AddrDpWorkQ) int
	DpL2AddrDel(*L2AddrDpWorkQ) int
	DpRouterMacAdd(*RouterMacDpWorkQ) int
	DpRouterMacDel(*RouterMacDpWorkQ) int
	DpNextHopAdd(*NextHopDpWorkQ) int
	DpNextHopDel(*NextHopDpWorkQ) int
	DpRouteAdd(*RouteDpWorkQ) int
	DpRouteDel(*RouteDpWorkQ) int
	DpNatLbRuleAdd(*NatDpWorkQ) int
	DpNatLbRuleDel(*NatDpWorkQ) int
	DpFwRuleAdd(w *FwDpWorkQ) int
	DpFwRuleDel(w *FwDpWorkQ) int
	DpStat(*StatDpWorkQ) int
	DpUlClAdd(w *UlClDpWorkQ) int
	DpUlClDel(w *UlClDpWorkQ) int
	DpTableGet(w *TableDpWorkQ) (DpRetT, error)
}

// DpH - datapath context container
type DpH struct {
	ToDpCh   chan interface{}
	FromDpCh chan interface{}
	ToFinCh  chan int
	DpHooks  DpHookInterface
}

// DpWorkOnPort - routine to work on a port work queue request
func (dp *DpH) DpWorkOnPort(pWq *PortDpWorkQ) DpRetT {
	if pWq.Work == DpCreate {
		return dp.DpHooks.DpPortPropAdd(pWq)
	} else if pWq.Work == DpRemove {
		return dp.DpHooks.DpPortPropDel(pWq)
	}

	return DpWqUnkErr
}

// DpWorkOnL2Addr - routine to work on a l2 addr work queue request
func (dp *DpH) DpWorkOnL2Addr(pWq *L2AddrDpWorkQ) DpRetT {
	if pWq.Work == DpCreate {
		return dp.DpHooks.DpL2AddrAdd(pWq)
	} else if pWq.Work == DpRemove {
		return dp.DpHooks.DpL2AddrDel(pWq)
	}

	return DpWqUnkErr
}

// DpWorkOnRtMac - routine to work on a rt-mac work queue request
func (dp *DpH) DpWorkOnRtMac(rmWq *RouterMacDpWorkQ) DpRetT {
	if rmWq.Work == DpCreate {
		return dp.DpHooks.DpRouterMacAdd(rmWq)
	} else if rmWq.Work == DpRemove {
		return dp.DpHooks.DpRouterMacDel(rmWq)
	}

	return DpWqUnkErr
}

// DpWorkOnNextHop - routine to work on a nexthop work queue request
func (dp *DpH) DpWorkOnNextHop(nhWq *NextHopDpWorkQ) DpRetT {
	if nhWq.Work == DpCreate {
		return dp.DpHooks.DpNextHopAdd(nhWq)
	} else if nhWq.Work == DpRemove {
		return dp.DpHooks.DpNextHopDel(nhWq)
	}

	return DpWqUnkErr
}

// DpWorkOnRoute - routine to work on a route work queue request
func (dp *DpH) DpWorkOnRoute(rtWq *RouteDpWorkQ) DpRetT {
	if rtWq.Work == DpCreate {
		return dp.DpHooks.DpRouteAdd(rtWq)
	} else if rtWq.Work == DpRemove {
		return dp.DpHooks.DpRouteDel(rtWq)
	}

	return DpWqUnkErr
}

// DpWorkOnNatLb - routine  to work on a NAT lb work queue request
func (dp *DpH) DpWorkOnNatLb(nWq *NatDpWorkQ) DpRetT {
	if nWq.Work == DpCreate {
		return dp.DpHooks.DpNatLbRuleAdd(nWq)
	} else if nWq.Work == DpRemove {
		return dp.DpHooks.DpNatLbRuleDel(nWq)
	}

	return DpWqUnkErr
}

// DpWorkOnUlCl - routine to work on a ulcl work queue request
func (dp *DpH) DpWorkOnUlCl(nWq *UlClDpWorkQ) DpRetT {
	if nWq.Work == DpCreate {
		return dp.DpHooks.DpUlClAdd(nWq)
	} else if nWq.Work == DpRemove {
		return dp.DpHooks.DpUlClDel(nWq)
	}

	return DpWqUnkErr
}

// DpWorkOnStat - routine to work on a stat work queue request
func (dp *DpH) DpWorkOnStat(nWq *StatDpWorkQ) DpRetT {
	return dp.DpHooks.DpStat(nWq)
}

// DpWorkOnTableOp - routine to work on a table work queue request
func (dp *DpH) DpWorkOnTableOp(nWq *TableDpWorkQ) (DpRetT, error) {
	return dp.DpHooks.DpTableGet(nWq)
}

// DpWorkOnPol - routine to work on a policer work queue request
func (dp *DpH) DpWorkOnPol(pWq *PolDpWorkQ) DpRetT {
	if pWq.Work == DpCreate {
		return dp.DpHooks.DpPolAdd(pWq)
	} else if pWq.Work == DpRemove {
		return dp.DpHooks.DpPolDel(pWq)
	}

	return DpWqUnkErr
}

// DpWorkOnMirr - routine to work on a mirror work queue request
func (dp *DpH) DpWorkOnMirr(mWq *MirrDpWorkQ) DpRetT {
	if mWq.Work == DpCreate {
		return dp.DpHooks.DpMirrAdd(mWq)
	} else if mWq.Work == DpRemove {
		return dp.DpHooks.DpMirrDel(mWq)
	}

	return DpWqUnkErr
}

// DpWorkOnFw - routine to work on a firewall work queue request
func (dp *DpH) DpWorkOnFw(fWq *FwDpWorkQ) DpRetT {
	if fWq.Work == DpCreate {
		return dp.DpHooks.DpFwRuleAdd(fWq)
	} else if fWq.Work == DpRemove {
		return dp.DpHooks.DpFwRuleDel(fWq)
	}

	return DpWqUnkErr
}

// DpWorkSingle - routine to work on a single dp work queue request
func DpWorkSingle(dp *DpH, m interface{}) DpRetT {
	var ret DpRetT
	switch mq := m.(type) {
	case *MirrDpWorkQ:
		ret = dp.DpWorkOnMirr(mq)
	case *PolDpWorkQ:
		ret = dp.DpWorkOnPol(mq)
	case *PortDpWorkQ:
		ret = dp.DpWorkOnPort(mq)
	case *L2AddrDpWorkQ:
		ret = dp.DpWorkOnL2Addr(mq)
	case *RouterMacDpWorkQ:
		ret = dp.DpWorkOnRtMac(mq)
	case *NextHopDpWorkQ:
		ret = dp.DpWorkOnNextHop(mq)
	case *RouteDpWorkQ:
		ret = dp.DpWorkOnRoute(mq)
	case *NatDpWorkQ:
		ret = dp.DpWorkOnNatLb(mq)
	case *UlClDpWorkQ:
		ret = dp.DpWorkOnUlCl(mq)
	case *StatDpWorkQ:
		ret = dp.DpWorkOnStat(mq)
	case *TableDpWorkQ:
		ret, _ = dp.DpWorkOnTableOp(mq)
	case *FwDpWorkQ:
		ret = dp.DpWorkOnFw(mq)
	default:
		tk.LogIt(tk.LogError, "unexpected type %T\n", mq)
		ret = DpWqUnkErr
	}
	return ret
}

// DpWorker - DP worker routine listening on a channel
func DpWorker(dp *DpH, f chan int, ch chan interface{}) {
	for {
		for n := 0; n < DpWorkQLen; n++ {
			select {
			case m := <-ch:
				DpWorkSingle(dp, m)
			case <-f:
				return
			default:
				continue
			}
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

// DpBrokerInit - initialize the DP broker subsystem
func DpBrokerInit(dph DpHookInterface) *DpH {
	nDp := new(DpH)

	nDp.ToDpCh = make(chan interface{}, DpWorkQLen)
	nDp.FromDpCh = make(chan interface{}, DpWorkQLen)
	nDp.ToFinCh = make(chan int)
	nDp.DpHooks = dph

	go DpWorker(nDp, nDp.ToFinCh, nDp.ToDpCh)

	return nDp
}

// DpMapGetCt4 - get DP conntrack information as a map
func (dp *DpH) DpMapGetCt4() []cmn.CtInfo {
	var CtInfoArr []cmn.CtInfo
	nTable := new(TableDpWorkQ)
	nTable.Work = DpMapGet
	nTable.Name = MapNameCt4

	ret, err := mh.dp.DpWorkOnTableOp(nTable)
	if err != nil {
		return nil
	}

	switch r := ret.(type) {
	case map[string]*DpCtInfo:
		for _, dCti := range r {
			cti := cmn.CtInfo{Dip: dCti.DIP, Sip: dCti.SIP, Dport: dCti.Dport, Sport: dCti.Sport,
				Proto: dCti.Proto, CState: dCti.CState, CAct: dCti.CAct,
				Pkts: dCti.Packets, Bytes: dCti.Bytes}
			CtInfoArr = append(CtInfoArr, cti)
		}
	}

	return CtInfoArr
}
