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
package loxilib

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const (
	TRIE_SUCCESS    = 0
	TRIE_ERR_GEN    = -1
	TRIE_ERR_EXISTS = -2
	TRIE_ERR_NOENT  = -3
	TRIE_ERR_NOMEM  = -4
	TRIE_ERR_UNK    = -5
	TRIE_ERR_PREFIX = -6
)

const (
	TRIE_JMP_LENGTH  = 8
	PREFIX_ARR_LEN   = (1 << (TRIE_JMP_LENGTH + 1)) - 1
	PREFIX_ARR_NBITS = ((PREFIX_ARR_LEN + TRIE_JMP_LENGTH) & ^TRIE_JMP_LENGTH) / TRIE_JMP_LENGTH
	PTR_ARR_LEN      = (1 << TRIE_JMP_LENGTH)
	PTR_ARR_NBITS    = ((PTR_ARR_LEN + TRIE_JMP_LENGTH) & ^TRIE_JMP_LENGTH) / TRIE_JMP_LENGTH
)

type TrieData interface {
	// Emtpy Interface
}

type TrieIterIntf interface {
	TrieNodeWalker(b string)
	TrieData2String(d TrieData) string
}

type trieVar struct {
	prefix [16]byte
}

type trieState struct {
	trieData        TrieData
	lastMatchLevel  int
	lastMatchPfxLen int
	lastMatchEmpty  bool
	lastMatchTv     trieVar
	matchFound      bool
	maxLevels       int
	errCode         int
}

type TrieRoot struct {
	prefixArr  [PREFIX_ARR_NBITS]uint8
	ptrArr     [PTR_ARR_NBITS]uint8
	prefixData [PREFIX_ARR_LEN]TrieData
	ptrData    [PTR_ARR_LEN]*TrieRoot
}

func TrieInit() *TrieRoot {
	var root = new(TrieRoot)
	return root
}

func prefix2TrieVar(ipPrefix net.IP, pIndex int) trieVar {
	var tv trieVar

	if ipPrefix.To4() != nil {
		ipAddr := binary.BigEndian.Uint32(ipPrefix.To4())
		tv.prefix[0] = (uint8((ipAddr >> 24)) & 0xff)
		tv.prefix[1] = (uint8((ipAddr >> 16)) & 0xff)
		tv.prefix[2] = (uint8((ipAddr >> 8)) & 0xff)
		tv.prefix[3] = (uint8(ipAddr) & 0xff)
	} else {
		for i := 0; i < 16; i++ {
			tv.prefix[i] = uint8(ipPrefix[i])
		}
	}

	return tv
}

func grabByte(tv *trieVar, pIndex int) (uint8, error) {

	if pIndex > 15 {
		return 0xff, errors.New("Out of range")
	}

	return tv.prefix[pIndex], nil
}

func cidr2TrieVar(cidr string, tv *trieVar) (pfxLen int) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return -1
	}

	pfx := ipNet.IP.Mask(ipNet.Mask)
	pfxLen, _ = ipNet.Mask.Size()
	*tv = prefix2TrieVar(pfx, 0)
	return pfxLen
}

func shrinkPrefixArrDat(arr []TrieData, startPos int) {
	if startPos < 0 || startPos >= len(arr) {
		return
	}

	for i := startPos; i < len(arr)-1; i++ {
		arr[i] = arr[i+1]
	}
}

func shrinkPtrArrDat(arr []*TrieRoot, startPos int) {
	if startPos < 0 || startPos >= len(arr) {
		return
	}

	for i := startPos; i < len(arr)-1; i++ {
		arr[i] = arr[i+1]
	}
}

func expPrefixArrDat(arr []TrieData, startPos int) {
	if startPos < 0 || startPos >= len(arr) {
		return
	}

	for i := len(arr) - 2; i >= startPos; i-- {
		arr[i+1] = arr[i]
	}
}

func expPtrArrDat(arr []*TrieRoot, startPos int) {
	if startPos < 0 || startPos >= len(arr) {
		return
	}

	for i := len(arr) - 2; i >= startPos; i-- {
		arr[i+1] = arr[i]
	}
}

