package handler

import (
	"fmt"
	"loxilb/api/restapi/operations"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostLoadbalancer(params operations.PostConfigLoadbalancerParams) middleware.Responder {
	//return middleware.NotImplemented("operation operations.PostConfigLoadbalancer has not yet been implemented")

	/*
		lbServ := LbServiceArg{"10.10.10.1", 2020, "tcp"}
		lbEps := []LbEndPointArg{
			{
			"32.32.32.1",
			5001,
			1,
			},
			{
			"32.32.32.1",
			5001,
			2,
			},
		}

		mh.zr.Rules.AddNatLbRule(lbServ, lbEps[:])
	*/
	fmt.Printf("params.Attr: %v\n", params.Attr)
	for _, data := range params.Attr.EndPoint {
		fmt.Printf("data.EndpointIPAddress: %v\n", data.EndpointIPAddress)
		fmt.Printf("data: %v\n", data)
	}

	return &ResultResponse{Result: "Hi"}
}
