/*
 * Copyright (c) 2023 NetLOX Inc
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
	"testing"

	cmn "github.com/loxilb-io/loxilb/common"
	"github.com/prometheus/client_golang/prometheus"
)

// The Prometheus exposition is not covered by the swagger contract, so nothing
// generated can police it. These tests are the only guard on the naming
// side-contract the management UI depends on.

// gather returns the sum of all samples in the named family, and whether the
// family is present in the exposition at all. Presence and zero must stay
// distinguishable — that distinction is the whole point of several of these
// tests.
func gather(t *testing.T, name string) (float64, bool) {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		var sum float64
		for _, m := range mf.GetMetric() {
			switch {
			case m.GetGauge() != nil:
				sum += m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				sum += m.GetCounter().GetValue()
			}
		}
		return sum, true
	}
	return 0, false
}

func mustGather(t *testing.T, name string) float64 {
	t.Helper()
	v, ok := gather(t, name)
	if !ok {
		t.Fatalf("metric family %q is absent from the exposition", name)
	}
	return v
}

// gatherOrZero reads a baseline before a mutation. A labeled family with no
// children yet is genuinely absent from the exposition, which is correct
// behaviour and not something a "before" reading should fail on.
func gatherOrZero(t *testing.T, name string) float64 {
	t.Helper()
	v, _ := gather(t, name)
	return v
}

// TestDualEmitGaugesAgree asserts every dual-emitted gauge publishes both its
// legacy and its canonical name, carrying the same value. A consumer resolving
// either name must see the same number.
func TestDualEmitGaugesAgree(t *testing.T) {
	cases := []struct {
		metric    *dualGauge
		legacy    string
		canonical string
		set       float64
	}{
		{activeConntrackCount, LegacyMetricActiveConntrackCount, MetricActiveConntrackCount, 15},
		{activeFlowCountTcp, LegacyMetricActiveFlowCountTCP, MetricActiveFlowCountTCP, 7},
		{activeFlowCountUdp, LegacyMetricActiveFlowCountUDP, MetricActiveFlowCountUDP, 5},
		{activeFlowCountSctp, LegacyMetricActiveFlowCountSCTP, MetricActiveFlowCountSCTP, 3},
		{healthyHostCount, LegacyMetricHealthyEndpointsCount, MetricHealthyEndpointsCount, 3},
		{unHealthyHostCount, LegacyMetricUnhealthyEndpointsCount, MetricUnhealthyEndpointsCount, 1},
		{ruleCount, LegacyMetricLBRuleCount, MetricLBRuleCount, 4},
		{newFlowCount, LegacyMetricNewFlowCount, MetricNewFlowCount, 9},
	}

	for _, tc := range cases {
		t.Run(tc.canonical, func(t *testing.T) {
			tc.metric.Set(tc.set)
			if got := mustGather(t, tc.legacy); got != tc.set {
				t.Errorf("legacy %s = %v, want %v", tc.legacy, got, tc.set)
			}
			if got := mustGather(t, tc.canonical); got != tc.set {
				t.Errorf("canonical %s = %v, want %v", tc.canonical, got, tc.set)
			}
		})
	}
}

// TestDualEmitCountersAgree is the counter equivalent. Counters are global and
// monotonic, so this measures a delta around the Add rather than an absolute.
func TestDualEmitCountersAgree(t *testing.T) {
	cases := []struct {
		metric    *dualCounter
		legacy    string
		canonical string
	}{
		{processedBytes, LegacyMetricProcessedBytesTotal, MetricProcessedBytesTotal},
		{processedPackets, LegacyMetricProcessedPacketsTotal, MetricProcessedPacketsTotal},
		{processedTCPBytes, LegacyMetricProcessedTCPBytes, MetricProcessedTCPBytes},
		{processedUDPBytes, LegacyMetricProcessedUDPBytes, MetricProcessedUDPBytes},
		{processedSCTPBytes, LegacyMetricProcessedSCTPBytes, MetricProcessedSCTPBytes},
		{totalRequests, LegacyMetricTotalRequests, MetricTotalRequests},
		{totalErrors, LegacyMetricTotalErrors, MetricTotalErrors},
	}

	const add = 42

	for _, tc := range cases {
		t.Run(tc.canonical, func(t *testing.T) {
			legacyBefore := mustGather(t, tc.legacy)
			canonicalBefore := mustGather(t, tc.canonical)

			tc.metric.Add(add)

			if got := mustGather(t, tc.legacy) - legacyBefore; got != add {
				t.Errorf("legacy %s advanced by %v, want %v", tc.legacy, got, add)
			}
			if got := mustGather(t, tc.canonical) - canonicalBefore; got != add {
				t.Errorf("canonical %s advanced by %v, want %v", tc.canonical, got, add)
			}
		})
	}
}

// TestDualEmitVecsAgree covers the labeled families.
func TestDualEmitVecsAgree(t *testing.T) {
	const add = 11

	legacyBefore := gatherOrZero(t, LegacyMetricLBRuleInteractionBytes)
	canonicalBefore := gatherOrZero(t, MetricLBRuleInteractionBytes)

	lbRuleInteractionBytes.WithLabelValues("svc", "1.1.1.1", "2.2.2.2").Add(add)

	if got := mustGather(t, LegacyMetricLBRuleInteractionBytes) - legacyBefore; got != add {
		t.Errorf("legacy vec advanced by %v, want %v", got, add)
	}
	if got := mustGather(t, MetricLBRuleInteractionBytes) - canonicalBefore; got != add {
		t.Errorf("canonical vec advanced by %v, want %v", got, add)
	}

	// Per-service request counters share the label set.
	svcLegacyBefore := gatherOrZero(t, LegacyMetricTotalRequestsPerService)
	svcCanonicalBefore := gatherOrZero(t, MetricTotalRequestsPerService)
	totalRequestsPerService.WithLabelValues("svc").Add(add)
	if got := mustGather(t, LegacyMetricTotalRequestsPerService) - svcLegacyBefore; got != add {
		t.Errorf("legacy per-service vec advanced by %v, want %v", got, add)
	}
	if got := mustGather(t, MetricTotalRequestsPerService) - svcCanonicalBefore; got != add {
		t.Errorf("canonical per-service vec advanced by %v, want %v", got, add)
	}
}

// TestLegacyOnlyFamiliesHaveNoCanonicalTwin pins the deliberate omissions. The
// gateway dropped these as non-measurements; publishing a canonical name for
// them would re-import what it removed and make the UI read them.
func TestLegacyOnlyFamiliesHaveNoCanonicalTwin(t *testing.T) {
	inActiveFlowCount.Set(1)
	consumedLcus.Set(1)
	endpointLoadDistsPerService.WithLabelValues("svc", "2.2.2.2").Set(0.5)
	totalLoadDistsPerService.WithLabelValues("svc").Set(0.5)

	for _, legacy := range []string{
		LegacyMetricInactiveFlowCount,
		LegacyMetricConsumedLcus,
		LegacyMetricEndpointLoadDistsPerService,
		LegacyMetricTotalLoadDistsPerService,
	} {
		if _, ok := gather(t, legacy); !ok {
			t.Errorf("legacy-only family %q must still be published", legacy)
		}
		canonical := "loxilb_" + legacy
		if _, ok := gather(t, canonical); ok {
			t.Errorf("family %q must NOT exist: %q is deliberately legacy-only", canonical, legacy)
		}
	}
}

// TestCanonicalOnlyFamiliesAreRegistered covers the families added for UI
// parity that have no legacy counterpart.
func TestCanonicalOnlyFamiliesAreRegistered(t *testing.T) {
	processedTCPPackets.Add(1)
	processedUDPPackets.Add(1)
	processedSCTPPackets.Add(1)
	serviceTrafficBytes.WithLabelValues("svc").Add(1)
	serviceTrafficPackets.WithLabelValues("svc").Add(1)
	endpointTrafficBytes.WithLabelValues("svc", "2.2.2.2").Add(1)
	clientTrafficPackets.WithLabelValues("svc", "1.1.1.1").Add(1)
	fwRuleCount.Set(3)
	fwDropPacketsTotal.Add(1)
	fwRuleDropPacketsTotal.WithLabelValues("100").Add(1)

	for _, name := range []string{
		MetricProcessedTCPPackets,
		MetricProcessedUDPPackets,
		MetricProcessedSCTPPackets,
		MetricServiceTrafficBytes,
		MetricServiceTrafficPackets,
		MetricEndpointTrafficBytes,
		MetricClientTrafficPackets,
		MetricFirewallRulesCount,
		MetricTotalFwDrops,
		MetricTotalFwDropsPerRule,
		MetricConntrackStatResets,
		MetricClosedConnectionsProcessed,
	} {
		if _, ok := gather(t, name); !ok {
			t.Errorf("canonical family %q is absent", name)
		}
	}
}

// TestUnsampledUtilizationFamiliesAreAbsent is the regression guard for the
// lying-zero defect: an unsampled utilization gauge must not appear in the
// exposition at all, because a consumer cannot tell a registered-but-unset 0
// from a genuine 0% measurement.
//
// This test only holds while nothing else in the run has sampled them, so it
// asserts absence BEFORE its own Set calls.
func TestUnsampledUtilizationFamiliesAreAbsent(t *testing.T) {
	for _, name := range []string{
		MetricSystemCPUUtilization,
		MetricSystemMemoryUtilization,
		MetricSystemDiskUtilization,
		MetricConntrackMaxEntries,
	} {
		if _, ok := gather(t, name); ok {
			t.Errorf("family %q must be absent until first sampled, not published as 0", name)
		}
	}

	systemCPUUtilization.Set(12.5)
	systemMemoryUtilization.Set(40)
	systemDiskUtilization.Set(60)
	SetConntrackMaxEntries(65536)

	if got := mustGather(t, MetricSystemCPUUtilization); got != 12.5 {
		t.Errorf("cpu utilization = %v, want 12.5", got)
	}
	if got := mustGather(t, MetricConntrackMaxEntries); got != 65536 {
		t.Errorf("conntrack max = %v, want 65536", got)
	}
}

// TestLazyGaugeIsAbsentUntilSet asserts the mechanism itself on a private
// registry, so the guarantee holds regardless of what else in the process has
// already sampled. This is the invariant the whole "not reported" vs "reported
// as 0" distinction rests on.
func TestLazyGaugeIsAbsentUntilSet(t *testing.T) {
	reg := prometheus.NewRegistry()
	g := &lazyGauge{name: "test_lazy_gauge", help: "test", reg: reg}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	if len(families) != 0 {
		t.Fatalf("unsampled lazyGauge registered %d families, want 0", len(families))
	}

	g.Set(7)

	families, err = reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}
	if len(families) != 1 || families[0].GetName() != "test_lazy_gauge" {
		t.Fatalf("after Set, gathered %v, want one test_lazy_gauge family", families)
	}
	if got := families[0].GetMetric()[0].GetGauge().GetValue(); got != 7 {
		t.Errorf("lazyGauge value = %v, want 7", got)
	}

	// A second Set must reuse the same registration rather than panicking on a
	// duplicate.
	g.Set(9)
	families, _ = reg.Gather()
	if got := families[0].GetMetric()[0].GetGauge().GetValue(); got != 9 {
		t.Errorf("lazyGauge value after second Set = %v, want 9", got)
	}
}

// TestSetConntrackMaxEntriesIgnoresNonPositive — a 0 capacity would make every
// utilization ratio divide by zero, so it must leave the family absent.
func TestSetConntrackMaxEntriesIgnoresNonPositive(t *testing.T) {
	before, existed := gather(t, MetricConntrackMaxEntries)
	SetConntrackMaxEntries(0)
	SetConntrackMaxEntries(-1)
	after, exists := gather(t, MetricConntrackMaxEntries)
	if existed != exists {
		t.Errorf("non-positive capacity changed family presence: %v -> %v", existed, exists)
	}
	if existed && before != after {
		t.Errorf("non-positive capacity overwrote %v with %v", before, after)
	}
}

func TestParseCounterPacketsBytes(t *testing.T) {
	cases := []struct {
		in      string
		packets uint64
		bytes   uint64
		ok      bool
	}{
		{"10:200", 10, 200, true},
		{"0:0", 0, 0, true},
		{" 10 : 200 ", 10, 200, true},
		{"10", 0, 0, false},
		{"", 0, 0, false},
		{"abc:200", 0, 0, false},
		{"10:abc", 0, 0, false},
		{"-1:200", 0, 0, false},
	}
	for _, tc := range cases {
		p, b, ok := parseCounterPacketsBytes(tc.in)
		if ok != tc.ok || p != tc.packets || b != tc.bytes {
			t.Errorf("parseCounterPacketsBytes(%q) = (%d, %d, %v), want (%d, %d, %v)",
				tc.in, p, b, ok, tc.packets, tc.bytes, tc.ok)
		}
	}
}

// TestParseFwCounterPackets pins the fix for the firewall counter bug: the
// data-plane string is "packets:bytes", so the previous strconv.Atoi over the
// whole string failed on every rule and left total_fw_drops at 0 forever.
func TestParseFwCounterPackets(t *testing.T) {
	cases := []struct {
		in      string
		packets uint64
		ok      bool
	}{
		// The three counter strings observed live on the testbed while the
		// old strconv.Atoi was rejecting every one of them.
		{"0:0", 0, true},
		{"120:7044", 120, true},
		{"30:1761", 30, true},
		{"5:100", 5, true},
		{"12", 12, true},
		{"", 0, false},
		{":100", 0, false},
		{"abc:100", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseFwCounterPackets(tc.in)
		if ok != tc.ok || got != tc.packets {
			t.Errorf("parseFwCounterPackets(%q) = (%d, %v), want (%d, %v)",
				tc.in, got, ok, tc.packets, tc.ok)
		}
	}
}

func lbRule(name, proto, servIP string, port uint16, eps ...cmn.LbEndPointArg) cmn.LbRuleMod {
	return cmn.LbRuleMod{
		Serv: cmn.LbServiceArg{
			ServIP:   servIP,
			ServPort: port,
			Proto:    proto,
			Name:     name,
		},
		Eps: eps,
	}
}

// TestCollectLbRuleTrafficSeedsThenReportsDeltas is the core of the accuracy
// port. The data-plane counters are cumulative, so:
//   - the first cycle must publish NOTHING (pre-existing traffic is not a burst
//     that happened at enable time);
//   - later cycles must publish the delta, not the cumulative value;
//   - a counter that goes backwards (rule re-created) must be treated as a
//     fresh counter whose full value is the delta.
func TestCollectLbRuleTrafficSeedsThenReportsDeltas(t *testing.T) {
	// Isolate from any other test's baselines.
	prevLbEpStats = make(map[string]Stats)
	lbStatsFirstCycle = true

	bytesBefore := mustGather(t, MetricProcessedBytesTotal)
	pktsBefore := mustGather(t, MetricProcessedPacketsTotal)
	tcpPktsBefore := mustGather(t, MetricProcessedTCPPackets)

	rule := lbRule("seedsvc", "tcp", "10.0.0.1", 80,
		cmn.LbEndPointArg{EpIP: "10.1.1.1", EpPort: 8080, Counters: "100:5000"})

	// Cycle 1: seed only.
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})
	if got := mustGather(t, MetricProcessedBytesTotal) - bytesBefore; got != 0 {
		t.Errorf("first cycle published %v bytes; pre-existing counters must only seed a baseline", got)
	}
	if got := mustGather(t, MetricProcessedPacketsTotal) - pktsBefore; got != 0 {
		t.Errorf("first cycle published %v packets; want 0", got)
	}

	// Cycle 2: +50 packets, +2000 bytes.
	rule.Eps[0].Counters = "150:7000"
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})
	if got := mustGather(t, MetricProcessedBytesTotal) - bytesBefore; got != 2000 {
		t.Errorf("second cycle published %v bytes, want the 2000-byte delta", got)
	}
	if got := mustGather(t, MetricProcessedPacketsTotal) - pktsBefore; got != 50 {
		t.Errorf("second cycle published %v packets, want the 50-packet delta", got)
	}
	if got := mustGather(t, MetricProcessedTCPPackets) - tcpPktsBefore; got != 50 {
		t.Errorf("per-protocol TCP packets advanced by %v, want 50", got)
	}

	// Cycle 3: counter reset — the endpoint was re-created, so the full current
	// value is the delta.
	rule.Eps[0].Counters = "10:400"
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})
	if got := mustGather(t, MetricProcessedBytesTotal) - bytesBefore; got != 2400 {
		t.Errorf("after a counter reset total advanced to %v, want 2400 (2000 + full 400)", got)
	}
}

// TestCollectLbRuleTrafficDropsStaleBaselines — rule churn must not leak
// baselines, or a long-running instance accumulates one entry per endpoint that
// ever existed.
func TestCollectLbRuleTrafficDropsStaleBaselines(t *testing.T) {
	prevLbEpStats = make(map[string]Stats)
	lbStatsFirstCycle = true

	two := lbRule("churn", "tcp", "10.0.0.2", 80,
		cmn.LbEndPointArg{EpIP: "10.2.2.1", EpPort: 8080, Counters: "1:10"},
		cmn.LbEndPointArg{EpIP: "10.2.2.2", EpPort: 8080, Counters: "1:10"})
	collectLbRuleTraffic([]cmn.LbRuleMod{two})
	if len(prevLbEpStats) != 2 {
		t.Fatalf("baselines after seeding = %d, want 2", len(prevLbEpStats))
	}

	one := lbRule("churn", "tcp", "10.0.0.2", 80,
		cmn.LbEndPointArg{EpIP: "10.2.2.1", EpPort: 8080, Counters: "2:20"})
	collectLbRuleTraffic([]cmn.LbRuleMod{one})
	if len(prevLbEpStats) != 1 {
		t.Errorf("baselines after removing an endpoint = %d, want 1", len(prevLbEpStats))
	}
}

// TestCollectLbRuleTrafficSkipsUnnamedServices — an unnamed rule carries the "-"
// placeholder. Emitting it would create a series labelled `service="-"` that
// aggregates unrelated rules together.
func TestCollectLbRuleTrafficSkipsUnnamedServices(t *testing.T) {
	prevLbEpStats = make(map[string]Stats)
	lbStatsFirstCycle = true

	svcBefore := gatherOrZero(t, MetricServiceTrafficBytes)
	totalBefore := gatherOrZero(t, MetricProcessedBytesTotal)

	rule := lbRule("-", "tcp", "10.0.0.3", 80,
		cmn.LbEndPointArg{EpIP: "10.3.3.1", EpPort: 8080, Counters: "1:100"})
	collectLbRuleTraffic([]cmn.LbRuleMod{rule}) // seed
	rule.Eps[0].Counters = "2:300"
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})

	if got := mustGather(t, MetricServiceTrafficBytes) - svcBefore; got != 0 {
		t.Errorf("unnamed rule contributed %v to the per-service family, want 0", got)
	}
	// It must still count toward the aggregate — the traffic is real.
	if got := mustGather(t, MetricProcessedBytesTotal) - totalBefore; got != 200 {
		t.Errorf("unnamed rule contributed %v to the aggregate, want its 200-byte delta", got)
	}
}

// TestCollectLbRuleTrafficIgnoresUnparsableCounters — a malformed counter must
// be skipped, never read as 0, which on the next cycle would surface the whole
// cumulative value as a phantom burst.
func TestCollectLbRuleTrafficIgnoresUnparsableCounters(t *testing.T) {
	prevLbEpStats = make(map[string]Stats)
	lbStatsFirstCycle = true

	before := mustGather(t, MetricProcessedBytesTotal)

	rule := lbRule("bad", "tcp", "10.0.0.4", 80,
		cmn.LbEndPointArg{EpIP: "10.4.4.1", EpPort: 8080, Counters: "not-a-counter"})
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})
	collectLbRuleTraffic([]cmn.LbRuleMod{rule})

	if len(prevLbEpStats) != 0 {
		t.Errorf("unparsable counter left %d baselines, want 0", len(prevLbEpStats))
	}
	if got := mustGather(t, MetricProcessedBytesTotal) - before; got != 0 {
		t.Errorf("unparsable counter contributed %v, want 0", got)
	}
}

func TestClampPercent(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{-5, 0}, {0, 0}, {42.5, 42.5}, {100, 100}, {150, 100},
	}
	for _, tc := range cases {
		if got := clampPercent(tc.in); got != tc.want {
			t.Errorf("clampPercent(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestReadCPUUtilizationBaseline — the first sample has no delta, and must
// report that as the errCPUBaseline sentinel rather than 0%, which would claim
// an idle CPU. Skipped where /proc/stat does not exist.
func TestReadCPUUtilizationBaseline(t *testing.T) {
	cpuInited = false
	if _, err := readCPUUtilization(); err != nil {
		if err == errCPUBaseline {
			return // expected on Linux
		}
		t.Skipf("no readable /proc/stat here: %v", err)
	}
	t.Error("first CPU sample returned a value; want the errCPUBaseline sentinel")
}
