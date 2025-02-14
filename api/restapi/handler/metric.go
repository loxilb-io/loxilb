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
package handler

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/prometheus" // Updated import path
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigGetFlowCount(params operations.GetMetricsFlowcountParams, principal interface{}) middleware.Responder {
	var result models.FlowCountMetrics

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetFlowCountSM()
	result.ActiveConntrackCount = metrics["active_conntrack_count"]
	result.ActiveFlowCountTCP = metrics["active_flow_count_tcp"]
	result.ActiveFlowCountUDP = metrics["active_flow_count_udp"]
	result.ActiveFlowCountSctp = metrics["active_flow_count_sctp"]
	result.InactiveFlowCount = metrics["inactive_flow_count"]

	return operations.NewGetMetricsFlowcountOK().WithPayload(&result)
}

func ConfigGetLbRuleCount(params operations.GetMetricsLbrulecountParams, principal interface{}) middleware.Responder {
	var result models.LbRuleCountMetrics

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	// Metric lb_rule_count not found
	metrics := prometheus.GetLBRuleCountSM()
	result.LbRuleCount = metrics["lb_rule_count"]

	return operations.NewGetMetricsLbrulecountOK().WithPayload(&result)
}

func ConfigGetNewFlowCount(params operations.GetMetricsNewflowcountParams, principal interface{}) middleware.Responder {
	var result models.NewFlowCountMetrics

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetNetFlowCountSM()
	result.NewFlowCount = metrics["lb_rule_count"]

	return operations.NewGetMetricsNewflowcountOK().WithPayload(&result)
}

