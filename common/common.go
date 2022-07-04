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
package common

import (
	"net"
)

const (
	AU_WORKQ_LEN = 1024
	LU_WORKQ_LEN = 1024
	NU_WORKQ_LEN = 1024
	RU_WORKQ_LEN = 40827
)

const (
	PORT_REAL     = 0x1
	PORT_BONDSIF  = 0x2
	PORT_BOND     = 0x4
	PORT_VLANSIF  = 0x8
	PORT_VLANBR   = 0x10
	PORT_VXLANSIF = 0x20
	PORT_VXLANBR  = 0x40
	PORT_WG       = 0x80
)

type PortMod struct {
	Dev       string
	LinkIndex int
	Ptype     int
	MacAddr   [6]byte
	Link      bool
	State     bool
	Mtu       int
	Master    string
	Real      string
	TunId     int
}

type VlanMod struct {
	Vid       int
	Dev       string
	LinkIndex int
	MacAddr   [6]byte
	Link      bool
	State     bool
	Mtu       int
	TunId     uint32
}

type VlanPortMod struct {
	Vid    int
	Dev    string
	Tagged bool
}

const (
	FDB_PHY  = 0
	FDB_TUN  = 1
	FDB_VLAN = 2
)

type FdbMod struct {
	MacAddr  [6]byte
	BridgeId int
	Dev      string
	Dst      net.IP
	Type     int
}

type Ipv4AddrMod struct {
	Dev string
	Ip  string
}

type Neighv4Mod struct {
	Ip           net.IP
	LinkIndex    int
	State        int
	HardwareAddr net.HardwareAddr
}

type Routev4Mod struct {
	Protocol  int
	Flags     int
	Gw        net.IP
	LinkIndex int
	Dst       net.IPNet
}

type EpSelect uint

const (
	LB_SEL_RR EpSelect = iota
	LB_SEL_HASH
	LB_SEL_PRIO
)

type LbServiceArg struct {
	ServIP   string   `json:"externalIP"`
	ServPort uint16   `json:"port"`
	Proto    string   `json:"protocol"`
	Sel      EpSelect `json:"sel"`
}

type LbEndPointArg struct {
	EpIP   string `json:"endpointIP"`
	EpPort uint16 `json:"targetPort"`
	Weight uint8  `json:"weight"`
}

type LbRuleMod struct {
	Serv LbServiceArg    `json:"serviceArguments"`
	Eps  []LbEndPointArg `json:"endpoints"`
}

type CtInfo struct {
	Dip    net.IP `json:"destinationIP"`
	Sip    net.IP `json:"sourceIP"`
	Dport  uint16 `json:"destinationPort"`
	Sport  uint16 `json:"sourcePort"`
	Proto  string `json:"protocol"`
	CState string `json:"conntrackState"`
	CAct   string `json:"conntrackAct"`
}

type NetHookInterface interface {
	NetPortAdd(*PortMod) (int, error)
	NetPortDel(*PortMod) (int, error)
	NetVlanAdd(*VlanMod) (int, error)
	NetVlanDel(*VlanMod) (int, error)
	NetVlanPortAdd(*VlanPortMod) (int, error)
	NetVlanPortDel(*VlanPortMod) (int, error)
	NetFdbAdd(*FdbMod) (int, error)
	NetFdbDel(*FdbMod) (int, error)
	NetIpv4AddrAdd(*Ipv4AddrMod) (int, error)
	NetIpv4AddrDel(*Ipv4AddrMod) (int, error)
	NetNeighv4Add(*Neighv4Mod) (int, error)
	NetNeighv4Del(*Neighv4Mod) (int, error)
	NetRoutev4Add(*Routev4Mod) (int, error)
	NetRoutev4Del(*Routev4Mod) (int, error)
	NetLbRuleAdd(*LbRuleMod) (int, error)
	NetLbRuleDel(*LbRuleMod) (int, error)
	NetLbRuleGet() ([]LbRuleMod, error)
	NetCtInfoGet() ([]CtInfo, error)
}
