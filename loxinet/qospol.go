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
	 //"errors"
	 //"fmt"
	 //cmn "github.com/loxilb-io/loxilb/common"
	 tk "github.com/loxilb-io/loxilib"
	 //"net"
 )
 
 const (
	 POL_ERR_BASE = iota - 100000
	 POL_MOD_ERR
	 POL_NOEXIST_ERR
	 POL_EXISTS_ERR
 )
 
 const (
	 MIN_ROL_RATE = 8*1000*1000
	 MAX_POLS = 8*1024
	 DFL_BLK_SZ = 6*1000*1000
 )
 
 type PolKey struct {
	 PolName string
 }
 
 type PolStats struct {
	 PacketsOk  uint64
	 PacketsNok uint64
 }

 type PolAttr struct {
	 CommitedInfoRate  uint64
	 PeakInfoRate      uint64
	 CommittedBlkSize  uint64
	 ExcessBlkSize     uint64
 }
 
 type PolEntry struct {
	 Key   PolKey
	 Attr  PolAttr
 }
 
 type PolH struct {
	 PolMap map[PolKey]*PolEntry
	 Zone    *Zone
	 HwMark  *tk.Counter
 }
 
 func PolInit(zone *Zone) *PolH {
	 var nPh = new(PolH)
	 nPh.PolMap = make(map[PolKey]*PolEntry)
	 nPh.Zone = zone
	 nPh.HwMark = tk.NewCounter(1, MAX_POLS)
	 return nPh
 }