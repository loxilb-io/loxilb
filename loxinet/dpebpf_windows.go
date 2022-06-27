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
)

const (
    EBPF_ERR_BASE     = iota-DP_ERR_BASE-1000
)

type DpEbpfH struct {	
}

func (e *DpEbpfH) DpPortPropAdd(w *portDpWorkQ) int {
    fmt.Println(*w)
    return 0
}
    
func (e *DpEbpfH)DpPortPropDel(w *portDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpL2AddrAdd(w *l2AddrDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpL2AddrDel(w *l2AddrDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpRouterMacAdd(w *routerMacDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpRouterMacDel(w *routerMacDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpNextHopAdd(w *nextHopDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpNextHopDel(w *nextHopDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpRouteAdd(w *RouteDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func (e *DpEbpfH)DpRouteDel(w *RouteDpWorkQ) int {
    fmt.Println(*w)
    return 0
}

func DpEbpfInit() *DpEbpfH {
    ne := new(DpEbpfH)
    return ne
}