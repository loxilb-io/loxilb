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
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/prometheus"
	"net/http"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	"github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func ConfigGetPrometheusCounter(params operations.GetMetricsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "api: Prometheus %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	if !options.Opts.Prometheus {
		return operations.NewGetMetricsOK().WithPayload("Prometheus option is disabled.")
	}
	return CustomResponder(func(w http.ResponseWriter, _ runtime.Producer) {
		promhttp.Handler().ServeHTTP(w, params.HTTPRequest)
	})
}

func ConfigGetPrometheusOption(params operations.GetConfigMetricsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Prometheus %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	return operations.NewGetConfigMetricsOK().WithPayload(&models.MetricsConfig{Prometheus: &options.Opts.Prometheus})
}

func ConfigPostPrometheus(params operations.PostConfigMetricsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Prometheus %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	err := prometheus.TurnOn()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeletePrometheus(params operations.DeleteConfigMetricsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Prometheus %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	err := prometheus.Off()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}

	return &ResultResponse{Result: "Success"}
}
