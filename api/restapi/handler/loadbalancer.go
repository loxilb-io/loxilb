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
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostLoadbalancer(params operations.PostConfigLoadbalancerParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbRules cmn.LbRuleMod

	lbRules.Serv.ServIP = params.Attr.ServiceArguments.ExternalIP
	lbRules.Serv.ServPort = uint16(params.Attr.ServiceArguments.Port)
	lbRules.Serv.Proto = params.Attr.ServiceArguments.Protocol
	lbRules.Serv.Sel = cmn.EpSelect(params.Attr.ServiceArguments.Sel)
	lbRules.Serv.Bgp = params.Attr.ServiceArguments.Bgp
	lbRules.Serv.Mode = params.Attr.ServiceArguments.Mode

	for _, data := range params.Attr.Endpoints {
		lbRules.Eps = append(lbRules.Eps, cmn.LbEndPointArg{
			EpIP:   data.EndpointIP,
			EpPort: uint16(data.TargetPort),
			Weight: uint8(data.Weight),
		})
	}

	tk.LogIt(tk.LogDebug, "[API] lbRules : %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleAdd(&lbRules)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteLoadbalancer(params operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbServ cmn.LbServiceArg
	var lbRules cmn.LbRuleMod
	lbServ.ServIP = params.IPAddress
	lbServ.ServPort = uint16(params.Port)
	lbServ.Proto = params.Proto
	if params.Bgp != nil {
		lbServ.Bgp = *params.Bgp
	}

	lbRules.Serv = lbServ
	tk.LogIt(tk.LogDebug, "[API] lbRules : %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleDel(&lbRules)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetLoadbalancer(params operations.GetConfigLoadbalancerAllParams) middleware.Responder {
	// Get LB rules
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetLbRuleGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.LoadbalanceEntry
	result = make([]*models.LoadbalanceEntry, 0)
	for _, lb := range res {
		var tmpLB models.LoadbalanceEntry
		var tmpSvc models.LoadbalanceEntryServiceArguments

		// Service Arg match
		tmpSvc.ExternalIP = lb.Serv.ServIP
		tmpSvc.Bgp = lb.Serv.Bgp
		tmpSvc.Port = int64(lb.Serv.ServPort)
		tmpSvc.Protocol = lb.Serv.Proto
		tmpSvc.Sel = int64(lb.Serv.Sel)
		tmpSvc.Mode = lb.Serv.Mode

		tmpLB.ServiceArguments = &tmpSvc

		// Endpoints match
		for _, ep := range lb.Eps {
			tmpEp := new(models.LoadbalanceEntryEndpointsItems0)
			tmpEp.EndpointIP = ep.EpIP
			tmpEp.TargetPort = int64(ep.EpPort)
			tmpEp.Weight = int64(ep.Weight)
			tmpLB.Endpoints = append(tmpLB.Endpoints, tmpEp)
		}

		result = append(result, &tmpLB)
	}
	return operations.NewGetConfigLoadbalancerAllOK().WithPayload(&operations.GetConfigLoadbalancerAllOKBody{LbAttr: result})
}
func ConfigDeleteAllLoadbalancer(params operations.DeleteConfigLoadbalancerAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetLbRuleGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	for _, lbRules := range res {

		tk.LogIt(tk.LogDebug, "[API] lbRules : %v\n", lbRules)
		_, err := ApiHooks.NetLbRuleDel(&lbRules)
		if err != nil {
			tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		}
	}

	return &ResultResponse{Result: "Success"}
}
