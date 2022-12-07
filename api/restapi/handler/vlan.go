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

func ConfigPostVLAN(params operations.PostConfigVlanParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Vlan %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVLANNoHook(int(params.Attr.Vid))
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVLAN(params operations.DeleteConfigVlanVlanIDParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Vlan %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVLANNoHook(int(params.VlanID))
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostVLANMember(params operations.PostConfigVlanVlanIDMemberParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Vlan %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVLANMemberNoHook(int(params.VlanID), params.Attr.Dev, params.Attr.Tagged)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVLANMember(params operations.DeleteConfigVlanVlanIDMemberIfNameTaggedTaggedParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Vlan %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVLANMemberNoHook(int(params.VlanID), params.IfName, params.Tagged)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetVLAN(params operations.GetConfigVlanAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Vlan   %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, _ := ApiHooks.NetVlanGet()
	var result []*models.VlanGetEntry
	result = make([]*models.VlanGetEntry, 0)
	for _, vlan := range res {
		var tmpResult models.VlanGetEntry
		tmpResult.Dev = vlan.Dev
		tmpResult.Vid = int64(vlan.Vid)
		for _, vpm := range vlan.Member {
			member := models.VlanMemberEntry{
				Dev:    vpm.Dev,
				Tagged: vpm.Tagged,
			}
			tmpResult.Member = append(tmpResult.Member, &member)
		}
		tmpResult.VlanStatistic = new(models.VlanGetEntryVlanStatistic)
		tmpResult.VlanStatistic.InBytes = int64(vlan.Stat.InBytes)
		tmpResult.VlanStatistic.InPackets = int64(vlan.Stat.InPackets)
		tmpResult.VlanStatistic.OutBytes = int64(vlan.Stat.OutBytes)
		tmpResult.VlanStatistic.OutPackets = int64(vlan.Stat.OutPackets)

		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigVlanAllOK().WithPayload(&operations.GetConfigVlanAllOKBody{VlanAttr: result})
}
