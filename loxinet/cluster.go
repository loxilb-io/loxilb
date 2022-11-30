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

	"errors"
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

// CIStateH - HA context handler
type CIStateH struct {
	ClusterMap map[string]ClusterInstance
	StateMap   map[string]int
}

// HAInit - routine to initialize HA context
func HAInit() *CIStateH {
	var nHh = new(CIStateH)
	nHh.StateMap = make(map[string]int)
	nHh.StateMap["MASTER"] = cmn.CIStateMaster
	nHh.StateMap["BACKUP"] = cmn.CIStateBackup
	nHh.StateMap["FAULT"] = cmn.CIStateConflict
	nHh.StateMap["STOP"] = cmn.CIStateNotDefined
	nHh.StateMap["NOT_DEFINED"] = cmn.CIStateNotDefined

	nHh.ClusterMap = make(map[string]ClusterInstance)
	var ci ClusterInstance
	ci.State = cmn.CIStateNotDefined
	ci.StateStr = "NOT_DEFINED"
	nHh.ClusterMap["default"] = ci
	return nHh
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
		return ci.State, nil
	} else {
		tk.LogIt(tk.LogError, "[HA] Invalid State: %s\n", ham.State)
		return ci.State, errors.New("Invalid HA state")
	}
}
