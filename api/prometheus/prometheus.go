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
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/errors"
	"github.com/loxilb-io/loxilb/options"

	"encoding/json"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

// Define the struct for the metrics
type DipMetric struct {
	Dip   string  `json:"dip"`
	Value float64 `json:"value"`
	Ratio float64 `json:"ratio"`
}

// Define the map type for the outer object
type DipMetrics map[string][]DipMetric

// Define the struct for the metrics
type ServiceDistMetric struct {
	Value float64 `json:"value"`
	Ratio float64 `json:"ratio"`
}

// Define the map type for the outer object
type ServiceDistMetrics map[string]ServiceDistMetric

// Define the struct for the service metrics
type ServiceMetric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Define the map type for the outer object
type RequestMetrics struct {
	TotalRequests           float64         `json:"total_requests"`
	TotalRequestsPerService []ServiceMetric `json:"total_requests_per_service"`
}

// Define the struct for the error metrics
type ErrorMetrics struct {
	TotalErrors           float64         `json:"total_errors"`
	TotalErrorsPerService []ServiceMetric `json:"total_errors_per_service"`
}

// Define the struct for the interaction metrics
type InteractionMetric struct {
	Service string  `json:"service"`
	Sip     string  `json:"sip"`
	Dip     string  `json:"dip"`
	Value   float64 `json:"value"`
}

// Define the map type for the outer object
type ProcessedTrafficMetrics struct {
	LbRuleInteractionBytes   []InteractionMetric `json:"lb_rule_interaction_bytes"`
	LbRuleInteractionPackets []InteractionMetric `json:"lb_rule_interaction_packets"`
}

// Define the struct for the firewall drop metrics per rule
type FwDropMetric struct {
	FwRule string  `json:"fw_rule"`
	Value  float64 `json:"value"`
}

// Define the struct for the firewall drop metrics
type FwDropsMetrics struct {
	TotalFwDrops        float64        `json:"total_fw_drops"`
	TotalFwDropsPerRule []FwDropMetric `json:"total_fw_drops_per_rule"`
}

// Define the Node structure
type Node struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Subtitle      string  `json:"subtitle"`
	Mainstat      float64 `json:"mainstat"`
	Secondarystat float64 `json:"secondarystat,omitempty"`
	Color         string  `json:"color"`
	Icon          string  `json:"icon"`
	NodeRadius    int     `json:"nodeRadius"`
}

// Define the Edge structure
type Edge struct {
	ID            string  `json:"id"`
	Source        string  `json:"source"`
	Target        string  `json:"target"`
	Mainstat      float64 `json:"mainstat"`
	Secondarystat float64 `json:"secondarystat,omitempty"`
	Thickness     int     `json:"thickness"`
	Color         string  `json:"color"`
}

