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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"time"
)

const (
	MAP_NAME_CT4  = "CT4"
	MAP_NAME_CT6  = "CT6"
	MAP_NAME_NAT4 = "NAT4"
	MAP_NAME_BD   = "BD"
	MAP_NAME_RXBD = "RXBD"
	MAP_NAME_TXBD = "TXBD"
	MAP_NAME_RT4  = "RT4"
	MAP_NAME_ULCL = "ULCL"
	MAP_NAME_IPOL = "IPOL"
)

const (
	DP_ERR_BASE = iota - L3_ERR_BASE - 1000
	DP_WQ_UNK_ERR
)

type DpWorkT uint8

const (
	DP_CREATE DpWorkT = iota + 1
	DP_REMOVE
	DP_CHANGE
	DP_STATS_GET
	DP_STATS_CLR
	DP_TABLE_GET
)

type DpStatusT uint8

const (
	DP_CREATE_ERR DpStatusT = iota + 1
	DP_REMOVE_ERR
	DP_CHANGE_ERR
	DP_UNKNOWN_ERR
	DP_INPROGRESS_ERR
)

const (
	DP_WORKQ_LEN = 1024
)

type MirrDpWorkQ struct {
	Work       DpWorkT
	Name       string
	HwMark     int
	MiPortNum  int
	MiBD       int
	Status     *DpStatusT
}

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

type L2AddrDpWorkQ struct {
	Work    DpWorkT
	Status  *DpStatusT
	l2Addr  [6]uint8
	Tun     DpTunT
	NhNum   int
	PortNum int
	BD      int
	Tagged  int
}

type DpTunT uint8

const (
	DP_TUN_VXLAN DpTunT = iota + 1
	DP_TUN_GRE
	DP_TUN_GTP
	DP_TUN_STT
)

type RouterMacDpWorkQ struct {
	Work    DpWorkT
	Status  *DpStatusT
	l2Addr  [6]uint8
	PortNum int
	BD      int
	TunId   uint32
	TunType DpTunT
	NhNum   int
}

type NextHopDpWorkQ struct {
	Work        DpWorkT
	Status      *DpStatusT
	tunNh       bool
	tunID       uint32
	rIP         net.IP
	sIP         net.IP
	nNextHopNum int
	nextHopNum  int
	resolved    bool
	dstAddr     [6]uint8
	srcAddr     [6]uint8
	BD          int
}

type RouteDpWorkQ struct {
	Work     DpWorkT
	Status   *DpStatusT
	ZoneNum  int
	Dst      net.IPNet
	RtType   int
	RtHwMark int
	NHwMark  int
}

type StatDpWorkQ struct {
	Work         DpWorkT
	Name         string
	HwMark       uint32
	Packets      *uint64
	Bytes        *uint64
	DropPackets  *uint64
}

type TableDpWorkQ struct {
	Work DpWorkT
	Name string
}

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

type NatT uint8

const (
	DP_SNAT NatT = iota + 1
	DP_DNAT
	DP_HSNAT
	DP_HDNAT
)

type NatSel uint8

const (
	EP_RR NatSel = iota + 1
	EP_HASH
	EP_PRIO
)

type NatEP struct {
	xIP      net.IP
	xPort    uint16
	weight   uint8
	inActive bool
}

type NatDpWorkQ struct {
	Work      DpWorkT
	Status    *DpStatusT
	ZoneNum   int
	ServiceIP net.IP
	L4Port    uint16
	Proto     uint8
	HwMark    int
	NatType   NatT
	EpSel     NatSel
	endPoints []NatEP
}

type DpCtInfo struct {
	dip     net.IP
	sip     net.IP
	dport   uint16
	sport   uint16
	proto   string
	cState  string
	cAct    string
	packets uint64
	bytes   uint64
}

type UlClDpWorkQ struct {
	Work   DpWorkT
	Status *DpStatusT
	mDip   net.IP
	mSip   net.IP
	mTeID  uint32
	Zone   int
	Qfi    uint8
	HwMark int
	tDip   net.IP
	tSip   net.IP
	tTeID  uint32
}

func (ct *DpCtInfo) Key() string {
	str := fmt.Sprintf("%s%s%d%d%s", ct.dip.String(), ct.sip.String(), ct.dport, ct.sport, ct.proto)
	return str
}

type DpRetT interface {
}

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
	DpStat(*StatDpWorkQ) int
	DpUlClAdd(w *UlClDpWorkQ) int
	DpUlClDel(w *UlClDpWorkQ) int
	DpTableGet(w *TableDpWorkQ) (error, DpRetT)
}

type DpH struct {
	ToDpCh   chan interface{}
	FromDpCh chan interface{}
	ToFinCh  chan int
	DpHooks  DpHookInterface
}

