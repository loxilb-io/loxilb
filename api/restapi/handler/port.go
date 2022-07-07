package handler

import (
	"loxilb/api/restapi/operations"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetPort(params operations.GetConfigPortAllParams) middleware.Responder {
	// Get Port informations
	ports, err := ApiHooks.NetPortGet()
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &PortResponse{PortAttr: ports}
}
