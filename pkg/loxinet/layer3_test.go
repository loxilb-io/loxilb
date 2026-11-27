/*
 * Copyright (c) 2026 LoxiLB Authors
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
	"fmt"
	"net"
	"testing"
)

const testPortOsID = 7

// testL3 - a L3H with one port and the given ifas, without the data-path.
// A ruleVIP is registered as addVIPSys does, addrs as "obj=cidr"
func testL3(t *testing.T, routes []string, ruleVIPs []string, addrs map[string]string) *L3H {
	t.Helper()

	zone := new(Zone)
	zone.Rt = RtInit(zone)
	zone.Ports = PortInit()
	port := new(Port)
	port.Name = "eth0"
	zone.Ports.portOmap[testPortOsID] = port
	l3 := L3Init(zone)

	add := func(obj, cidr string) {
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("bad cidr %s: %v", cidr, err)
		}
		key := IfaKey{obj}
		if l3.IfaMap[key] == nil {
			l3.IfaMap[key] = &Ifa{Key: key, Zone: zone}
		}
		l3.IfaMap[key].Ifas = append(l3.IfaMap[key].Ifas, &IfaEnt{IfaAddr: ip, IfaNet: *ipNet})
	}
	for _, vip := range ruleVIPs {
		add(fmt.Sprintf("llb-rule-%s", vip), vip+"/32")
	}
	for obj, cidr := range addrs {
		add(obj, cidr)
	}
	for _, rt := range routes {
		osID := testPortOsID
		if ret := zone.Rt.Trie4.AddTrie(rt, &osID); ret != 0 {
			t.Fatalf("trie add %s: %d", rt, ret)
		}
	}
	return l3
}

func TestIfaSelectAnyRuleVIP(t *testing.T) {
	const vip = "10.0.0.100"
	const portIP = "10.0.0.2"
	const ep = "10.0.0.21"

	tests := []struct {
		name    string
		routes  []string
		addrs   map[string]string
		addr    string
		findAny bool
		wantErr bool
		want    string
	}{
		{
			name:    "before the port address, no route: no source",
			addr:    ep,
			findAny: true,
			wantErr: true,
		},
		{
			// IfaSelect's own fallback: left as it is, neighbour resolution shares it
			name:    "route known, port has no address yet: IfaSelect fallback, unchanged",
			routes:  []string{"10.0.0.0/24"},
			addr:    ep,
			findAny: true,
			want:    vip,
		},
		{
			name:    "after the port address, connected route",
			routes:  []string{"10.0.0.0/24"},
			addrs:   map[string]string{"eth0": portIP + "/24"},
			addr:    ep,
			findAny: true,
			want:    portIP,
		},
		{
			name:    "port address, no route to the end-point: not the VIP",
			addrs:   map[string]string{"eth1": "192.168.0.2/24"},
			addr:    ep,
			findAny: true,
			want:    "192.168.0.2",
		},
		{
			name:    "end-point is the VIP itself",
			addr:    vip,
			findAny: true,
			want:    vip,
		},
		{
			name:    "findAny off, nothing contains the address",
			addr:    ep,
			findAny: false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// IfaMap iterates in random order: repeat so that a VIP visited first shows up
			for i := 0; i < 50; i++ {
				l3 := testL3(t, tt.routes, []string{vip}, tt.addrs)
				ret, sIP, _ := l3.IfaSelectAny(net.ParseIP(tt.addr), tt.findAny)
				if tt.wantErr {
					if ret == 0 {
						t.Fatalf("got source %s, want none", sIP)
					}
					continue
				}
				if ret != 0 || !sIP.Equal(net.ParseIP(tt.want)) {
					t.Fatalf("got %d %s, want %s", ret, sIP, tt.want)
				}
			}
		})
	}
}

// With findAny off the rule VIP filter is never reached: tunnel neighbour
// resolution and the NAT64 source get what they always got
func TestIfaSelectAnyNoFindAnyUnchanged(t *testing.T) {
	const vip = "10.0.0.100"
	const portIP = "10.0.0.2"
	const ep = "10.0.0.21"

	tests := []struct {
		name    string
		routes  []string
		addrs   map[string]string
		addr    string
		wantRet int
		want    string
	}{
		{
			name:    "nothing contains the address",
			addr:    ep,
			wantRet: L3AddrErr,
		},
		{
			name:    "route known, port has no address yet",
			routes:  []string{"10.0.0.0/24"},
			addr:    ep,
			wantRet: L3ObjErr,
		},
		{
			name:   "connected route",
			routes: []string{"10.0.0.0/24"},
			addrs:  map[string]string{"eth0": portIP + "/24"},
			addr:   ep,
			want:   portIP,
		},
		{
			name:  "no route, port subnet contains the address",
			addrs: map[string]string{"eth0": portIP + "/24"},
			addr:  ep,
			want:  portIP,
		},
		{
			name:    "no route, port subnet does not contain the address",
			addrs:   map[string]string{"eth1": "192.168.0.2/24"},
			addr:    ep,
			wantRet: L3AddrErr,
		},
		{
			name: "address is the VIP itself",
			addr: vip,
			want: vip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				l3 := testL3(t, tt.routes, []string{vip}, tt.addrs)
				ret, sIP, _ := l3.IfaSelectAny(net.ParseIP(tt.addr), false)
				if ret != tt.wantRet {
					t.Fatalf("got %d %s, want %d", ret, sIP, tt.wantRet)
				}
				if tt.want != "" && !sIP.Equal(net.ParseIP(tt.want)) {
					t.Fatalf("got %s, want %s", sIP, tt.want)
				}
			}
		})
	}
}

// Neighbour resolution uses this fallback directly: a rule VIP stays usable there
func TestIfaSelectFallbackUnchanged(t *testing.T) {
	const vip = "10.0.0.100"

	l3 := testL3(t, nil, []string{vip}, nil)
	ret, sIP, _ := l3.IfaSelect("eth0", net.ParseIP("10.0.0.21"), true)
	if ret != 0 || !sIP.Equal(net.ParseIP(vip)) {
		t.Fatalf("got %d %s, want %s", ret, sIP, vip)
	}
}
