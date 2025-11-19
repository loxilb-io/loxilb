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

func ConfigPostFDB(params operations.PostConfigFdbParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: FDB %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	err := loxinlp.AddFDBNoHook(*params.Attr.MacAddress, *params.Attr.Dev)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteFDB(params operations.DeleteConfigFdbMacAddressDevIfNameParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: FDB %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	err := loxinlp.DelFDBNoHook(params.MacAddress, params.IfName)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetFDB(params operations.GetConfigFdbAllParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: FDB  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	fdbs, _ := loxinlp.GetFDBNoHook()
	var result []*models.FDBEntry
	result = make([]*models.FDBEntry, 0)
	for _, fdb := range fdbs {
		var tmpResult models.FDBEntry
		mac := fdb["macAddress"]
		dev := fdb["dev"]
		tmpResult.MacAddress = &mac
		tmpResult.Dev = &dev
		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigFdbAllOK().WithPayload(&operations.GetConfigFdbAllOKBody{FdbAttr: result})
}