func (t *TrieRoot) addTrieInt(tv *trieVar, currLevel int, rPfxLen int, ts *trieState) int {

	if rPfxLen < 0 || ts.errCode != 0 {
		return -1
	}

	// This assumes stride of length 8
	var cval uint8 = tv.prefix[currLevel]
	var nextRoot *TrieRoot

	if rPfxLen > TRIE_JMP_LENGTH {
		rPfxLen -= TRIE_JMP_LENGTH
		ptrIdx := countSetBitsInArr(t.ptrArr[:], int(cval)-1)
		if isBitSetInArr(t.ptrArr[:], int(cval)) == true {
			nextRoot = t.ptrData[ptrIdx]
			if nextRoot == nil {
				ts.errCode = TRIE_ERR_UNK
				return -1
			}
		} else {
			// If no pointer exists, then allocate it
			// Make pointer references
			nextRoot = new(TrieRoot)
			if t.ptrData[ptrIdx] != nil {
				expPtrArrDat(t.ptrData[:], ptrIdx)
				t.ptrData[ptrIdx] = nil
			}
			t.ptrData[ptrIdx] = nextRoot
			setBitInArr(t.ptrArr[:], int(cval))
		}
		return nextRoot.addTrieInt(tv, currLevel+1, rPfxLen, ts)
	} else {
		shftBits := TRIE_JMP_LENGTH - rPfxLen
		basePos := (1 << rPfxLen) - 1
		// Find value relevant to currently remaining prefix len
		cval = cval >> shftBits
		idx := basePos + int(cval)
		if isBitSetInArr(t.prefixArr[:], idx) == true {
			return TRIE_ERR_EXISTS
		}
		pfxIdx := countSetBitsInArr(t.prefixArr[:], idx)
		if t.prefixData[pfxIdx] != 0 {
			expPrefixArrDat(t.prefixData[:], pfxIdx)
			t.prefixData[pfxIdx] = 0
		}
		setBitInArr(t.prefixArr[:], idx)
		t.prefixData[pfxIdx] = ts.trieData
		return 0
	}
}

func (t *TrieRoot) deleteTrieInt(tv *trieVar, currLevel int, rPfxLen int, ts *trieState) int {

	if rPfxLen < 0 || ts.errCode != 0 {
		return -1
	}

	// This assumes stride of length 8
	var cval uint8 = tv.prefix[currLevel]
	var nextRoot *TrieRoot

	if rPfxLen > TRIE_JMP_LENGTH {
		rPfxLen -= TRIE_JMP_LENGTH
		ptrIdx := countSetBitsInArr(t.ptrArr[:], int(cval)-1)
		if isBitSetInArr(t.ptrArr[:], int(cval)) == false {
			ts.matchFound = false
			return -1
		}

		nextRoot = t.ptrData[ptrIdx]
		if nextRoot == nil {
			ts.matchFound = false
			ts.errCode = TRIE_ERR_UNK
			return -1
		}
		nextRoot.deleteTrieInt(tv, currLevel+1, rPfxLen, ts)
		if ts.matchFound == true && ts.lastMatchEmpty == true {
			t.ptrData[ptrIdx] = nil
			shrinkPtrArrDat(t.ptrData[:], ptrIdx)
			unSetBitInArr(t.ptrArr[:], int(cval))
		}
		if ts.lastMatchEmpty == true {
			if countAllSetBitsInArr(t.prefixArr[:]) == 0 &&
				countAllSetBitsInArr(t.ptrArr[:]) == 0 {
				ts.lastMatchEmpty = true
			} else {
				ts.lastMatchEmpty = false
			}
		}
		if ts.errCode != 0 {
			return -1
		}
		return 0
	} else {
		shftBits := TRIE_JMP_LENGTH - rPfxLen
		basePos := (1 << rPfxLen) - 1

		// Find value relevant to currently remaining prefix len
		cval = cval >> shftBits
		idx := basePos + int(cval)
		if isBitSetInArr(t.prefixArr[:], idx) == false {
			ts.matchFound = false
			return TRIE_ERR_NOENT
		}
		pfxIdx := countSetBitsInArr(t.prefixArr[:], idx-1)
		// Note - This assumes that prefix data should be non-zero
		if t.prefixData[pfxIdx] != 0 {
			t.prefixData[pfxIdx] = 0
			shrinkPrefixArrDat(t.prefixData[:], pfxIdx)
			unSetBitInArr(t.prefixArr[:], idx)
			ts.matchFound = true
			if countAllSetBitsInArr(t.prefixArr[:]) == 0 &&
				countAllSetBitsInArr(t.ptrArr[:]) == 0 {
				ts.lastMatchEmpty = true
			}

			return 0
		}
		ts.matchFound = false
		ts.errCode = TRIE_ERR_UNK
		return -1
	}
}

func (t *TrieRoot) findTrieInt(tv *trieVar, currLevel int, ts *trieState) int {

	var idx int = 0
	if ts.errCode != 0 {
		return -1
	}

	// This assumes stride of length 8
	var cval uint8 = tv.prefix[currLevel]

	for rPfxLen := TRIE_JMP_LENGTH; rPfxLen >= 0; rPfxLen-- {
		shftBits := TRIE_JMP_LENGTH - rPfxLen
		basePos := (1 << rPfxLen) - 1
		// Find value relevant to currently remaining prefix len
		cval = cval >> shftBits
		idx = basePos + int(cval)
		pfxVal := (idx - basePos) << shftBits

		if isBitSetInArr(t.prefixArr[:], idx) == true {
			ts.lastMatchLevel = currLevel
			ts.lastMatchPfxLen = 8*currLevel + rPfxLen
			ts.matchFound = true
			ts.lastMatchTv.prefix[currLevel] = byte(pfxVal)
			break
		}
	}

	cval = tv.prefix[currLevel]
	ptrIdx := countSetBitsInArr(t.ptrArr[:], int(cval)-1)
	if isBitSetInArr(t.ptrArr[:], int(cval)) == true {
		if t.ptrData[ptrIdx] != nil {
			nextRoot := t.ptrData[ptrIdx]
			ts.lastMatchTv.prefix[currLevel] = byte(cval)
			nextRoot.findTrieInt(tv, currLevel+1, ts)
		}
	}

	if ts.lastMatchLevel == currLevel {
		pfxIdx := countSetBitsInArr(t.prefixArr[:], idx-1)
		ts.trieData = t.prefixData[pfxIdx]
	}

	return 0
}

