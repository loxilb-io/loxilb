package handler

import (
	"loxilb/api/restapi/operations"
	cmn "loxilb/common"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostLoadbalancer(params operations.PostConfigLoadbalancerParams) middleware.Responder {
	var lbServ cmn.LbServiceArg
	var lbEps []cmn.LbEndPointArg
	var lbRules cmn.LbRuleMod

	lbServ.ServIP = params.Attr.ExternalIPAddress
	lbServ.ServPort = uint16(params.Attr.Port)
	lbServ.Proto = params.Attr.Protocol

	for _, data := range params.Attr.Endpoints {
		lbEps = append(lbEps, cmn.LbEndPointArg{
			EpIP:   data.EndpointIPAddress,
			EpPort: uint16(data.TargetPort),
			Weight: uint8(data.Weight),
		})
	}

	lbRules.Serv = lbServ
	lbRules.Eps = append(lbRules.Eps, lbEps...)

	//fmt.Printf("lbEps: %v\n", lbEps)
	//fmt.Printf("lbServ: %v\n", lbServ)
	//fmt.Printf("lbRules: %v\n", lbRules)

	_, err := ApiHooks.NetLbRuleAdd(&lbRules)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
