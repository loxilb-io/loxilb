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
	"fmt"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
)

const (
	L3_ERR_BASE = iota - RT_ERR_BASE - 1000
	L3_ADDR_ERR
	L3_OBJ_ERR
	L3_NOFDB_ERR
)

type IfaKey struct {
	Obj string
}

type IfaEnt struct {
	IfaAddr   net.IP
	IfaNet    net.IPNet
	Secondary bool
}

type Ifa struct {
	Key  IfaKey
	Zone *Zone
	Sync DpStatusT
	Ifas []*IfaEnt
}

type L3H struct {
	IfaMap map[IfaKey]*Ifa
	Zone   *Zone
}

func L3Init(zone *Zone) *L3H {
	var nl3h = new(L3H)
	nl3h.IfaMap = make(map[IfaKey]*Ifa)
	nl3h.Zone = zone

	return nl3h
}

// Adds an interface IP address (primary or secondary) and associate it with Obj
// Obj can be anything but usually it is the name of a valid interface
func (l3 *L3H) IfaAdd(Obj string, Cidr string) (int, error) {
	var sec bool = false
	addr, network, err := net.ParseCIDR(Cidr)
	if err != nil {
		return L3_ADDR_ERR, errors.New("ip address parse error")
	}

	key := IfaKey{Obj}
	ifa := l3.IfaMap[key]

	if ifa == nil {
		ifa = new(Ifa)
		ifaEnt := new(IfaEnt)
		ifa.Key.Obj = Obj
		ifa.Zone = l3.Zone
		ifaEnt.IfaAddr = addr
		ifaEnt.IfaNet = *network
		ifa.Ifas = append(ifa.Ifas, ifaEnt)
		l3.IfaMap[key] = ifa

		// ifa needs related self-routes
		ra := RtAttr{0, 0, false}
		_, err = mh.zr.Rt.RtAdd(*network, ROOT_ZONE, ra, nil)
		if err != nil {
			tk.LogIt(tk.LOG_DEBUG, "ifa add - %s:%s self-rt error", addr.String(), Obj)
			return L3_ADDR_ERR, errors.New("self-route add error")
		}

		ifa.DP(DP_CREATE)

		return 0, nil
	}

	for _, ifaEnt := range ifa.Ifas {
		if ifaEnt.IfaAddr.Equal(addr) {
			tk.LogIt(tk.LOG_DEBUG, "ifa add - exists %s:%s", addr.String(), Obj)
			return L3_ADDR_ERR, errors.New("ip address exists")
		}

		// if network part of an added ifa is equal to previously 
		// existing ifa, then it is considered a secondary ifa
		if ifaEnt.IfaNet.IP.Equal(network.IP) {
			pfxSz1, _ := ifaEnt.IfaNet.Mask.Size()
			pfxSz2, _ := network.Mask.Size()

			if pfxSz1 == pfxSz2 {
				sec = true
			}
		}
	}

	ifaEnt := new(IfaEnt)
	ifa.Key.Obj = Obj
	ifaEnt.IfaAddr = addr
	ifaEnt.IfaNet = *network
	ifaEnt.Secondary = sec
	ifa.Ifas = append(ifa.Ifas, ifaEnt)

	// ifa needs to related self-routes
	// FIXME - Code duplication with primary address route above
	ra := RtAttr{0, 0, false}
	_, err = mh.zr.Rt.RtAdd(*network, ROOT_ZONE, ra, nil)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "ifa add - %s:%s self-rt error", addr.String(), Obj)
		return L3_ADDR_ERR, errors.New("self-route add error")
	}

	ifa.DP(DP_CREATE)

	tk.LogIt(tk.LOG_DEBUG, "ifa added %s:%s", addr.String(), Obj)

	return 0, nil
}

// Deletes an interface IP address (primary or secondary) and de-associate from Obj
// Obj can be anything but usually it is the name of a valid interface
func (l3 *L3H) IfaDelete(Obj string, Cidr string) (int, error) {
	var found bool = false
	addr, network, err := net.ParseCIDR(Cidr)
	if err != nil {
		tk.LogIt(tk.LOG_ERROR, "ifa delete - malformed %s:%s", addr.String(), Obj)
		return L3_ADDR_ERR, errors.New("ip address parse error")
	}

	key := IfaKey{Obj}
	ifa := l3.IfaMap[key]

	if ifa == nil {
		tk.LogIt(tk.LOG_ERROR, "ifa delete - no such %s:%s", addr.String(), Obj)
		return L3_ADDR_ERR, errors.New("no such ip address")
	}

	for index, ifaEnt := range ifa.Ifas {
		if ifaEnt.IfaAddr.Equal(addr) {

			if ifaEnt.IfaNet.IP.Equal(network.IP) {
				pfxSz1, _ := ifaEnt.IfaNet.Mask.Size()
				pfxSz2, _ := network.Mask.Size()

				if pfxSz1 == pfxSz2 {
					ifa.Ifas = append(ifa.Ifas[:index], ifa.Ifas[index+1:]...)
					found = true
				}
			}
		}
	}

	if found == true {
		// delete self-routes related to this ifa
		_, err = mh.zr.Rt.RtDelete(*network, ROOT_ZONE)
		if err != nil {
			tk.LogIt(tk.LOG_ERROR, "ifa delete %s:%s self-rt error", addr.String(), Obj)
			// Continue after logging error because there is noway to fallback
		}
		if len(ifa.Ifas) == 0 {
			delete(l3.IfaMap, ifa.Key)

			ifa.DP(DP_REMOVE)

			tk.LogIt(tk.LOG_DEBUG, "ifa deleted %s:%s", addr.String(), Obj)
		}
		return 0, nil
	}

	tk.LogIt(tk.LOG_DEBUG, "ifa delete - no such %s:%s", addr.String(), Obj)
	return L3_ADDR_ERR, errors.New("no such ifa")
}

