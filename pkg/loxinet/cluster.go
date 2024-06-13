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
	nlp "github.com/loxilb-io/loxilb/api/loxinlp"
	cmn "github.com/loxilb-io/loxilb/common"
	bfd "github.com/loxilb-io/loxilb/pkg/proto"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"os"
	"time"
)

// error codes for cluster module
const (
	CIErrBase = iota - 90000
	CIModErr
	CIStateErr
)

// ClusterInstance - Struct for Cluster Instance information
type ClusterInstance struct {
	State    int
	StateStr string
	Vip      net.IP
}

// ClusterNode - Struct for Cluster Node Information
type ClusterNode struct {
	Addr   net.IP
	Status DpStatusT
}

// CIKAArgs - Struct for cluster BFD args
type CIKAArgs struct {
	SpawnKa  bool
	RemoteIP net.IP
	SourceIP net.IP
	Interval int64
}

// CIStateH - Cluster context handler
type CIStateH struct {
	SpawnKa    bool
	RemoteIP   net.IP
	SourceIP   net.IP
	Interval   int64
	ClusterMap map[string]*ClusterInstance
	StateMap   map[string]int
	NodeMap    map[string]*ClusterNode
	Bs         *bfd.Struct
}

func (ci *CIStateH) BFDSessionNotify(instance string, remote string, ciState string) {
	var sm cmn.HASMod

	sm.Instance = instance
	sm.State = ciState
	sm.Vip = net.ParseIP("0.0.0.0")
	tk.LogIt(tk.LogInfo, "ci-change instance %s - state %s vip %v\n", instance, ciState, sm.Vip)
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	ci.CIStateUpdate(sm)
}

func (ci *CIStateH) startBFDProto(bfdSessConfigArgs bfd.ConfigArgs) {

	if ci.Bs == nil {
		return
	}

	mh.dp.WaitXsyncReady("ka")
	// We need some cool-off period for loxilb to self sync-up in the cluster
	time.Sleep(KAInitTiVal * time.Second)

	txInterval := uint32(bfd.BFDDflSysTXIntervalUs)
	if ci.Interval != 0 && ci.Interval >= bfd.BFDMinSysTXIntervalUs {
		txInterval = uint32(ci.Interval)
	}

	err := ci.Bs.BFDAddRemote(bfdSessConfigArgs, ci)
	if err != nil {
		tk.LogIt(tk.LogCritical, "KA - Cant add BFD remote: %s\n", err.Error())
		//os.Exit(1)
		return
	}
	tk.LogIt(tk.LogInfo, "KA - Added BFD remote %s:%s:%vus\n", ci.RemoteIP.String(), ci.SourceIP.String(), txInterval)
}

// CITicker - Periodic ticker for Cluster module
func (h *CIStateH) CITicker() {
	// Nothing to do currently
}

// CISpawn - Spawn CI application
func (ci *CIStateH) CISpawn() {
	bs := bfd.StructNew(3784)
	ci.Bs = bs
	if _, err := os.Stat("/etc/loxilb/BFDconfig.txt"); !errors.Is(err, os.ErrNotExist) {
		nlp.ApplyBFDConfig()
		return
	}

	if ci.SpawnKa {
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: ci.RemoteIP.String(), SourceIP: ci.SourceIP.String(),
			Port: cmn.BFDPort, Interval: bfd.BFDMinSysTXIntervalUs, Multi: cmn.BFDDefRetryCount, Instance: cmn.CIDefault}
		go ci.startBFDProto(bfdSessConfigArgs)
	}
}

