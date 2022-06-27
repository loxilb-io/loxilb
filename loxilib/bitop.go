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
    "math/bits"
)

func countAllSetBitsInArr(arr []uint8) int {
    var bCount int = 0
    sz := len(arr)
    
    for i := 0; i < sz; i++ {
        //fmt.Printf("idx %d val 0x%x\n", i, val)
        bCount += bits.OnesCount8(arr[i])
    }

    return bCount
}

func countSetBitsInArr(arr []uint8, bPos int) int {
    bCount := 0
    if int(bPos) >= 8*len(arr) {
        return -1
    }

    arrIdx := bPos / 8
    bPosIdx := 7 - (bPos % 8)

    for i := 0; i <= int(arrIdx); i++ {
        var val uint8
        if i == int(arrIdx) {
            val = arr[i] >> bPosIdx & 0xff

        } else {
            val = arr[i]
        }
        //fmt.Printf("idx %d val 0x%x\n", i, val)
        bCount += bits.OnesCount8(val)
    }

    return bCount
}

func isBitSetInArr(arr []uint8, bPos int) bool {

    if int(bPos) >= 8*len(arr) {
        return false
    }

    arrIdx := bPos / 8
    bPosIdx := 7 - (bPos % 8)

    if (arr[arrIdx]>>bPosIdx)&0x1 == 0x1 {
        return true
    }

    return false
}

func setBitInArr(arr []uint8, bPos int) {

    if int(bPos) >= 8*len(arr) {
        return
    }

    arrIdx := bPos / 8
    bPosIdx := 7 - (bPos % 8)
    arr[arrIdx] |= 0x1 << bPosIdx

    //fmt.Printf("setBit:idx %d val 0x%x\n", arrIdx, arr[arrIdx])
    return
}

func unSetBitInArr(arr []uint8, bPos int) {

    if int(bPos) >= 8*len(arr) {
        return
    }

    arrIdx := bPos / 8
    bPosIdx := 7 - (bPos % 8)
    arr[arrIdx] &= ^(0x1 << bPosIdx)

    //fmt.Printf("setBit:idx %d val 0x%x\n", arrIdx, arr[arrIdx])
    return
}