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
	"net"
	"sync"
	"testing"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
)

// The cluster sync worker and RulesApplyClusterState are exercised without
// the datapath: the worker takes its apply and hook functions as arguments,
// and RulesApplyClusterState only needs mh.has, mh.cloudHook and an empty
// RuleH.

const testInst = "sync-test"

// ciTestSetState - what a BFD notification or the API does: a state change
// recorded under mh.mtx
func ciTestSetState(ch *CIStateH, inst, state string) {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	ch.CIStateUpdate(cmn.HASMod{Instance: inst, State: state, Vip: net.IPv4zero})
}

// syncRecorder - collects what the worker applied and hooked
type syncRecorder struct {
	mx      sync.Mutex
	applied []string
	hooked  []string
	fail    int // how many of the next applies report an abandoned pass
	ch      *CIStateH
}

func (r *syncRecorder) apply(inst string) (string, net.IP, bool) {
	r.mx.Lock()
	defer r.mx.Unlock()
	if r.fail > 0 {
		r.fail--
		return "", nil, false
	}
	state, vip := r.ch.CIStateVipGetInst(inst)
	r.applied = append(r.applied, state)
	return state, vip, true
}

func (r *syncRecorder) hook(_, state, _ string) {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.hooked = append(r.hooked, state)
}

func (r *syncRecorder) snapshot() (applied, hooked []string) {
	r.mx.Lock()
	defer r.mx.Unlock()
	return append([]string(nil), r.applied...), append([]string(nil), r.hooked...)
}

// waitFor - poll until cond holds or the deadline passes
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func startTestWorker(t *testing.T) (*CIStateH, *syncRecorder) {
	t.Helper()
	ch := CIInit(CIKAArgs{})
	rec := &syncRecorder{ch: ch}
	go ch.ciSyncWorker(rec.apply, rec.hook)
	t.Cleanup(func() { ch.syncOnce.Do(func() { close(ch.syncFin) }) })
	return ch, rec
}

// A burst of transitions from several goroutines: whatever the worker applied
// last is the live state, and the hook saw every change of the applied state
// exactly once, in order.
func TestCISyncWorkerAppliesLiveState(t *testing.T) {
	ch, rec := startTestWorker(t)

	states := []string{cmn.CIMasterStateString, cmn.CIBackupStateString}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ciTestSetState(ch, testInst, states[(g+i)%2])
			}
		}(g)
	}
	wg.Wait()
	// Settle on a known final state.
	ciTestSetState(ch, testInst, cmn.CIMasterStateString)

	waitFor(t, "final state applied", func() bool {
		applied, _ := rec.snapshot()
		return len(applied) > 0 && applied[len(applied)-1] == cmn.CIMasterStateString
	})
	// Give any straggler a chance to show up, then check the worker is quiet.
	time.Sleep(50 * time.Millisecond)

	applied, hooked := rec.snapshot()
	live, _ := ch.CIStateGetInst(testInst)
	if applied[len(applied)-1] != live {
		t.Fatalf("last applied %s, live state %s", applied[len(applied)-1], live)
	}
	if len(applied) > 202 {
		t.Fatalf("%d applies for 201 transitions: the worker is not collapsing", len(applied))
	}
	if len(hooked) == 0 || hooked[len(hooked)-1] != live {
		t.Fatalf("hook last saw %v, live state %s", hooked, live)
	}
	for i := 1; i < len(hooked); i++ {
		if hooked[i] == hooked[i-1] {
			t.Fatalf("hook called twice in a row with %s: %v", hooked[i], hooked)
		}
	}
}

// Marks that land while the worker is busy collapse into one pass, and the
// same applied state is not hooked again.
func TestCISyncWorkerCollapsesAndDedupesHook(t *testing.T) {
	ch, rec := startTestWorker(t)

	ciTestSetState(ch, testInst, cmn.CIMasterStateString)
	waitFor(t, "first apply", func() bool { a, _ := rec.snapshot(); return len(a) == 1 })

	// A flip and flip-back: two dirty marks, at most two applies, the hook
	// sees MASTER once more only if BACKUP was applied in between.
	ciTestSetState(ch, testInst, cmn.CIBackupStateString)
	ciTestSetState(ch, testInst, cmn.CIMasterStateString)
	waitFor(t, "flip-back applied", func() bool {
		a, _ := rec.snapshot()
		return len(a) >= 2 && a[len(a)-1] == cmn.CIMasterStateString
	})
	time.Sleep(50 * time.Millisecond)

	applied, hooked := rec.snapshot()
	if len(applied) > 3 {
		t.Fatalf("applied %v for three transitions", applied)
	}
	for i := 1; i < len(hooked); i++ {
		if hooked[i] == hooked[i-1] {
			t.Fatalf("hook not deduplicated: %v", hooked)
		}
	}
	if hooked[len(hooked)-1] != cmn.CIMasterStateString {
		t.Fatalf("hook last saw %v", hooked)
	}
}

