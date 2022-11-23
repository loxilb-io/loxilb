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
	tk "github.com/loxilb-io/loxilib"
)

// error codes for HA state
const (
	HAErrBase = iota - 90000
	HAModErr
	HAStateErr
)

const (
	HAStateMaster = iota
	HAStateSlave
	HAStateConflict
	HAStateNotDefined
)

// HAStateH - HA context handler
type HAStateH struct {
	StateStr string
}

// HAInit - routine to initialize HA context
func HAInit() *HAStateH {
	var nHh = new(HAStateH)
	nHh.StateStr = "Not Defined"
	return nHh
}

// HAStateGet - routine to get HA state
func (h *HAStateH) HAStateGet() (string, error) {
	return h.StateStr, nil
}

// HAStateUpdate - routine to update HA state
func (h *HAStateH) HAStateUpdate(state string) (int, error) {
	tk.LogIt(tk.LogDebug, "[HA] Current State %s Updated State: %s\n", h.StateStr, state)
	h.StateStr = state
	return 0, nil
}
