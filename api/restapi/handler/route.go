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
	"strconv"
	"strings"
)

func ConfigPostRoute(params operations.PostConfigRouteParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Route  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddRouteNoHook(params.Attr.DestinationIPNet, params.Attr.Gateway, params.Attr.Protocol)
	if ret != 0 {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", ret)
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteRoute(params operations.DeleteConfigRouteDestinationIPNetIPAddressMaskParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Route  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	DstIP := fmt.Sprintf("%s/%d", params.IPAddress, params.Mask)
	ret := loxinlp.DelRouteNoHook(DstIP)
	if ret != 0 {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", ret)
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetRoute(params operations.GetConfigRouteAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Route  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, _ := ApiHooks.NetRouteGet()
	var result []*models.RouteGetEntry
	result = make([]*models.RouteGetEntry, 0)
	for _, route := range res {
		var tmpResult models.RouteGetEntry
		tmpResult.DestinationIPNet = route.Dst
		tmpResult.Flags = strings.TrimSpace(route.Flags)
		tmpResult.Gateway = route.Gw
		tmpResult.HardwareMark = int64(route.HardwareMark)
		protoStr := strconv.Itoa(route.Protocol)
		switch route.Protocol {
		case 0:
			protoStr = "unspec"
		case 1:
			protoStr = "redirect"
		case 2:
			protoStr = "kernel"
		case 3:
			protoStr = "boot"
		case 4:
			protoStr = "static"
		}
		tmpResult.Protocol = protoStr
		tmpResult.Sync = int64(route.Sync)

		tmpStats := new(models.RouteGetEntryStatistic)

		tmpBytes := int64(route.Statistic.Bytes)
		tmpStats.Bytes = &tmpBytes
		tmpPackets := int64(route.Statistic.Packets)
		tmpStats.Packets = &tmpPackets
		tmpResult.Statistic = tmpStats

		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigRouteAllOK().WithPayload(&operations.GetConfigRouteAllOKBody{RouteAttr: result})
}