// An abandoned pass is not hooked, and the instance is applied on the next
// mark.
func TestCISyncWorkerAbandonedPass(t *testing.T) {
	ch, rec := startTestWorker(t)

	rec.mx.Lock()
	rec.fail = 1
	rec.mx.Unlock()

	ciTestSetState(ch, testInst, cmn.CIMasterStateString)
	time.Sleep(50 * time.Millisecond)
	applied, hooked := rec.snapshot()
	if len(applied) != 0 || len(hooked) != 0 {
		t.Fatalf("abandoned pass was applied %v / hooked %v", applied, hooked)
	}

	// The flip that forced the abandonment re-marks the instance.
	ciTestSetState(ch, testInst, cmn.CIBackupStateString)
	waitFor(t, "apply after abandoned pass", func() bool { a, _ := rec.snapshot(); return len(a) == 1 })
	applied, hooked = rec.snapshot()
	if applied[0] != cmn.CIBackupStateString || len(hooked) != 1 || hooked[0] != cmn.CIBackupStateString {
		t.Fatalf("applied %v hooked %v", applied, hooked)
	}
}

// Same-state and invalid updates neither change nor queue anything.
func TestCIStateUpdateNoOpCases(t *testing.T) {
	ch := CIInit(CIKAArgs{})

	if _, err := ch.CIStateUpdate(cmn.HASMod{Instance: testInst, State: cmn.CIMasterStateString, Vip: net.IPv4zero}); err != nil {
		t.Fatal(err)
	}
	ch.mx.Lock()
	delete(ch.syncDirty, testInst)
	ch.mx.Unlock()
	<-ch.syncSig

	if _, err := ch.CIStateUpdate(cmn.HASMod{Instance: testInst, State: cmn.CIMasterStateString, Vip: net.IPv4zero}); err != nil {
		t.Fatal(err)
	}
	if _, err := ch.CIStateUpdate(cmn.HASMod{Instance: testInst, State: "NO_SUCH_STATE", Vip: net.IPv4zero}); err == nil {
		t.Fatal("invalid state accepted")
	}
	ch.mx.Lock()
	_, dirty := ch.syncDirty[testInst]
	ch.mx.Unlock()
	if dirty {
		t.Fatal("no-op update queued the instance")
	}
	select {
	case <-ch.syncSig:
		t.Fatal("no-op update signalled the worker")
	default:
	}
	if st, _ := ch.CIStateGetInst(testInst); st != cmn.CIMasterStateString {
		t.Fatalf("state changed to %s", st)
	}
}

// testCloudHook - counts prepare and unprepare calls and can make prepare
// fail
type testCloudHook struct {
	prepares, unprepares int
	prepareErr           error
}

func (c *testCloudHook) CloudAPIInit(string) error { return nil }
func (c *testCloudHook) CloudPrepareVIPNetWork() error {
	c.prepares++
	return c.prepareErr
}
func (c *testCloudHook) CloudUnPrepareVIPNetWork() error { c.unprepares++; return nil }
func (c *testCloudHook) CloudDestroyVIPNetWork() error   { return nil }
func (c *testCloudHook) CloudUpdatePrivateIP(net.IP, net.IP, bool) error {
	return nil
}
func (c *testCloudHook) CloudGetPrivateInterfaceID() (int, error) { return 0, nil }

// withApplyFixture - point mh.has and mh.cloudHook at test doubles for the
// duration of the test
func withApplyFixture(t *testing.T, hook *testCloudHook) (*CIStateH, *RuleH) {
	t.Helper()
	savedHas, savedHook := mh.has, mh.cloudHook
	ch := CIInit(CIKAArgs{})
	mh.has = ch
	mh.cloudHook = hook
	t.Cleanup(func() { mh.has, mh.cloudHook = savedHas, savedHook })
	return ch, new(RuleH)
}