// Given any ip address, select optimal ip address from Obj's ifa list
// This is useful to determine source ip address when sending traffic
// to the given ip address
func (l3 *L3H) IfaSelect(Obj string, addr net.IP) (int, net.IP) {

	key := IfaKey{Obj}
	ifa := l3.IfaMap[key]

	if ifa == nil {
		return L3_OBJ_ERR, net.IPv4(0, 0, 0, 0)
	}

	for _, ifaEnt := range ifa.Ifas {
		if ifaEnt.Secondary == true {
			continue
		}

		if ifaEnt.IfaNet.Contains(addr) {
			return 0, ifaEnt.IfaAddr
		}
	}

	// Select first IP
	if len(ifa.Ifas) > 0 {
		return 0, ifa.Ifas[0].IfaAddr
	}

	return L3_ADDR_ERR, net.IPv4(0, 0, 0, 0)
}

func Ifa2String(ifa *Ifa, it IterIntf) {
	var str string
	for _, ifaEnt := range ifa.Ifas {
		var flagStr string
		if ifaEnt.Secondary {
			flagStr = "Secondary"
		} else {
			flagStr = "Primary"
		}
		plen, _ := ifaEnt.IfaNet.Mask.Size()
		str = fmt.Sprintf("%s/%d - %s", ifaEnt.IfaAddr.String(), plen, flagStr)
	}

	it.NodeWalker(str)
}

func (l3 *L3H) Ifas2String(it IterIntf) error {
	for _, ifa := range l3.IfaMap {
		Ifa2String(ifa, it)
	}
	return nil
}

func IfaMkString(ifa *Ifa) string {
	var str string
	for _, ifaEnt := range ifa.Ifas {
		var flagStr string
		if ifaEnt.Secondary {
			flagStr = "Secondary"
		} else {
			flagStr = "Primary"
		}
		plen, _ := ifaEnt.IfaNet.Mask.Size()
		str = fmt.Sprintf("%s/%d - %s", ifaEnt.IfaAddr.String(), plen, flagStr)
	}

	return str
}

func (l3 *L3H) IfObjMkString(obj string) string {
	key := IfaKey{obj}
	ifa := l3.IfaMap[key]
	if ifa != nil {
		return IfaMkString(ifa)
	}

	return ""
}

// Sync state of L3 entities to data-path
func (ifa *Ifa) DP(work DpWorkT) int {
	port := ifa.Zone.Ports.PortFindByName(ifa.Key.Obj)

	if port == nil {
		return -1
	}

	// In case of remove request, we need to make sure
	// there are no other port IFAs with similar l2 address
	if work == DP_REMOVE {
		for _, ent := range ifa.Zone.L3.IfaMap {
			if ifa.Zone.Ports.PortL2AddrMatch(ent.Key.Obj, port) == true {
				return 0
			}
		}
	}

	rmWq := new(RouterMacDpWorkQ)
	rmWq.Work = work
	rmWq.Status = &ifa.Sync

	for i := 0; i < 6; i++ {
		rmWq.l2Addr[i] = uint8(port.HInfo.MacAddr[i])
	}

	rmWq.PortNum = port.PortNo

	mh.dp.ToDpCh <- rmWq

	if port.SInfo.PortType&cmn.PORT_VXLANBR == cmn.PORT_VXLANBR {
		rmWq := new(RouterMacDpWorkQ)
		rmWq.Work = work
		rmWq.Status = &ifa.Sync

		if port.SInfo.PortReal == nil {
			return 0
		}

		up := port.SInfo.PortReal

		for i := 0; i < 6; i++ {
			rmWq.l2Addr[i] = uint8(up.HInfo.MacAddr[i])
		}

		rmWq.PortNum = up.PortNo
		rmWq.TunId = port.HInfo.TunId
		rmWq.TunType = DP_TUN_VXLAN
		rmWq.BD = port.L2.Vid

		mh.dp.ToDpCh <- rmWq

	}

	return 0
}
