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
	"net"
)

func ConfigGetCIState(params operations.GetConfigCistateAllParams) middleware.Responder {
	var result []*models.CIStatusGetEntry
	result = make([]*models.CIStatusGetEntry, 0)
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	hasMod, err := ApiHooks.NetCIStateGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	for _, h := range hasMod {
		var tempResult models.CIStatusGetEntry
		tempResult.Instance = h.Instance
		tempResult.State = h.State
		tempResult.Vip = h.Vip.String()
		result = append(result, &tempResult)
	}

	return operations.NewGetConfigCistateAllOK().WithPayload(&operations.GetConfigCistateAllOKBody{Attr: result})
}

func ConfigPostCIState(params operations.PostConfigCistateParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] HA %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var hasMod cmn.HASMod

	// Set HA State
	hasMod.Instance = params.Attr.Instance
	hasMod.State = params.Attr.State
	hasMod.Vip = net.ParseIP(params.Attr.Vip)
	tk.LogIt(tk.LogDebug, "[API] Instance %s New HA State : %v, VIP: %s\n",
		hasMod.Instance, hasMod.State, hasMod.Vip)
	_, err := ApiHooks.NetCIStateMod(&hasMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetBFDSession(params operations.GetConfigBfdAllParams) middleware.Responder {
	var result []*models.BfdGetEntry
	result = make([]*models.BfdGetEntry, 0)
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	bfdMod, err := ApiHooks.NetBFDGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	for _, h := range bfdMod {
		var tempResult models.BfdGetEntry
		tempResult.Instance = h.Instance
		tempResult.RemoteIP = h.RemoteIP.String()
		tempResult.SourceIP = h.SourceIP.String()
		tempResult.Interval = h.Interval
		tempResult.Port = h.Port
		tempResult.RetryCount = h.RetryCount
		tempResult.State = h.State

		result = append(result, &tempResult)
	}

	return operations.NewGetConfigBfdAllOK().WithPayload(&operations.GetConfigBfdAllOKBody{Attr: result})
}

func ConfigPostBFDSession(params operations.PostConfigBfdParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] HA %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var bfdMod cmn.BFDMod

	// Update BFD Session
	bfdMod.Instance = params.Attr.Instance
	bfdMod.RemoteIP = net.ParseIP(params.Attr.RemoteIP)
	bfdMod.SourceIP = net.ParseIP(params.Attr.SourceIP)
	bfdMod.Interval = params.Attr.Interval
	bfdMod.RetryCount = params.Attr.RetryCount

	tk.LogIt(tk.LogDebug, "[API] Instance %s BFD session add : %s, Interval: %d, RetryCount: %d\n",
		bfdMod.Instance, bfdMod.RemoteIP, bfdMod.Interval, bfdMod.RetryCount)
	_, err := ApiHooks.NetBFDAdd(&bfdMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBFDSession(params operations.DeleteConfigBfdRemoteIPRemoteIPParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] HA %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var bfdMod cmn.BFDMod

	// Delete BFD Session
	if params.Instance != nil {
		bfdMod.Instance = *params.Instance
	}

	bfdMod.RemoteIP = net.ParseIP(params.RemoteIP)

	tk.LogIt(tk.LogDebug, "[API] Instance %s BFD session delete : %s\n",
		bfdMod.Instance, bfdMod.RemoteIP)
	_, err := ApiHooks.NetBFDDel(&bfdMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