func (dp *DpH) DpWorkOnPort(pWq *PortDpWorkQ) DpRetT {
	if pWq.Work == DP_CREATE {
		return dp.DpHooks.DpPortPropAdd(pWq)
	} else if pWq.Work == DP_REMOVE {
		return dp.DpHooks.DpPortPropDel(pWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnL2Addr(pWq *L2AddrDpWorkQ) DpRetT {
	if pWq.Work == DP_CREATE {
		return dp.DpHooks.DpL2AddrAdd(pWq)
	} else if pWq.Work == DP_REMOVE {
		return dp.DpHooks.DpL2AddrDel(pWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnRtMac(rmWq *RouterMacDpWorkQ) DpRetT {
	if rmWq.Work == DP_CREATE {
		return dp.DpHooks.DpRouterMacAdd(rmWq)
	} else if rmWq.Work == DP_REMOVE {
		return dp.DpHooks.DpRouterMacDel(rmWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnNextHop(nhWq *NextHopDpWorkQ) DpRetT {
	if nhWq.Work == DP_CREATE {
		return dp.DpHooks.DpNextHopAdd(nhWq)
	} else if nhWq.Work == DP_REMOVE {
		return dp.DpHooks.DpNextHopDel(nhWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnRoute(rtWq *RouteDpWorkQ) DpRetT {
	if rtWq.Work == DP_CREATE {
		return dp.DpHooks.DpRouteAdd(rtWq)
	} else if rtWq.Work == DP_REMOVE {
		return dp.DpHooks.DpRouteDel(rtWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnNatLb(nWq *NatDpWorkQ) DpRetT {
	if nWq.Work == DP_CREATE {
		return dp.DpHooks.DpNatLbRuleAdd(nWq)
	} else if nWq.Work == DP_REMOVE {
		return dp.DpHooks.DpNatLbRuleDel(nWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnUlCl(nWq *UlClDpWorkQ) DpRetT {
	if nWq.Work == DP_CREATE {
		return dp.DpHooks.DpUlClAdd(nWq)
	} else if nWq.Work == DP_REMOVE {
		return dp.DpHooks.DpUlClDel(nWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnStat(nWq *StatDpWorkQ) DpRetT {
	return dp.DpHooks.DpStat(nWq)
}

func (dp *DpH) DpWorkOnTableOp(nWq *TableDpWorkQ) (error, DpRetT) {
	return dp.DpHooks.DpTableGet(nWq)
}

func (dp *DpH) DpWorkOnPol(pWq *PolDpWorkQ) DpRetT {
	if pWq.Work == DP_CREATE {
		return dp.DpHooks.DpPolAdd(pWq)
	} else if pWq.Work == DP_REMOVE {
		return dp.DpHooks.DpPolDel(pWq)
	}

	return DP_WQ_UNK_ERR
}

func (dp *DpH) DpWorkOnMirr(mWq *MirrDpWorkQ) DpRetT {
	if mWq.Work == DP_CREATE {
		return dp.DpHooks.DpMirrAdd(mWq)
	} else if mWq.Work == DP_REMOVE {
		return dp.DpHooks.DpMirrDel(mWq)
	}

	return DP_WQ_UNK_ERR
}

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
	default:
		tk.LogIt(tk.LOG_ERROR, "unexpected type %T\n", mq)
		ret = DP_WQ_UNK_ERR
	}
	return ret
}

func DpWorker(dp *DpH, f chan int, ch chan interface{}) {
	for {
		for n := 0; n < DP_WORKQ_LEN; n++ {
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

func DpBrokerInit(dph DpHookInterface) *DpH {
	nDp := new(DpH)

	nDp.ToDpCh = make(chan interface{}, DP_WORKQ_LEN)
	nDp.FromDpCh = make(chan interface{}, DP_WORKQ_LEN)
	nDp.ToFinCh = make(chan int)
	nDp.DpHooks = dph

	go DpWorker(nDp, nDp.ToFinCh, nDp.ToDpCh)

	return nDp
}

func (dp *DpH) DpMapGetCt4() []cmn.CtInfo {
	var CtInfoArr []cmn.CtInfo
	nTable := new(TableDpWorkQ)
	nTable.Work = DP_TABLE_GET
	nTable.Name = MAP_NAME_CT4

	err, ret := mh.dp.DpWorkOnTableOp(nTable)
	if err != nil {
		return nil
	}

	switch r := ret.(type) {
	case map[string]*DpCtInfo:
		for _, dCti := range r {
			cti := cmn.CtInfo{Dip: dCti.dip, Sip: dCti.sip, Dport: dCti.dport, Sport: dCti.sport,
				Proto: dCti.proto, CState: dCti.cState, CAct: dCti.cAct,
				Pkts: dCti.packets, Bytes: dCti.bytes}
			CtInfoArr = append(CtInfoArr, cti)
			fmt.Println(CtInfoArr)
		}
	}

	return CtInfoArr
}