// Define the Nodegraph structure
type NodeGraphShcmea struct {
	SchemaVersion int `json:"schemaVersion"`
	Meta          struct {
		PreferredVisualisationType string `json:"preferredVisualisationType"`
	} `json:"meta"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Stats struct {
	Bytes   uint64
	Packets uint64
}
type ConntrackKey string

type SharedMetric struct {
	Name   string            `json:"name"`
	Value  float64           `json:"value"`
	Labels map[string]string `json:"labels,omitempty"` // Optional labels
}

var (
	hooks                  cmn.NetHookInterface
	ConntrackInfo          []cmn.CtInfo
	EndPointInfo           []cmn.EndPointMod
	LBRuleInfo             []cmn.LbRuleMod
	FWRuleInfo             []cmn.FwRuleMod
	err                    error
	mutex                  *sync.Mutex
	ConntrackStats         map[ConntrackKey]Stats // Key [string] : sip dip pro sport dport
	PreFlowCounts          int
	PromethusDefaultPeriod = 10 * time.Second
	PromethusPartialPeriod = (PromethusDefaultPeriod / 6)
	PromethusLongPeriod    = (PromethusDefaultPeriod * 600) // To reset Period
	prometheusCtx          context.Context
	prometheusCancel       context.CancelFunc
	// Core metrics are published under two names each — the legacy name and the
	// canonical `loxilb_*` name shared with loxilb-inference-gateway. See
	// metric_names.go for the naming contract and dual.go for the mirroring.
	activeConntrackCount = newDualGauge(
		LegacyMetricActiveConntrackCount, MetricActiveConntrackCount,
		"Number of active established connections from clients to targets.",
	)
	activeFlowCountTcp = newDualGauge(
		LegacyMetricActiveFlowCountTCP, MetricActiveFlowCountTCP,
		"Number of concurrent TCP flows (or connections) from clients to targets.",
	)
	activeFlowCountUdp = newDualGauge(
		LegacyMetricActiveFlowCountUDP, MetricActiveFlowCountUDP,
		"Number of concurrent UDP flows (or connections) from clients to targets.",
	)
	activeFlowCountSctp = newDualGauge(
		LegacyMetricActiveFlowCountSCTP, MetricActiveFlowCountSCTP,
		"Number of concurrent SCTP flows (or connections) from clients to targets.",
	)
	// Legacy-only: counts entries that have already left the conntrack table,
	// which the gateway dropped as not being a measurement of anything live.
	inActiveFlowCount = newDualGauge(
		LegacyMetricInactiveFlowCount, "",
		"The average number of concurrent closed flows (or connections) from clients to targets.",
	)
	healthyHostCount = newDualGauge(
		LegacyMetricHealthyEndpointsCount, MetricHealthyEndpointsCount,
		"Number of healthy targets.",
	)
	unHealthyHostCount = newDualGauge(
		LegacyMetricUnhealthyEndpointsCount, MetricUnhealthyEndpointsCount,
		"Number of unhealthy targets.",
	)
	ruleCount = newDualGauge(
		LegacyMetricLBRuleCount, MetricLBRuleCount,
		"Total number of load balancing rules.",
	)
	// Legacy-only: a synthetic cost estimate rather than an observation.
	consumedLcus = newDualGauge(
		LegacyMetricConsumedLcus, "",
		"The number of LCUs used by the load balancer.",
	)
	newFlowCount = newDualGauge(
		LegacyMetricNewFlowCount, MetricNewFlowCount,
		"The number of new connections from clients to targets observed in the last sweep.",
	)
	processedBytes = newDualCounter(
		LegacyMetricProcessedBytesTotal, MetricProcessedBytesTotal,
		"The total number of bytes processed by the load balancer, including protocol and IP headers. Fed from the cumulative data-plane rule counters (exact, includes flows of any lifetime).",
	)
	processedTCPBytes = newDualCounter(
		LegacyMetricProcessedTCPBytes, MetricProcessedTCPBytes,
		"The total number of TCP bytes processed by the load balancer, including TCP/IP headers.",
	)
	processedUDPBytes = newDualCounter(
		LegacyMetricProcessedUDPBytes, MetricProcessedUDPBytes,
		"The total number of UDP bytes processed by the load balancer, including UDP/IP headers.",
	)
	processedSCTPBytes = newDualCounter(
		LegacyMetricProcessedSCTPBytes, MetricProcessedSCTPBytes,
		"The total number of SCTP bytes processed by the load balancer, including SCTP/IP headers.",
	)
	processedPackets = newDualCounter(
		LegacyMetricProcessedPacketsTotal, MetricProcessedPacketsTotal,
		"The total number of packets processed by the load balancer. Fed from the cumulative data-plane rule counters (exact, includes flows of any lifetime).",
	)

	// Per-protocol packet counters. Canonical-only — the legacy exporter
	// tracked bytes per protocol but never packets.
	processedTCPPackets = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricProcessedTCPPackets,
			Help: "The total number of TCP packets processed by the load balancer.",
		},
	)
	processedUDPPackets = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricProcessedUDPPackets,
			Help: "The total number of UDP packets processed by the load balancer.",
		},
	)
	processedSCTPPackets = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricProcessedSCTPPackets,
			Help: "The total number of SCTP packets processed by the load balancer.",
		},
	)

	// ProcessedBytes per LB Rule PromQL : sum(rate(loxilb_lb_rule_interaction_bytes_total[1m])) by (service)
	// ProcessedBytes per endpoint PromQL: sum(rate(loxilb_lb_rule_interaction_bytes_total[1m])) by (dip)
	lbRuleInteractionBytes = newDualCounterVec(
		LegacyMetricLBRuleInteractionBytes, MetricLBRuleInteractionBytes,
		"Total bytes exchanged between load balancer and IPs.",
		[]string{"service", "sip", "dip"},
	)
	lbRuleInteractionPackets = newDualCounterVec(
		LegacyMetricLBRuleInteractionPackets, MetricLBRuleInteractionPackets,
		"Total packets exchanged between load balancer and IPs.",
		[]string{"service", "sip", "dip"},
	)

	// Traffic distribution counters. Canonical-only.
	//
	// service_traffic_* comes from the exact data-plane rule counters. The
	// endpoint_/client_ breakdowns can only come from conntrack, because that
	// is the only place flow identity exists — so they are a PERSISTENT-FLOW
	// VIEW and undercount flows shorter than one sweep. The Help strings say so
	// rather than leaving an operator to discover it from a discrepancy.
	serviceTrafficBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricServiceTrafficBytes,
			Help: "Total bytes per NAMED service, from the exact data-plane rule counters. Unnamed LB rules have no per-service series.",
		},
		[]string{"service"},
	)
	serviceTrafficPackets = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricServiceTrafficPackets,
			Help: "Total packets per NAMED service, from the exact data-plane rule counters. Unnamed LB rules have no per-service series.",
		},
		[]string{"service"},
	)
	endpointTrafficBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricEndpointTrafficBytes,
			Help: "Total bytes per endpoint per service. PERSISTENT-FLOW VIEW: conntrack-sweep derived; flows shorter than one sweep are not counted.",
		},
		[]string{"service", "dip"},
	)
	clientTrafficPackets = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricClientTrafficPackets,
			Help: "Total packets per client per service. PERSISTENT-FLOW VIEW: conntrack-sweep derived; flows shorter than one sweep are not counted.",
		},
		[]string{"service", "sip"},
	)

	// Request counters; requests-per-second is derived in PromQL:
	// rate(loxilb_requests_total[1m])
	totalRequests = newDualCounter(
		LegacyMetricTotalRequests, MetricTotalRequests,
		"Sampled new sessions observed by the conntrack sweep. Sessions born and closed within one sweep are not counted - treat as a trend indicator, not an exact request count.",
	)

	totalRequestsPerService = newDualCounterVec(
		LegacyMetricTotalRequestsPerService, MetricTotalRequestsPerService,
		"Sampled new sessions per service observed by the conntrack sweep. Sessions born and closed within one sweep are not counted.",
		[]string{"service"},
	)

	totalErrors = newDualCounter(
		LegacyMetricTotalErrors, MetricTotalErrors,
		"Total number of errored sessions observed by the conntrack sweep.",
	)

	totalErrorsPerService = newDualCounterVec(
		LegacyMetricTotalErrorsPerService, MetricTotalErrorsPerService,
		"Total number of errored sessions per service.",
		[]string{"service"},
	)

	// Firewall drops. The legacy families are GAUGES holding the cumulative
	// data-plane counter; the canonical families are real COUNTERS fed
	// per-cycle deltas so rate() works. A gauge cannot be turned into a counter
	// without breaking its consumers, so the two are computed side by side in
	// RunFwStatistic rather than mirrored through a dual wrapper.
	totalDropsByFw = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: LegacyMetricTotalFwDrops,
			Help: deprecatedHelp("Cumulative packets dropped by firewall rules.", MetricTotalFwDrops),
		},
	)

	totalDropsByFwPerRule = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: LegacyMetricTotalFwDropsPerRule,
			Help: deprecatedHelp("Cumulative packets dropped by firewall, per rule.", MetricTotalFwDropsPerRule),
		},
		[]string{"fw_rule"},
	)

	fwDropPacketsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricTotalFwDrops,
			Help: "Total number of packets dropped by firewall rules.",
		},
	)

	fwRuleDropPacketsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: MetricTotalFwDropsPerRule,
			Help: "Total number of packets dropped by firewall, per rule preference.",
		},
		[]string{"fw_rule"},
	)

	fwRuleCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: MetricFirewallRulesCount,
			Help: "Number of active firewall rules.",
		},
	)

	// Collection-pipeline self-diagnostics. Canonical-only.
	//
	// These describe the exporter's own view of the datapath rather than
	// traffic, and exist so a suspicious rate() can be attributed: a burst of
	// resets explains a traffic counter that appears to jump.
	counterResetEvents = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricConntrackStatResets,
			Help: "Total number of conntrack statistics reset events detected (a per-flow cumulative counter that went backwards).",
		},
	)
	closedConnectionsProcessed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: MetricClosedConnectionsProcessed,
			Help: "Total number of closed connections whose final metrics were captured, counted once per flow as it leaves the conntrack table.",
		},
	)

	// Legacy-only ratio gauges: a ratio is derivable in PromQL from the traffic
	// counters and a pre-computed one cannot be re-aggregated over a range.
	endpointLoadDistsPerService = newDualGaugeVec(
		LegacyMetricEndpointLoadDistsPerService, "",
		"Ratio of traffic distribution across backend endpoints per service",
		[]string{"service", "dip"},
	)

	totalLoadDistsPerService = newDualGaugeVec(
		LegacyMetricTotalLoadDistsPerService, "",
		"Ratio of total traffic distribution across backend endpoints per service",
		[]string{"service"},
	)

	prevConntrackStats = make(map[ConntrackKey]Stats)
	prevConntrackInfo  = make(map[ConntrackKey]bool)

	// Baselines for the cumulative data-plane counters. The exporter reports
	// deltas, so it must remember what it last saw per endpoint / per rule.
	prevLbEpStats     = make(map[string]Stats)
	lbStatsFirstCycle = true
	prevFwRuleDrops   = make(map[string]uint64)

	// Shared metrics
	sharedMetrics = struct {
		sync.RWMutex
		data map[string]SharedMetric
	}{data: make(map[string]SharedMetric)}

	enableSharedMetrics = true
)

func PrometheusRegister(hook cmn.NetHookInterface) {
	hooks = hook
}

// PrometheusInit initializes the Prometheus metrics and starts the necessary goroutines
func CheckInit() error {
	if hooks == nil {
		return errors.New(http.StatusBadRequest, "Prometheus API hooks are not registered")
	}
	if prometheusCtx == nil {
		return errors.New(http.StatusBadRequest, "Prometheus is not running")
	}
	return nil
}

// OptionStateChange sets the state of Prometheus
func OptionStateChange(state bool) {
	options.Opts.Prometheus = state
}

// PrometheusTurnOff turns off the Prometheus
// prometheusCtx and hooks are set to nil for garbage collection
func PrometheusTurnOff() error {
	prometheusCancel()
	prometheusCancel = nil
	prometheusCtx = nil
	hooks = nil
	return nil
}

// Helper functions for shared metrics
func SetSharedMetric(name string, value float64) {
	sharedMetrics.Lock()
	defer sharedMetrics.Unlock()
	sharedMetrics.data[name] = SharedMetric{Name: name, Value: value}
}

func AddSharedMetric(name string, increment float64) {
	sharedMetrics.Lock()
	defer sharedMetrics.Unlock()
	if metric, exists := sharedMetrics.data[name]; exists {
		metric.Value += increment
		sharedMetrics.data[name] = metric
	} else {
		sharedMetrics.data[name] = SharedMetric{Name: name, Value: increment}
	}
}

func AddLabeledMetric(name string, labels map[string]string, increment float64) {
	sharedMetrics.Lock()
	defer sharedMetrics.Unlock()
	labelsKey := generateLabelsKey(name, labels)
	if metric, exists := sharedMetrics.data[labelsKey]; exists {
		metric.Value += increment
		sharedMetrics.data[labelsKey] = metric
	} else {
		sharedMetrics.data[labelsKey] = SharedMetric{Name: name, Value: increment, Labels: labels}
	}
}

func generateLabelsKey(name string, labels map[string]string) string {
	var builder strings.Builder
	builder.WriteString(name)
	for key, value := range labels {
		builder.WriteString(fmt.Sprintf("|%s=%s", key, value))
	}
	return builder.String()
}

// Helper function to retrieve specific metrics from shared metrics
func metricJSON(metricNames []string) map[string]float64 {
	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	metrics := make(map[string]float64)
	for _, name := range metricNames {
		if value, exists := sharedMetrics.data[name]; exists {
			metrics[name] = float64(value.Value)
		} else {
			tk.LogIt(tk.LogDebug, "Metric %s not found\n", name)
		}
	}
	return metrics
}

// Function to get labeled metrics
func GetLabeledMetrics() []SharedMetric {
	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	metrics := make([]SharedMetric, 0, len(sharedMetrics.data))
	for _, metric := range sharedMetrics.data {
		metrics = append(metrics, metric)
	}
	return metrics
}

func GetFlowCountSM() map[string]float64 {
	// API URL : /metrics/flowcount
	metricNames := []string{
		"active_conntrack_count",
		"active_flow_count_tcp",
		"active_flow_count_udp",
		"active_flow_count_sctp",
		"inactive_flow_count",
	}
	return metricJSON(metricNames)
}

func GetHostCountSM() map[string]float64 {
	// API URL : /metrics/hostcount
	metricNames := []string{
		"healthy_host_count",
		"unhealthy_host_count",
	}
	return metricJSON(metricNames)
}

func GetLBRuleCountSM() map[string]float64 {
	// API URL : /metrics/lbrulecount
	metricNames := []string{
		"lb_rule_count",
	}
	return metricJSON(metricNames)
}

func GetNetFlowCountSM() map[string]float64 {
	// API URL : /metrics/newflowcount
	metricNames := []string{
		"new_flow_count",
	}
	return metricJSON(metricNames)
}

func GetReqCountSM() RequestMetrics {
	metricNames := []string{
		"total_requests",
	}

	metrics := RequestMetrics{}
	metrics.TotalRequests = metricJSON(metricNames)["total_requests"]

	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	totalRequestsPerService := make([]ServiceMetric, 0)
	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "total_requests_per_service") {
			service, ok := metric.Labels["service"]
			if !ok || service == "" {
				service = "default"
			}
			totalRequestsPerService = append(totalRequestsPerService, ServiceMetric{
				Name:  service,
				Value: float64(metric.Value),
			})
		}
	}
	metrics.TotalRequestsPerService = totalRequestsPerService

	return metrics
}

func GetErrCountSM() ErrorMetrics {
	metricNames := []string{
		"total_errors",
	}

	metrics := ErrorMetrics{}
	metrics.TotalErrors = metricJSON(metricNames)["total_errors"]

	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	totalErrorsPerService := make([]ServiceMetric, 0)
	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "total_errors_per_service") {
			service, ok := metric.Labels["service"]
			if !ok || service == "" {
				service = "default"
			}
			totalErrorsPerService = append(totalErrorsPerService, ServiceMetric{
				Name:  service,
				Value: float64(metric.Value),
			})
		}
	}

	metrics.TotalErrorsPerService = totalErrorsPerService

	return metrics
}

func GetProcessedTrafficVecSM() map[string]float64 {
	metricNames := []string{
		"processed_bytes",
		"processed_tcp_bytes",
		"processed_sctp_bytes",
		"processed_udp_bytes",
		"processed_packets",
	}
	return metricJSON(metricNames)
}

func GetLBProcessedTrafficVecSM() ProcessedTrafficMetrics {
	metrics := ProcessedTrafficMetrics{
		LbRuleInteractionBytes:   make([]InteractionMetric, 0),
		LbRuleInteractionPackets: make([]InteractionMetric, 0),
	}

	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	for key, metric := range sharedMetrics.data {
		service, ok := metric.Labels["service"]
		if !ok || service == "" {
			service = "default"
		}

		interactionMetric := InteractionMetric{
			Service: service,
			Sip:     metric.Labels["sip"],
			Dip:     metric.Labels["dip"],
			Value:   float64(metric.Value),
		}

		if strings.HasPrefix(key, "lb_rule_interaction_bytes") {
			metrics.LbRuleInteractionBytes = append(metrics.LbRuleInteractionBytes, interactionMetric)
		} else if strings.HasPrefix(key, "lb_rule_interaction_packets") {
			metrics.LbRuleInteractionPackets = append(metrics.LbRuleInteractionPackets, interactionMetric)
		}
	}

	return metrics
}

func GetEpDistTrafficVecSM() DipMetrics {
	// API URL : /metrics/epdisttraffic
	serviceTraffic := make(map[string]float64)
	serviceDipTraffic := make(map[string]map[string]float64)

	// Read lock to ensure thread-safe access to sharedMetrics.data
	sharedMetrics.RLock()
	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "lb_rule_interaction_bytes") {
			service, ok := metric.Labels["service"]
			if !ok || service == "" || service == "-" {
				service = "default"
			}
			dip := metric.Labels["dip"]

			if _, exists := serviceTraffic[service]; !exists {
				serviceTraffic[service] = 0
				serviceDipTraffic[service] = make(map[string]float64)
			}

			serviceTraffic[service] += metric.Value
			serviceDipTraffic[service][dip] += metric.Value
		}
	}
	sharedMetrics.RUnlock()

	// Calculate distribution ratio
	metrics := make(DipMetrics)
	for service, totalTraffic := range serviceTraffic {
		distribution := make([]DipMetric, 0)
		for dip, dipTraffic := range serviceDipTraffic[service] {
			ratio := float64(dipTraffic) / float64(totalTraffic)
			distribution = append(distribution, DipMetric{
				Dip:   dip,
				Value: dipTraffic,
				Ratio: ratio,
			})
		}
		metrics[service] = distribution
	}

	return metrics
}

func GetServiceDistTrafficVecSM() ServiceDistMetrics {
	// API URL : /metrics/servicedisttraffic
	serviceTraffic := make(map[string]float64)

	// Read lock to ensure thread-safe access to sharedMetrics.data
	sharedMetrics.RLock()
	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "lb_rule_interaction_bytes") {
			service, ok := metric.Labels["service"]
			if !ok || service == "" || service == "-" {
				service = "default"
			}

			if _, exists := serviceTraffic[service]; !exists {
				serviceTraffic[service] = 0
			}

			serviceTraffic[service] += metric.Value
		}
	}
	sharedMetrics.RUnlock()

	// Calculate distribution ratio
	metrics := make(ServiceDistMetrics)
	totalTraffic := 0.0
	for _, traffic := range serviceTraffic {
		totalTraffic += traffic
	}

	for service, traffic := range serviceTraffic {
		ratio := traffic / totalTraffic
		metrics[service] = ServiceDistMetric{
			Value: traffic,
			Ratio: ratio,
		}
	}

	return metrics
}

func GetFwDropsSM() FwDropsMetrics {
	metricNames := []string{
		"total_fw_drops",
	}

	metrics := FwDropsMetrics{}
	metrics.TotalFwDrops = metricJSON(metricNames)["total_fw_drops"]

	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	totalDropsPerRule := make([]FwDropMetric, 0)
	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "total_fw_drops_per_rule") {
			totalDropsPerRule = append(totalDropsPerRule, FwDropMetric{
				FwRule: metric.Labels["fw_rule"],
				Value:  float64(metric.Value),
			})
		}
	}
	metrics.TotalFwDropsPerRule = totalDropsPerRule

	return metrics
}

func GetReqCountPerClientSM() map[string]float64 {
	clientRequests := make(map[string]float64)

	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "lb_rule_interaction_packets") {
			// EXTRACT CLIENT IP(ip) FROM LABELS
			clientIP := metric.Labels["sip"]
			if _, exists := clientRequests[clientIP]; !exists {
				clientRequests[clientIP] = 0
			}
			clientRequests[clientIP] += float64(metric.Value)
		}
	}

	resp := make(map[string]float64)
	for clientIP, count := range clientRequests {
		resp[clientIP] = count
	}

	return resp
}

func GetNodeGraphSM() NodeGraphShcmea {
	return generateNodeGraphSchema("")
}

func GetNodeGraphServiceSM(service string) NodeGraphShcmea {
	return generateNodeGraphSchema(service)
}

func generateNodeGraphSchema(service string) NodeGraphShcmea {
	sharedMetrics.RLock()
	defer sharedMetrics.RUnlock()

	// Define temp data
	tmpData := make([]map[string]interface{}, 0, len(sharedMetrics.data))

	for key, metric := range sharedMetrics.data {
		if strings.HasPrefix(key, "lb_rule_interaction_bytes") && (service == "" || metric.Labels["service"] == service) {
			svc := metric.Labels["service"]
			if svc == "" || svc == "-" {
				svc = "default"
				continue // Skip appending to tmpData
			}
			dip := metric.Labels["dip"]
			if dip == "" {
				dip = "na"
			}
			sip := metric.Labels["sip"]
			if sip == "" {
				sip = "na"
			}
			value := float64(metric.Value)
			tmpData = append(tmpData, map[string]interface{}{
				"service": svc,
				"dip":     dip,
				"sip":     sip,
				"value":   value,
			})
		}
	}

	// Generate Node data
	nodeMap := make(map[string]Node)
	for _, data := range tmpData {
		dip := data["dip"].(string)
		sip := data["sip"].(string)
		value := data["value"].(float64)
		service := data["service"].(string)

		if node, exists := nodeMap[service]; exists {
			node.Mainstat += value
			nodeMap[service] = node
		} else {
			nodeMap[service] = Node{
				ID:       service,
				Title:    service,
				Mainstat: value,
				Color:    "blue",
			}
		}

		if node, exists := nodeMap[dip]; exists {
			node.Mainstat += value
			nodeMap[dip] = node
		} else {
			nodeMap[dip] = Node{
				ID:       dip,
				Title:    dip,
				Mainstat: value,
				Color:    "green",
			}
		}

		if node, exists := nodeMap[sip]; exists {
			node.Mainstat += value
			nodeMap[sip] = node
		} else {
			nodeMap[sip] = Node{
				ID:       sip,
				Title:    sip,
				Mainstat: value,
				Color:    "yellow",
			}
		}
	}

	nodes := make([]Node, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	edges := make([]Edge, 0, len(tmpData)*2)
	for _, data := range tmpData {
		dip := data["dip"].(string)
		sip := data["sip"].(string)
		service := data["service"].(string)
		value := data["value"].(float64)

		edges = append(edges, Edge{
			ID:        fmt.Sprintf("%s-%s", sip, service),
			Source:    sip,
			Target:    service,
			Mainstat:  value,
			Thickness: 4,
			Color:     "cyan",
		})

		edges = append(edges, Edge{
			ID:        fmt.Sprintf("%s-%s", service, dip),
			Source:    service,
			Target:    dip,
			Mainstat:  value,
			Thickness: 4,
			Color:     "orange",
		})
	}

	return NodeGraphShcmea{
		SchemaVersion: 37,
		Meta: struct {
			PreferredVisualisationType string `json:"preferredVisualisationType"`
		}{
			PreferredVisualisationType: "nodeGraph",
		},
		Nodes: nodes,
		Edges: edges,
	}
}

func Init() {
	prometheusCtx, prometheusCancel = context.WithCancel(context.Background())

	// Make Conntrack Statistic map
	ConntrackStats = make(map[ConntrackKey]Stats)
	mutex = &sync.Mutex{}

	// Reset the data-plane counter baselines so a re-enable starts fresh and
	// pre-existing cumulative counters are not replayed as a burst.
	prevLbEpStats = make(map[string]Stats)
	lbStatsFirstCycle = true
	prevFwRuleDrops = make(map[string]uint64)

	go RunGetConntrack(prometheusCtx)
	go RunGetEndpoint(prometheusCtx)
	go RunGetFwRule(prometheusCtx)

	go RunActiveConntrackCount(prometheusCtx)
	go RunHostCount(prometheusCtx)
	go RunProcessedStatistic(prometheusCtx)
	go RunResetCounts(prometheusCtx)
	go RunGetLBRule(prometheusCtx)
	go RunLcusCalculator(prometheusCtx)
	go RunFwStatistic(prometheusCtx)
	go RunSystemUtilization(prometheusCtx)

}

func toJSON(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(bytes)
}

func MakeConntrackKey(c cmn.CtInfo) (key ConntrackKey) {
	return ConntrackKey(fmt.Sprintf("%s|%05d|%s|%05d|%v|%s",
		c.Sip, c.Sport, c.Dip, c.Dport, c.Proto, c.ServiceName))
}

func isErrorState(c cmn.CtInfo) bool {
	// Define your error conditions here.
	return c.CState == "h/e" || c.CState == "closed-wait" || c.CAct == "err" || c.CAct == "abort"
}

func RunResetCounts(ctx context.Context) {
	ticker := time.NewTicker(PromethusLongPeriod)
	defer ticker.Stop()
	for {
		// Statistic reset
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mutex.Lock()
			ConntrackStats = map[ConntrackKey]Stats{}
			mutex.Unlock()
		}
	}
}

func RunGetConntrack(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ConntrackInfo, err = hooks.NetCtInfoGet()
		if err != nil {
			tk.LogIt(tk.LogDebug, "[Prometheus] Error occurred while getting conntrack info: %v\n", err)
			continue
		}
		localStats := make(map[ConntrackKey]Stats, len(ConntrackInfo))
		for _, ct := range ConntrackInfo {
			key := MakeConntrackKey(ct)
			localStats[key] = Stats{
				Bytes:   ct.Bytes,
				Packets: ct.Pkts,
			}
		}

		mutex.Lock()
		ConntrackStats = localStats
		mutex.Unlock()

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetEndpoint(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			info, err := hooks.NetEpHostGet()
			if err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occurred while getting endpoint info: %v\n", err)
				continue
			}

			mutex.Lock()
			EndPointInfo = info
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetLBRule(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		info, err := hooks.NetLbRuleGet()
		if err != nil {
			tk.LogIt(tk.LogDebug, "[Prometheus] Error occurred while getting LB rule info: %v\n", err)
			continue
		}

		mutex.Lock()
		LBRuleInfo = info
		mutex.Unlock()

		ruleCount.Set(float64(len(info)))

		if enableSharedMetrics {
			SetSharedMetric("lb_rule_count", float64(len(info)))
		}

		collectLbRuleTraffic(info)

		time.Sleep(PromethusDefaultPeriod)
	}
}

// parseCounterPacketsBytes parses a data-plane counter string formatted as
// "packets:bytes" (see pkg/loxinet/rules.go).
func parseCounterPacketsBytes(counter string) (packets, bytes uint64, ok bool) {
	fields := strings.SplitN(counter, ":", 2)
	if len(fields) != 2 {
		return 0, 0, false
	}
	p, errP := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	b, errB := strconv.ParseUint(strings.TrimSpace(fields[1]), 10, 64)
	if errP != nil || errB != nil {
		return 0, 0, false
	}
	return p, b, true
}

// parseFwCounterPackets parses the same "packets:bytes" data-plane counter and
// returns only the packet count, which is what a firewall drop counter means.
func parseFwCounterPackets(counter string) (uint64, bool) {
	fields := strings.SplitN(counter, ":", 2)
	if len(fields) == 0 || fields[0] == "" {
		return 0, false
	}
	v, parseErr := strconv.ParseUint(strings.TrimSpace(fields[0]), 10, 64)
	if parseErr != nil {
		return 0, false
	}
	return v, true
}

// collectLbRuleTraffic feeds the aggregate traffic counters (processed_* and
// the per-service service_traffic_*) from the CUMULATIVE data-plane per-endpoint
// rule counters, accumulating per-cycle deltas.
//
// Rationale: the conntrack-sweep path only ever sees flows that are alive across
// a sweep boundary, so with short-lived connections it undercounts
// systematically (measured on the gateway: DP rule counter +869 pkts vs +43 via
// the sweep). The DP counters are exact. Flow-identity breakdowns (per-client
// sip, per-endpoint dip) and the active-connection gauges necessarily stay
// conntrack-derived and are documented as a persistent-flow view.
func collectLbRuleTraffic(info []cmn.LbRuleMod) {
	seed := lbStatsFirstCycle

	var totBytes, totPackets uint64
	protoBytes := make(map[string]uint64, 3)
	protoPackets := make(map[string]uint64, 3)
	svcBytes := make(map[string]uint64)
	svcPackets := make(map[string]uint64)

	currentEps := make(map[string]bool, len(prevLbEpStats))
	for i := range info {
		rule := &info[i]
		proto := strings.ToLower(rule.Serv.Proto)
		svc := rule.Serv.Name
		if svc == "-" { // placeholder for unnamed rules, same contract as MakeConntrackKey
			svc = ""
		}
		ruleIdent := fmt.Sprintf("%s|%d|%d|%s", rule.Serv.ServIP, rule.Serv.ServPort, rule.Serv.BlockNum, proto)
		for j := range rule.Eps {
			ep := &rule.Eps[j]
			pkts, bytes, ok := parseCounterPacketsBytes(ep.Counters)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s|%s|%d", ruleIdent, ep.EpIP, ep.EpPort)
			currentEps[key] = true

			// First sight, or a counter reset because the rule/endpoint was
			// re-created with a fresh DP counter: the full current value is the
			// delta.
			deltaPkts, deltaBytes := pkts, bytes
			if prev, seen := prevLbEpStats[key]; seen && pkts >= prev.Packets && bytes >= prev.Bytes {
				deltaPkts = pkts - prev.Packets
				deltaBytes = bytes - prev.Bytes
			}
			prevLbEpStats[key] = Stats{Bytes: bytes, Packets: pkts}
			if seed {
				// Baseline-only: the cumulative totals of rules that already
				// existed must not appear as a phantom burst on (re-)enable.
				continue
			}

			totBytes += deltaBytes
			totPackets += deltaPkts
			protoBytes[proto] += deltaBytes
			protoPackets[proto] += deltaPkts
			if svc != "" {
				svcBytes[svc] += deltaBytes
				svcPackets[svc] += deltaPkts
			}
		}
	}

	// Drop baselines for endpoints that no longer exist so rule churn cannot
	// leak memory.
	for key := range prevLbEpStats {
		if !currentEps[key] {
			delete(prevLbEpStats, key)
		}
	}

	lbStatsFirstCycle = false
	if seed {
		return
	}

	processedBytes.Add(float64(totBytes))
	processedPackets.Add(float64(totPackets))
	processedTCPBytes.Add(float64(protoBytes["tcp"]))
	processedUDPBytes.Add(float64(protoBytes["udp"]))
	processedSCTPBytes.Add(float64(protoBytes["sctp"]))
	processedTCPPackets.Add(float64(protoPackets["tcp"]))
	processedUDPPackets.Add(float64(protoPackets["udp"]))
	processedSCTPPackets.Add(float64(protoPackets["sctp"]))

	for svc, b := range svcBytes {
		serviceTrafficBytes.WithLabelValues(svc).Add(float64(b))
	}
	for svc, p := range svcPackets {
		serviceTrafficPackets.WithLabelValues(svc).Add(float64(p))
	}

	if enableSharedMetrics {
		AddSharedMetric("processed_bytes", float64(totBytes))
		AddSharedMetric("processed_packets", float64(totPackets))
		AddSharedMetric("processed_tcp_bytes", float64(protoBytes["tcp"]))
		AddSharedMetric("processed_udp_bytes", float64(protoBytes["udp"]))
		AddSharedMetric("processed_sctp_bytes", float64(protoBytes["sctp"]))
	}
}

func RunActiveConntrackCount(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			info := make([]cmn.CtInfo, len(ConntrackInfo))
			copy(info, ConntrackInfo)
			mutex.Unlock()

			// Initialize counters
			var (
				tcpCount    int
				udpCount    int
				sctpCount   int
				closedCount int
				activeCount int
				newFlows    int
				errorCount  int
				newRequests = make(map[string]int)
				newErrors   = make(map[string]int)
			)

			// Constants for protocol and state
			const (
				ProtoTCP    = "tcp"
				ProtoUDP    = "udp"
				ProtoSCTP   = "sctp"
				StateClosed = "closed"
			)

			currentConntrackInfo := make(map[ConntrackKey]bool)

			for _, ct := range info {
				if ct.CState == StateClosed {
					closedCount++
				} else {
					// Generate key and check for new flows
					key := MakeConntrackKey(ct)
					if !prevConntrackInfo[key] {
						newFlows++
						newRequests[ct.ServiceName]++
					}
					activeCount++
					switch ct.Proto {
					case ProtoTCP:
						tcpCount++
					case ProtoUDP:
						udpCount++
					case ProtoSCTP:
						sctpCount++
					}
					currentConntrackInfo[key] = true

					// Check for error state
					if isErrorState(ct) {
						errorCount++
						newErrors[ct.ServiceName]++
					}
				}
			}

			// Calculate deleted flows which are not present in the current conntrack info
			// but were present in the previous conntrack info
			// This is done to calculate the number of flows that have been closed
			// and are no longer present in the conntrack table
			// A key present last sweep and absent now has left the table for
			// good: prevConntrackInfo is replaced by currentConntrackInfo at the
			// end of this sweep, so each flow reaches this branch exactly once
			// and the counter cannot double-count it. Entries still sitting in
			// the table with CState "closed" are deliberately NOT counted here —
			// they persist across sweeps, and counting them every sweep is what
			// would inflate the counter. They are reflected in the
			// inactive_flow_count gauge instead.
			for key := range prevConntrackInfo {
				if !currentConntrackInfo[key] {
					closedCount++
					closedConnectionsProcessed.Inc()
				}
			}

			// Update Prometheus metrics
			activeConntrackCount.Set(float64(activeCount))
			activeFlowCountTcp.Set(float64(tcpCount))
			activeFlowCountUdp.Set(float64(udpCount))
			activeFlowCountSctp.Set(float64(sctpCount))
			inActiveFlowCount.Set(float64(closedCount))
			newFlowCount.Set(float64(newFlows))

			// Increment the total requests and errors counters
			totalRequests.Add(float64(newFlows))
			totalErrors.Add(float64(errorCount))

			// Update shared metrics
			if enableSharedMetrics {
				SetSharedMetric("active_conntrack_count", float64(activeCount))
				SetSharedMetric("active_flow_count_tcp", float64(tcpCount))
				SetSharedMetric("active_flow_count_udp", float64(udpCount))
				SetSharedMetric("active_flow_count_sctp", float64(sctpCount))
				SetSharedMetric("inactive_flow_count", float64(closedCount))
				SetSharedMetric("new_flow_count", float64(newFlows))

				AddSharedMetric("total_requests", float64(newFlows))
				AddSharedMetric("total_errors", float64(errorCount))
			}

			// Increment the total requests and errors counters per service
			for service, count := range newRequests {
				totalRequestsPerService.WithLabelValues(service).Add(float64(count))
				if enableSharedMetrics {
					AddLabeledMetric("total_requests_per_service", map[string]string{"service": service}, float64(count))
				}
			}
			for service, count := range newErrors {
				totalErrorsPerService.WithLabelValues(service).Add(float64(count))
				if enableSharedMetrics {
					AddLabeledMetric("total_errors_per_service", map[string]string{"service": service}, float64(count))
				}
			}

			// If there is no newErros, set init value
			if len(newErrors) == 0 {
				totalErrorsPerService.WithLabelValues("default").Add(float64(0))
				if enableSharedMetrics {
					AddLabeledMetric("total_errors_per_service", map[string]string{"service": "default"}, float64(0))
				}
			}

			// Update the previous conntrack info
			mutex.Lock()
			prevConntrackInfo = currentConntrackInfo
			mutex.Unlock()
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunHostCount(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		mutex.Lock()
		localEndPointInfo := EndPointInfo
		mutex.Unlock()

		healthyCount := 0
		unHealthyCount := 0

		for _, ep := range localEndPointInfo {
			if ep.CurrState == "ok" {
				healthyCount++
			} else if ep.CurrState == "nok" {
				unHealthyCount++
			}
		}

		healthyHostCount.Set(float64(healthyCount))
		unHealthyHostCount.Set(float64(unHealthyCount))

		if enableSharedMetrics {
			SetSharedMetric("healthy_host_count", float64(healthyCount))
			SetSharedMetric("unhealthy_host_count", float64(unHealthyCount))
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func parseConntrackKey(key ConntrackKey) (sip, sport, dip, dport, proto, serviceName string) {
	parts := strings.Split(string(key), "|")
	if len(parts) == 6 {
		return parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
	}
	return "", "", "", "", "", ""
}

func RunProcessedStatistic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			localPrevConntrackStats := make(map[ConntrackKey]Stats, len(ConntrackStats))
			serviceTraffic := make(map[string]float64)
			serviceDipTraffic := make(map[string]map[string]float64)

			for k, ct := range ConntrackStats {
				localPrevConntrackStats[k] = ct
			}
			mutex.Unlock()

			for k, ct := range localPrevConntrackStats {
				prevStats, exists := prevConntrackStats[k]
				if !exists {
					prevStats = Stats{Bytes: 0, Packets: 0}
				}

				var diffBytes uint64
				var diffPackets uint64

				// A cumulative per-flow counter that went backwards means the
				// datapath entry was recreated. Taking the current value as the
				// delta is correct, but the event is worth counting: a burst of
				// resets is what explains a traffic counter that looks like it
				// jumped.
				if prevStats.Bytes > ct.Bytes || prevStats.Packets > ct.Packets {
					counterResetEvents.Inc()
				}

				if prevStats.Bytes > ct.Bytes {
					diffBytes = ct.Bytes
				} else {
					diffBytes = ct.Bytes - prevStats.Bytes
				}

				if prevStats.Packets > ct.Packets {
					diffPackets = ct.Packets
				} else {
					diffPackets = ct.Packets - prevStats.Packets
				}

				if diffBytes > 0 || diffPackets > 0 {
					// NOTE: the aggregate processed_* and per-service
					// service_traffic_* counters are deliberately NOT fed here.
					// They come from the exact data-plane rule counters in
					// collectLbRuleTraffic — adding the sweep deltas on top
					// would double-count. This path owns only what needs flow
					// identity, which exists nowhere else.
					sip, _, dip, _, _, serviceName := parseConntrackKey(k)
					lbRuleInteractionBytes.WithLabelValues(serviceName, sip, dip).Add(float64(diffBytes))
					lbRuleInteractionPackets.WithLabelValues(serviceName, sip, dip).Add(float64(diffPackets))

					// Update total traffic per service and traffic per dip
					// serviceTraffic calculates the total traffic per service
					// serviceDipTraffic calculates the total traffic per dip per service
					// This is used to calculate the distribution ratio of traffic across backend endpoints per service
					// and the total traffic distribution across backend endpoints per service
					if _, exists := serviceTraffic[serviceName]; !exists {
						serviceTraffic[serviceName] = 0
						serviceDipTraffic[serviceName] = make(map[string]float64)
					}
					serviceTraffic[serviceName] += float64(ct.Bytes)
					serviceDipTraffic[serviceName][dip] += float64(ct.Bytes)

					// Per-endpoint and per-client breakdowns take the DELTA, not
					// the cumulative value: they feed monotonic counters.
					endpointTrafficBytes.WithLabelValues(serviceName, dip).Add(float64(diffBytes))
					clientTrafficPackets.WithLabelValues(serviceName, sip).Add(float64(diffPackets))

					// Update shared metrics if enabled
					if enableSharedMetrics {
						AddLabeledMetric("lb_rule_interaction_bytes", map[string]string{"service": serviceName, "sip": sip, "dip": dip}, float64(diffBytes))
						AddLabeledMetric("lb_rule_interaction_packets", map[string]string{"service": serviceName, "sip": sip, "dip": dip}, float64(diffPackets))
					}
				}
			}

			// Calculate distribution ratio (endpoint load dist per service) and update the metrics
			// Calculate distribution ratio (load dist per service) and update the metrics
			totalTraffic := 0.0
			for _, traffic := range serviceTraffic {
				totalTraffic += traffic
			}

			for service, traffic := range serviceTraffic {
				for dip, dipTraffic := range serviceDipTraffic[service] {
					ratio := dipTraffic / traffic
					endpointLoadDistsPerService.WithLabelValues(service, dip).Set(ratio)
					if enableSharedMetrics {
						AddLabeledMetric("endpoint_load_dists_per_service", map[string]string{"service": service, "dip": dip}, ratio)
					}
					// Log for debug
					tk.LogIt(tk.LogDebug, "Service: %s, DIP: %s, Ratio: %f\n", service, dip, ratio)
				}

				serviceRatio := traffic / totalTraffic

				totalLoadDistsPerService.WithLabelValues(service).Set(serviceRatio)
				if enableSharedMetrics {
					AddLabeledMetric("service_distribution_ratio", map[string]string{"service": service}, serviceRatio)
				}
				// Log for debug
				tk.LogIt(tk.LogDebug, "Service: %s, Total Traffic: %f, Service Ratio: %f\n", service, traffic, serviceRatio)
			}

			mutex.Lock()
			prevConntrackStats = localPrevConntrackStats
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunLcusCalculator(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var LCUNewFlowCount = &dto.Metric{}
			var LCUActiveFlowCount = &dto.Metric{}
			var LCURuleCount = &dto.Metric{}
			var LCUProcessedBytes = &dto.Metric{}

			mutex.Lock()

			if err := newFlowCount.Write(LCUNewFlowCount); err != nil {
				tk.LogIt(tk.LogError, "[Prometheus] Error writing newFlowCount: %v\n", err)
			}
			if err := activeConntrackCount.Write(LCUActiveFlowCount); err != nil {
				tk.LogIt(tk.LogError, "[Prometheus] Error writing activeConntrackCount: %v\n", err)
			}
			if err := ruleCount.Write(LCURuleCount); err != nil {
				tk.LogIt(tk.LogError, "[Prometheus] Error writing ruleCount: %v\n", err)
			}
			if err := processedBytes.Write(LCUProcessedBytes); err != nil {
				tk.LogIt(tk.LogError, "[Prometheus] Error writing processedBytes: %v\n", err)
			}
			localConntrackStatsLen := len(ConntrackStats)
			mutex.Unlock()

			// LCU of accumulated Flow count = Flowcount / 2160000
			// LCU of Rule = ruleCount/1000
			// LCU of Byte = processedBytes(Gb)/1h
			//
			// processedBytes is a COUNTER, so its dto.Metric carries .Counter,
			// never .Gauge. Reading it as a gauge made the whole condition
			// false on every cycle and left consumed_lcus pinned at 0.
			if LCURuleCount.Gauge != nil && LCURuleCount.Gauge.Value != nil && LCUProcessedBytes.Counter != nil && LCUProcessedBytes.Counter.Value != nil {
				consumedLcus.Set(float64(localConntrackStatsLen)/2160000 +
					*LCURuleCount.Gauge.Value/1000 +
					(*LCUProcessedBytes.Counter.Value*8)/360000000000) // (byte * 8)/ (60*60*1G)/10
			}
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetFwRule(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		info, err := hooks.NetFwRuleGet()
		if err != nil {
			tk.LogIt(tk.LogDebug, "[Prometheus] Error occurred while getting firewall rule info: %v\n", err)
			continue
		}

		mutex.Lock()
		FWRuleInfo = info
		mutex.Unlock()

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunFwStatistic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			localFWRuleInfo := make([]cmn.FwRuleMod, len(FWRuleInfo))
			copy(localFWRuleInfo, FWRuleInfo)
			mutex.Unlock()

			var totalDrops uint64
			currentFwRules := make(map[string]bool, len(localFWRuleInfo))

			for _, rule := range localFWRuleInfo {
				// The data-plane counter is "packets:bytes" (see
				// pkg/loxinet/rules.go), so the old strconv.Atoi on the whole
				// string failed on every rule and left total_fw_drops pinned at
				// 0 for the lifetime of the process.
				counter, ok := parseFwCounterPackets(rule.Opts.Counter)
				if !ok {
					tk.LogIt(tk.LogDebug, "[Prometheus] Unparsable firewall counter %q for pref %d\n",
						rule.Opts.Counter, rule.Rule.Pref)
					continue
				}

				// Only SrcIP/DstIP are strings; the six port/proto/pref fields
				// are integers. The old all-%s format made every one of them
				// render as a Go error marker, so this label has always read
				// `10.0.0.0/8_20.0.0.0/8_%!s(uint16=0)_...`. Correcting the
				// verbs changes the label text, but only from a formatting bug
				// to the value it was always meant to carry.
				ruleSpecLabel := fmt.Sprintf("%s_%s_%d_%d_%d_%d_%d_%d",
					rule.Rule.SrcIP, rule.Rule.DstIP, rule.Rule.SrcPortMin, rule.Rule.SrcPortMax,
					rule.Rule.DstPortMin, rule.Rule.DstPortMax, rule.Rule.Proto, rule.Rule.Pref)

				// Legacy surface: gauges holding the cumulative DP counter.
				totalDropsByFwPerRule.WithLabelValues(ruleSpecLabel).Set(float64(counter))
				totalDrops += counter

				// Canonical surface: a real counter fed per-cycle deltas, keyed
				// by rule preference so the label is bounded and stable across
				// an edit to the rule's match fields.
				ruleID := strconv.Itoa(int(rule.Rule.Pref))
				currentFwRules[ruleID] = true
				delta := counter
				if prev, seen := prevFwRuleDrops[ruleID]; seen && counter >= prev {
					delta = counter - prev
				}
				// On first sight, or a counter reset because the rule was
				// re-created, the full current value is the delta.
				if delta > 0 {
					fwRuleDropPacketsTotal.WithLabelValues(ruleID).Add(float64(delta))
					fwDropPacketsTotal.Add(float64(delta))
				}
				prevFwRuleDrops[ruleID] = counter

				if enableSharedMetrics {
					AddLabeledMetric("total_fw_drops_per_rule", map[string]string{"fw_rule": ruleSpecLabel}, float64(counter))
				}
			}

			// Drop series and baselines for rules that no longer exist, so a
			// deleted rule stops being exported instead of freezing at its last
			// value.
			for ruleID := range prevFwRuleDrops {
				if !currentFwRules[ruleID] {
					delete(prevFwRuleDrops, ruleID)
					fwRuleDropPacketsTotal.DeleteLabelValues(ruleID)
				}
			}

			// If there is no localFWRuleInfo, set init value
			if len(localFWRuleInfo) == 0 {
				totalDropsByFwPerRule.WithLabelValues("no_rule").Set(float64(0))
			}

			totalDropsByFw.Set(float64(totalDrops))
			fwRuleCount.Set(float64(len(localFWRuleInfo)))

			if enableSharedMetrics {
				SetSharedMetric("total_fw_drops", float64(totalDrops))
				SetSharedMetric("firewall_rules_count", float64(len(localFWRuleInfo)))
			}
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}
