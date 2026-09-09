/*
 * Copyright (c) 2026 LoxiLB Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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
	"time"
)

func TestDpCtGetAsyncCoalescesWithoutBlocking(t *testing.T) {
	dp := &DpEbpfH{ctBcast: make(chan bool, 1)}
	dp.DpCtGetAsync()

	done := make(chan struct{})
	go func() {
		dp.DpCtGetAsync()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a duplicate CT broadcast request blocked the RPC handler")
	}

	if got := len(dp.ctBcast); got != 1 {
		t.Fatalf("queued CT broadcast requests = %d, want 1", got)
	}
}

func TestDpCTMapFinishSyncUsesRPCResult(t *testing.T) {
	original := mh.dpEbpf
	t.Cleanup(func() { mh.dpEbpf = original })

	ct := &DpCtInfo{
		DIP:    net.ParseIP("172.30.250.201"),
		SIP:    net.ParseIP("192.168.64.2"),
		Dport:  18081,
		Sport:  50100,
		Proto:  "tcp",
		CState: "est",
		NTs:    time.Unix(1, 0),
		XSync:  true,
	}
	mh.dpEbpf = &DpEbpfH{ctMap: map[string]*DpCtInfo{ct.Key(): ct}}
	block := []DpCtInfo{cloneDpCtInfo(ct)}

	dpCTMapFinishSync(block, false, false)
	if !ct.XSync {
		t.Fatal("failed CT add was marked synchronized")
	}

	dpCTMapFinishSync(block, false, true)
	if ct.XSync {
		t.Fatal("successful CT add remained pending")
	}
}

func TestDpCTMapFinishSyncDoesNotAcknowledgeNewerUpdate(t *testing.T) {
	original := mh.dpEbpf
	t.Cleanup(func() { mh.dpEbpf = original })

	ct := &DpCtInfo{
		DIP:    net.ParseIP("172.30.250.202"),
		SIP:    net.ParseIP("192.168.64.2"),
		Dport:  18081,
		Sport:  50100,
		Proto:  "tcp",
		CState: "est",
		NTs:    time.Unix(1, 0),
		XSync:  true,
	}
	mh.dpEbpf = &DpEbpfH{ctMap: map[string]*DpCtInfo{ct.Key(): ct}}
	block := []DpCtInfo{cloneDpCtInfo(ct)}

	ct.NTs = time.Unix(2, 0)
	dpCTMapFinishSync(block, false, true)
	if !ct.XSync {
		t.Fatal("an old RPC result acknowledged a newer CT update")
	}
}
