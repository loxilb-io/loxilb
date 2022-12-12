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
	"github.com/loxilb-io/loxilb/api/loxinlp"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostFDB(params operations.PostConfigFdbParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddFDBNoHook(params.Attr.MacAddress, params.Attr.Dev)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteFDB(params operations.DeleteConfigFdbMacAddressDevIfNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelFDBNoHook(params.MacAddress, params.IfName)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetFDB(params operations.GetConfigFdbAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	fdbs, _ := loxinlp.GetFDBNoHook()
	var result []*models.FDBEntry
	result = make([]*models.FDBEntry, 0)
	for _, fdb := range fdbs {
		var tmpResult models.FDBEntry
		tmpResult.MacAddress = fdb["macAddress"]
		tmpResult.Dev = fdb["dev"]
		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigFdbAllOK().WithPayload(&operations.GetConfigFdbAllOKBody{FdbAttr: result})
}
