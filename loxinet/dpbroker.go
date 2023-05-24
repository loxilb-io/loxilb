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
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"sync"
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
	NMark   int
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
	CsumDis   bool
	Proto     uint8
	Mark      int
	NatType   NatT
	EpSel     NatSel
	InActTo   uint64
	endPoints []NatEP
	secIP     []net.IP
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
	CI      string
	Packets uint64
	Bytes   uint64
	Deleted int
	PKey    []byte
	PVal    []byte
	LTs     time.Time
	NTs     time.Time
	XSync   bool

	// LB Association Data
	ServiceIP  net.IP
	ServProto  string
	L4ServPort uint16
	BlockNum   uint16
}

type XSync struct {
	RemoteID int
	RPCState bool
	// For peer to peer RPC
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
	DpCtGetAsync()
}

type DpPeer struct {
	Peer   net.IP
	Client *rpc.Client
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

// dialHTTPPath connects to an HTTP RPC server
// at the specified network address and path.
// This is based on rpc package's DialHTTPPath but with added timeout
func dialHTTPPath(network, address, path string) (*rpc.Client, error) {
	var connected = "200 Connected to Go RPC"
	timeOut := 2 * time.Second

	conn, err := net.DialTimeout(network, address, timeOut)
	if err != nil {
		return nil, err
	}
	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return rpc.NewClient(conn), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  network + " " + address,
		Addr: nil,
		Err:  err,
	}
}

func (dp *DpH) DpXsyncRpcReset() int {
	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()
	for idx := range mh.dp.Peers {
		pe := &mh.dp.Peers[idx]
		if pe.Client != nil {
			pe.Client.Close()
			pe.Client = nil
		}
		if pe.Client == nil {
			cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)
			pe.Client, _ = dialHTTPPath("tcp", cStr, rpc.DefaultRPCPath)
			if pe.Client == nil {
				return -1
			}
			tk.LogIt(tk.LogInfo, "XSync RPC - %s :Reset\n", cStr)
		}
	}
	return 0
}

func (dp *DpH) DpXsyncInSync() bool {
	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()

	if len(dp.Remotes) >= len(mh.has.NodeMap) {
		return true
	}
	return false
}

func (dp *DpH) WaitXsyncReady(who string) {
	begin := time.Now()
	for {
		if dp.DpXsyncInSync() {
			return
		}
		if time.Duration(time.Now().Sub(begin).Seconds()) >= 90 {
			return
		}
		tk.LogIt(tk.LogDebug, "%s:waiting for Xsync..\n", who)
		time.Sleep(2 * time.Second)
	}
}

func (dp *DpH) DpXsyncRpc(op DpSyncOpT, cti *DpCtInfo) int {
	var reply int
	timeout := 2 * time.Second
	dp.SyncMtx.Lock()
	defer dp.SyncMtx.Unlock()

	if len(mh.has.NodeMap) != len(mh.dp.Peers) {
		return -1
	}

	rpcRetries := 0
	rpcErr := false

	for idx := range mh.dp.Peers {
	restartRPC:
		pe := &mh.dp.Peers[idx]
		if pe.Client == nil {
			cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)
			var err error
			pe.Client, err = dialHTTPPath("tcp", cStr, rpc.DefaultRPCPath)
			if pe.Client == nil {
				tk.LogIt(tk.LogInfo, "XSync RPC - %s :Fail(%s)\n", cStr, err)
				rpcErr = true
				continue
			}
			tk.LogIt(tk.LogInfo, "XSync RPC - %s :Connected\n", cStr)
		}

		reply = 0
		rpcCallStr := ""
		if op == DpSyncAdd || op == DpSyncBcast {
			rpcCallStr = "XSync.DpWorkOnCtAdd"
		} else if op == DpSyncDelete {
			rpcCallStr = "XSync.DpWorkOnCtDelete"
		} else if op == DpSyncGet {
			rpcCallStr = "XSync.DpWorkOnCtGet"
		} else {
			return -1
		}

		var call *rpc.Call
		if op == DpSyncAdd || op == DpSyncDelete || op == DpSyncBcast {
			if cti == nil {
				return -1
			}
			if op != DpSyncBcast {
				// FIXME - There is a race condition here
				cIState, _ := mh.has.CIStateGetInst(cti.CI)
				if cIState != "MASTER" {
					return 0
				}
			}
			call = pe.Client.Go(rpcCallStr, *cti, &reply, make(chan *rpc.Call, 1))
		} else {
			async := 1
			call = pe.Client.Go(rpcCallStr, async, &reply, make(chan *rpc.Call, 1))
		}
		select {
		case <-time.After(timeout):
			tk.LogIt(tk.LogError, "rpc call timeout(%v)\n", timeout)
			if pe.Client != nil {
				pe.Client.Close()
			}
			pe.Client = nil
			rpcRetries++
			if rpcRetries < 2 {
				goto restartRPC
			}
			rpcErr = true
		case resp := <-call.Done:
			if resp != nil && resp.Error != nil {
				tk.LogIt(tk.LogError, "rpc call failed(%s)\n", resp.Error)
				rpcErr = true
			}
			if reply != 0 {
				rpcErr = true
			}
		}
	}

	if rpcErr {
		return -1
	}
	return 0
}

// DpBrokerInit - initialize the DP broker subsystem
func DpBrokerInit(dph DpHookInterface) *DpH {
	nDp := new(DpH)

	nDp.ToDpCh = make(chan interface{}, DpWorkQLen)
	nDp.FromDpCh = make(chan interface{}, DpWorkQLen)
	nDp.ToFinCh = make(chan int)
	nDp.DpHooks = dph
	nDp.RPC = new(XSync)

	go DpWorker(nDp, nDp.ToFinCh, nDp.ToDpCh)

	return nDp
}

// DpWorkOnCtAdd - Add a CT entry from remote
func (xs *XSync) DpWorkOnCtAdd(cti DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	if cti.Proto == "xsync" {
		mh.dp.SyncMtx.Lock()
		defer mh.dp.SyncMtx.Unlock()

		for idx := range mh.dp.Remotes {
			r := &mh.dp.Remotes[idx]
			if r.RemoteID == int(cti.Sport) {
				r.RPCState = true
				*ret = 0
				return nil
			}
		}

		r := XSync{RemoteID: int(cti.Sport), RPCState: true}
		mh.dp.Remotes = append(mh.dp.Remotes, r)

		tk.LogIt(tk.LogDebug, "RPC - CT Xsync Remote-%v\n", cti.Sport)

		*ret = 0
		return nil
	}

	tk.LogIt(tk.LogDebug, "RPC - CT Add %s\n", cti.Key())
	r := mh.dp.DpHooks.DpCtAdd(&cti)
	*ret = r
	return nil
}

// DpWorkOnCtDelete - Delete a CT entry from remote
func (xs *XSync) DpWorkOnCtDelete(cti DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}
	tk.LogIt(tk.LogDebug, "RPC -  CT Del %s\n", cti.Key())
	r := mh.dp.DpHooks.DpCtDel(&cti)
	*ret = r
	return nil
}

// DpWorkOnCtGet - Get all CT entries asynchronously
func (xs *XSync) DpWorkOnCtGet(async int, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	// Most likely need to reset reverse rpc channel
	mh.dp.DpXsyncRpcReset()

	tk.LogIt(tk.LogDebug, "RPC -  CT Get %d\n", async)
	mh.dp.DpHooks.DpCtGetAsync()
	*ret = 0

	return nil
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
					pe.Client.Close()
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