func (t *TrieRoot) walkTrieInt(tv *trieVar, level int, ts *trieState, tf TrieIterIntf) int {
	var p int = 0
	var pfxIdx int
	var pfxStr string

	n := 1
	pfxLen := 0
	basePos := 0

	for p = 0; p < PREFIX_ARR_LEN; p++ {
		if n <= 0 {
			pfxLen++
			n = 1 << pfxLen
			basePos = n - 1
		}
		if isBitSetInArr(t.prefixArr[:], p) == true {
			shftBits := TRIE_JMP_LENGTH - pfxLen
			pLevelPfxLen := level * TRIE_JMP_LENGTH
			cval := (p - basePos) << shftBits
			if p == 0 {
				pfxIdx = 0
			} else {
				pfxIdx = countSetBitsInArr(t.prefixArr[:], p-1)
			}
			pfxStr = ""
			for i := 0; i < ts.maxLevels; i++ {
				var pfxVal = int(tv.prefix[i])
				var apStr string = "."
				if i == level {
					pfxVal = cval
				}
				if i == ts.maxLevels-1 {
					apStr = ""
				}
				pfxStr += fmt.Sprintf("%d%s", pfxVal, apStr)
			}
			td := tf.TrieData2String(t.prefixData[pfxIdx])
			tf.TrieNodeWalker(fmt.Sprintf("%20s/%d : %s", pfxStr, int(pfxLen)+pLevelPfxLen, td))
		}
		n--
	}
	for p = 0; p < PTR_ARR_LEN; p++ {
		if isBitSetInArr(t.ptrArr[:], p) == true {
			cval := p
			ptrIdx := countSetBitsInArr(t.ptrArr[:], p-1)

			if t.ptrData[ptrIdx] != nil {
				nextRoot := t.ptrData[ptrIdx]
				tv.prefix[level] = byte(cval)
				nextRoot.walkTrieInt(tv, level+1, ts, tf)
			}
		}
	}
	return 0
}

func (t *TrieRoot) AddTrie(cidr string, data TrieData) int {
	var tv trieVar
	var ts = trieState{data, 0, 0, false, trieVar{}, false, 4, 0}

	pfxLen := cidr2TrieVar(cidr, &tv)

	if pfxLen < 0 {
		return TRIE_ERR_PREFIX
	}

	ret := t.addTrieInt(&tv, 0, pfxLen, &ts)
	if ret != 0 || ts.errCode != 0 {
		return ret
	}

	return 0
}

func (t *TrieRoot) DelTrie(cidr string) int {
	var tv trieVar
	var ts = trieState{0, 0, 0, false, trieVar{}, false, 4, 0}

	pfxLen := cidr2TrieVar(cidr, &tv)

	if pfxLen < 0 {
		return TRIE_ERR_PREFIX
	}

	ret := t.deleteTrieInt(&tv, 0, pfxLen, &ts)
	if ret != 0 || ts.errCode != 0 {
		return TRIE_ERR_NOENT
	}

	return 0
}

func (t *TrieRoot) FindTrie(IP string) (int, *string, TrieData) {
	var tv trieVar
	var ts = trieState{0, 0, 0, false, trieVar{}, false, 4, 0}
	var mCidr string

	cidr := IP + "/32"
	pfxLen := cidr2TrieVar(cidr, &tv)

	if pfxLen < 0 {
		return TRIE_ERR_PREFIX, nil, 0
	}

	t.findTrieInt(&tv, 0, &ts)

	if ts.matchFound == true {
		for i := 0; i < 4; i++ {
			suffix := "."
			if i == 3 {
				suffix = ""
			}
			mCidr += fmt.Sprintf("%d%s", ts.lastMatchTv.prefix[i], suffix)
		}
		mCidr += fmt.Sprintf("/%d", ts.lastMatchPfxLen)
		return 0, &mCidr, ts.trieData
	}
	return TRIE_ERR_NOENT, nil, 0
}

func (t *TrieRoot) Trie2String(tf TrieIterIntf) {
	var ts = trieState{0, 0, 0, false, trieVar{}, false, 4, 0}
	t.walkTrieInt(&trieVar{}, 0, &ts, tf)
}
