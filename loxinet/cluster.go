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
	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"

	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// error codes for cluster module
const (
	CIErrBase = iota - 90000
	CIModErr
	CIStateErr
)

// Config related constants
const (
	KAConfigFile = "/etc/keepalived/keepalived.conf"
	KAPidFile1   = "/var/run/keepalived.pid"
	KAPidFile2   = "/var/run/vrrp.pid"
)

type ClusterInstance struct {
	State    int
	StateStr string
	Vip      net.IP
}

type ClusterNode struct {
	Addr   net.IP
	Status DpStatusT
}

// CIStateH - Cluster context handler
type CIStateH struct {
	SpawnKa    bool
	kaMode     bool
	ClusterMap map[string]*ClusterInstance
	StateMap   map[string]int
	NodeMap    map[string]*ClusterNode
}

func kaSpawn() {
	url := fmt.Sprintf("http://127.0.0.1:%d/config/params", opts.Opts.Port)
	for true {
		if IsLoxiAPIActive(url) == true {
			break
		}
		tk.LogIt(tk.LogDebug, "KA - waiting for API server\n")
		time.Sleep(1 * time.Second)
	}

	RunCommand("rm -f /etc/shared/keepalive.state", false)
	RunCommand("pkill keepalived", false)
	mh.dp.WaitXsyncReady("ka")
	// We need some cool-off period for loxilb to self sync-up in the cluster
	time.Sleep(KAInitTiVal * time.Second)

	for {
		if exists := FileExists(KAConfigFile); !exists {
			time.Sleep(2000 * time.Millisecond)
			continue
		}

		pid := ReadPIDFile(KAPidFile1)
		if pid != 0 {
			time.Sleep(5000 * time.Millisecond)
			continue
		}

		tk.LogIt(tk.LogInfo, "KA spawning\n")
		cmd := exec.Command("/usr/sbin/keepalived", "-f", KAConfigFile, "-n")
		err := cmd.Run()
		if err != nil {
			tk.LogIt(tk.LogError, "Error in running KA:%s\n", err)
		} else {
			tk.LogIt(tk.LogInfo, "KA found dead. Reaping\n")
		}

		rmf := fmt.Sprintf("rm -f %s", KAPidFile1)
		RunCommand(rmf, false)
		rmf = fmt.Sprintf("rm -f %s", KAPidFile2)
		RunCommand(rmf, false)

		time.Sleep(2000 * time.Millisecond)
	}
}

func (ci *CIStateH) CISync() {
	var sm cmn.HASMod
	var ciState int
	var ok bool
	clusterStateFile := "/etc/shared/keepalive.state"
	rf, err := os.Open(clusterStateFile)
	if err == nil {

		fsc := bufio.NewScanner(rf)
		fsc.Split(bufio.ScanLines)

		for fsc.Scan() {
			var inst string
			var state string
			var vip string
			// Format style -
			// INSTANCE default is in BACKUP state
			_, err = fmt.Sscanf(fsc.Text(), "INSTANCE %s is in %s state vip %s", &inst, &state, &vip)
			if err != nil {
				continue
			}

			if ciState, ok = ci.StateMap[state]; !ok {
				continue
			}

			notify := false

			if eci, ok := ci.ClusterMap[inst]; !ok {
				notify = true
			} else {
				if eci.State != ciState {
					notify = true
				}
			}

			if notify {
				sm.Instance = inst
				sm.State = state
				sm.Vip = net.ParseIP(vip)
				tk.LogIt(tk.LogInfo, "ci-change instance %s - state %s vip %v\n", inst, state, sm.Vip)
				ci.CIStateUpdate(sm)
			}
		}

		rf.Close()
	}
}

// CITicker - Periodic ticker for Cluster module
func (ci *CIStateH) CITicker() {
	mh.mtx.Lock()
	ci.CISync()
	mh.mtx.Unlock()
}

// CISpawn - Spawn CI application
func (ci *CIStateH) CISpawn() {
	if ci.SpawnKa {
		go kaSpawn()
	}
}

// CIInit - routine to initialize Cluster context
func CIInit(spawnKa bool, kaMode bool) *CIStateH {
	var nCIh = new(CIStateH)
	nCIh.StateMap = make(map[string]int)
	nCIh.StateMap["MASTER"] = cmn.CIStateMaster
	nCIh.StateMap["BACKUP"] = cmn.CIStateBackup
	nCIh.StateMap["FAULT"] = cmn.CIStateConflict
	nCIh.StateMap["STOP"] = cmn.CIStateNotDefined
	nCIh.StateMap["NOT_DEFINED"] = cmn.CIStateNotDefined
	nCIh.SpawnKa = spawnKa
	nCIh.kaMode = kaMode
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

	return "NOT_DEFINED", errors.New("Not found")
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
	return net.IPv4zero, errors.New("Not found")
}

// IsCIKAMode - routine to get HA state
func (h *CIStateH) IsCIKAMode() bool {
	return h.kaMode
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
		return -1, errors.New("Cluster instance not found")
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
		if h.SpawnKa && (cm.State == "FAULT" || cm.State == "STOP") {
			RunCommand("pkill keepalived", false)
		}
		if mh.bgp != nil {
			mh.bgp.UpdateCIState(cm.Instance, ci.State, ci.Vip)
		}
		return ci.State, nil
	} else {
		tk.LogIt(tk.LogError, "[CLUSTER] Invalid State: %s\n", cm.State)
		return ci.State, errors.New("Invalid Cluster state")
	}
}

// ClusterNodeAdd - routine to update cluster nodes
func (h *CIStateH) ClusterNodeAdd(node cmn.CluserNodeMod) (int, error) {

	cNode := h.NodeMap[node.Addr.String()]

	if cNode != nil {
		return -1, errors.New("Exisitng Cnode")
	}

	cNode = new(ClusterNode)
	cNode.Addr = node.Addr
	h.NodeMap[node.Addr.String()] = cNode

	cNode.DP(DpCreate)

	return 0, nil
}

// ClusterNodeDelete - routine to delete cluster node
func (h *CIStateH) ClusterNodeDelete(node cmn.CluserNodeMod) (int, error) {

	cNode := h.NodeMap[node.Addr.String()]

	if cNode == nil {
		return -1, errors.New("No such Cnode")
	}

	delete(h.NodeMap, node.Addr.String())

	cNode.DP(DpRemove)
	return 0, nil
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
