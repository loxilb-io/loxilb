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
	sessionMod.Ip = net.ParseIP(params.Attr.IPAddress)
	// AnTun Setting
	sessionMod.AnTun.TeID = uint32(params.Attr.AccessNetworkTunnel.TeID)
	sessionMod.AnTun.Addr = net.ParseIP(params.Attr.AccessNetworkTunnel.Address)
	// CnTul Setting
	sessionMod.CnTun.TeID = uint32(params.Attr.ConnectionNetworkTunnel.TeID)
	sessionMod.CnTun.Addr = net.ParseIP(params.Attr.ConnectionNetworkTunnel.Address)

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
	sessionulclMod.Ident = params.Attr.Ident
	// UlCl Argument setting
	sessionulclMod.Args.Addr = net.ParseIP(params.Attr.UlclArgument.Address)
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