// CIInit - routine to initialize Cluster context
func CIInit(args CIKAArgs) *CIStateH {
	var nCIh = new(CIStateH)
	nCIh.StateMap = make(map[string]int)
	nCIh.StateMap["MASTER"] = cmn.CIStateMaster
	nCIh.StateMap["BACKUP"] = cmn.CIStateBackup
	nCIh.StateMap["FAULT"] = cmn.CIStateConflict
	nCIh.StateMap["STOP"] = cmn.CIStateNotDefined
	nCIh.StateMap["NOT_DEFINED"] = cmn.CIStateNotDefined
	nCIh.SpawnKa = args.SpawnKa
	nCIh.RemoteIP = args.RemoteIP
	nCIh.SourceIP = args.SourceIP
	nCIh.Interval = args.Interval
	nCIh.ClusterMap = make(map[string]*ClusterInstance)

	if _, ok := nCIh.ClusterMap[cmn.CIDefault]; !ok {
		ci := &ClusterInstance{State: cmn.CIStateNotDefined,
			StateStr: "NOT_DEFINED",
			Vip:      net.IPv4zero,
		}
		nCIh.ClusterMap[cmn.CIDefault] = ci
		if mh.bgp != nil {
			mh.bgp.UpdateCIState(cmn.CIDefault, ci.State, ci.Vip)
		}
	}

	nCIh.NodeMap = make(map[string]*ClusterNode)
	return nCIh
}

// CIStateGetInst - routine to get HA state
func (h *CIStateH) CIStateGetInst(inst string) (string, error) {

	if ci, ok := h.ClusterMap[inst]; ok {
		return ci.StateStr, nil
	}

	return "NOT_DEFINED", errors.New("not found")
}

// CIStateGet - routine to get HA state
func (h *CIStateH) CIStateGet() ([]cmn.HASMod, error) {
	var res []cmn.HASMod

	for i, s := range h.ClusterMap {
		var temp cmn.HASMod
		temp.Instance = i
		temp.State = s.StateStr
		temp.Vip = s.Vip
		res = append(res, temp)
	}
	return res, nil
}

// CIVipGet - routine to get HA state
func (h *CIStateH) CIVipGet(inst string) (net.IP, error) {
	if ci, ok := h.ClusterMap[inst]; ok {
		if ci.Vip != nil && !ci.Vip.IsUnspecified() {
			return ci.Vip, nil
		}
	}
	return net.IPv4zero, errors.New("not found")
}

// IsCIKAMode - routine to get KA mode
func (h *CIStateH) IsCIKAMode() bool {
	return false
}

// CIStateUpdate - routine to update cluster state
func (h *CIStateH) CIStateUpdate(cm cmn.HASMod) (int, error) {

	if _, ok := h.ClusterMap[cm.Instance]; !ok {
		h.ClusterMap[cm.Instance] = &ClusterInstance{State: cmn.CIStateNotDefined,
			StateStr: "NOT_DEFINED",
			Vip:      net.IPv4zero}
		tk.LogIt(tk.LogDebug, "[CLUSTER] New Instance %s created\n", cm.Instance)
	}

	ci, found := h.ClusterMap[cm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] New Instance %s find error\n", cm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	if ci.StateStr == cm.State {
		return ci.State, nil
	}

	if _, ok := h.StateMap[cm.State]; ok {
		tk.LogIt(tk.LogDebug, "[CLUSTER] Instance %s Current State %s Updated State: %s VIP : %s\n",
			cm.Instance, ci.StateStr, cm.State, cm.Vip.String())
		ci.StateStr = cm.State
		ci.State = h.StateMap[cm.State]
		ci.Vip = cm.Vip

		if mh.bgp != nil {
			mh.bgp.UpdateCIState(cm.Instance, ci.State, ci.Vip)
		}
		go mh.zr.Rules.RuleVIPSyncToClusterState()
		return ci.State, nil
	}

	tk.LogIt(tk.LogError, "[CLUSTER] Invalid State: %s\n", cm.State)
	return ci.State, errors.New("invalid cluster-state")

}

// ClusterNodeAdd - routine to update cluster nodes
func (h *CIStateH) ClusterNodeAdd(node cmn.ClusterNodeMod) (int, error) {

	cNode := h.NodeMap[node.Addr.String()]

	if cNode != nil {
		return -1, errors.New("existing cnode")
	}

	cNode = new(ClusterNode)
	cNode.Addr = node.Addr
	h.NodeMap[node.Addr.String()] = cNode

	cNode.DP(DpCreate)

	return 0, nil
}

