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
	"os"
	"runtime/debug"
	"sync"
	"time"

	tk "github.com/loxilb-io/loxilib"

	cmn "github.com/loxilb-io/loxilb/common"
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
	DpErrBase = iota - 103000
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
	DpStatsGetImm
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
	XSyncPort  = 22222
	DpTiVal    = 20
)

// MirrDpWorkQ - work queue entry for mirror operation
type MirrDpWorkQ struct {
	Work      DpWorkT
	Name      string
	Mark      int
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
	DpTunIPIP
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
	TunType     DpTunT
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
	Work    DpWorkT
	Status  *DpStatusT
	ZoneNum int
	Dst     net.IPNet
	RtType  int
	RtMark  int
	NMax    int
	NMark   [8]int
}

// StatDpWorkQ - work queue entry for stat operation
type StatDpWorkQ struct {
	Work        DpWorkT
	Name        string
	Mark        uint32
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
	Mark   int
	Cir    uint64
	Pir    uint64
	Cbs    uint64
	Ebs    uint64
	Color  bool
	Srt    bool
	Status *DpStatusT
}

// PeerDpWorkQ - work queue entry for peer association
type PeerDpWorkQ struct {
	Work   DpWorkT
	PeerIP net.IP
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
	Mark     int
	FwType   FwOpT
	FwVal1   uint16
	FwVal2   uint32
	FwRecord bool
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
	DpFullProxy
)

// NatSel - type of nat end-point selection algorithm
type NatSel uint8

// nat selection algorithm constants
const (
	EpRR NatSel = iota + 1
	EpHash
	EpPrio
	EpRRPersist
	EpLeastConn
	EpN2
)

// NatEP - a nat end-point
type NatEP struct {
	XIP      net.IP
	RIP      net.IP
	XPort    uint16
	Weight   uint8
	InActive bool
}

// SecT - type of SecT
type SecT uint8

// security type constants
const (
	DpTermHTTPS SecT = iota + 1
	DpE2EHTTPS
)

// NatDpWorkQ - work queue entry for nat related operation
type NatDpWorkQ struct {
	Work      DpWorkT
	Status    *DpStatusT
	ZoneNum   int
	ServiceIP net.IP
	L4Port    uint16
	BlockNum  uint16
	DsrMode   bool
	CsumDis   bool
	SecMode   SecT
	HostURL   string
	Proto     uint8
	Mark      int
	NatType   NatT
	EpSel     NatSel
	InActTo   uint64
	PersistTo uint64
	endPoints []NatEP
	secIP     []net.IP
}

// DpCtInfo - representation of a datapath conntrack information
type DpCtInfo struct {
	DIP     net.IP    `json:"dip"`
	SIP     net.IP    `json:"sip"`
	Dport   uint16    `json:"dport"`
	Sport   uint16    `json:"sport"`
	Proto   string    `json:"proto"`
	CState  string    `json:"cstate"`
	CAct    string    `json:"cact"`
	CI      string    `json:"ci"`
	Packets uint64    `json:"packets"`
	Bytes   uint64    `json:"bytes"`
	Deleted int       `json:"deleted"`
	PKey    []byte    `json:"pkey"`
	PVal    []byte    `json:"pval"`
	LTs     time.Time `json:"lts"`
	NTs     time.Time `json:"nts"`
	XSync   bool      `json:"xsync"`

	// LB Association Data
	ServiceIP  net.IP `json:"serviceip"`
	ServProto  string `json:"servproto"`
	L4ServPort uint16 `json:"l4servproto"`
	BlockNum   uint16 `json:"blocknum"`
	RuleID     uint32 `json:"ruleid"`
}

const (
	RPCTypeNetRPC = iota
	RPCTypeGRPC
)

type RPCHookInterface interface {
	RPCConnect(*DpPeer) int
	RPCClose(*DpPeer) int
	RPCReset(*DpPeer) int
	RPCSend(*DpPeer, string, any) (int, error)
}

// XSync - Remote sync peer information
type XSync struct {
	RemoteID int
	RPCState bool
	// For peer to peer RPC
	RPCType  int
	RPCHooks RPCHookInterface
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
	Mark   int
	TDip   net.IP
	TSip   net.IP
	TTeID  uint32
	Type   DpTunT
}

// SockVIPDpWorkQ - work queue entry for local VIP-port rewrite
type SockVIPDpWorkQ struct {
	Work   DpWorkT
	VIP    net.IP
	Port   uint16
	RwPort uint16
	Status *DpStatusT
}

// DpSyncOpT - Sync Operation type
type DpSyncOpT uint8

// Sync Operation type codes
const (
	DpSyncAdd DpSyncOpT = iota + 1
	DpSyncDelete
	DpSyncGet
	DpSyncBcast
)

// Key - outputs a key string for given DpCtInfo pointer
func (ct *DpCtInfo) Key() string {
	str := fmt.Sprintf("%s%s%d%d%s", ct.DIP.String(), ct.SIP.String(), ct.Dport, ct.Sport, ct.Proto)
	return str
}

