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
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostLoadbalancer(params operations.PostConfigLoadbalancerParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbRules cmn.LbRuleMod

	lbRules.Serv.ServIP = params.Attr.ServiceArguments.ExternalIP
	lbRules.Serv.ServPort = uint16(params.Attr.ServiceArguments.Port)
	lbRules.Serv.Proto = params.Attr.ServiceArguments.Protocol
	lbRules.Serv.Sel = cmn.EpSelect(params.Attr.ServiceArguments.Sel)
	lbRules.Serv.Bgp = params.Attr.ServiceArguments.Bgp

	for _, data := range params.Attr.Endpoints {
		lbRules.Eps = append(lbRules.Eps, cmn.LbEndPointArg{
			EpIP:   data.EndpointIP,
			EpPort: uint16(data.TargetPort),
			Weight: uint8(data.Weight),
		})
	}

	tk.LogIt(tk.LOG_DEBUG, "[API] lbRules : %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleAdd(&lbRules)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteLoadbalancer(params operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoParams) middleware.Responder {
	tk.LogIt(tk.LOG_DEBUG, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbServ cmn.LbServiceArg
	var lbRules cmn.LbRuleMod
	lbServ.ServIP = params.IPAddress
	lbServ.ServPort = uint16(params.Port)
	lbServ.Proto = params.Proto
	if params.Bgp != nil {
		lbServ.Bgp = *params.Bgp
	}

	lbRules.Serv = lbServ
	tk.LogIt(tk.LOG_DEBUG, "[API] lbRules : %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleDel(&lbRules)
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetLoadbalancer(params operations.GetConfigLoadbalancerAllParams) middleware.Responder {
	// Get LB rules
	tk.LogIt(tk.LOG_DEBUG, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetLbRuleGet()
	if err != nil {
		tk.LogIt(tk.LOG_DEBUG, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &LbResponse{Attr: res}
}
