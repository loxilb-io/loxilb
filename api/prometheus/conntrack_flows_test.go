/*
 * Copyright (c) 2026 NetLOX Inc
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
package prometheus

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
)

// stringKey is the key these metrics used before ConntrackKey was a struct. It
// stays here as the definition of what a flow is.
func stringKey(c cmn.CtInfo) string {
	return fmt.Sprintf("%s|%05d|%s|%05d|%v|%s", c.Sip, c.Sport, c.Dip, c.Dport, c.Proto, c.ServiceName)
}

// referenceCounts is the sweep as it was over the slice of a read, with string
// keys. countFlows must agree with it on any sequence of reads.
func referenceCounts(info []cmn.CtInfo, prev map[string]bool) (flowCounts, map[string]bool) {
	c := flowCounts{newRequests: make(map[string]int), newErrors: make(map[string]int)}
	cur := make(map[string]bool)
	for _, ct := range info {
		if ct.CState == conntrackStateClosed {
			c.closed++
			continue
		}
		key := stringKey(ct)
		if !prev[key] {
			c.newFlows++
			c.newRequests[ct.ServiceName]++
		}
		c.active++
		switch ct.Proto {
		case "tcp":
			c.tcp++
		case "udp":
			c.udp++
		case "sctp":
			c.sctp++
		}
		cur[key] = true
		if isErrorState(ct) {
			c.errors++
			c.newErrors[ct.ServiceName]++
		}
	}
	for key := range prev {
		if !cur[key] {
			c.closed++
			c.left++
		}
	}
	return c, cur
}

func randomRead(r *rand.Rand) []cmn.CtInfo {
	ips := []net.IP{net.IPv4(10, 0, 0, 9), net.IPv4(10, 0, 0, 10), net.IPv4(10, 0, 0, 9).To4(), net.ParseIP("fd00::1"), net.ParseIP("fd00::2")}
	protos := []string{"tcp", "udp", "sctp", "icmp"}
	states := []string{"est", "closed", "h/e", "closed-wait", "fin-wait"}
	acts := []string{"hsync", "err", "abort", ""}
	services := []string{"", "-", "svc-a", "svc-b"}
	info := make([]cmn.CtInfo, r.Intn(40))
	for i := range info {
		// Small ranges, so that flows repeat within a read and across reads
		info[i] = cmn.CtInfo{Sip: ips[r.Intn(len(ips))], Dip: ips[r.Intn(len(ips))], Sport: uint16(r.Intn(3)), Dport: uint16(r.Intn(2)),
			Proto: protos[r.Intn(len(protos))], CState: states[r.Intn(len(states))], CAct: acts[r.Intn(len(acts))], ServiceName: services[r.Intn(len(services))],
			Bytes: uint64(r.Intn(1000)), Pkts: uint64(r.Intn(10))}
	}
	return info
}

func TestCountFlowsMatchesSliceSweep(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for seq := 0; seq < 200; seq++ {
		prev := map[ConntrackKey]flowEntry{}
		refPrev := map[string]bool{}
		for read := 0; read < 6; read++ {
			info := randomRead(r)
			flows := buildConntrackFlows(info)
			got := countFlows(flows, prev)
			want, refCur := referenceCounts(info, refPrev)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("seq %d read %d:\n got %+v\nwant %+v\ninfo %+v", seq, read, got, want, info)
			}
			prev, refPrev = flows, refCur
		}
	}
}

func TestSameKeyAsStringKey(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for i := 0; i < 500; i++ {
		info := randomRead(r)
		byStruct := map[ConntrackKey]string{}
		byString := map[string]ConntrackKey{}
		for _, ct := range info {
			k, s := MakeConntrackKey(ct), stringKey(ct)
			if old, ok := byStruct[k]; ok && old != s {
				t.Fatalf("one struct key, two string keys: %q %q", old, s)
			}
			if old, ok := byString[s]; ok && old != k {
				t.Fatalf("one string key, two struct keys: %+v %+v", old, k)
			}
			byStruct[k], byString[s] = s, k
			if got, want := keyIP(k.Sip), ct.Sip.String(); got != want {
				t.Fatalf("source label %q, want %q", got, want)
			}
			if got, want := keyIP(k.Dip), ct.Dip.String(); got != want {
				t.Fatalf("destination label %q, want %q", got, want)
			}
		}
	}
}

func TestFlowStatsAreTheLastEntrys(t *testing.T) {
	ct := cmn.CtInfo{Sip: net.IPv4(1, 1, 1, 1), Dip: net.IPv4(2, 2, 2, 2), Proto: "tcp", CState: "est", Bytes: 5, Pkts: 1}
	last := ct
	last.Ident, last.Bytes, last.Pkts = "1:2", 7, 2
	flows := buildConntrackFlows([]cmn.CtInfo{ct, last})
	e := flows[MakeConntrackKey(ct)]
	if len(flows) != 1 || e.Stats != (Stats{Bytes: 7, Packets: 2}) || e.nActive != 2 {
		t.Fatalf("got %d flows, %+v", len(flows), e)
	}
}

type ctHook struct {
	cmn.NetHookInterface
	info []cmn.CtInfo
}

func (h ctHook) NetCtInfoGet() ([]cmn.CtInfo, error) { return h.info, nil }

// Meant for -race: the read and the sweeps run on their own, as in Init
func TestConntrackLoopsDoNotRace(t *testing.T) {
	period, oldHooks, oldMutex := PromethusDefaultPeriod, hooks, mutex
	PromethusDefaultPeriod = time.Millisecond
	hooks = ctHook{info: randomRead(rand.New(rand.NewSource(3)))}
	mutex = &sync.Mutex{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	for _, loop := range []func(context.Context){RunGetConntrack, RunActiveConntrackCount, RunProcessedStatistic} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx)
		}()
	}
	wg.Wait()
	PromethusDefaultPeriod, hooks, mutex = period, oldHooks, oldMutex
}
