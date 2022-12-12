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

func ConfigPostVxLAN(params operations.PostConfigTunnelVxlanParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] VxLAN %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVxLANBridgeNoHook(int(params.Attr.VxlanID), params.Attr.EpIntf)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVxLAN(params operations.DeleteConfigTunnelVxlanVxlanIDParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] VxLAN %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVxLANNoHook(int(params.VxlanID))
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostVxLANPeer(params operations.PostConfigTunnelVxlanVxlanIDPeerParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] VxLAN %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVxLANPeerNoHook(int(params.VxlanID), params.Attr.PeerIP)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVxLANPeer(params operations.DeleteConfigTunnelVxlanVxlanIDPeerPeerIPParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] VxLAN %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVxLANPeerNoHook(int(params.VxlanID), params.PeerIP)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetVxLAN(params operations.GetConfigTunnelVxlanAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] VxLAN   %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	peers, _ := loxinlp.GetVxLANPeerNoHook()
	ports, err := ApiHooks.NetPortGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.VxlanEntry
	result = make([]*models.VxlanEntry, 0)

	for _, port := range ports {
		if port.SInfo.PortType&0x40 == 0x40 { // 0x40 is const of the PortVxlanBr
			// Vxlan Port
			var tmpResult models.VxlanEntry
			tmpResult.PeerIP = peers[port.SInfo.OsID]
			tmpResult.VxlanName = port.Name
			tmpResult.VxlanID = int64(port.HInfo.TunID)
			tmpResult.EpIntf = port.HInfo.Real
			result = append(result, &tmpResult)

		}
	}

	return operations.NewGetConfigTunnelVxlanAllOK().WithPayload(&operations.GetConfigTunnelVxlanAllOKBody{VxlanAttr: result})
}
