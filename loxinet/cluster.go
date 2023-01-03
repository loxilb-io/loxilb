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
	tk "github.com/loxilb-io/loxilib"

	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

// error codes for HA state
const (
	HAErrBase = iota - 90000
	HAModErr
	CIStateErr
)

type ClusterInstance struct {
	State    int
	StateStr string
}

type ClusterNode struct {
	Addr   net.IP
	Status DpStatusT
}

// CIStateH - HA context handler
type CIStateH struct {
	SpawnKa    bool
	ClusterMap map[string]ClusterInstance
	StateMap   map[string]int
	NodeMap    map[string]*ClusterNode
}

func kaSpawn() {
	command := "sudo pkill keepalived"
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError,"Error in stoping KA:%s", err)
	}
	for {
		if _, err := os.Stat("/etc/keepalived/keepalived.conf"); errors.Is(err, os.ErrNotExist) {
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		if _, err2 := os.Stat( "/var/run/keepalived.pid"); errors.Is(err2, os.ErrNotExist) {
			tk.LogIt(tk.LogError,"KA Dead, need to restart\n")
		} else {
			time.Sleep(1000 * time.Millisecond)
			continue
		}
		command = "sudo keepalived -f /etc/keepalived/keepalived.conf"
		cmd = exec.Command("bash", "-c", command)
		err = cmd.Run()
		if err != nil {
			tk.LogIt(tk.LogError,"Error in starting KA:%s\n", err)
		}
		time.Sleep(2000 * time.Millisecond)
		tk.LogIt(tk.LogInfo,"KA spawned\n")
	}
}


// HAInit - routine to initialize HA context
func CIInit(spawnKa bool) *CIStateH {
	var nCIh = new(CIStateH)
	nCIh.StateMap = make(map[string]int)
	nCIh.StateMap["MASTER"] = cmn.CIStateMaster
	nCIh.StateMap["BACKUP"] = cmn.CIStateBackup
	nCIh.StateMap["FAULT"] = cmn.CIStateConflict
	nCIh.StateMap["STOP"] = cmn.CIStateNotDefined
	nCIh.StateMap["NOT_DEFINED"] = cmn.CIStateNotDefined
	nCIh.SpawnKa = spawnKa
	nCIh.ClusterMap = make(map[string]ClusterInstance)
	if spawnKa {
		go kaSpawn()
	}

	clusterStateFile := "/etc/shared/keepalive.state"
	rf, err := os.Open(clusterStateFile)
	if err == nil {

		fsc := bufio.NewScanner(rf)
		fsc.Split(bufio.ScanLines)

		for fsc.Scan() {
			var inst string
			var state string
			// Format style -
			// INSTANCE default is in BACKUP state
			fmt.Println(fsc.Text())
			_, err = fmt.Sscanf(fsc.Text(), "INSTANCE %s is in %s state", &inst, &state)
			if err != nil {
				continue
			}

			tk.LogIt(tk.LogInfo, "instance %s - state %s\n", inst, state)

			ciState := nCIh.StateMap[state]
			if ciState != cmn.CIStateMaster && ciState != cmn.CIStateBackup {
				continue
			}

			ci := ClusterInstance{State: ciState, StateStr: state}
			nCIh.ClusterMap[inst] = ci
		}

		rf.Close()
	} else {
		var ci ClusterInstance
		ci.State = cmn.CIStateNotDefined
		ci.StateStr = "NOT_DEFINED"
		nCIh.ClusterMap["default"] = ci
	}

	nCIh.NodeMap = make(map[string]*ClusterNode)
	return nCIh
}

// CIStateGet - routine to get HA state
func (h *CIStateH) CIStateGet() ([]cmn.HASMod, error) {
	var res []cmn.HASMod

	for i, s := range h.ClusterMap {
		var temp cmn.HASMod
		temp.Instance = i
		temp.State = s.StateStr
		res = append(res, temp)
	}
	return res, nil
}

// CIStateUpdate - routine to update HA state
func (h *CIStateH) CIStateUpdate(ham cmn.HASMod) (int, error) {

	if _, ok := h.ClusterMap[ham.Instance]; !ok {
		h.ClusterMap[ham.Instance] = ClusterInstance{cmn.CIStateNotDefined, "NOT_DEFINED"}
		tk.LogIt(tk.LogDebug, "[HA] New Instance %s created\n", ham.Instance)
	}

	ci := h.ClusterMap[ham.Instance]

	if _, ok := h.StateMap[ham.State]; ok {
		tk.LogIt(tk.LogDebug, "[HA] Instance %s Current State %s Updated State: %s\n", ham.Instance, ci.StateStr, ham.State)
		ci.StateStr = ham.State
		ci.State = h.StateMap[ham.State]
		h.ClusterMap[ham.Instance] = ci
		if h.SpawnKa && ham.State == "FAULT" {
			command := "sudo pkill keepalived"
			cmd := exec.Command("bash", "-c", command)
			err := cmd.Run()
			if err != nil {
				tk.LogIt(tk.LogError,"Error in stoping KA:%s", err)
			}
		}
		return ci.State, nil
	} else {
		tk.LogIt(tk.LogError, "[HA] Invalid State: %s\n", ham.State)
		return ci.State, errors.New("Invalid HA state")
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
