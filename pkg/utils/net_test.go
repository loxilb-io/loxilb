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

package utils

import (
	"fmt"
	"net"
	"testing"

	nlp "github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var testMac = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}

// testLinks - the links the fake lookup below knows about
//
//	2 eth0  usable
//	3 eth1  usable
//	4 down  admin down
//	5 l3    no MAC, as Go reports an all-zero IFLA_ADDRESS
//	6 tun0  point to point
var testLinks = map[int]*net.Interface{
	2: {Index: 2, Name: "eth0", Flags: net.FlagUp, HardwareAddr: testMac},
	3: {Index: 3, Name: "eth1", Flags: net.FlagUp, HardwareAddr: testMac},
	4: {Index: 4, Name: "down", HardwareAddr: testMac},
	5: {Index: 5, Name: "l3", Flags: net.FlagUp},
	6: {Index: 6, Name: "tun0", Flags: net.FlagUp | net.FlagPointToPoint, HardwareAddr: testMac},
}

func testLookup(index int) (*net.Interface, error) {
	if ifi, ok := testLinks[index]; ok {
		return ifi, nil
	}
	return nil, fmt.Errorf("no such link %d", index)
}

func mustCIDR(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("parse %s: %v", cidr, err)
	}
	return n
}

func TestAdvIfUsable(t *testing.T) {
	tests := []struct {
		name string
		ifi  *net.Interface
		want bool
	}{
		{"nil", nil, false},
		{"up with mac", testLinks[2], true},
		{"admin down", testLinks[4], false},
		{"no mac", testLinks[5], false},
		{"point to point", testLinks[6], false},
		{"short mac", &net.Interface{Name: "x", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{1, 2, 3}}, false},
		{"infiniband mac", &net.Interface{Name: "ib0", Flags: net.FlagUp, HardwareAddr: make(net.HardwareAddr, 20)}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AdvIfUsable(tc.ifi); got != tc.want {
				t.Fatalf("AdvIfUsable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMainTableEgressCandsIPv4(t *testing.T) {
	dst := net.ParseIP("10.1.2.3")

	routes := []nlp.Route{
		// default route, netlink leaves Dst nil so only Family says v4
		{Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 2, Priority: 100},
		// covering route, more specific
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.0.0.0/8"), LinkIndex: 3, Priority: 0},
		// multipath: the egress is the first nexthop, LinkIndex is zero
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.1.0.0/16"), Priority: 50,
			MultiPath: []*nlp.NexthopInfo{{LinkIndex: 3}, {LinkIndex: 2}}},
		// blackhole and friends carry no RTA_OIF
		{Type: unix.RTN_BLACKHOLE, Dst: mustCIDR(t, "10.1.2.0/24")},
		{Type: unix.RTN_UNREACHABLE, Dst: mustCIDR(t, "10.1.2.0/24")},
		// does not cover dst
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "192.168.0.0/16"), LinkIndex: 3},
		// covering, but the egress link is unusable
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.1.2.0/24"), LinkIndex: 4},
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.1.2.0/24"), LinkIndex: 5},
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.1.2.0/24"), LinkIndex: 6},
		// covering, but the link is gone by the time we look it up
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "10.1.2.0/24"), LinkIndex: 99},
		// wrong family
		{Type: unix.RTN_UNICAST, Family: unix.AF_INET6, LinkIndex: 2},
	}

	want := []EgressCand{
		{PrefixLen: 0, Priority: 100, IfIndex: 2, IfName: "eth0"},
		{PrefixLen: 8, Priority: 0, IfIndex: 3, IfName: "eth1"},
		{PrefixLen: 16, Priority: 50, IfIndex: 3, IfName: "eth1"},
	}

	got := MainTableEgressCands(dst, routes, testLookup)
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMainTableEgressCandsIPv6(t *testing.T) {
	dst := net.ParseIP("2001:db8::1")

	routes := []nlp.Route{
		{Type: unix.RTN_UNICAST, Family: unix.AF_INET6, LinkIndex: 2, Priority: 1024},
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "2001:db8::/32"), LinkIndex: 3},
		// v4 default route must not answer for a v6 VIP
		{Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 3},
		{Type: unix.RTN_UNICAST, Dst: mustCIDR(t, "2001:db9::/32"), LinkIndex: 3},
	}

	want := []EgressCand{
		{PrefixLen: 0, Priority: 1024, IfIndex: 2, IfName: "eth0"},
		{PrefixLen: 32, Priority: 0, IfIndex: 3, IfName: "eth1"},
	}

	got := MainTableEgressCands(dst, routes, testLookup)
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMainTableEgressCandsNoDst(t *testing.T) {
	routes := []nlp.Route{{Type: unix.RTN_UNICAST, Family: unix.AF_INET, LinkIndex: 2}}
	if got := MainTableEgressCands(nil, routes, testLookup); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

// TestAdvertiseReqParamGuards - neither frame builder should reach a socket
// with parameters it cannot build a frame from. The MAC length rule they also
// enforce is covered by TestAdvIfUsable; exercising it here would need a real
// MAC-less link.
func TestAdvertiseReqParamGuards(t *testing.T) {
	v4 := net.ParseIP("20.20.20.1")
	v6 := net.ParseIP("2001:db8::1")

	tests := []struct {
		name string
		call func() (int, error)
	}{
		{"v4 nil ip", func() (int, error) { return NetAdvertiseVIP4Req(nil, "eth0") }},
		{"v4 empty ifname", func() (int, error) { return NetAdvertiseVIP4Req(v4, "") }},
		{"v4 loopback", func() (int, error) { return NetAdvertiseVIP4Req(v4, "lo") }},
		{"v6 nil ip", func() (int, error) { return NetAdvertiseVI64Req(nil, "eth0") }},
		{"v6 empty ifname", func() (int, error) { return NetAdvertiseVI64Req(v6, "") }},
		{"v6 loopback", func() (int, error) { return NetAdvertiseVI64Req(v6, "lo") }},
		{"v6 with a v4 address", func() (int, error) { return NetAdvertiseVI64Req(v4, "eth0") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ret, err := tc.call()
			if ret == 0 || err == nil {
				t.Fatalf("accepted bad parameters: ret %d err %v", ret, err)
			}
		})
	}
}
