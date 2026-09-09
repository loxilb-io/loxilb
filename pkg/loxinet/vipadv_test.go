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
	"context"
	"net"
	"testing"
	"time"

	"github.com/loxilb-io/loxilb/pkg/utils"
)

// portsAre - an isPort test that accepts the listed link indexes
func portsAre(indexes ...int) func(int) bool {
	set := make(map[int]bool, len(indexes))
	for _, i := range indexes {
		set[i] = true
	}
	return func(ifIndex int) bool { return set[ifIndex] }
}

func TestPickEgressCand(t *testing.T) {
	tests := []struct {
		name   string
		cands  []utils.EgressCand
		isPort func(int) bool
		want   int
	}{
		{
			name:   "no candidates",
			cands:  nil,
			isPort: portsAre(2, 3),
			want:   -1,
		},
		{
			name: "longest prefix wins",
			cands: []utils.EgressCand{
				{PrefixLen: 0, Priority: 0, IfIndex: 2},
				{PrefixLen: 24, Priority: 100, IfIndex: 3},
				{PrefixLen: 8, Priority: 0, IfIndex: 3},
			},
			isPort: portsAre(2, 3),
			want:   1,
		},
		{
			name: "lowest priority breaks a tie",
			cands: []utils.EgressCand{
				{PrefixLen: 24, Priority: 200, IfIndex: 2},
				{PrefixLen: 24, Priority: 50, IfIndex: 3},
				{PrefixLen: 24, Priority: 100, IfIndex: 2},
			},
			isPort: portsAre(2, 3),
			want:   1,
		},
		{
			name: "first of an exact tie wins",
			cands: []utils.EgressCand{
				{PrefixLen: 16, Priority: 10, IfIndex: 2},
				{PrefixLen: 16, Priority: 10, IfIndex: 3},
			},
			isPort: portsAre(2, 3),
			want:   0,
		},
		{
			name: "a non port loses to a shorter prefix",
			cands: []utils.EgressCand{
				{PrefixLen: 8, Priority: 0, IfIndex: 2},
				{PrefixLen: 24, Priority: 0, IfIndex: 9},
			},
			isPort: portsAre(2, 3),
			want:   0,
		},
		{
			name: "every candidate is a non port",
			cands: []utils.EgressCand{
				{PrefixLen: 24, Priority: 0, IfIndex: 9},
				{PrefixLen: 0, Priority: 0, IfIndex: 8},
			},
			isPort: portsAre(2, 3),
			want:   -1,
		},
		{
			name: "default route is a valid last resort",
			cands: []utils.EgressCand{
				{PrefixLen: 0, Priority: 100, IfIndex: 2},
			},
			isPort: portsAre(2),
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickEgressCand(tc.cands, tc.isPort); got != tc.want {
				t.Fatalf("pickEgressCand = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestLogVipAdvIfTransitions - the reporter must stay quiet while a VIP keeps
// resolving to the same interface, and speak up on every change including the
// first resolution and the first failure. tk.LogIt is a no-op without a logger,
// so this checks the state it keeps rather than the lines it writes.
func TestLogVipAdvIfTransitions(t *testing.T) {
	const key = "20.20.20.1"
	ip := net.ParseIP(key)

	R := &RuleH{vipMap: map[string]*vipElem{key: {ref: 1}}}
	ent := R.vipMap[key]

	steps := []struct {
		iface  string
		report bool
	}{
		{"", true},      // first pass, nothing resolves
		{"", false},     // still nothing, stay quiet
		{"eth0", true},  // resolved at last
		{"eth0", false}, // unchanged
		{"eth1", true},  // routing moved
		{"", true},      // link went away
		{"eth1", true},  // and came back
	}

	for i, st := range steps {
		beforeIf, beforeSet := ent.advIf, ent.advIfSet
		R.logVipAdvIf(ip, ip, st.iface)

		reported := ent.advIf != beforeIf || ent.advIfSet != beforeSet
		if reported != st.report {
			t.Fatalf("step %d (%q): reported %v, want %v", i, st.iface, reported, st.report)
		}
		if ent.advIf != st.iface || !ent.advIfSet {
			t.Fatalf("step %d (%q): state %q set %v", i, st.iface, ent.advIf, ent.advIfSet)
		}
	}

	// A VIP that is not in the map must not be tracked or panic.
	R.logVipAdvIf(ip, net.ParseIP("20.20.20.2"), "eth0")
	R.logVipAdvIf(ip, nil, "eth0")
}

// burstProbe - a run function that reports when it starts and when its
// context is cancelled
type burstProbe struct {
	started   chan struct{}
	cancelled chan struct{}
}

func newBurstProbe() *burstProbe {
	return &burstProbe{started: make(chan struct{}), cancelled: make(chan struct{})}
}

func (p *burstProbe) run(ctx context.Context) {
	close(p.started)
	<-ctx.Done()
	close(p.cancelled)
}

func waitClosed(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

func assertOpen(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatalf("%s happened, should not have", what)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestVipAdvBurstSchedReplaces(t *testing.T) {
	var s vipAdvBurstSched

	first := newBurstProbe()
	s.arm("inst0", first.run)
	waitClosed(t, first.started, "first burst start")
	assertOpen(t, first.cancelled, "first burst cancel")

	// A second promotion of the same instance replaces the burst.
	second := newBurstProbe()
	s.arm("inst0", second.run)
	waitClosed(t, first.cancelled, "first burst cancel on replace")
	waitClosed(t, second.started, "second burst start")
	assertOpen(t, second.cancelled, "second burst cancel")

	// Another instance runs alongside, untouched.
	other := newBurstProbe()
	s.arm("inst1", other.run)
	waitClosed(t, other.started, "other instance burst start")
	assertOpen(t, second.cancelled, "second burst cancel by other instance")

	// A demotion (nil run) only stops what is running.
	s.arm("inst0", nil)
	waitClosed(t, second.cancelled, "second burst cancel on demotion")
	assertOpen(t, other.cancelled, "other instance burst cancel by inst0 demotion")

	s.arm("inst1", nil)
	waitClosed(t, other.cancelled, "other instance burst cancel")
}

func TestVipAdvBurstSchedForgetsFinished(t *testing.T) {
	var s vipAdvBurstSched

	done := make(chan struct{})
	s.arm("inst0", func(ctx context.Context) { close(done) })
	waitClosed(t, done, "burst run")

	// A finished burst leaves nothing behind, so a later replacement does
	// not cancel a stranger's context.
	deadline := time.After(2 * time.Second)
	for {
		s.mx.Lock()
		n := len(s.active)
		s.mx.Unlock()
		if n == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("finished burst still registered")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Demoting an instance with no burst is a no-op.
	s.arm("inst0", nil)
	s.arm("never", nil)
}
