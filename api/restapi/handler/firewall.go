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
	"fmt"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostFW(params operations.PostConfigFirewallParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Firewall %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	Opts := cmn.FwOptArg{}
	Rules := cmn.FwRuleArg{}
	FW := cmn.FwRuleMod{}
	//Body Maker
	Rules.DstIP = params.Attr.RuleArguments.DestinationIP
	Rules.DstPortMax = uint16(params.Attr.RuleArguments.MaxDestinationPort)
	Rules.DstPortMin = uint16(params.Attr.RuleArguments.MinDestinationPort)
	Rules.InPort = params.Attr.RuleArguments.PortName
	Rules.Pref = uint16(params.Attr.RuleArguments.Preference)
	Rules.Proto = uint8(params.Attr.RuleArguments.Protocol)
	Rules.SrcIP = params.Attr.RuleArguments.SourceIP
	Rules.SrcPortMax = uint16(params.Attr.RuleArguments.MaxSourcePort)
	Rules.SrcPortMin = uint16(params.Attr.RuleArguments.MinSourcePort)

	if Rules.DstIP == "" {
		Rules.DstIP = "0.0.0.0/0"
	}

	if Rules.SrcIP == "" {
		Rules.SrcIP = "0.0.0.0/0"
	}
	// opts
	Opts.Allow = params.Attr.Opts.Allow
	Opts.Drop = params.Attr.Opts.Drop
	Opts.Rdr = params.Attr.Opts.Redirect
	Opts.RdrPort = params.Attr.Opts.RedirectPortName
	Opts.Trap = params.Attr.Opts.Trap
	Opts.Record = params.Attr.Opts.Record
	Opts.Mark = uint32(params.Attr.Opts.FwMark)
	Opts.DoSnat = params.Attr.Opts.DoSnat
	Opts.ToIP = params.Attr.Opts.ToIP
	Opts.ToPort = uint16(params.Attr.Opts.ToPort)

	FW.Rule = Rules
	FW.Opts = Opts
	fmt.Printf("FW: %v\n", FW)
	_, err := ApiHooks.NetFwRuleAdd(&FW)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteFW(params operations.DeleteConfigFirewallParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Firewall %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	Rules := cmn.FwRuleArg{}
	FW := cmn.FwRuleMod{}
	// Body Make
	// Rule
	fmt.Printf("params.DestinationIP: %v\n", params.DestinationIP)
	if params.DestinationIP != nil {
		Rules.DstIP = *params.DestinationIP
	}
	if params.MaxDestinationPort != nil {
		Rules.DstPortMax = uint16(*params.MaxDestinationPort)
	}

	if params.MinDestinationPort != nil {

		Rules.DstPortMin = uint16(*params.MinDestinationPort)
	}
	if params.PortName != nil {
		Rules.InPort = *params.PortName
	}
	if params.Preference != nil {
		Rules.Pref = uint16(*params.Preference)
	}
	if params.Protocol != nil {
		Rules.Proto = uint8(*params.Protocol)
	}
	if params.SourceIP != nil {
		Rules.SrcIP = *params.SourceIP
	}

	if Rules.DstIP == "" {
		Rules.DstIP = "0.0.0.0/0"
	}

	if Rules.SrcIP == "" {
		Rules.SrcIP = "0.0.0.0/0"
	}

	if params.MinSourcePort != nil {
		Rules.SrcPortMin = uint16(*params.MinSourcePort)
	}

	if params.MaxSourcePort != nil {
		Rules.SrcPortMax = uint16(*params.MaxSourcePort)
	}

	FW.Rule = Rules
	fmt.Printf("FW: %v\n", FW)
	ret, err := ApiHooks.NetFwRuleDel(&FW)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}

	return &ResultResponse{Result: "Success"}
}

func ConfigGetFW(params operations.GetConfigFirewallAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Firewall %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, _ := ApiHooks.NetFwRuleGet()
	var result []*models.FirewallEntry
	result = make([]*models.FirewallEntry, 0)
	for _, FW := range res {
		var tmpResult models.FirewallEntry
		var tmpRule models.FirewallRuleEntry
		var tmpOpts models.FirewallOptionEntry
		// Rule
		tmpRule.DestinationIP = FW.Rule.DstIP
		tmpRule.MaxDestinationPort = int64(FW.Rule.DstPortMax)
		tmpRule.MinDestinationPort = int64(FW.Rule.DstPortMin)
		tmpRule.PortName = FW.Rule.InPort
		tmpRule.Preference = int64(FW.Rule.Pref)
		tmpRule.Protocol = int64(FW.Rule.Proto)
		tmpRule.SourceIP = FW.Rule.SrcIP
		tmpRule.MaxSourcePort = int64(FW.Rule.SrcPortMax)
		tmpRule.MinSourcePort = int64(FW.Rule.SrcPortMin)

		// Opts
		tmpOpts.Allow = FW.Opts.Allow
		tmpOpts.Drop = FW.Opts.Drop
		tmpOpts.Redirect = FW.Opts.Rdr
		tmpOpts.RedirectPortName = FW.Opts.RdrPort
		tmpOpts.Trap = FW.Opts.Trap
		tmpOpts.Record = FW.Opts.Record
		tmpOpts.FwMark = int64(FW.Opts.Mark)
		tmpOpts.DoSnat = FW.Opts.DoSnat
		tmpOpts.ToIP = FW.Opts.ToIP
		tmpOpts.ToPort = int64(FW.Opts.ToPort)
		tmpOpts.Counter = FW.Opts.Counter

		tmpResult.RuleArguments = &tmpRule
		tmpResult.Opts = &tmpOpts

		result = append(result, &tmpResult)
	}
	return operations.NewGetConfigFirewallAllOK().WithPayload(&operations.GetConfigFirewallAllOKBody{FwAttr: result})
}
