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
	"net/http"
	"strings"

	"github.com/loxilb-io/loxilb/api/models"
	cmn "github.com/loxilb-io/loxilb/common"

	"github.com/go-openapi/runtime"
)

var ApiHooks cmn.NetHookInterface

type CustomResponder func(http.ResponseWriter, runtime.Producer)

type ResultResponse struct {
	Result string `json:"result"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Payload *models.Error
}

func (result *ResultResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}

func (c CustomResponder) WriteResponse(w http.ResponseWriter, p runtime.Producer) {
	c(w, p)
}

func (e *ErrorResponse) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {
	rw.WriteHeader(int(e.Payload.Code))
	producer.Produce(rw, e.Payload)
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func ResultErrorResponseErrorMessage(msg string) *models.Error {
	m := strings.ToLower(msg)

	// 404 Not Found
	if containsAny(m,
		"not-exists", " not exists", "not found", "no such", "not such",
		"no neigh found", "no-nh error", "no ulcl", "ephost-notfound",
		"host-notfound", "no-zone error", "no loxi-eni found", "no-master",
		"no bfd session", "my discriminator not found", "no-portimap", "no-portomap", "no-port error",
		"no such fdb", "no such route", "no such port", "no such mirror",
		"no such allowed src prefix", "no such policer", "not found interface",
		"no such ifa", "no such addrs", "vlan not created", "vlan not yet created",
		"phy port not created", "no-realport", "no realport", "no-user error", "no-rule error", "file not found",
	) {
		return &models.Error{Code: 404, Message: "Resource not found", Result: msg}
	}

	// 401 Auth or Token
	if containsAny(m,
		"invalid token", "token is expired", "token not fou",
		"invalid refresh token", "invalid token format", "authentication failed",
		"user not found",
	) {
		return &models.Error{Code: 401, Message: "Invalid authentication credentials", Result: msg}
	}

	// 409 Conflict
	if containsAny(m,
		"lbrule-exist error", "lbrule-exists error", "fwrule-exists", "sess-exists",
		"mirr-exists", "pol-exists", "prop-exists", "zone exists", "existing zone",
		"already created", " existing ", " exists",
		"vlan has ports configured", "port exists", "vlan tag port exists",
		"vlan untag port exists", "same fdb", "rt exists", "nh exists",
		"username already exists", "lb rule-referred", "cant modify",
		"ep-host add failed as cluster node", "vlan bridge already added",
	) {
		return &models.Error{
			Code:    409,
			Message: "Resource conflict: Resource already exists OR dependency not found",
			Result:  msg,
		}
	}

	// 400 Bad Request
	if containsAny(m,
		"malformed", "parse error", "invalid parameters", "invalid ",
		"mask format is wrong", "not ipv4 address", "proto error", "malformed-proto",
		"malformed service proto", "unknown work type", "unknown log level", "unknown ep-host-state",
		"host-args unknown probe", "unknown probe port", "vxlan can not be tagged",
		"range", "overflow", "fwmark", "rule-mark error", "rule-snat error",
		"rule-allowed-src error", "service-args error", "non-udp-n3-args error",
		"secondaryip-args", "serv-port-args range", "endpoints-range",
		"source address malformed", "address malformed", "remoteip address malformed",
		"ip address parse error", "myip address parse error",
		"malformed-service", "malformed-secip", "malformed-lbep", "malformed-rule",
		"invalid gws", "zone number err", "zone is not set", "vlan zone err",
		"invalid vlanid", "fdb attr error", "fdb v6 dst unsupported",
		"host-args error", "hostarm-args error",
		"password must ", "password must not ", "password must be at least",
		"Cors URL cannot be empty", "wildcard '*' is not allowed",
		"Failed to add Cors", "Failed to delete Cors", "filename is required", "file is empty",
		"no configuration file provided", "invalid json format",
	) {
		return &models.Error{Code: 400, Message: "Malformed arguments for API call", Result: msg}
	}

	// 403 Forbidden
	if containsAny(m,
		"capacity", " hwm", "ulhwm", "dlhwm", "nh-hwm", "rule-hwm",
		"need-realdev", "loxilb bgp mode is disabled", "running in bgp only mode",
	) {
		return &models.Error{Code: 403, Message: "Capacity insufficient", Result: msg}
	}

	// 503 Service Unavailable
	if containsAny(m,
		"not-ready", "timeout", "unexpected http response", "maintenance", "no-master",
		"netrpc call timeout",
	) {
		return &models.Error{Code: 503, Message: "Maintenance mode", Result: msg}
	}

	// 500 Internal Server Error (default)
	return &models.Error{Code: 500, Message: msg, Result: msg}
}