// Promoted between the first read and the locked section, with the cloud
// network not prepared: the pass starts over so that prepare runs first.
func TestRulesApplyClusterStateRestartsOnPromotion(t *testing.T) {
	hook := &testCloudHook{}
	ch, R := withApplyFixture(t, hook)
	ciTestSetState(ch, cmn.CIDefault, cmn.CIBackupStateString)

	seams := 0
	R.applyTestSeam = func() {
		seams++
		if seams == 1 {
			ciTestSetState(ch, cmn.CIDefault, cmn.CIMasterStateString)
		}
	}

	state, _, ok := R.RulesApplyClusterState(cmn.CIDefault)
	if !ok || state != cmn.CIMasterStateString {
		t.Fatalf("applied %q ok=%v", state, ok)
	}
	if seams != 2 {
		t.Fatalf("pass ran %d times, want 2", seams)
	}
	if hook.prepares != 1 || !R.cloudPrepared {
		t.Fatalf("prepares %d cloudPrepared %v", hook.prepares, R.cloudPrepared)
	}
}

// A failed prepare is tried once per promotion: the pass goes on and does not
// restart.
func TestRulesApplyClusterStatePrepareFailsOnce(t *testing.T) {
	hook := &testCloudHook{prepareErr: errors.New("aws down")}
	ch, R := withApplyFixture(t, hook)
	ciTestSetState(ch, cmn.CIDefault, cmn.CIMasterStateString)

	seams := 0
	R.applyTestSeam = func() { seams++ }

	state, _, ok := R.RulesApplyClusterState(cmn.CIDefault)
	if !ok || state != cmn.CIMasterStateString {
		t.Fatalf("applied %q ok=%v", state, ok)
	}
	if seams != 1 || hook.prepares != 1 || R.cloudPrepared {
		t.Fatalf("passes %d prepares %d cloudPrepared %v", seams, hook.prepares, R.cloudPrepared)
	}
}

// A promotion that happens while already prepared does not restart, a
// demotion unprepares once, and a second demotion pass does nothing.
func TestRulesApplyClusterStatePrepareUnprepareOnce(t *testing.T) {
	hook := &testCloudHook{}
	ch, R := withApplyFixture(t, hook)

	ciTestSetState(ch, cmn.CIDefault, cmn.CIMasterStateString)
	if _, _, ok := R.RulesApplyClusterState(cmn.CIDefault); !ok || hook.prepares != 1 {
		t.Fatalf("first promotion: ok=%v prepares %d", ok, hook.prepares)
	}
	// Already prepared: a flip-back between the reads must not restart.
	ciTestSetState(ch, cmn.CIDefault, cmn.CIBackupStateString)
	seams := 0
	R.applyTestSeam = func() {
		seams++
		if seams == 1 {
			ciTestSetState(ch, cmn.CIDefault, cmn.CIMasterStateString)
		}
	}
	if state, _, _ := R.RulesApplyClusterState(cmn.CIDefault); state != cmn.CIMasterStateString || seams != 1 || hook.prepares != 1 {
		t.Fatalf("flip-back: state %s passes %d prepares %d", state, seams, hook.prepares)
	}
	R.applyTestSeam = nil

	ciTestSetState(ch, cmn.CIDefault, cmn.CIBackupStateString)
	R.RulesApplyClusterState(cmn.CIDefault)
	R.RulesApplyClusterState(cmn.CIDefault)
	if hook.unprepares != 1 || R.cloudPrepared {
		t.Fatalf("unprepares %d cloudPrepared %v", hook.unprepares, R.cloudPrepared)
	}
}

// Instances other than the default never touch the cloud hook.
func TestRulesApplyClusterStateNonDefaultSkipsCloud(t *testing.T) {
	hook := &testCloudHook{}
	ch, R := withApplyFixture(t, hook)
	ciTestSetState(ch, testInst, cmn.CIMasterStateString)

	state, _, ok := R.RulesApplyClusterState(testInst)
	if !ok || state != cmn.CIMasterStateString || hook.prepares != 0 {
		t.Fatalf("state %s ok=%v prepares %d", state, ok, hook.prepares)
	}
}
