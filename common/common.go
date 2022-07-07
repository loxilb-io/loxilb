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

type PortProp uint8

const (
	PORT_PROP_UPP PortProp = 1 << iota
	PORT_PROP_SPAN
)

type DpStatusT uint8

type PortDump struct {
	Name   string         `json:"portName"`
	PortNo int            `json:"portNo"`
	Zone   string         `json:"zone"`
	SInfo  PortSwInfo     `json:"portSoftwareInformation"`
	HInfo  PortHwInfo     `json:"portHardwareInformation"`
	Stats  PortStatsInfo  `json:"portStatisticInformation"`
	L3     PortLayer3Info `json:"portL3Information"`
	L2     PortLayer2Info `json:"portL2Information"`
	Sync   DpStatusT      `json:"DataplaneSync"`
}

type PortStatsInfo struct {
	RxBytes   uint64 `json:"rxBytes"`
	TxBytes   uint64 `json:"txBytes"`
	RxPackets uint64 `json:"rxPackets"`
	TxPackets uint64 `json:"txPackets"`
	RxError   uint64 `json:"rxErrors"`
	TxError   uint64 `json:"txErrors"`
}

type PortHwInfo struct {
	MacAddr    [6]byte `json:"rawMacAddress"`
	MacAddrStr string  `json:"macAddress"`
	Link       bool    `json:"link"`
	State      bool    `json:"state"`
	Mtu        int     `json:"mtu"`
	Master     string  `json:"master"`
	Real       string  `json:"real"`
	TunId      uint32  `json:"tunnelId"`
}

type PortLayer3Info struct {
	Routed     bool     `json:"routed"`
	Ipv4_addrs []string `json:"IPv4Address"`
	Ipv6_addrs []string `json:"IPv6Address"`
}

type PortSwInfo struct {
	OsId       int       `json:"osId"`
	PortType   int       `json:"portType"`
	PortProp   PortProp  `json:"portProp"`
	PortActive bool      `json:"portActive"`
	PortReal   *PortDump `json:"portReal"`
	PortOvl    *PortDump `json:"portOvl"`
	BpfLoaded  bool      `json:"bpfLoaded"`
}

type PortLayer2Info struct {
	IsPvid bool `json:"isPvid"`
	Vid    int  `json:"vid"`
}

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
	Bgp      bool     `json:"bgp"`
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

type UlClArg struct {
	Addr net.IP
	Qfi  uint8
}

type SessTun struct {
	TeID uint32
	Addr net.IP
}

func (ut *SessTun) Equal(ut1 *SessTun) bool {
	if ut.TeID == ut1.TeID && ut.Addr.Equal(ut1.Addr) {
		return true
	}
	return false
}

type SessionMod struct {
	Ident string
	Ip    net.IP
	AnTun SessTun
	CnTun SessTun
}

type SessionUlClMod struct {
	Ident string
	Args  UlClArg
}

type NetHookInterface interface {
	NetPortGet() ([]PortDump, error)
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
	NetSessionAdd(*SessionMod) (int, error)
	NetSessionDel(*SessionMod) (int, error)
	NetSessionUlClAdd(*SessionUlClMod) (int, error)
	NetSessionUlClDel(*SessionUlClMod) (int, error)
}
