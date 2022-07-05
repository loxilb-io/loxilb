package handler

import (
	"loxilb/api/restapi/operations"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetConntrack(params operations.GetConfigConntrackAllParams) middleware.Responder {
	// Get Conntrack informations
	contr, err := ApiHooks.NetCtInfoGet()
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &CtResponse{CtAttr: contr}
}
