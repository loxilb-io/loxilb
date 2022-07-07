package handler

import (
	"loxilb/api/restapi/operations"
	cmn "loxilb/common"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostRoute(params operations.PostConfigLoadbalancerParams) middleware.Responder {
	var rotueMod cmn.Routev4Mod
	_, err := ApiHooks.NetRoutev4Add(&rotueMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteRoute(params operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoParams) middleware.Responder {
	var rotueMod cmn.Routev4Mod
	_, err := ApiHooks.NetRoutev4Del(&rotueMod)

	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
