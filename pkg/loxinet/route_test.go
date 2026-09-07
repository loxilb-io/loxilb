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
	"fmt"
	"net"
	"os"
	"testing"

	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
)

// ensureLoxinet - bring up the stack these tests need. TestLoxinet usually gets
// there first; loxiNetInit must not run twice.
func ensureLoxinet(t *testing.T) {
	t.Helper()

	if mh.zr != nil {
		return
	}

	// loxiNetInit calls log.Fatal when it cannot open its log file, which would
	// take the whole test binary down. Check first and skip instead.
	logfile := fmt.Sprintf("/var/log/loxilb%s.log", os.Getenv("HOSTNAME"))
	f, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Skipf("needs write access to %s: %v", logfile, err)
	}
	f.Close()

	opts.Opts.NoNlp = true
	opts.Opts.NoAPI = true
	opts.Opts.CPUProfile = "none"
	opts.Opts.Prometheus = false
	opts.Opts.K8sAPI = "none"
	opts.Opts.ClusterNodes = "none"
	// These tests only look at RtMap, the trie and the mark pool. Keep eBPF out
	// of it so a run does not touch the host's TC hooks.
	opts.Opts.ProxyModeOnly = true

	loxiNetInit()

	if mh.zr == nil {
		t.Fatal("loxinet init failed")
	}
}

// newTestRtH - a private route table over the initialized zone, so a test can
// pick its own mark pool size and not disturb the shared one.
func newTestRtH(marks uint64) *RtH {
	r := new(RtH)
	r.RtMap = make(map[RtKey]*Rt)
	r.Trie4 = tk.TrieInit(false)
	r.Trie6 = tk.TrieInit(true)
	r.Zone = mh.zr
	r.Mark = tk.NewCounter(1, marks)
	return r
}

func hostCIDR(t *testing.T, ip string) net.IPNet {
	t.Helper()
	_, dst, err := net.ParseCIDR(ip + "/32")
	if err != nil {
		t.Fatalf("parse %s: %v", ip, err)
	}
	return *dst
}

// TestRtDeleteVIPHostRouteReclaimsMark - a rule VIP's self-route must free its
// RtMap entry and its route mark on delete, even though RtAdd never put it in
// the trie so the trie delete misses.
//
// This is the LoadBalancer service churn path. DeleteRuleVIP drops the vipMap
// entry synchronously, while the kernel address event that deletes the route
// arrives later, so IsIPRuleVIP disagrees between insert and delete. Before the
// fix the miss aborted rtDeleteCommon and every distinct VIP leaked one entry
// and one mark for the lifetime of the process.
func TestRtDeleteVIPHostRouteReclaimsMark(t *testing.T) {
	ensureLoxinet(t)

	const marks = 8
	r := newTestRtH(marks)

	// More rounds than the pool holds: a leaked mark per round exhausts it.
	for i := 0; i < marks*3; i++ {
		vip := fmt.Sprintf("198.51.100.%d", i+1)
		dst := hostCIDR(t, vip)

		// The rule exists when the address, and so the self-route, is added.
		mh.zr.Rules.vipMap[vip] = &vipElem{ref: 1}

		ret, err := r.RtAdd(dst, RootZone, RtAttr{Ifi: -1}, nil)
		if ret != 0 || err != nil {
			t.Fatalf("round %d: rt add %s: ret %d err %v", i, vip, ret, err)
		}

		rt := r.RtFind(dst, RootZone)
		if rt == nil {
			t.Fatalf("round %d: rt add %s: not in RtMap", i, vip)
		}
		if rt.Mark == ^uint64(0) {
			t.Fatalf("round %d: rt add %s: mark pool exhausted", i, vip)
		}

		// RtAdd keeps rule VIP host routes out of the trie.
		if terr, _, _ := r.Trie4.FindTrie(vip); terr == 0 {
			t.Fatalf("round %d: %s unexpectedly in trie", i, vip)
		}

		// DeleteRuleVIP forgets the VIP before the address event lands.
		delete(mh.zr.Rules.vipMap, vip)

		ret, err = r.RtDelete(dst, RootZone)
		if ret != 0 || err != nil {
			t.Fatalf("round %d: rt delete %s: ret %d err %v", i, vip, ret, err)
		}
		if r.RtFind(dst, RootZone) != nil {
			t.Fatalf("round %d: rt delete %s: still in RtMap", i, vip)
		}
	}

	if len(r.RtMap) != 0 {
		t.Fatalf("RtMap holds %d entries after %d add/delete rounds", len(r.RtMap), marks*3)
	}
}

// TestRtDeleteClearsTrieEntryAddedBeforeVIP - the other direction of the same
// disagreement. A host route that entered the trie before its rule was
// registered must still leave the trie on delete.
func TestRtDeleteClearsTrieEntryAddedBeforeVIP(t *testing.T) {
	ensureLoxinet(t)

	r := newTestRtH(16)

	const vip = "203.0.113.7"
	dst := hostCIDR(t, vip)

	// No rule yet, so RtAdd does put this one in the trie.
	if ret, err := r.RtAdd(dst, RootZone, RtAttr{Ifi: -1}, nil); ret != 0 || err != nil {
		t.Fatalf("rt add %s: ret %d err %v", vip, ret, err)
	}
	if terr, _, _ := r.Trie4.FindTrie(vip); terr != 0 {
		t.Fatalf("rt add %s: expected a trie entry", vip)
	}

	// The rule shows up afterwards, e.g. NlpGet racing rule registration.
	mh.zr.Rules.vipMap[vip] = &vipElem{ref: 1}
	defer delete(mh.zr.Rules.vipMap, vip)

	if ret, err := r.RtDelete(dst, RootZone); ret != 0 || err != nil {
		t.Fatalf("rt delete %s: ret %d err %v", vip, ret, err)
	}
	if terr, _, _ := r.Trie4.FindTrie(vip); terr == 0 {
		t.Fatalf("rt delete %s: trie entry left behind", vip)
	}
}