// ClusterNodeDelete - routine to delete cluster node
func (h *CIStateH) ClusterNodeDelete(node cmn.ClusterNodeMod) (int, error) {

	cNode := h.NodeMap[node.Addr.String()]

	if cNode == nil {
		return -1, errors.New("no such cnode")
	}

	delete(h.NodeMap, node.Addr.String())

	cNode.DP(DpRemove)
	return 0, nil
}

// CIBFDSessionAdd - routine to add BFD session
func (h *CIStateH) CIBFDSessionAdd(bm cmn.BFDMod) (int, error) {

	if h.Bs == nil {
		return -1, errors.New("bfd not initialized")
	}

	if bm.Interval != 0 && bm.Interval < bfd.BFDMinSysTXIntervalUs {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD session Interval value too low\n")
		return -1, errors.New("bfd interval too low")
	}

	_, found := h.ClusterMap[bm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD SU - Cluster Instance %s not found\n", bm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	ip := net.ParseIP(bm.RemoteIP.String())
	if ip == nil {
		return -1, errors.New("remoteIP address malformed")
	}

	if !h.SpawnKa {
		myIP := net.ParseIP(bm.SourceIP.String())
		if myIP == nil {
			return -1, errors.New("source address malformed")
		}

		tk.LogIt(tk.LogInfo, "[CLUSTER] Cluster Instance %s starting BFD..\n", bm.Instance)
		h.SpawnKa = true

		h.RemoteIP = bm.RemoteIP
		h.SourceIP = bm.SourceIP
		h.Interval = int64(bm.Interval)
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String(), SourceIP: bm.SourceIP.String(),
			Port: cmn.BFDPort, Interval: uint32(bm.Interval),
			Multi: bm.RetryCount, Instance: bm.Instance}
		go h.startBFDProto(bfdSessConfigArgs)
	} else {
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String(), SourceIP: bm.SourceIP.String(),
			Port: cmn.BFDPort, Interval: uint32(bm.Interval),
			Multi: bm.RetryCount, Instance: bm.Instance}
		err := h.Bs.BFDAddRemote(bfdSessConfigArgs, h)
		if err != nil {
			tk.LogIt(tk.LogCritical, "KA - Cant add BFD remote: %s\n", err.Error())
			return -1, err
		}
		tk.LogIt(tk.LogInfo, "KA - BFD remote %s:%s:%vus Added\n", bm.RemoteIP.String(), bm.SourceIP.String(), bm.Interval)
	}
	return 0, nil
}

// CIBFDSessionDel - routine to delete BFD session
func (h *CIStateH) CIBFDSessionDel(bm cmn.BFDMod) (int, error) {

	if !h.SpawnKa {
		tk.LogIt(tk.LogError, "[CLUSTER] Cluster Instance %s not running BFD\n", bm.Instance)
		return -1, errors.New("bfd session not running")
	}

	_, found := h.ClusterMap[bm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD SU - Cluster Instance %s not found\n", bm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String()}
	err := h.Bs.BFDDeleteRemote(bfdSessConfigArgs)
	if err != nil {
		tk.LogIt(tk.LogCritical, "KA - Cant delete BFD remote\n")
		return -1, err
	}
	h.SpawnKa = false
	tk.LogIt(tk.LogInfo, "KA - BFD remote %s:%s deleted\n", bm.Instance, bm.RemoteIP.String())
	return 0, nil
}

// CIBFDSessionGet - routine to get BFD session info
func (h *CIStateH) CIBFDSessionGet() ([]cmn.BFDMod, error) {
	if !h.SpawnKa || h.Bs == nil {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD sessions not running\n")
		return nil, errors.New("bfd session not running")
	}

	return h.Bs.BFDGet()
}

// DP - sync state of cluster-node entity to data-path
func (cn *ClusterNode) DP(work DpWorkT) int {

	pwq := new(PeerDpWorkQ)
	pwq.Work = work
	pwq.PeerIP = cn.Addr

	pwq.Status = &cn.Status

	mh.dp.ToDpCh <- pwq

	return 0
}