func ConfigGetRequestCount(params operations.GetMetricsRequestcountParams, principal interface{}) middleware.Responder {
	var result models.RequestCountMetrics
	tk.LogIt(tk.LogDebug, "ConfigGetRequestCount: [API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	metrics := prometheus.GetReqCountSM()
	result.TotalRequests = metrics.TotalRequests

	// Extract total_requests_per_service
	for _, serviceMetric := range metrics.TotalRequestsPerService {
		result.TotalRequestsPerService = append(result.TotalRequestsPerService, &models.RequestCountMetricsTotalRequestsPerServiceItems0{
			Name:  serviceMetric.Name,
			Value: serviceMetric.Value,
		})
	}

	return operations.NewGetMetricsRequestcountOK().WithPayload(&result)
}

func ConfigGetErrorCount(params operations.GetMetricsErrorcountParams, principal interface{}) middleware.Responder {
	var result models.ErrorCountMetrics
	result.TotalErrorsPerService = make([]*models.ErrorCountMetricsTotalErrorsPerServiceItems0, 0)

	tk.LogIt(tk.LogDebug, "ConfigGetErrorCount: [API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetErrCountSM()
	result.TotalErrors = metrics.TotalErrors

	tk.LogIt(tk.LogDebug, "ConfigGetErrorCount: TotalErrors: %d\n", result.TotalErrors)

	// Extract total_errors_per_service
	for _, serviceMetric := range metrics.TotalErrorsPerService {
		tk.LogIt(tk.LogDebug, "ConfigGetErrorCount: Service: %s, Value: %d\n", serviceMetric.Name, serviceMetric.Value)

		result.TotalErrorsPerService = append(result.TotalErrorsPerService, &models.ErrorCountMetricsTotalErrorsPerServiceItems0{
			Name:  serviceMetric.Name,
			Value: serviceMetric.Value,
		})
	}

	return operations.NewGetMetricsErrorcountOK().WithPayload(&result)
}

func ConfigGetProcessedTraffic(params operations.GetMetricsProcessedtrafficParams, principal interface{}) middleware.Responder {
	var result models.ProcessedTrafficMetrics

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetProcessedTrafficVecSM()
	result.ProcessedBytes = metrics["processed_bytes"]
	result.ProcessedPackets = metrics["processed_packets"]
	result.ProcessedSctpBytes = metrics["processed_sctp_bytes"]
	result.ProcessedTCPBytes = metrics["processed_tcp_bytes"]
	result.ProcessedUDPBytes = metrics["processed_udp_bytes"]

	return operations.NewGetMetricsProcessedtrafficOK().WithPayload(&result)
}

func ConfigGetLbProcessedTraffic(params operations.GetMetricsLbprocessedtrafficParams, principal interface{}) middleware.Responder {
	var result models.LbProcessedTrafficMetrics

	tk.LogIt(tk.LogDebug, "[API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetLBProcessedTrafficVecSM()

	// Extract lb_rule_interaction_bytes
	for _, interactionMetric := range metrics.LbRuleInteractionBytes {
		result.LbRuleInteractionBytes = append(result.LbRuleInteractionBytes, &models.LbProcessedTrafficMetricsLbRuleInteractionBytesItems0{
			Dip:     interactionMetric.Dip,
			Service: interactionMetric.Service,
			Sip:     interactionMetric.Sip,
			Value:   interactionMetric.Value,
		})
	}

	// Extract lb_rule_interaction_packets
	for _, interactionMetric := range metrics.LbRuleInteractionPackets {
		result.LbRuleInteractionPackets = append(result.LbRuleInteractionPackets, &models.LbProcessedTrafficMetricsLbRuleInteractionPacketsItems0{
			Dip:     interactionMetric.Dip,
			Service: interactionMetric.Service,
			Sip:     interactionMetric.Sip,
			Value:   interactionMetric.Value,
		})
	}

	return operations.NewGetMetricsLbprocessedtrafficOK().WithPayload(&result)
}

func ConfigGetEpDistTraffic(params operations.GetMetricsEpdisttrafficParams, principal interface{}) middleware.Responder {
	var result models.EpDistTrafficMetrics = make(models.EpDistTrafficMetrics)

	tk.LogIt(tk.LogDebug, "[API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetEpDistTrafficVecSM()

	// Iterate over the metrics map
	for serviceName, serviceMetrics := range metrics {
		for _, metric := range serviceMetrics {
			dip := metric.Dip
			ratio := metric.Ratio
			value := metric.Value

			result[serviceName] = append(result[serviceName], models.EpDistTrafficMetricsItems0{
				Dip:   dip,
				Ratio: ratio,
				Value: value,
			})
		}
	}

	return operations.NewGetMetricsEpdisttrafficOK().WithPayload(result)
}

func ConfigGetServiceDistTraffic(params operations.GetMetricsServicedisttrafficParams, principal interface{}) middleware.Responder {
	var result models.ServiceDistTrafficMetrics = make(models.ServiceDistTrafficMetrics)

	tk.LogIt(tk.LogDebug, "[API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	metrics := prometheus.GetServiceDistTrafficVecSM()

	// Iterate over the metrics map
	for serviceName, serviceMetric := range metrics {
		value := serviceMetric.Value
		ratio := serviceMetric.Ratio

		result[serviceName] = models.ServiceDistTrafficMetricsAnon{
			Value: value,
			Ratio: ratio,
		}
	}

	return operations.NewGetMetricsServicedisttrafficOK().WithPayload(result)
}

func ConfigGetFwDrops(params operations.GetMetricsFwdropsParams, principal interface{}) middleware.Responder {
	var result models.FwDropsMetrics
	result.TotalFwDropsPerRule = make([]*models.FwDropsMetricsTotalFwDropsPerRuleItems0, 0)

	tk.LogIt(tk.LogDebug, "[API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetFwDropsSM()

	// Exception handling if the metrics are not found or no data is available
	if metrics.TotalFwDrops == 0 {
		return operations.NewGetMetricsFwdropsInternalServerError().WithPayload(&models.Error{
			Message: "Failed to fetch metrics for total_fw_drops",
		})
	}

	result.TotalFwDrops = metrics.TotalFwDrops

	// For each service, add the total requests
	for _, serviceMetric := range metrics.TotalFwDropsPerRule {
		result.TotalFwDropsPerRule = append(result.TotalFwDropsPerRule, &models.FwDropsMetricsTotalFwDropsPerRuleItems0{
			FwRule: serviceMetric.FwRule,
			Value:  serviceMetric.Value,
		})
	}

	return operations.NewGetMetricsFwdropsOK().WithPayload(&result)
}

func ConfigGetReqCounterPerClient(params operations.GetMetricsReqcountperclientParams, principal interface{}) middleware.Responder {
	var result models.ReqCountPerClientMetrics = make(models.ReqCountPerClientMetrics)

	tk.LogIt(tk.LogDebug, "[API] version %s API called. URL: %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetReqCountPerClientSM()

	for clientIP, count := range metrics {
		if clientIP == "" {
			tk.LogIt(tk.LogError, "Empty clientIP key found in metrics: %v\n", metrics)
			continue
		}

		result[clientIP] = count
	}

	return operations.NewGetMetricsReqcountperclientOK().WithPayload(result)
}

func ConfigGetHostCount(params operations.GetMetricsHostcountParams, principal interface{}) middleware.Responder {
	var result models.HostCountMetrics

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetHostCountSM()

	result.HealthyHostCount = metrics["healthy_host_count"]
	result.UnhealthyHostCount = metrics["unhealthy_host_count"]

	return operations.NewGetMetricsHostcountOK().WithPayload(&result)
}
