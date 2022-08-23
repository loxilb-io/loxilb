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
	"loxilb/api/restapi/operations"
	cmn "loxilb/common"
	"net"

	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostSession(params operations.PostConfigSessionParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Session %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var sessionMod cmn.SessionMod
	// Default Setting
	sessionMod.Ident = params.Attr.Ident
	sessionMod.Ip = net.ParseIP(params.Attr.SessionIP)
	// AnTun Setting
	sessionMod.AnTun.TeID = uint32(params.Attr.AccessNetworkTunnel.TeID)
	sessionMod.AnTun.Addr = net.ParseIP(params.Attr.AccessNetworkTunnel.TunnelIP)
	// CnTul Setting
	sessionMod.CnTun.TeID = uint32(params.Attr.CoreNetworkTunnel.TeID)
	sessionMod.CnTun.Addr = net.ParseIP(params.Attr.CoreNetworkTunnel.TunnelIP)

	tk.LogIt(tk.LOG_DEBUG, "[API] Session sessionMod : %v\n", sessionMod)
	_, err := ApiHooks.NetSessionAdd(&sessionMod)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteSession(params operations.DeleteConfigSessionIdentIdentParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Session %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var sessionMod cmn.SessionMod
	// Default Setting
	sessionMod.Ident = params.Ident
	tk.LogIt(tk.LOG_DEBUG, "[API] Session sessionMod : %v\n", sessionMod)
	_, err := ApiHooks.NetSessionDel(&sessionMod)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostSessionUlCl(params operations.PostConfigSessionulclParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Session UlCl %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var sessionulclMod cmn.SessionUlClMod
	// Default Setting
	sessionulclMod.Ident = params.Attr.UlclIdent
	// UlCl Argument setting
	sessionulclMod.Args.Addr = net.ParseIP(params.Attr.UlclArgument.UlclIP)
	sessionulclMod.Args.Qfi = uint8(params.Attr.UlclArgument.Qfi)

	tk.LogIt(tk.LOG_DEBUG, "[API] Session sessionMod : %v\n", sessionulclMod)
	_, err := ApiHooks.NetSessionUlClAdd(&sessionulclMod)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteSessionUlCl(params operations.DeleteConfigSessionulclIdentIdentUlclAddressIPAddressParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Session UlCl %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var sessionulclMod cmn.SessionUlClMod

	// Default Setting
	sessionulclMod.Ident = params.Ident
	// UlCl Argument setting
	sessionulclMod.Args.Addr = net.ParseIP(params.IPAddress)

	tk.LogIt(tk.LOG_DEBUG, "[API] Session sessionMod : %v\n", sessionulclMod)
	_, err := ApiHooks.NetSessionUlClDel(&sessionulclMod)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetSession(params operations.GetConfigSessionAllParams) middleware.Responder {
	// Get Session rules
	tk.LogIt(tk.LOG_DEBUG, "[API] Session %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetSessionGet()
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &SessionResponse{Attr: res}
}

func ConfigGetSessionUlCl(params operations.GetConfigSessionulclAllParams) middleware.Responder {
	// Get Ulcl rules
	tk.LogIt(tk.LOG_DEBUG, "[API] Session UlCl %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetSessionUlClGet()
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &SessionUlClResponse{Attr: res}
}
