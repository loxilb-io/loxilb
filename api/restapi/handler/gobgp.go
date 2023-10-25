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
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
)

func ConfigPostBGPNeigh(params operations.PostConfigBgpNeighParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpNeighMod cmn.GoBGPNeighMod

	// IP address
	bgpNeighMod.Addr = net.ParseIP(params.Attr.IPAddress)

	// Remote AS
	bgpNeighMod.RemoteAS = uint32(params.Attr.RemoteAs)

	// Remote Port
	bgpNeighMod.RemotePort = uint16(params.Attr.RemotePort)

	// Multi-hop or not
	bgpNeighMod.MultiHop = params.Attr.SetMultiHop

	tk.LogIt(tk.LogDebug, "[API] GoBGP neighAdd : %v\n", bgpNeighMod)
	_, err := ApiHooks.NetGoBGPNeighAdd(&bgpNeighMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBGPNeigh(params operations.DeleteConfigBgpNeighIPAddressParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpNeighMod cmn.GoBGPNeighMod

	// IP address
	bgpNeighMod.Addr = net.ParseIP(params.IPAddress)

	// Remote AS
	bgpNeighMod.RemoteAS = uint32(*params.RemoteAs)

	tk.LogIt(tk.LogDebug, "[API] GoBGP neighDel : %v\n", bgpNeighMod)
	_, err := ApiHooks.NetGoBGPNeighDel(&bgpNeighMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostBGPGlobal(params operations.PostConfigBgpGlobalParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Global Config %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpG cmn.GoBGPGlobalConfig

	// Router ID
	bgpG.RouterID = params.Attr.RouterID

	// Local AS
	bgpG.LocalAs = params.Attr.LocalAs

	// Export policy list
	bgpG.SetNHSelf = params.Attr.SetNextHopSelf

	// Listen Port
	bgpG.ListenPort = uint16(params.Attr.ListenPort)
	if bgpG.ListenPort == 0 {
		bgpG.ListenPort = 179
	}

	tk.LogIt(tk.LogDebug, "[API] GoBGP GCAdd : %v\n", bgpG)
	_, err := ApiHooks.NetGoBGPGCAdd(&bgpG)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
