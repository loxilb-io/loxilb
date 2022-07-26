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

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostSession(params operations.PostConfigSessionParams) middleware.Responder {
	var sessionMod cmn.SessionMod
	// Default Setting
	sessionMod.Ident = params.Attr.Ident
	sessionMod.Ip = net.ParseIP(params.Attr.SessionIP)
	// AnTun Setting
	sessionMod.AnTun.TeID = uint32(params.Attr.AccessNetworkTunnel.TeID)
	sessionMod.AnTun.Addr = net.ParseIP(params.Attr.AccessNetworkTunnel.TunnelIP)
	// CnTul Setting
	sessionMod.CnTun.TeID = uint32(params.Attr.ConnectionNetworkTunnel.TeID)
	sessionMod.CnTun.Addr = net.ParseIP(params.Attr.ConnectionNetworkTunnel.TunnelIP)

	_, err := ApiHooks.NetSessionAdd(&sessionMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteSession(params operations.DeleteConfigSessionIdentIdentParams) middleware.Responder {
	var sessionMod cmn.SessionMod
	// Default Setting
	sessionMod.Ident = params.Ident

	_, err := ApiHooks.NetSessionDel(&sessionMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostSessionUlCl(params operations.PostConfigSessionulclParams) middleware.Responder {
	var sessionulclMod cmn.SessionUlClMod
	// Default Setting
	sessionulclMod.Ident = params.Attr.UlclIdent
	// UlCl Argument setting
	sessionulclMod.Args.Addr = net.ParseIP(params.Attr.UlclArgument.UlclIP)
	sessionulclMod.Args.Qfi = uint8(params.Attr.UlclArgument.Qfi)

	_, err := ApiHooks.NetSessionUlClAdd(&sessionulclMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteSessionUlCl(params operations.DeleteConfigSessionulclIdentIdentUlclAddressIPAddressParams) middleware.Responder {
	var sessionulclMod cmn.SessionUlClMod

	// Default Setting
	sessionulclMod.Ident = params.Ident
	// UlCl Argument setting
	sessionulclMod.Args.Addr = net.ParseIP(params.IPAddress)

	_, err := ApiHooks.NetSessionUlClDel(&sessionulclMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetSession(params operations.GetConfigSessionAllParams) middleware.Responder {
	// Get Session rules
	res, err := ApiHooks.NetSessionGet()
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &SessionResponse{Attr: res}
}

func ConfigGetSessionUlCl(params operations.GetConfigSessionulclAllParams) middleware.Responder {
	// Get Ulcl rules
	res, err := ApiHooks.NetSessionUlClGet()
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &SessionUlClResponse{Attr: res}
}
