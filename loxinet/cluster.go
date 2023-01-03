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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// error codes for cluster module
const (
	CIErrBase = iota - 90000
	CIModErr
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

// CIStateH - Cluster context handler
type CIStateH struct {
	SpawnKa    bool
	ClusterMap map[string]ClusterInstance
	StateMap   map[string]int
	NodeMap    map[string]*ClusterNode
}

func isAPIActive(url string) bool {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{Timeout: timeout}
	_, e := client.Get(url)
	return e == nil
}

func readKaPID(pf string) int {

	d, err := ioutil.ReadFile(pf)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(d)))
	if err != nil {
		return 0
	}

	p, err1 := os.FindProcess(int(pid))
	if err1 != nil {
		return 0
	}

	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return 0
	}

	return pid
}

func kaSpawn() {
	url := fmt.Sprintf("http://127.0.0.1:%d/config/params", opts.Opts.Port)
	for true {
		if isAPIActive(url) == true {
			break
		}
		tk.LogIt(tk.LogDebug, "KA - waiting for API server\n")
		time.Sleep(1 * time.Second)
	}

	command := "sudo pkill keepalived"
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError, "Error in stopping KA:%s\n", err)
	}
	for {
		if _, err := os.Stat("/etc/keepalived/keepalived.conf"); errors.Is(err, os.ErrNotExist) {
			time.Sleep(2000 * time.Millisecond)
			continue
		}

		if _, err2 := os.Stat("/var/run/keepalived.pid"); errors.Is(err2, os.ErrNotExist) {
			tk.LogIt(tk.LogError, "KA Dead, need to restart\n")
		} else {
			pid := readKaPID("/var/run/keepalived.pid")
			if pid != 0 {
				time.Sleep(5000 * time.Millisecond)
				continue
			}
		}
		command = "sudo keepalived -f /etc/keepalived/keepalived.conf"
		cmd = exec.Command("bash", "-c", command)
		err = cmd.Run()
		if err != nil {
			tk.LogIt(tk.LogError, "Error in starting KA:%s\n", err)
		}
		time.Sleep(2000 * time.Millisecond)
		tk.LogIt(tk.LogInfo, "KA spawned\n")
	}
}

func (ci *CIStateH) CISync(doNotify bool) {
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
			// Format style -
			// INSTANCE default is in BACKUP state
			_, err = fmt.Sscanf(fsc.Text(), "INSTANCE %s is in %s state", &inst, &state)
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

			if notify && doNotify {
				tk.LogIt(tk.LogInfo, "ci-change instance %s - state %s\n", inst, state)
				sm.Instance = inst
				sm.State = state
				ci.CIStateUpdate(sm)
			} else {
				if ciState != cmn.CIStateMaster && ciState != cmn.CIStateBackup {
					continue
				}

				nci := ClusterInstance{State: ciState, StateStr: state}
				ci.ClusterMap[inst] = nci
			}
		}

		rf.Close()
	}
}

// CITicker - Periodic ticker for Cluster module
func (ci *CIStateH) CITicker() {
	mh.mtx.Lock()
	ci.CISync(true)
	mh.mtx.Unlock()
}

// CIInit - routine to initialize Cluster context
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

	// nCIh.CISync(false)

	if _, ok := nCIh.ClusterMap["default"]; !ok {
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

// CIStateUpdate - routine to update cluster state
func (h *CIStateH) CIStateUpdate(cm cmn.HASMod) (int, error) {

	if _, ok := h.ClusterMap[cm.Instance]; !ok {
		h.ClusterMap[cm.Instance] = ClusterInstance{cmn.CIStateNotDefined, "NOT_DEFINED"}
		tk.LogIt(tk.LogDebug, "[CLUSTER] New Instance %s created\n", cm.Instance)
	}

	ci := h.ClusterMap[cm.Instance]

	if ci.StateStr == cm.State {
		return ci.State, nil
	}

	if _, ok := h.StateMap[cm.State]; ok {
		tk.LogIt(tk.LogDebug, "[CLUSTER] Instance %s Current State %s Updated State: %s\n", cm.Instance, ci.StateStr, cm.State)
		ci.StateStr = cm.State
		ci.State = h.StateMap[cm.State]
		h.ClusterMap[cm.Instance] = ci
		if h.SpawnKa && cm.State == "FAULT" {
			command := "sudo pkill keepalived"
			cmd := exec.Command("bash", "-c", command)
			err := cmd.Run()
			if err != nil {
				tk.LogIt(tk.LogError, "Error in stoping KA:%s", err)
			}
		}
		if mh.bgp != nil {
			mh.bgp.UpdateCIState(ci.State)
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
