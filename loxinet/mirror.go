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
	MIRR_ERR_BASE = iota - 101000
	MIRR_MOD_ERR
	MIRR_INFO_ERR
	MIRR_ATTACH_ERR
	MIRR_NOEXIST_ERR
	MIRR_EXISTS_ERR
	MIRR_ALLOC_ERR
)

const (
	MAX_MIRRS = 32
)

type MirrKey struct {
	Name string
}

type MirrStats struct {
	PacketsOk uint64
	Bytes     uint64
}

type MirAttachObjT interface {
}

type MirrObjInfo struct {
	Args      cmn.MirrObj
	AttachObj MirAttachObjT
	Parent    *MirrEntry
	Sync      DpStatusT
}

type MirrEntry struct {
	Key   MirrKey
	Info  cmn.MirrInfo
	Zone  *Zone
	HwNum int
	Stats PolStats
	Sync  DpStatusT
	MObjs []MirrObjInfo
}

type MirrH struct {
	MirrMap map[MirrKey]*MirrEntry
	Zone    *Zone
	HwMark  *tk.Counter
}

func MirrInit(zone *Zone) *MirrH {
	var nMh = new(MirrH)
	nMh.MirrMap = make(map[MirrKey]*MirrEntry)
	nMh.Zone = zone
	nMh.HwMark = tk.NewCounter(1, MAX_MIRRS)
	return nMh
}

func MirrInfoValidate(mInfo *cmn.MirrInfo) bool {
	if mInfo.MirrType != cmn.MIRR_TYPE_SPAN &&
		mInfo.MirrType != cmn.MIRR_TYPE_RSPAN &&
		mInfo.MirrType != cmn.MIRR_TYPE_ERSPAN {
		return false
	}

	if mInfo.MirrType == cmn.MIRR_TYPE_RSPAN &&
		mInfo.MirrVlan != 0 {
		return false
	}

	if mInfo.MirrType == cmn.MIRR_TYPE_ERSPAN {
		if mInfo.MirrRip.IsUnspecified() ||
			mInfo.MirrSip.IsUnspecified() ||
			mInfo.MirrTid == 0 {
			return false
		}
	}

	return true
}

func MirrObjValidate(mObj *cmn.MirrObj) bool {

	if mObj.AttachMent != cmn.MIRR_ATTACH_PORT && mObj.AttachMent != cmn.MIRR_ATTACH_LB_RULE {
		return false
	}

	return true
}

func MirrInfoCmp(mInfo1, mInfo2 *cmn.MirrInfo) bool {
	if mInfo1.MirrType == mInfo2.MirrType &&
		mInfo1.MirrPort == mInfo2.MirrPort &&
		mInfo1.MirrVlan == mInfo2.MirrVlan &&
		mInfo1.MirrRip.Equal(mInfo2.MirrRip) &&
		mInfo1.MirrSip.Equal(mInfo2.MirrSip) &&
		mInfo1.MirrTid == mInfo2.MirrTid {
		return true
	}
	return false
}

// Add a mirror in loxinet
func (M *MirrH) MirrAdd(name string, mInfo cmn.MirrInfo, mObjArgs cmn.MirrObj) (int, error) {

	if MirrObjValidate(&mObjArgs) == false {
		tk.LogIt(tk.LOG_ERROR, "mirror add - %s: bad attach point\n", name)
		return MIRR_ATTACH_ERR, errors.New("mirr-attachpoint error")
	}

	if MirrInfoValidate(&mInfo) == false {
		tk.LogIt(tk.LOG_ERROR, "mirror add - %s: info error\n", name)
		return MIRR_INFO_ERR, errors.New("mirr-info error")
	}

	key := MirrKey{name}
	m, found := M.MirrMap[key]

	if found == true {
		if MirrInfoCmp(&m.Info, &mInfo) == false {
			M.MirrDelete(name)
		} else {
			return MIRR_EXISTS_ERR, errors.New("mirr-exists error")
		}
	}

	m = new(MirrEntry)
	m.Key.Name = name
	m.Info = mInfo
	m.Zone = M.Zone
	m.HwNum, _ = M.HwMark.GetCounter()
	if m.HwNum < 0 {
		return MIRR_ALLOC_ERR, errors.New("mirr-alloc error")
	}

	mObjInfo := MirrObjInfo{Args: mObjArgs}
	mObjInfo.Parent = m

	M.MirrMap[key] = m

	m.DP(DP_CREATE)
	mObjInfo.MirrObj2DP(DP_CREATE)

	m.MObjs = append(m.MObjs, mObjInfo)

	tk.LogIt(tk.LOG_INFO, "mirror added - %s\n", name)

	return 0, nil
}

