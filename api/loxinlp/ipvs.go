/*
 * Copyright (c) 2023 NetLOX Inc
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

package loxinlp

import (
	"github.com/loxilb-io/ipvs"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"os"
	"reflect"
	"time"
)

type ipVSKey struct {
	Address  string
	Protocol string
	Port     uint16
}

type ipvsEndPoint struct {
	EpIP   string
	EpPort uint16
	Weight uint8
}

type ipVSEntry struct {
	Key       ipVSKey
	Type      string
	InValid   bool
	EndPoints []ipvsEndPoint
}

type IpVSH struct {
	RMap   map[ipVSKey]*ipVSEntry
	ticker *time.Ticker
	tDone  chan bool
	handle *ipvs.Handle
}

var ipVSCtx *IpVSH

func (ctx *IpVSH) BuildIpVSDB() []*ipVSEntry {

	var ipVSList []*ipVSEntry
	svcs, err := ctx.handle.GetServices()
	if err != nil {
		tk.LogIt(tk.LogError, "[ipvs] failed to get services\n")
		return nil
	}

	for _, svc := range svcs {
		var newEntry ipVSEntry

		endPoints, err := ctx.handle.GetDestinations(svc)
		if err != nil {
			continue
		}

		newEntry.Type = svc.SchedName
		if svc.SchedName != "rr" {
			continue
		}

		proto := ""
		if svc.Protocol == 1 {
			proto = "icmp"
		} else if svc.Protocol == 6 {
			proto = "tcp"
		} else if svc.Protocol == 17 {
			proto = "udp"
		} else if svc.Protocol == 132 {
			proto = "sctp"
		} else {
			continue
		}

		key := ipVSKey{Address: svc.Address.String(), Protocol: proto, Port: svc.Port}
		for _, endPoint := range endPoints {
			newEntry.EndPoints = append(newEntry.EndPoints, ipvsEndPoint{EpIP: endPoint.Address.String(), EpPort: endPoint.Port, Weight: uint8(endPoint.Weight)})
		}

		if len(newEntry.EndPoints) != 0 {
			if eEnt := ctx.RMap[key]; eEnt != nil {
				if reflect.DeepEqual(eEnt.EndPoints, newEntry.EndPoints) {
					eEnt.InValid = false
					continue
				}
			}

			newEntry.Key = key
			ipVSList = append(ipVSList, &newEntry)
		}
	}
	return ipVSList
}

func IpVSSync() {
	for {
		select {
		case <-ipVSCtx.tDone:
			return
		case <-ipVSCtx.ticker.C:

			for _, ent := range ipVSCtx.RMap {
				ent.InValid = true
			}

			tk.LogIt(tk.LogInfo, "IPVS BuildDB\n")
			ipVSList := ipVSCtx.BuildIpVSDB()

			for _, ent := range ipVSCtx.RMap {
				if ent.InValid {
					lbrule := cmn.LbRuleMod{Serv: cmn.LbServiceArg{ServIP: ent.Key.Address, ServPort: ent.Key.Port, Proto: ent.Key.Protocol, Sel: cmn.LbSelRr}}
					_, err := hooks.NetLbRuleDel(&lbrule)
					if err != nil {
						tk.LogIt(tk.LogError, "IPVS LB %v delete failed\n", ent.Key)
					}
					tk.LogIt(tk.LogInfo, "IPVS ent %v deleted\n", ent.Key)
					delete(ipVSCtx.RMap, ent.Key)
				}
			}

			for _, newEnt := range ipVSList {
				lbrule := cmn.LbRuleMod{Serv: cmn.LbServiceArg{ServIP: newEnt.Key.Address, ServPort: newEnt.Key.Port, Proto: newEnt.Key.Protocol, Sel: cmn.LbSelRr}}
				for _, ep := range newEnt.EndPoints {
					lbrule.Eps = append(lbrule.Eps, cmn.LbEndPointArg{EpIP: ep.EpIP, EpPort: ep.EpPort, Weight: 1})
				}

				_, err := hooks.NetLbRuleAdd(&lbrule)
				if err != nil {
					tk.LogIt(tk.LogError, "IPVS LB %v add failed\n", newEnt.Key)
					continue
				}
				ipVSCtx.RMap[newEnt.Key] = newEnt
				tk.LogIt(tk.LogError, "IPVS ent %v added\n", newEnt.Key)
			}
		}
	}
}

func IpVSInit() {
	ipVSCtx = new(IpVSH)
	ipVSCtx.ticker = time.NewTicker(10 * time.Second)
	ipVSCtx.RMap = make(map[ipVSKey]*ipVSEntry)
	ipVSCtx.tDone = make(chan bool)
	handle, err := ipvs.New("")
	if err != nil {
		tk.LogIt(tk.LogError, "ipvs.New: %s\n", err)
		os.Exit(1)
	}
	ipVSCtx.handle = handle
	go IpVSSync()
}