// KeyState - outputs a key string for given DpCtInfo pointer with state info
func (ct *DpCtInfo) KeyState() string {
	str := fmt.Sprintf("%s%s%d%d%s-%s", ct.DIP.String(), ct.SIP.String(), ct.Dport, ct.Sport, ct.Proto, ct.CState)
	return str
}

// String - stringify the given DpCtInfo
func (ct *DpCtInfo) String() string {
	str := fmt.Sprintf("%s:%d->%s:%d (%s), ", ct.SIP.String(), ct.Sport, ct.DIP.String(), ct.Dport, ct.Proto)
	str += fmt.Sprintf("%s:%s [%v:%v]", ct.CState, ct.CAct, ct.Packets, ct.Bytes)
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
	DpCtAdd(w *DpCtInfo) int
	DpCtDel(w *DpCtInfo) int
	DpSockVIPAdd(w *SockVIPDpWorkQ) int
	DpSockVIPDel(w *SockVIPDpWorkQ) int
	DpTableGC()
	DpCtGetAsync()
	DpGetLock()
	DpRelLock()
	DpEbpfUnInit()
}

// DpPeer - Remote DP Peer information
type DpPeer struct {
	Peer net.IP
	//Client *rpc.Client
	Client interface{}
}

// DpH - datapath context container
type DpH struct {
	ToDpCh   chan interface{}
	FromDpCh chan interface{}
	ToFinCh  chan int
	DpHooks  DpHookInterface
	SyncMtx  sync.RWMutex
	Peers    []DpPeer
	RPC      *XSync
	Remotes  []XSync
}

// DpXsyncRPCReset - Routine to reset Sunc RPC Client connections
func (dp *DpH) DpXsyncRPCReset() int {
	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()
	for idx := range mh.dp.Peers {
		pe := &mh.dp.Peers[idx]
		dp.RPC.RPCHooks.RPCReset(pe)
	}
	return 0
}

// DpXsyncInSync - Routine to check if remote peer is in sync
func (dp *DpH) DpXsyncInSync() bool {
	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()

	return len(dp.Remotes) >= len(mh.has.NodeMap)
}

// WaitXsyncReady - Routine to wait till it ready for syncing the peer entity
func (dp *DpH) WaitXsyncReady(who string) {
	begin := time.Now()
	for {
		if dp.DpXsyncInSync() {
			return
		}
		if time.Duration(time.Since(begin).Seconds()) >= 90 {
			return
		}
		tk.LogIt(tk.LogDebug, "%s:waiting for Xsync..\n", who)
		time.Sleep(2 * time.Second)
	}
}

// DpXsyncRPC - Routine for syncing connection information with peers
func (dp *DpH) DpXsyncRPC(op DpSyncOpT, arg interface{}) int {
	var ret int
	var err error

	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()

	if len(mh.has.NodeMap) != len(mh.dp.Peers) {
		return -1
	}

	rpcRetries := 0
	rpcErr := false
	var cti *DpCtInfo
	var blkCti []DpCtInfo

	switch na := arg.(type) {
	case *DpCtInfo:
		cti = na
	case []DpCtInfo:
		blkCti = na
	}

	for idx := range mh.dp.Peers {
	restartRPC:
		pe := &mh.dp.Peers[idx]
		if pe.Client == nil {
			ret = dp.RPC.RPCHooks.RPCConnect(pe)
			if ret != 0 {
				rpcErr = true
				continue
			}
		}

		reply := 0
		rpcCallStr := ""
		if op == DpSyncAdd || op == DpSyncBcast {
			if len(blkCti) > 0 {
				rpcCallStr = "XSync.DpWorkOnBlockCtAdd"
			} else {
				rpcCallStr = "XSync.DpWorkOnCtAdd"
			}
		} else if op == DpSyncDelete {
			if len(blkCti) > 0 {
				rpcCallStr = "XSync.DpWorkOnBlockCtDelete"
			} else {
				rpcCallStr = "XSync.DpWorkOnCtDelete"
			}
		} else if op == DpSyncGet {
			rpcCallStr = "XSync.DpWorkOnCtGet"
		} else {
			return -1
		}

		if op == DpSyncAdd || op == DpSyncDelete || op == DpSyncBcast {
			if op != DpSyncBcast {
				if cti == nil && len(blkCti) <= 0 {
					return -1
				}

				var tmpCti *DpCtInfo
				if cti == nil {
					tmpCti = &blkCti[0]
				} else {
					tmpCti = cti
				}
				// FIXME - There is a race condition here
				cIState, _ := mh.has.CIStateGetInst(tmpCti.CI)
				if cIState != "MASTER" {
					return 0
				}
			}
			if cti != nil {
				reply, err = dp.RPC.RPCHooks.RPCSend(pe, rpcCallStr, *cti)
			} else {
				reply, err = dp.RPC.RPCHooks.RPCSend(pe, rpcCallStr, blkCti)
			}
		} else {
			async := 1
			reply, err = dp.RPC.RPCHooks.RPCSend(pe, rpcCallStr, int32(async))
		}

		if err != nil {
			tk.LogIt(tk.LogError, "XSync call failed(%s)\n", err)
			rpcErr = true
			pe.Client = nil
			rpcRetries++
			if rpcRetries < 2 {
				goto restartRPC
			}
		}
		if reply != 0 {
			tk.LogIt(tk.LogError, "Xsync server returned error (%d)\n", reply)
			rpcErr = true
		}
	}

	if rpcErr {
		return -1
	}
	return 0
}

