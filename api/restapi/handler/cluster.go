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
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
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
	tk.LogIt(tk.LogDebug, "[API] Instance %s New HA State : %v\n", hasMod.Instance, hasMod.State)
	_, err := ApiHooks.NetCIStateMod(&hasMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
