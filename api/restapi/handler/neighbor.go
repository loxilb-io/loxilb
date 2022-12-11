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

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/loxinlp"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostNeighbor(params operations.PostConfigNeighborParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddNeighNoHook(params.Attr.IPAddress, params.Attr.Dev, params.Attr.MacAddress)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteNeighbor(params operations.DeleteConfigNeighborIPAddressDevIfNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 Neighbor   %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelNeighNoHook(params.IPAddress, params.IfName)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetNeighbor(params operations.GetConfigNeighborAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 Neighbor  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, _ := ApiHooks.NetNeighGet()
	var result []*models.NeighborEntry
	result = make([]*models.NeighborEntry, 0)
	for _, neighbor := range res {
		var tmpResult models.NeighborEntry
		tmpResult.MacAddress = neighbor.HardwareAddr.String()
		tmpResult.IPAddress = neighbor.IP.String()
		tmpResult.Dev, _ = loxinlp.GetLinkNameByIndex(neighbor.LinkIndex)
		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigNeighborAllOK().WithPayload(&operations.GetConfigNeighborAllOKBody{NeighborAttr: result})
}
