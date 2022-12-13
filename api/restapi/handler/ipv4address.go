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
	"fmt"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/loxinlp"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostIPv4Address(params operations.PostConfigIpv4addressParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 address %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddAddrNoHook(params.Attr.IPAddress, params.Attr.Dev)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteIPv4Address(params operations.DeleteConfigIpv4addressIPAddressMaskDevIfNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 address   %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ipNet := fmt.Sprintf("%s/%s", params.IPAddress, params.Mask)
	ret := loxinlp.DelAddrNoHook(ipNet, params.IfName)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetIPv4Address(params operations.GetConfigIpv4addressAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] IPv4 address   %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, _ := ApiHooks.NetAddrGet()
	var result []*models.IPV4AddressGetEntry
	result = make([]*models.IPV4AddressGetEntry, 0)
	for _, ipaddrs := range res {
		var tmpResult models.IPV4AddressGetEntry
		tmpResult.Dev = ipaddrs.Dev
		helperSync := int64(ipaddrs.Sync)
		tmpResult.Sync = &helperSync
		tmpResult.IPAddress = ipaddrs.IP
		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigIpv4addressAllOK().WithPayload(&operations.GetConfigIpv4addressAllOKBody{IPAttr: result})
}
