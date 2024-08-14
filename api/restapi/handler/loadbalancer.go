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
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbRules cmn.LbRuleMod

	lbRules.Serv.ServIP = params.Attr.ServiceArguments.ExternalIP
	lbRules.Serv.PrivateIP = params.Attr.ServiceArguments.PrivateIP
	lbRules.Serv.ServPort = uint16(params.Attr.ServiceArguments.Port)
	lbRules.Serv.Proto = params.Attr.ServiceArguments.Protocol
	lbRules.Serv.BlockNum = params.Attr.ServiceArguments.Block
	lbRules.Serv.Sel = cmn.EpSelect(params.Attr.ServiceArguments.Sel)
	lbRules.Serv.Bgp = params.Attr.ServiceArguments.Bgp
	lbRules.Serv.Monitor = params.Attr.ServiceArguments.Monitor
	lbRules.Serv.Mode = cmn.LBMode(params.Attr.ServiceArguments.Mode)
	lbRules.Serv.Security = cmn.LBSec(params.Attr.ServiceArguments.Security)
	lbRules.Serv.InactiveTimeout = uint32(params.Attr.ServiceArguments.InactiveTimeOut)
	lbRules.Serv.Managed = params.Attr.ServiceArguments.Managed
	lbRules.Serv.ProbeType = params.Attr.ServiceArguments.Probetype
	lbRules.Serv.ProbePort = params.Attr.ServiceArguments.Probeport
	lbRules.Serv.ProbeReq = params.Attr.ServiceArguments.Probereq
	lbRules.Serv.ProbeResp = params.Attr.ServiceArguments.Proberesp
	lbRules.Serv.ProbeTimeout = params.Attr.ServiceArguments.ProbeTimeout
	lbRules.Serv.ProbeRetries = int(params.Attr.ServiceArguments.ProbeRetries)
	lbRules.Serv.Name = params.Attr.ServiceArguments.Name
	lbRules.Serv.Oper = cmn.LBOp(params.Attr.ServiceArguments.Oper)
	lbRules.Serv.HostUrl = params.Attr.ServiceArguments.Host

	if lbRules.Serv.Proto == "sctp" {
		for _, data := range params.Attr.SecondaryIPs {
			lbRules.SecIPs = append(lbRules.SecIPs, cmn.LbSecIPArg{
				SecIP: data.SecondaryIP,
			})
		}
	}

	for _, data := range params.Attr.Endpoints {
		lbRules.Eps = append(lbRules.Eps, cmn.LbEndPointArg{
			EpIP:   data.EndpointIP,
			EpPort: uint16(data.TargetPort),
			Weight: uint8(data.Weight),
		})
	}

	if lbRules.Serv.Mode == cmn.LBModeDSR && lbRules.Serv.Sel != cmn.LbSelHash {
		return &ResultResponse{Result: "Error: Only Hash Selection criteria allowed for DSR mode"}
	}

	tk.LogIt(tk.LogDebug, "[API] lbRules : %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleAdd(&lbRules)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteLoadbalancer(params operations.DeleteConfigLoadbalancerHosturlHosturlExternalipaddressIPAddressPortPortProtocolProtoParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbServ cmn.LbServiceArg
	var lbRules cmn.LbRuleMod
	lbServ.ServIP = params.IPAddress
	lbServ.ServPort = uint16(params.Port)
	lbServ.Proto = params.Proto
	if params.Hosturl == "any" {
		lbServ.HostUrl = ""
	} else {
		lbServ.HostUrl = params.Hosturl
	}
	if params.Block != nil {
		lbServ.BlockNum = uint16(*params.Block)
	}
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

func ConfigDeleteLoadbalancerWithoutPath(params operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var lbServ cmn.LbServiceArg
	var lbRules cmn.LbRuleMod
	lbServ.ServIP = params.IPAddress
	lbServ.ServPort = uint16(params.Port)
	lbServ.Proto = params.Proto
	lbServ.HostUrl = ""
	if params.Block != nil {
		lbServ.BlockNum = uint16(*params.Block)
	}
	if params.Bgp != nil {
		lbServ.Bgp = *params.Bgp
	}

	lbRules.Serv = lbServ
	tk.LogIt(tk.LogDebug, "[API] lbRules (w/o Path): %v\n", lbRules)
	_, err := ApiHooks.NetLbRuleDel(&lbRules)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetLoadbalancer(params operations.GetConfigLoadbalancerAllParams) middleware.Responder {
	// Get LB rules
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

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
		tmpSvc.Block = uint16(lb.Serv.BlockNum)
		tmpSvc.Sel = int64(lb.Serv.Sel)
		tmpSvc.Mode = int32(lb.Serv.Mode)
		tmpSvc.Security = int32(lb.Serv.Security)
		tmpSvc.InactiveTimeOut = int32(lb.Serv.InactiveTimeout)
		tmpSvc.Monitor = lb.Serv.Monitor
		tmpSvc.Managed = lb.Serv.Managed
		tmpSvc.Probetype = lb.Serv.ProbeType
		tmpSvc.Probeport = lb.Serv.ProbePort
		tmpSvc.Name = lb.Serv.Name
		tmpSvc.Snat = lb.Serv.Snat
		tmpSvc.Host = lb.Serv.HostUrl

		tmpLB.ServiceArguments = &tmpSvc

		for _, sip := range lb.SecIPs {
			tmpSIP := new(models.LoadbalanceEntrySecondaryIPsItems0)
			tmpSIP.SecondaryIP = sip.SecIP
			tmpLB.SecondaryIPs = append(tmpLB.SecondaryIPs, tmpSIP)
		}

		// Endpoints match
		for _, ep := range lb.Eps {
			tmpEp := new(models.LoadbalanceEntryEndpointsItems0)
			tmpEp.EndpointIP = ep.EpIP
			tmpEp.TargetPort = int64(ep.EpPort)
			tmpEp.Weight = int64(ep.Weight)
			tmpEp.State = ep.State
			tmpEp.Counter = ep.Counters
			tmpLB.Endpoints = append(tmpLB.Endpoints, tmpEp)
		}

		result = append(result, &tmpLB)
	}
	return operations.NewGetConfigLoadbalancerAllOK().WithPayload(&operations.GetConfigLoadbalancerAllOKBody{LbAttr: result})
}

func ConfigDeleteAllLoadbalancer(params operations.DeleteConfigLoadbalancerAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

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

func ConfigDeleteLoadbalancerByName(params operations.DeleteConfigLoadbalancerNameLbNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Load balancer %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	res, err := ApiHooks.NetLbRuleGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	for _, lbRules := range res {

		if lbRules.Serv.Name != params.LbName {
			continue
		}

		tk.LogIt(tk.LogDebug, "[API] lbRules : %v\n", lbRules)
		_, err := ApiHooks.NetLbRuleDel(&lbRules)
		if err != nil {
			tk.LogIt(tk.LogDebug, "[API] Error : %v\n", err)
		}
	}

	return &ResultResponse{Result: "Success"}
}
