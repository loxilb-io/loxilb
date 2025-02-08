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
	"github.com/loxilb-io/loxilb/api/prometheus"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

// Placeholder definition for prometheus.Meta
// Ensure this matches the actual definition in the prometheus package
type Meta struct {
	PreferredVisualisationType string `json:"preferredVisualisationType"`
}

func convertPrometheusNodesToModelsNodes(prometheusNodes []prometheus.Node) []*models.Node {
	var nodes []*models.Node
	for _, node := range prometheusNodes {
		nodes = append(nodes, &models.Node{
			ID:            node.ID,
			Title:         node.Title,
			Subtitle:      node.Subtitle,
			Mainstat:      node.Mainstat,
			Secondarystat: node.Secondarystat,
			Color:         node.Color,
			Icon:          node.Icon,
			NodeRadius:    int64(node.NodeRadius),
		})
	}
	return nodes
}

func convertPrometheusEdgesToModelsEdges(prometheusEdges []prometheus.Edge) []*models.Edge {
	var edges []*models.Edge
	for _, edge := range prometheusEdges {
		edges = append(edges, &models.Edge{
			ID:            edge.ID,
			Source:        edge.Source,
			Target:        edge.Target,
			Mainstat:      edge.Mainstat,
			Secondarystat: edge.Secondarystat,
			Thickness:     int64(edge.Thickness),
			Color:         edge.Color,
		})
	}
	return edges
}

func ConfigGetNodeGraph(params operations.GetNodegraphAllParams, principal interface{}) middleware.Responder {
	var result models.NodeGraphShcmea

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetNodeGraphSM()

	result.Nodes = convertPrometheusNodesToModelsNodes(metrics.Nodes)
	result.Edges = convertPrometheusEdgesToModelsEdges(metrics.Edges)
	result.SchemaVersion = int64(metrics.SchemaVersion)

	return operations.NewGetNodegraphAllOK().WithPayload(&result)
}

func ConfigGetNodeGraphService(params operations.GetNodegraphServiceParams, principal interface{}) middleware.Responder {
	service := params.Service
	var result models.NodeGraphShcmea

	tk.LogIt(tk.LogDebug, "[API] version  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	metrics := prometheus.GetNodeGraphServiceSM(service)

	result.Nodes = convertPrometheusNodesToModelsNodes(metrics.Nodes)
	result.Edges = convertPrometheusEdgesToModelsEdges(metrics.Edges)
	result.SchemaVersion = int64(metrics.SchemaVersion)

	return operations.NewGetNodegraphServiceOK().WithPayload(&result)
}
