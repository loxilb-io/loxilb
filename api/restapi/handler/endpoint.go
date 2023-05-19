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
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigGetEndPoint(params operations.GetConfigEndpointAllParams) middleware.Responder {
	// Get endpoint rules
	tk.LogIt(tk.LogDebug, "[API] EndPoint %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetEpHostGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.EndPointGetEntry
	result = make([]*models.EndPointGetEntry, 0)
	for _, ep := range res {
		var tmpEP models.EndPointGetEntry

		// Service Arg match
		tmpEP.HostName = ep.HostName
		tmpEP.Name = ep.Name
		tmpEP.InactiveReTries = int64(ep.InActTries)
		tmpEP.ProbeType = ep.ProbeType
		tmpEP.ProbeReq = ep.ProbeReq
		tmpEP.ProbeResp = ep.ProbeResp
		tmpEP.ProbeDuration = int64(ep.ProbeDuration)
		tmpEP.ProbePort = int64(ep.ProbePort)
		tmpEP.MinDelay = ep.MinDelay
		tmpEP.AvgDelay = ep.AvgDelay
		tmpEP.MaxDelay = ep.MaxDelay
		tmpEP.CurrState = ep.CurrState
		tmpEP.ProbePort = int64(ep.ProbePort)

		result = append(result, &tmpEP)
	}
	return operations.NewGetConfigEndpointAllOK().WithPayload(&operations.GetConfigEndpointAllOKBody{Attr: result})
}

func ConfigPostEndPoint(params operations.PostConfigEndpointParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] EndPoint %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	EP := cmn.EndPointMod{}
	EP.HostName = params.Attr.HostName
	EP.Name = params.Attr.Name
	EP.ProbeType = params.Attr.ProbeType
	EP.InActTries = int(params.Attr.InactiveReTries)
	EP.ProbeReq = params.Attr.ProbeReq
	EP.ProbeResp = params.Attr.ProbeResp
	EP.ProbeDuration = uint32(params.Attr.ProbeDuration)
	EP.ProbePort = uint16(params.Attr.ProbePort)

	_, err := ApiHooks.NetEpHostAdd(&EP)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteEndPoint(params operations.DeleteConfigEndpointEpipaddressIPAddressParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] EndPoint %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	EP := cmn.EndPointMod{}
	EP.HostName = params.IPAddress

	if params.Name != nil {
		EP.Name = *params.Name
	}

	if params.ProbeType != nil {
		EP.ProbeType = *params.ProbeType
	}

	if params.ProbePort != nil {
		EP.ProbePort = uint16(*params.ProbePort)
	}
	_, err := ApiHooks.NetEpHostDel(&EP)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
