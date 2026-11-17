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
	"net"
	"testing"

	cmn "github.com/loxilb-io/loxilb/common"
)

// newEPSrcRuleH - a rule handler over a private zone with the given addresses
// per interface, and routes from a prefix to the interface it leaves through.
func newEPSrcRuleH(t *testing.T, addrs map[string][]string, routes map[string]string) *RuleH {
	t.Helper()

	z := &Zone{Rt: newTestRtH(8)}
	z.L3 = L3Init(z)

	for obj, cidrs := range addrs {
		ifa := &Ifa{Key: IfaKey{obj}, Zone: z}
		for _, cidr := range cidrs {
			ip, network, err := net.ParseCIDR(cidr)
			if err != nil {
				t.Fatalf("parse %s: %v", cidr, err)
			}
			ifa.Ifas = append(ifa.Ifas, &IfaEnt{IfaAddr: ip, IfaNet: *network})
		}
		z.L3.IfaMap[ifa.Key] = ifa
	}

	for prefix, obj := range routes {
		nh := &Neigh{OifPort: &Port{Name: obj}}
		if ret := z.Rt.Trie4.AddTrie(prefix, nh); ret != 0 {
			t.Fatalf("add route %s: ret %d", prefix, ret)
		}
	}

	return &RuleH{zone: z}
}

// newFullNATRule - a fullnat lb-rule with one end-point
func newFullNATRule(t *testing.T, z *Zone, vip, privIP, ep string) *ruleEnt {
	t.Helper()

	r := &ruleEnt{zone: z}
	ip, network, err := net.ParseCIDR(vip)
	if err != nil {
		t.Fatalf("parse %s: %v", vip, err)
	}
	r.tuples.l3Dst.addr = net.IPNet{IP: ip, Mask: network.Mask}
	if privIP != "" {
		r.privIP = net.ParseIP(privIP)
	}
	r.act.action = &ruleLBActs{
		mode:      cmn.LBModeFullNAT,
		endPoints: []ruleLBEp{{xIP: net.ParseIP(ep)}},
	}
	return r
}

// TestElectEPSrcFullNATSource - fullnat sources traffic from the VIP unless the
// VIP and the end-point are of different families. An IPv6 VIP in the source of
// an IPv4 end-point reached the datapath as its low 32 bits, and the end-point
// saw packets from an address that is not the LB's (fd00:1::102 -> 0.0.1.2).
func TestElectEPSrcFullNATSource(t *testing.T) {
	ensureLoxinet(t)

	const (
		vip6 = "fd00:1::102/128"
		vip4 = "192.0.2.10/32"
	)
	dualStack := map[string][]string{
		"hs0": {"fd00:1::1/64", "10.0.0.254/24"},
		"hs1": {"172.16.0.254/24"},
	}

	tests := []struct {
		name     string
		addrs    map[string][]string
		routes   map[string]string
		vip      string
		privIP   string
		ep       string
		want     string
		wantRslv bool
	}{
		{
			name:  "NAT64 to a connected end-point uses that subnet's address",
			addrs: dualStack,
			vip:   vip6,
			ep:    "10.0.0.21",
			want:  "10.0.0.254",
		},
		{
			name:  "NAT64 picks the address on the end-point's own subnet",
			addrs: dualStack,
			vip:   vip6,
			ep:    "172.16.0.21",
			want:  "172.16.0.254",
		},
		{
			name:   "NAT64 to a routed end-point uses the egress port's address",
			addrs:  dualStack,
			routes: map[string]string{"198.18.0.0/16": "hs1"},
			vip:    vip6,
			ep:     "198.18.0.21",
			want:   "172.16.0.254",
		},
		{
			name:     "NAT64 waits when no IPv4 address exists",
			addrs:    map[string][]string{"hs0": {"fd00:1::1/64"}},
			vip:      vip6,
			ep:       "10.0.0.21",
			want:     "0.0.0.0",
			wantRslv: true,
		},
		{
			name:     "NAT64 waits when the egress port has only IPv6 addresses",
			addrs:    map[string][]string{"hs0": {"fd00:1::1/64"}},
			routes:   map[string]string{"198.18.0.0/16": "hs0"},
			vip:      vip6,
			ep:       "198.18.0.21",
			want:     "0.0.0.0",
			wantRslv: true,
		},
		{
			name:  "same family IPv6 keeps the VIP",
			addrs: dualStack,
			vip:   vip6,
			ep:    "fd00:1::21",
			want:  "fd00:1::102",
		},
		{
			name:  "same family IPv4 keeps the VIP",
			addrs: dualStack,
			vip:   vip4,
			ep:    "10.0.0.21",
			want:  "192.0.2.10",
		},
		{
			name:   "an IPv4 private address is a valid source for an IPv6 VIP",
			addrs:  dualStack,
			vip:    vip6,
			privIP: "10.0.0.100",
			ep:     "10.0.0.21",
			want:   "10.0.0.100",
		},
		{
			name:  "an end-point equal to the VIP has no source",
			addrs: dualStack,
			vip:   vip4,
			ep:    "192.0.2.10",
			want:  "0.0.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			R := newEPSrcRuleH(t, tc.addrs, tc.routes)
			r := newFullNATRule(t, R.zone, tc.vip, tc.privIP, tc.ep)

			R.electEPSrc(r)

			got := r.act.action.(*ruleLBActs).endPoints[0].rIP
			if !got.Equal(net.ParseIP(tc.want)) {
				t.Errorf("source = %s, want %s", got, tc.want)
			}
			if r.addrRslv != tc.wantRslv {
				t.Errorf("addrRslv = %v, want %v", r.addrRslv, tc.wantRslv)
			}
		})
	}
}

// TestElectEPSrcNAT64SourceArrives - a NAT64 rule that had no IPv4 address to
// use must report a change once one appears, so that RulesSync programs it.
func TestElectEPSrcNAT64SourceArrives(t *testing.T) {
	ensureLoxinet(t)

	R := newEPSrcRuleH(t, map[string][]string{"hs0": {"fd00:1::1/64"}}, nil)
	r := newFullNATRule(t, R.zone, "fd00:1::102/128", "", "10.0.0.21")

	R.electEPSrc(r)
	if !r.addrRslv {
		t.Fatal("rule without an IPv4 address is not waiting for one")
	}

	ip, network, err := net.ParseCIDR("10.0.0.254/24")
	if err != nil {
		t.Fatal(err)
	}
	ifa := R.zone.L3.IfaMap[IfaKey{"hs0"}]
	ifa.Ifas = append(ifa.Ifas, &IfaEnt{IfaAddr: ip, IfaNet: *network})

	if !R.electEPSrc(r) {
		t.Error("no change reported after the IPv4 address appeared")
	}
	if r.addrRslv {
		t.Error("rule is still waiting for an address")
	}
	if got := r.act.action.(*ruleLBActs).endPoints[0].rIP; !got.Equal(ip) {
		t.Errorf("source = %s, want %s", got, ip)
	}
}
