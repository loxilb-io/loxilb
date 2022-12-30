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

func ConfigPostParams(params operations.PostConfigParamsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Params %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var param cmn.ParamMod

	// Set Param State
	param.LogLevel = params.Attr.LogLevel
	tk.LogIt(tk.LogDebug, "[API] New LogLevel %s\n", param.LogLevel)
	_, err := ApiHooks.NetParamSet(param)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetParams(params operations.GetConfigParamsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var param cmn.ParamMod

	var payload models.OperParams
	// Get Param state
	_, err := ApiHooks.NetParamGet(&param)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	payload.LogLevel = param.LogLevel
	tk.LogIt(tk.LogDebug, "[API] LogLevel %s\n", param.LogLevel)
	return operations.NewGetConfigParamsOK().WithPayload(&payload)
}
