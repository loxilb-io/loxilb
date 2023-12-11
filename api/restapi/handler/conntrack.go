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
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetConntrack(params operations.GetConfigConntrackAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Conntrack %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	// Get Conntrack informations
	res, err := ApiHooks.NetCtInfoGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.ConntrackEntry
	result = make([]*models.ConntrackEntry, 0)
	for _, conntrack := range res {
		var tmpResult models.ConntrackEntry
		tmpResult.Bytes = int64(conntrack.Bytes)
		tmpResult.ConntrackAct = conntrack.CAct
		tmpResult.ConntrackState = conntrack.CState
		tmpResult.DestinationIP = conntrack.Dip.String()
		tmpResult.DestinationPort = int64(conntrack.Dport)
		tmpResult.Packets = int64(conntrack.Pkts)
		tmpResult.Protocol = conntrack.Proto
		tmpResult.SourceIP = conntrack.Sip.String()
		tmpResult.SourcePort = int64(conntrack.Sport)
		tmpResult.ServName = conntrack.ServiceName
		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigConntrackAllOK().WithPayload(&operations.GetConfigConntrackAllOKBody{CtAttr: result})
}
