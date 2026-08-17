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

// Metric naming for the core exporter.
//
// Two families are published for every core metric:
//
//   - the LEGACY name this exporter has always used (`active_conntrack_count`,
//     `processed_bytes`, …). Kept verbatim so existing Grafana dashboards,
//     recording rules and alerts keep working. Marked DEPRECATED in the Help
//     string only — never renamed or removed here.
//   - the CANONICAL name, matching loxilb-inference-gateway: `loxilb_`
//     namespace, `_total` suffix on counters, unit suffix on ratios. This is
//     the surface the management UI reads (it resolves canonical-first and
//     falls back to the legacy table, see the UI's LOXILB_ALIASES).
//
// Both carry the same value; see dual.go for the mirroring wrappers. Metrics
// with no legacy counterpart (system utilization, conntrack capacity, per-proto
// packet counters, service/endpoint/client traffic, firewall rule count) are
// published under the canonical name only.
//
// The REST `/metrics/*` JSON endpoints are a SEPARATE, frozen surface keyed by
// the legacy names — see the shared-metric helpers in prometheus.go. Do not
// route those through the canonical names.
const (
	// -- Connection tracking ------------------------------------------------
	LegacyMetricActiveConntrackCount = "active_conntrack_count"
	MetricActiveConntrackCount       = "loxilb_active_conntrack_entries"

	// Capacity of the datapath conntrack table. Canonical-only: the legacy
	// exporter never published it, and it is what turns the raw entry count
	// into a utilization ratio.
	MetricConntrackMaxEntries = "loxilb_conntrack_max_entries"

	LegacyMetricActiveFlowCountTCP  = "active_flow_count_tcp"
	MetricActiveFlowCountTCP        = "loxilb_active_flow_count_tcp"
	LegacyMetricActiveFlowCountUDP  = "active_flow_count_udp"
	MetricActiveFlowCountUDP        = "loxilb_active_flow_count_udp"
	LegacyMetricActiveFlowCountSCTP = "active_flow_count_sctp"
	MetricActiveFlowCountSCTP       = "loxilb_active_flow_count_sctp"
	LegacyMetricNewFlowCount        = "new_flow_count"
	MetricNewFlowCount              = "loxilb_new_flows"

	// Legacy-only. The gateway dropped both as non-measurements: `consumed_lcus`
	// is a synthetic cost estimate, `inactive_flow_count` counts entries that
	// have already left the table. Retained for dashboard compatibility, with
	// deliberately no canonical counterpart.
	LegacyMetricInactiveFlowCount = "inactive_flow_count"
	LegacyMetricConsumedLcus      = "consumed_lcus"

	// Collection-pipeline self-diagnostics. Canonical-only.
	// `loxilb_conntrack_stat_resets_total` is referenced by the shared
	// loxilb-alerts.yml rule set.
	MetricConntrackStatResets        = "loxilb_conntrack_stat_resets_total"
	MetricClosedConnectionsProcessed = "loxilb_closed_connections_processed_total"

	// -- Load balancer ------------------------------------------------------
	LegacyMetricLBRuleCount             = "lb_rule_count"
	MetricLBRuleCount                   = "loxilb_lb_rules"
	LegacyMetricTotalRequests           = "total_requests"
	MetricTotalRequests                 = "loxilb_requests_total"
	LegacyMetricTotalRequestsPerService = "total_requests_per_service"
	MetricTotalRequestsPerService       = "loxilb_service_requests_total"
	LegacyMetricTotalErrors             = "total_errors"
	MetricTotalErrors                   = "loxilb_errors_total"
	LegacyMetricTotalErrorsPerService   = "total_errors_per_service"
	MetricTotalErrorsPerService         = "loxilb_service_errors_total"

	// -- Endpoint health ----------------------------------------------------
	// The legacy names say "host"; the gateway counts the same thing and calls
	// it an endpoint. Same value, both published.
	LegacyMetricHealthyEndpointsCount   = "healthy_host_count"
	MetricHealthyEndpointsCount         = "loxilb_healthy_endpoints"
	LegacyMetricUnhealthyEndpointsCount = "unhealthy_host_count"
	MetricUnhealthyEndpointsCount       = "loxilb_unhealthy_endpoints"

	// -- Firewall -----------------------------------------------------------
	// The legacy families are gauges holding the cumulative datapath counter.
	// The canonical families are real counters fed per-cycle deltas, so
	// rate() works. A gauge cannot become a counter without breaking its
	// consumers, so the two are computed independently rather than mirrored.
	LegacyMetricTotalFwDrops        = "total_fw_drops"
	MetricTotalFwDrops              = "loxilb_fw_drop_packets_total"
	LegacyMetricTotalFwDropsPerRule = "total_fw_drops_per_rule"
	MetricTotalFwDropsPerRule       = "loxilb_fw_rule_drop_packets_total"
	MetricFirewallRulesCount        = "loxilb_firewall_rules"

	// -- System utilization (percentage [0-100]) ----------------------------
	// Canonical-only. Absence of these families is what makes the UI's system
	// usage card render an honest "not reported" rather than a 0%-used pie, so
	// they must stay unpublished when the underlying read fails.
	MetricSystemCPUUtilization    = "loxilb_system_cpu_utilization_percent"
	MetricSystemMemoryUtilization = "loxilb_system_memory_utilization_percent"
	MetricSystemDiskUtilization   = "loxilb_system_disk_utilization_percent"

	// -- Traffic processing -------------------------------------------------
	LegacyMetricProcessedBytesTotal   = "processed_bytes"
	MetricProcessedBytesTotal         = "loxilb_processed_bytes_total"
	LegacyMetricProcessedPacketsTotal = "processed_packets"
	MetricProcessedPacketsTotal       = "loxilb_processed_packets_total"

	LegacyMetricProcessedTCPBytes  = "processed_tcp_bytes"
	MetricProcessedTCPBytes        = "loxilb_processed_tcp_bytes_total"
	LegacyMetricProcessedUDPBytes  = "processed_udp_bytes"
	MetricProcessedUDPBytes        = "loxilb_processed_udp_bytes_total"
	LegacyMetricProcessedSCTPBytes = "processed_sctp_bytes"
	MetricProcessedSCTPBytes       = "loxilb_processed_sctp_bytes_total"

	// Canonical-only: the legacy exporter tracked bytes per protocol but not
	// packets, so there is no name to keep compatible with.
	MetricProcessedTCPPackets  = "loxilb_processed_tcp_packets_total"
	MetricProcessedUDPPackets  = "loxilb_processed_udp_packets_total"
	MetricProcessedSCTPPackets = "loxilb_processed_sctp_packets_total"

	// -- Per-rule interaction -----------------------------------------------
	LegacyMetricLBRuleInteractionBytes   = "lb_rule_interaction_bytes"
	MetricLBRuleInteractionBytes         = "loxilb_lb_rule_interaction_bytes_total"
	LegacyMetricLBRuleInteractionPackets = "lb_rule_interaction_packets"
	MetricLBRuleInteractionPackets       = "loxilb_lb_rule_interaction_packets_total"

	// -- Traffic distribution ----------------------------------------------
	// Canonical-only. `service_traffic_*` comes from the exact datapath rule
	// counters; the `endpoint_`/`client_` breakdowns are conntrack-derived and
	// are a persistent-flow view (see collectLbRuleTraffic).
	MetricServiceTrafficBytes   = "loxilb_service_traffic_bytes_total"
	MetricServiceTrafficPackets = "loxilb_service_traffic_packets_total"
	MetricEndpointTrafficBytes  = "loxilb_endpoint_traffic_bytes_total"
	MetricClientTrafficPackets  = "loxilb_client_traffic_packets_total"

	// Legacy-only ratio gauges. The gateway dropped them: a ratio is derivable
	// in PromQL from the traffic counters above, and a pre-computed one cannot
	// be re-aggregated across a time range.
	LegacyMetricEndpointLoadDistsPerService = "endpoint_load_dists_per_service"
	LegacyMetricTotalLoadDistsPerService    = "total_load_dists_per_service"
)

// deprecatedHelp tags a legacy Help string with the canonical name that
// replaces it, so `curl /metrics | grep DEPRECATED` is a complete migration
// checklist for an operator.
func deprecatedHelp(help, canonicalName string) string {
	return help + " DEPRECATED: use " + canonicalName + "."
}