// Delete a mirror from loxinet
func (M *MirrH) MirrDelete(name string) (int, error) {

	key := MirrKey{name}
	m, found := M.MirrMap[key]

	if found == false {
		tk.LogIt(tk.LOG_ERROR, "mirror delete - %s: not found error\n", name)
		return MIRR_NOEXIST_ERR, errors.New("no such mirror error")
	}

	for idx, mObj := range m.MObjs {
		var pM *MirrObjInfo = &m.MObjs[idx]
		mObj.MirrObj2DP(DP_REMOVE)
		pM.Parent = nil
	}

	m.DP(DP_REMOVE)

	delete(M.MirrMap, m.Key)

	tk.LogIt(tk.LOG_INFO, "mirror deleted - %s\n", name)

	return 0, nil
}

func (M *MirrH) MirrPortDelete(name string) {
	for _, m := range M.MirrMap {
		for idx, mObj := range m.MObjs {
			var pM *MirrObjInfo
			if mObj.Args.AttachMent == cmn.MIRR_ATTACH_PORT &&
				mObj.Args.MirrObjName == name {
				pM = &m.MObjs[idx]
				pM.Sync = 1
			}
		}
	}
}

func (M *MirrH) MirrDestructAll() {
	for _, m := range M.MirrMap {
		M.MirrDelete(m.Key.Name)
	}
}

func (M *MirrH) MirrTicker() {
	for _, m := range M.MirrMap {
		if m.Sync != 0 {
			m.DP(DP_CREATE)
			for _, mObj := range m.MObjs {
				mObj.MirrObj2DP(DP_CREATE)
			}
		} else {

			for idx, mObj := range m.MObjs {
				var pM *MirrObjInfo
				pM = &m.MObjs[idx]
				if pM.Sync != 0 {
					pM.MirrObj2DP(DP_CREATE)
				} else {
					if mObj.Args.AttachMent == cmn.MIRR_ATTACH_PORT {
						port := mObj.Parent.Zone.Ports.PortFindByName(mObj.Args.MirrObjName)
						if port == nil {
							pM.Sync = 1
						}
					}
				}
			}
		}
	}
}

// Sync state of mirror's attachment point with data-path
func (mObjInfo *MirrObjInfo) MirrObj2DP(work DpWorkT) int {

	// Only port attachment is supported currently
	if mObjInfo.Args.AttachMent != cmn.MIRR_ATTACH_PORT {
		return -1
	}

	port := mObjInfo.Parent.Zone.Ports.PortFindByName(mObjInfo.Args.MirrObjName)
	if port == nil {
		mObjInfo.Sync = 1
		return -1
	}

	if work == DP_CREATE {
		_, err := mObjInfo.Parent.Zone.Ports.PortUpdateProp(port.Name, cmn.PORT_PROP_SPAN,
			mObjInfo.Parent.Zone.Name, true, mObjInfo.Parent.HwNum)
		if err != nil {
			mObjInfo.Sync = 1
			return -1
		}
	} else if work == DP_REMOVE {
		mObjInfo.Parent.Zone.Ports.PortUpdateProp(port.Name, cmn.PORT_PROP_SPAN,
			mObjInfo.Parent.Zone.Name, false, 0)
	}

	mObjInfo.Sync = 0

	return 0
}

// Sync state of mirror with data-path
func (m *MirrEntry) DP(work DpWorkT) int {

	if m.Info.MirrType == cmn.MIRR_TYPE_ERSPAN {
		// Not supported currently
		return -1
	}

	mwq := new(MirrDpWorkQ)
	mwq.Work = work
	mwq.HwMark = m.HwNum
	if work == DP_CREATE {
		port := m.Zone.Ports.PortFindByName(m.Info.MirrPort)
		if port == nil {
			m.Sync = 1
			return -1
		}
		mwq.MiPortNum = port.PortNo
		mwq.MiBD = m.Info.MirrVlan
	}

	mwq.Status = &m.Sync

	mh.dp.ToDpCh <- mwq

	return 0
}