// DpBrokerInit - initialize the DP broker subsystem
func DpBrokerInit(dph DpHookInterface, rpcMode int) *DpH {
	nDp := new(DpH)

	nDp.ToDpCh = make(chan interface{}, DpWorkQLen)
	nDp.FromDpCh = make(chan interface{}, DpWorkQLen)
	nDp.ToFinCh = make(chan int)
	nDp.DpHooks = dph
	nDp.RPC = new(XSync)

	nDp.RPC.RPCType = rpcMode
	if rpcMode == RPCTypeNetRPC {
		nDp.RPC.RPCHooks = &netRPCClient{}
	} else {
		nDp.RPC.RPCHooks = &gRPCClient{}
	}

	go DpWorker(nDp, nDp.ToFinCh, nDp.ToDpCh)

	return nDp
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

// DpWorkOnSockVIP - routine to work on local VIP-port rewrite
func (dp *DpH) DpWorkOnSockVIP(vsWq *SockVIPDpWorkQ) DpRetT {
	if vsWq.Work == DpCreate {
		return dp.DpHooks.DpSockVIPAdd(vsWq)
	} else if vsWq.Work == DpRemove {
		return dp.DpHooks.DpSockVIPDel(vsWq)
	}

	return DpWqUnkErr
}

// DpWorkOnPeerOp - routine to work on a peer request for clustering
func (dp *DpH) DpWorkOnPeerOp(pWq *PeerDpWorkQ) DpRetT {
	if pWq.Work == DpCreate {
		var newPeer DpPeer
		for _, pe := range dp.Peers {
			if pe.Peer.Equal(pWq.PeerIP) {
				return DpCreateErr
			}
		}
		newPeer.Peer = pWq.PeerIP
		dp.Peers = append(dp.Peers, newPeer)
		tk.LogIt(tk.LogInfo, "Added cluster-peer %s\n", newPeer.Peer.String())
		return 0
	} else if pWq.Work == DpRemove {
		for idx := range dp.Peers {
			pe := &dp.Peers[idx]
			if pe.Peer.Equal(pWq.PeerIP) {
				if pe.Client != nil {
					dp.RPC.RPCHooks.RPCClose(pe)
				}
				dp.Peers = append(dp.Peers[:idx], dp.Peers[idx+1:]...)
				tk.LogIt(tk.LogInfo, "Deleted cluster-peer %s\n", pWq.PeerIP.String())
				return 0
			}
		}
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
	case *PeerDpWorkQ:
		ret = dp.DpWorkOnPeerOp(mq)
	case *SockVIPDpWorkQ:
		ret = dp.DpWorkOnSockVIP(mq)
	default:
		tk.LogIt(tk.LogError, "unexpected type %T\n", mq)
		ret = DpWqUnkErr
	}
	return ret
}

// DpWorker - DP worker routine listening on a channel
func DpWorker(dp *DpH, f chan int, ch chan interface{}) {
	// Stack trace logger
	defer func() {
		if e := recover(); e != nil {
			tk.LogIt(tk.LogCritical, "%s: %s", e, debug.Stack())
		}
		if mh.dp != nil {
			mh.dp.DpHooks.DpEbpfUnInit()
		}
		os.Exit(1)
	}()
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

// DpMapGetCt4 - get DP conntrack information as a map
func (dp *DpH) DpMapGetCt4() []cmn.CtInfo {
	var CtInfoArr []cmn.CtInfo
	var servName string

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
			servName = "-"
			mh.mtx.Lock()
			rule := mh.zr.Rules.GetNatLbRuleByID(dCti.RuleID)
			mh.mtx.Unlock()
			if rule != nil {
				servName = rule.name
			}
			cti := cmn.CtInfo{Dip: dCti.DIP, Sip: dCti.SIP, Dport: dCti.Dport, Sport: dCti.Sport,
				Proto: dCti.Proto, CState: dCti.CState, CAct: dCti.CAct,
				Pkts: dCti.Packets, Bytes: dCti.Bytes, ServiceName: servName}
			CtInfoArr = append(CtInfoArr, cti)
		}
	}

	dp.DpHooks.DpTableGC()

	return CtInfoArr
}
