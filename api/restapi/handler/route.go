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
	"loxilb/api/restapi/operations"
	cmn "loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostRoute(params operations.PostConfigRouteParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Route  %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var routeMod cmn.Routev4Mod
	_, Dst, err := net.ParseCIDR(params.Attr.DestinationIPNet)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	routeMod.Dst.IP = Dst.IP
	routeMod.Dst.Mask = Dst.Mask
	routeMod.Gw = net.ParseIP(params.Attr.Gateway)

	tk.LogIt(tk.LOG_DEBUG, "[API] routeMod : %v\n", routeMod)
	_, err = ApiHooks.NetRoutev4Add(&routeMod)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteRoute(params operations.DeleteConfigRouteDestinationIPNetIPAddressMaskParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Route  %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var routeMod cmn.Routev4Mod

	DstIP := fmt.Sprintf("%s/%d", params.IPAddress, params.Mask)
	_, Dst, err := net.ParseCIDR(DstIP)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}

	routeMod.Dst.IP = Dst.IP
	routeMod.Dst.Mask = Dst.Mask
	tk.LogIt(tk.LOG_DEBUG, "[API] routeMod : %v\n", routeMod)

	_, err = ApiHooks.NetRoutev4Del(&routeMod)

	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
