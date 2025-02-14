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
	activeConntrackCount   = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_conntrack_count",
			Help: "The average number of active established connections from clients to targets.",
		},
	)
	activeFlowCountTcp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_tcp",
			Help: "The average number of concurrent TCP flows (or connections) from clients to targets.",
		},
	)
	activeFlowCountUdp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_udp",
			Help: "The average number of concurrent UDP flows (or connections) from clients to targets.",
		},
	)
	activeFlowCountSctp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_sctp",
			Help: "The average number of concurrent SCTP flows (or connections) from clients to targets.",
		},
	)
	inActiveFlowCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "inactive_flow_count",
			Help: "The average number of concurrent closed flows (or connections) from clients to targets.",
		},
	)
	healthyHostCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "healthy_host_count",
			Help: "Average number of healthy targets.",
		},
	)
	unHealthyHostCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "unhealthy_host_count",
			Help: "Average number of unhealthy targets",
		},
	)
	ruleCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lb_rule_count",
			Help: "Average number of unhealthy targets",
		},
	)
	consumedLcus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "consumed_lcus",
			Help: "The number of LCUs used by the load balancer.",
		},
	)
	newFlowCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "new_flow_count",
			Help: "The number of new TCP connections from clients to targets.",
		},
	)
	processedBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedTCPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_tcp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedUDPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_udp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedSCTPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_sctp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedPackets = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_packets",
			Help: "The total number of packets processed by the load balancer.",
		},
	)
	// ProcessedBtyes per LB Rule PromQL : sum(rate(lb_rule_interaction_bytes[1m])) by (service)
	// ProcessedBtyes per endpoint PromQL: sum(rate(lb_rule_interaction_bytes[1m])) by (dip)
	lbRuleInteractionBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lb_rule_interaction_bytes",
			Help: "Total bytes exchanged between load banacer and IPs",
		},
		[]string{"service", "sip", "dip"},
	)
	lbRuleInteractionPackets = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lb_rule_interaction_packets",
			Help: "Total packets exchanged between load balancer and IPs",
		},
		[]string{"service", "sip", "dip"},
	)

	// Prometheus metrics for total requests and RPS
	// Can calculate Requests Per Second (RPS) by tracking the number of new flows over a specific time interval.
	// Can use a Prometheus counter to track the total number of requests
	// and then use the rate function in Prometheus to calculate the RPS
	// PromQL : rate(total_requests[1m])
	totalRequests = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "total_requests",
			Help: "Total number of requests",
		},
	)

	totalRequestsPerService = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "total_requests_per_service",
			Help: "Total number of requests per service",
		},
		[]string{"service"},
	)

	totalErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "total_errors",
			Help: "Total number of errors",
		},
	)

	totalErrorsPerService = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "total_errors_per_service",
			Help: "Total number of errors per service",
		},
		[]string{"service"},
	)

	totalDropsByFw = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_fw_drops",
			Help: "Total number of drops by firewall rule",
		},
	)

	totalDropsByFwPerRule = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_fw_drops_per_rule",
			Help: "Total number of drops by firewall per rule",
		},
		[]string{"fw_rule"},
	)

	endpointLoadDistsPerService = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_load_dists_per_service",
			Help: "Ratio of traffic distribution across backend endpoints per service",
		},
		[]string{"service", "dip"},
	)

	totalLoadDistsPerService = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_load_dists_per_service",
			Help: "Ratio of total traffic distribution across backend endpoints per service",
		},
		[]string{"service"},
	)

	prevConntrackStats = make(map[ConntrackKey]Stats)
	prevConntrackInfo  = make(map[ConntrackKey]bool)

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

}

func toJSON(v interface{}) string {
	bytes, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(bytes)
}

func Off() error {
	if !options.Opts.Prometheus {
		return errors.New(http.StatusBadRequest, "already prometheus turned off")
	}
	options.Opts.Prometheus = false
	prometheusCancel()
	return nil
}

