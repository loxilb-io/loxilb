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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)
 
 const (
	 POL_ERR_BASE = iota - 100000
	 POL_MOD_ERR
	 POL_INFO_ERR
	 POL_ATTACH_ERR
	 POL_NOEXIST_ERR
	 POL_EXISTS_ERR
	 POL_ALLOC_ERR
 )
 
 const (
	 MIN_ROL_RATE     = 8
	 MAX_POLS    	  = 8*1024
	 DFL_POL_BLK_SZ   = 6*1000*1000
 )

 const (
	ROL_TYPE_TRTCM = 0  // Default
	POL_TYPE_SRTCM = 1
)

type PolObjType uint

const (
	POL_ATTACH_PORT PolObjType = 1 << iota
	POL_ATTACH_LB_RULE
)
 
 type PolKey struct {
	 PolName string
 }
 
 type PolStats struct {
	 PacketsOk  uint64
	 PacketsNok uint64
 }

 type PolInfo struct {
	 PolType	       int
	 ColorAware		   bool
	 CommittedInfoRate uint64
	 PeakInfoRate      uint64
	 CommittedBlkSize  uint64
	 ExcessBlkSize     uint64
 }

 type PolAttachObjT interface {
 }

 type PolObj struct {
	PolObjName   string
	AttachMent   PolObjType
 }

 type PolObjInfo struct {
	Args		 PolObj
	AttachObj    PolAttachObjT
	Parent	     *PolEntry
	Sync         DpStatusT
 }
 
 type PolEntry struct {
	 Key    PolKey
	 Info   PolInfo
	 Zone   *Zone
	 HwNum  int
	 Stats  PolStats
	 Sync   DpStatusT
	 PObjs  []PolObjInfo
 }
 
 type PolH struct {
	 PolMap  map[PolKey]*PolEntry
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

 func PolInfoXlateValidate(pInfo *PolInfo) (bool) {
	if pInfo.CommittedInfoRate < MIN_ROL_RATE {
		return false
	}

	if pInfo.PeakInfoRate < MIN_ROL_RATE {
		return false
	}

	pInfo.CommittedInfoRate = pInfo.CommittedInfoRate*1000000
	pInfo.PeakInfoRate = pInfo.PeakInfoRate*1000000

	if pInfo.CommittedBlkSize == 0 {
		pInfo.CommittedBlkSize = DFL_POL_BLK_SZ
		pInfo.ExcessBlkSize = 2*DFL_POL_BLK_SZ
	} else {
		pInfo.ExcessBlkSize = 2*pInfo.CommittedBlkSize
	}
	return true
 }

 func PolObjValidate(pObj *PolObj) (bool) {

	if pObj.AttachMent != POL_ATTACH_PORT && pObj.AttachMent != POL_ATTACH_LB_RULE {
		return false
	}

	return true
 }

 // Add a policer in loxinet
func (P *PolH) PolAdd(pName string, pInfo PolInfo, pObjArgs PolObj) (int, error) {

	if PolObjValidate(&pObjArgs) == false {
		tk.LogIt(tk.LOG_ERROR, "policer add - %s: bad attach point\n", pName)
		return POL_ATTACH_ERR, errors.New("pol-attachpoint error")
	}

	if PolInfoXlateValidate(&pInfo) == false {
		tk.LogIt(tk.LOG_ERROR, "policer add - %s: info error\n", pName)
		return POL_INFO_ERR, errors.New("pol-info error")
	}

	key := PolKey{pName}
	p, found := P.PolMap[key]

	if found == true {
		if p.Info != pInfo {
			P.PolDelete(pName)
		} else {
			return POL_EXISTS_ERR, errors.New("pol-exists error")
		}
	}

	p = new(PolEntry)
	p.Key.PolName = pName
	p.Info = pInfo
	p.Zone = P.Zone
	p.HwNum, _ = P.HwMark.GetCounter()
	if p.HwNum < 0 {
		return POL_ALLOC_ERR, errors.New("pol-alloc error")
	}

	pObjInfo := PolObjInfo { Args:pObjArgs }
	pObjInfo.Parent = p
	p.PObjs = append(p.PObjs, pObjInfo)

	P.PolMap[key] = p

	p.DP(DP_CREATE)
	pObjInfo.PolObj2DP(DP_CREATE)

	tk.LogIt(tk.LOG_INFO, "policer added - %s\n", pName)

	return 0, nil
}

 // Delete a policer from loxinet
func (P *PolH) PolDelete(pName string) (int, error) {

	key := PolKey{pName}
	p, found := P.PolMap[key]

	if found == false {
		tk.LogIt(tk.LOG_ERROR, "policer delete - %s: not found error\n", pName)
		return POL_NOEXIST_ERR, errors.New("no such policer error")
	}

	for _, pObj := range p.PObjs {
		pObj.PolObj2DP(DP_REMOVE)
		pObj.Parent = nil
	}

	p.DP(DP_REMOVE)

	delete(P.PolMap, p.Key)

	tk.LogIt(tk.LOG_INFO, "policer deleted - %s\n", pName)

	return 0, nil
}

func (P *PolH) PolTicker() {
	for _, p := range P.PolMap {
		if p.Sync != 0 {
			p.DP(DP_CREATE)
			for _, pObj := range p.PObjs {
				pObj.PolObj2DP(DP_CREATE)
			}
		} else {
			for _, pObj := range p.PObjs {
				if pObj.Sync != 0 {
					pObj.PolObj2DP(DP_CREATE)
				}
			}
		}
	}
}

// Sync state of policer's attachment point with data-path
func (pObjInfo *PolObjInfo) PolObj2DP(work DpWorkT) int {

	// Only port attachment is supported currently
	if pObjInfo.Args.AttachMent != POL_ATTACH_PORT {
		return -1
	}

	port := pObjInfo.Parent.Zone.Ports.PortFindByName(pObjInfo.Args.PolObjName)
	if port == nil {
		pObjInfo.Sync = 1
		return -1
	}

	_, err:= pObjInfo.Parent.Zone.Ports.PortUpdateProp(port.Name, cmn.PORT_PROP_POL,
		pObjInfo.Parent.Zone.Name, true, pObjInfo.Parent.HwNum)
	if err != nil {
		pObjInfo.Sync = 1
		return -1
	}

	pObjInfo.Sync = 0

	return 0
}

// Sync state of policer with data-path
func (p *PolEntry) DP(work DpWorkT) int {

	pwq := new(PolDpWorkQ)
	pwq.Work = work
	pwq.Color = p.Info.ColorAware
	pwq.Cir = p.Info.CommittedInfoRate
	pwq.Pir = p.Info.PeakInfoRate
	pwq.Cbs = p.Info.CommittedBlkSize
	pwq.Ebs = p.Info.ExcessBlkSize
	pwq.Status = &p.Sync

	mh.dp.ToDpCh <- pwq

	return 0
}
