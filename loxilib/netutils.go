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
	"net"
	"unsafe"
)

func Ntohl(i uint32) uint32 {
	return binary.BigEndian.Uint32((*(*[4]byte)(unsafe.Pointer(&i)))[:])
}
func Htonl(i uint32) uint32 {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return *(*uint32)(unsafe.Pointer(&b[0]))
}

func Htons(i uint16) uint16 {
	var j = make([]byte, 2)
	binary.BigEndian.PutUint16(j[0:2], i)
	return *(*uint16)(unsafe.Pointer(&j[0]))
}

func Ntohs(i uint16) uint16 {
	return binary.BigEndian.Uint16((*(*[2]byte)(unsafe.Pointer(&i)))[:])
}

func IPtonl(ip net.IP) uint32 {
	var val uint32

	if len(ip) == 16 {
		val = uint32(ip[12])
		val |= uint32(ip[13]) << 8
		val |= uint32(ip[14]) << 16
		val |= uint32(ip[15]) << 24
	} else {
		val = uint32(ip[0])
		val |= uint32(ip[1]) << 8
		val |= uint32(ip[2]) << 16
		val |= uint32(ip[3]) << 24
	}

	return val
}

func NltoIP(addr uint32) net.IP {
	var dip net.IP

	dip = append(dip, uint8(addr&0xff))
	dip = append(dip, uint8(addr>>8&0xff))
	dip = append(dip, uint8(addr>>16&0xff))
	dip = append(dip, uint8(addr>>24&0xff))

	return dip
}