func TurnOn() error {
	if options.Opts.Prometheus {
		return errors.New(http.StatusBadRequest, "already prometheus turned on")
	}
	options.Opts.Prometheus = true
	Init()
	return nil
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
	for {
		// Statistic reset
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			ConntrackStats = map[ConntrackKey]Stats{}
			mutex.Unlock()
		}
		time.Sleep(PromethusLongPeriod)
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

		time.Sleep(PromethusDefaultPeriod)
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
			for key := range prevConntrackInfo {
				if !currentConntrackInfo[key] {
					closedCount++
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
					// Update processed bytes and packets
					processedBytes.Add(float64(diffBytes))
					processedPackets.Add(float64(diffPackets))

					// Update protocol-specific metrics
					if strings.Contains(string(k), "tcp") {
						processedTCPBytes.Add(float64(diffBytes))
					} else if strings.Contains(string(k), "udp") {
						processedUDPBytes.Add(float64(diffBytes))
					} else if strings.Contains(string(k), "sctp") {
						processedSCTPBytes.Add(float64(diffBytes))
					}

					// Update per-rule and per-endpoint metrics
					sip, _, dip, _, _, serviceName := parseConntrackKey(k)
					lbRuleInteractionBytes.WithLabelValues(serviceName, sip, dip).Add(float64(diffBytes))
					lbRuleInteractionPackets.WithLabelValues(serviceName, sip, dip).Add(float64(diffPackets))

					// Update total traffic per service and traffic per dip
					// serviceTraffic calculates the total traffic per service
					// serviceDipTraffic calculates the total traffic per dip per service
					// This is used to calculate the distribution ratio of traffic across backend endpoints per service
					// and the total traffic distribution across backend endpoints per service
					// This is used to calculate the total traffic distribution across backend endpoints per service
					if _, exists := serviceTraffic[serviceName]; !exists {
						serviceTraffic[serviceName] = 0
						serviceDipTraffic[serviceName] = make(map[string]float64)
					}
					serviceTraffic[serviceName] += float64(ct.Bytes)
					serviceDipTraffic[serviceName][dip] += float64(ct.Bytes)

					// Update shared metrics if enabled
					if enableSharedMetrics {
						AddSharedMetric("processed_bytes", float64(diffBytes))
						AddSharedMetric("processed_packets", float64(diffPackets))

						if strings.Contains(string(k), "tcp") {
							AddSharedMetric("processed_tcp_bytes", float64(diffBytes))
						} else if strings.Contains(string(k), "udp") {
							AddSharedMetric("processed_udp_bytes", float64(diffBytes))
						} else if strings.Contains(string(k), "sctp") {
							AddSharedMetric("processed_sctp_bytes", float64(diffBytes))
						}

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
			if LCURuleCount.Gauge != nil && LCURuleCount.Gauge.Value != nil && LCUProcessedBytes.Gauge != nil && LCUProcessedBytes.Gauge.Value != nil {
				consumedLcus.Set(float64(localConntrackStatsLen)/2160000 +
					*LCURuleCount.Gauge.Value/1000 +
					(*LCUProcessedBytes.Gauge.Value*8)/360000000000) // (byte * 8)/ (60*60*1G)/10
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

			totalDrops := 0

			for _, rule := range localFWRuleInfo {
				// FIXME: DBG:  2025/01/22 07:31:25 [Prometheus] Error converting counter: strconv.Atoi: parsing "0:0": invalid syntax
				counter, err := strconv.Atoi(rule.Opts.Counter)
				if err != nil {
					tk.LogIt(tk.LogDebug, "[Prometheus] Error converting counter: %v\n", err)
					continue
				}

				ruleSpecLabel := fmt.Sprintf("%s_%s_%s_%s_%s_%s_%s_%s",
					rule.Rule.SrcIP, rule.Rule.DstIP, rule.Rule.SrcPortMin, rule.Rule.SrcPortMax,
					rule.Rule.DstPortMin, rule.Rule.DstPortMax, rule.Rule.Proto, rule.Rule.Pref)

				totalDropsByFwPerRule.WithLabelValues(ruleSpecLabel).Set(float64(counter))
				totalDrops += counter

				if enableSharedMetrics {
					AddLabeledMetric("total_fw_drops_per_rule", map[string]string{"fw_rule": ruleSpecLabel}, float64(counter))
				}
			}

			// If there is no localFWRuleInfo, set init value
			if len(localFWRuleInfo) == 0 {
				totalDropsByFwPerRule.WithLabelValues("no_rule").Set(float64(0))
			}

			totalDropsByFw.Set(float64(totalDrops))

			if enableSharedMetrics {
				SetSharedMetric("total_fw_drops", float64(totalDrops))
			}
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}
