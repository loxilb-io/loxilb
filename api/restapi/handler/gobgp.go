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
	"net"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigGetBGPNeigh(params operations.GetConfigBgpNeighAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetGoBGPNeighGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.BGPNeighGetEntry
	result = make([]*models.BGPNeighGetEntry, 0)
	for _, nei := range res {
		tmpNeigh := models.BGPNeighGetEntry{}
		tmpNeigh.IPAddress = nei.Addr
		tmpNeigh.RemoteAs = int64(nei.RemoteAS)
		tmpNeigh.State = nei.State
		tmpNeigh.Updowntime = nei.Uptime

		result = append(result, &tmpNeigh)
	}

	return operations.NewGetConfigBgpNeighAllOK().WithPayload(&operations.GetConfigBgpNeighAllOKBody{BgpNeiAttr: result})
}
func ConfigPostBGPNeigh(params operations.PostConfigBgpNeighParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpNeighMod cmn.GoBGPNeighMod

	// IP address
	bgpNeighMod.Addr = net.ParseIP(params.Attr.IPAddress)

	// Remote AS
	bgpNeighMod.RemoteAS = uint32(params.Attr.RemoteAs)

	// Remote Port
	bgpNeighMod.RemotePort = uint16(params.Attr.RemotePort)

	// Multi-hop or not
	bgpNeighMod.MultiHop = params.Attr.SetMultiHop

	tk.LogIt(tk.LogDebug, "[API] GoBGP neighAdd : %v\n", bgpNeighMod)
	_, err := ApiHooks.NetGoBGPNeighAdd(&bgpNeighMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBGPNeigh(params operations.DeleteConfigBgpNeighIPAddressParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Neighbor %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpNeighMod cmn.GoBGPNeighMod

	// IP address
	bgpNeighMod.Addr = net.ParseIP(params.IPAddress)

	// Remote AS
	bgpNeighMod.RemoteAS = uint32(*params.RemoteAs)

	tk.LogIt(tk.LogDebug, "[API] GoBGP neighDel : %v\n", bgpNeighMod)
	_, err := ApiHooks.NetGoBGPNeighDel(&bgpNeighMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostBGPGlobal(params operations.PostConfigBgpGlobalParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Global Config %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpG cmn.GoBGPGlobalConfig

	// Router ID
	bgpG.RouterID = params.Attr.RouterID

	// Local AS
	bgpG.LocalAs = params.Attr.LocalAs

	// Export policy list
	bgpG.SetNHSelf = params.Attr.SetNextHopSelf

	// Listen Port
	bgpG.ListenPort = uint16(params.Attr.ListenPort)
	if bgpG.ListenPort == 0 {
		bgpG.ListenPort = 179
	}

	tk.LogIt(tk.LogDebug, "[API] GoBGP GCAdd : %v\n", bgpG)
	_, err := ApiHooks.NetGoBGPGCAdd(&bgpG)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostBGPPolicyDefinedsets(params operations.PostConfigBgpPolicyDefinedsetsDefinesetTypeParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy DefinedSet %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyDefinedSet cmn.GoBGPPolicyDefinedSetMod

	// name
	bgpPolicyDefinedSet.Name = params.Attr.Name
	bgpPolicyDefinedSet.DefinedTypeString = params.DefinesetType

	if bgpPolicyDefinedSet.DefinedTypeString == "prefix" || bgpPolicyDefinedSet.DefinedTypeString == "Prefix" {
		for _, prefix := range params.Attr.PrefixList {
			var tmpPrefix cmn.Prefix
			tmpPrefix.IpPrefix = prefix.IPPrefix
			tmpPrefix.MasklengthRange = prefix.MasklengthRange
			bgpPolicyDefinedSet.PrefixList = append(bgpPolicyDefinedSet.PrefixList, tmpPrefix)
		}

	} else {
		bgpPolicyDefinedSet.List = params.Attr.List
	}

	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyPrefix : %v\n", bgpPolicyDefinedSet)
	_, err := ApiHooks.NetGoBGPPolicyDefinedSetAdd(&bgpPolicyDefinedSet)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBGPPolicyDefinedsets(params operations.DeleteConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy DefinedSet %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyConfig cmn.GoBGPPolicyDefinedSetMod

	bgpPolicyConfig.Name = params.TypeName
	bgpPolicyConfig.DefinedTypeString = params.DefinesetType

	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyConfig : %v\n", bgpPolicyConfig)
	_, err := ApiHooks.NetGoBGPPolicyDefinedSetDel(&bgpPolicyConfig)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetBGPPolicyDefinedSetGet(params operations.GetConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP DefinedSet %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetGoBGPPolicyDefinedSetGet(params.TypeName, params.DefinesetType)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	result := make([]*models.BGPPolicyDefinedSetGetEntry, 0)
	for _, df := range res {
		tmpDf := models.BGPPolicyDefinedSetGetEntry{}
		tmpDf.Name = df.Name
		tmpDf.List = df.List

		if params.DefinesetType == "prefix" {
			tmpDf.PrefixList = make([]*models.BGPPolicyPrefix, 0)
			for _, prefix := range df.PrefixList {
				tmpPrefix := models.BGPPolicyPrefix{
					IPPrefix:        prefix.IpPrefix,
					MasklengthRange: prefix.MasklengthRange,
				}
				tmpDf.PrefixList = append(tmpDf.PrefixList, &tmpPrefix)
			}
		}

		result = append(result, &tmpDf)
	}

	return operations.NewGetConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameOK().WithPayload(&operations.GetConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameOKBody{DefinedsetsAttr: result})
}

func ConfigPostBGPPolicyDefinitions(params operations.PostConfigBgpPolicyDefinitionsParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy Definitions %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyConfig cmn.GoBGPPolicyDefinitionsMod

	// name
	bgpPolicyConfig.Name = params.Attr.Name
	// Statement
	for _, statement := range params.Attr.Statements {
		var tmpStatement cmn.Statement
		tmpStatement.Name = statement.Name
		// Condition part
		if statement.Conditions.MatchNeighborSet != nil {
			tmpStatement.Conditions.NeighborSet = cmn.MatchNeighborSet(*statement.Conditions.MatchNeighborSet)
		}
		if statement.Conditions.MatchPrefixSet != nil {
			tmpStatement.Conditions.PrefixSet = cmn.MatchPrefixSet(*statement.Conditions.MatchPrefixSet)
		}
		if statement.Conditions.BgpConditions != nil {
			if len(statement.Conditions.BgpConditions.AfiSafiIn) != 0 {
				tmpStatement.Conditions.BGPConditions.AfiSafiIn = statement.Conditions.BgpConditions.AfiSafiIn
			}

			if statement.Conditions.BgpConditions.AsPathLength != nil {
				tmpStatement.Conditions.BGPConditions.AsPathLength.Operator = statement.Conditions.BgpConditions.AsPathLength.Operator
				tmpStatement.Conditions.BGPConditions.AsPathLength.Value = int(statement.Conditions.BgpConditions.AsPathLength.Value)
			}

			if statement.Conditions.BgpConditions.MatchAsPathSet != nil {
				tmpStatement.Conditions.BGPConditions.AsPathSet = cmn.BGPAsPathSet(*statement.Conditions.BgpConditions.MatchAsPathSet)
			}
			if statement.Conditions.BgpConditions.MatchCommunitySet != nil {
				tmpStatement.Conditions.BGPConditions.CommunitySet = cmn.BGPCommunitySet(*statement.Conditions.BgpConditions.MatchCommunitySet)
			}
			if statement.Conditions.BgpConditions.MatchExtCommunitySet != nil {
				tmpStatement.Conditions.BGPConditions.ExtCommunitySet = cmn.BGPCommunitySet(*statement.Conditions.BgpConditions.MatchExtCommunitySet)
			}

			if statement.Conditions.BgpConditions.MatchLargeCommunitySet != nil {
				tmpStatement.Conditions.BGPConditions.LargeCommunitySet = cmn.BGPCommunitySet(*statement.Conditions.BgpConditions.MatchLargeCommunitySet)
			}
			if statement.Conditions.BgpConditions.Rpki != "" {
				tmpStatement.Conditions.BGPConditions.Rpki = statement.Conditions.BgpConditions.Rpki
			}
			if statement.Conditions.BgpConditions.RouteType != "" {
				tmpStatement.Conditions.BGPConditions.RouteType = statement.Conditions.BgpConditions.RouteType
			}

			if len(statement.Conditions.BgpConditions.NextHopInList) != 0 {
				tmpStatement.Conditions.BGPConditions.NextHopInList = statement.Conditions.BgpConditions.NextHopInList
			}
		}

		// Action Part
		tmpStatement.Actions.RouteDisposition = statement.Actions.RouteDisposition
		if statement.Actions.BgpActions != nil {
			if statement.Actions.BgpActions.SetAsPathPrepend != nil {
				tmpStatement.Actions.BGPActions.SetAsPathPrepend.ASN = statement.Actions.BgpActions.SetAsPathPrepend.As
				tmpStatement.Actions.BGPActions.SetAsPathPrepend.RepeatN = int(statement.Actions.BgpActions.SetAsPathPrepend.RepeatN)
			}
			if statement.Actions.BgpActions.SetCommunity != nil {
				tmpStatement.Actions.BGPActions.SetCommunity = cmn.SetCommunity(*statement.Actions.BgpActions.SetCommunity)
			}
			if statement.Actions.BgpActions.SetExtCommunity != nil {
				tmpStatement.Actions.BGPActions.SetExtCommunity = cmn.SetCommunity(*statement.Actions.BgpActions.SetExtCommunity)
			}
			if statement.Actions.BgpActions.SetLargeCommunity != nil {
				tmpStatement.Actions.BGPActions.SetLargeCommunity = cmn.SetCommunity(*statement.Actions.BgpActions.SetLargeCommunity)
			}
			if statement.Actions.BgpActions.SetMed != "" {
				tmpStatement.Actions.BGPActions.SetMed = statement.Actions.BgpActions.SetMed
			}
			if statement.Actions.BgpActions.SetLocalPerf != 0 {
				tmpStatement.Actions.BGPActions.SetLocalPerf = int(statement.Actions.BgpActions.SetLocalPerf)
			}
			if statement.Actions.BgpActions.SetNextHop != "" {
				tmpStatement.Actions.BGPActions.SetNextHop = statement.Actions.BgpActions.SetNextHop
			}

		}

		bgpPolicyConfig.Statement = append(bgpPolicyConfig.Statement, tmpStatement)
	}

	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyConfig : %v\n", bgpPolicyConfig)
	_, err := ApiHooks.NetGoBGPPolicyDefinitionAdd(&bgpPolicyConfig)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBGPPolicyDefinitions(params operations.DeleteConfigBgpPolicyDefinitionsPolicyNameParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy Definitions %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyConfig cmn.GoBGPPolicyDefinitionsMod

	// name
	bgpPolicyConfig.Name = params.PolicyName

	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyConfig : %v\n", bgpPolicyConfig)
	_, err := ApiHooks.NetGoBGPPolicyDefinitionDel(&bgpPolicyConfig)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetBGPPolicyDefinitions(params operations.GetConfigBgpPolicyDefinitionsAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy Definitions %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetGoBGPPolicyDefinitionsGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	result := make([]*models.BGPPolicyDefinitionsMod, 0)
	for _, df := range res {
		tmpDf := models.BGPPolicyDefinitionsMod{}
		tmpDf.Name = df.Name
		for _, stat := range df.Statement {
			tmpSt := models.BGPPolicyDefinitionsStatement{
				Name: stat.Name,
			}
			var BgpConditions models.BGPPolicyDefinitionsStatementConditionsBgpConditions
			var MatchNeighborSet models.BGPPolicyDefinitionsStatementConditionsMatchNeighborSet
			var MatchPrefixSet models.BGPPolicyDefinitionsStatementConditionsMatchPrefixSet

			MatchPrefixSet.PrefixSet = stat.Conditions.PrefixSet.PrefixSet
			MatchPrefixSet.MatchSetOption = stat.Conditions.PrefixSet.MatchSetOption

			MatchNeighborSet.NeighborSet = stat.Conditions.NeighborSet.NeighborSet
			MatchNeighborSet.MatchSetOption = stat.Conditions.NeighborSet.MatchSetOption

			BgpConditions.AfiSafiIn = stat.Conditions.BGPConditions.AfiSafiIn

			BgpConditions.AsPathLength = &models.BGPPolicyDefinitionsStatementConditionsBgpConditionsAsPathLength{
				Operator: stat.Conditions.BGPConditions.AsPathLength.Operator,
				Value:    int64(stat.Conditions.BGPConditions.AsPathLength.Value),
			}

			BgpConditions.MatchAsPathSet = &models.BGPPolicyDefinitionsStatementConditionsBgpConditionsMatchAsPathSet{
				AsPathSet:       stat.Conditions.BGPConditions.AsPathSet.AsPathSet,
				MatchSetOptions: stat.Conditions.BGPConditions.AsPathSet.MatchSetOptions,
			}

			BgpConditions.MatchCommunitySet = &models.BGPPolicyDefinitionsStatementConditionsBgpConditionsMatchCommunitySet{
				CommunitySet:    stat.Conditions.BGPConditions.CommunitySet.CommunitySet,
				MatchSetOptions: stat.Conditions.BGPConditions.CommunitySet.MatchSetOptions,
			}
			BgpConditions.MatchExtCommunitySet = &models.BGPPolicyDefinitionsStatementConditionsBgpConditionsMatchExtCommunitySet{
				CommunitySet:    stat.Conditions.BGPConditions.CommunitySet.CommunitySet,
				MatchSetOptions: stat.Conditions.BGPConditions.CommunitySet.MatchSetOptions,
			}
			BgpConditions.MatchLargeCommunitySet = &models.BGPPolicyDefinitionsStatementConditionsBgpConditionsMatchLargeCommunitySet{
				CommunitySet:    stat.Conditions.BGPConditions.CommunitySet.CommunitySet,
				MatchSetOptions: stat.Conditions.BGPConditions.CommunitySet.MatchSetOptions,
			}

			BgpConditions.NextHopInList = stat.Conditions.BGPConditions.NextHopInList

			BgpConditions.RouteType = stat.Conditions.BGPConditions.RouteType

			BgpConditions.Rpki = stat.Conditions.BGPConditions.Rpki

			Conditions := models.BGPPolicyDefinitionsStatementConditions{
				MatchPrefixSet:   &MatchPrefixSet,
				MatchNeighborSet: &MatchNeighborSet,
				BgpConditions:    &BgpConditions,
			}
			// action
			var BgpActions models.BGPPolicyDefinitionsStatementActionsBgpActions
			BgpActions.SetAsPathPrepend = &models.BGPPolicyDefinitionsStatementActionsBgpActionsSetAsPathPrepend{
				As:      stat.Actions.BGPActions.SetAsPathPrepend.ASN,
				RepeatN: int64(stat.Actions.BGPActions.SetAsPathPrepend.RepeatN),
			}
			BgpActions.SetCommunity = &models.BGPPolicyDefinitionsStatementActionsBgpActionsSetCommunity{
				Options:            stat.Actions.BGPActions.SetCommunity.Options,
				SetCommunityMethod: stat.Actions.BGPActions.SetCommunity.SetCommunityMethod,
			}
			BgpActions.SetExtCommunity = &models.BGPPolicyDefinitionsStatementActionsBgpActionsSetExtCommunity{
				Options:            stat.Actions.BGPActions.SetExtCommunity.Options,
				SetCommunityMethod: stat.Actions.BGPActions.SetExtCommunity.SetCommunityMethod,
			}
			BgpActions.SetLargeCommunity = &models.BGPPolicyDefinitionsStatementActionsBgpActionsSetLargeCommunity{
				Options:            stat.Actions.BGPActions.SetLargeCommunity.Options,
				SetCommunityMethod: stat.Actions.BGPActions.SetLargeCommunity.SetCommunityMethod,
			}
			BgpActions.SetLocalPerf = int64(stat.Actions.BGPActions.SetLocalPerf)
			BgpActions.SetMed = stat.Actions.BGPActions.SetMed
			BgpActions.SetNextHop = stat.Actions.BGPActions.SetNextHop

			Action := models.BGPPolicyDefinitionsStatementActions{
				BgpActions:       &BgpActions,
				RouteDisposition: stat.Actions.RouteDisposition,
			}
			tmpSt.Conditions = &Conditions
			tmpSt.Actions = &Action
			tmpDf.Statements = append(tmpDf.Statements, &tmpSt)
		}

		result = append(result, &tmpDf)
	}

	return operations.NewGetConfigBgpPolicyDefinitionsAllOK().WithPayload(&operations.GetConfigBgpPolicyDefinitionsAllOKBody{BgpPolicyAttr: result})
}

func ConfigPostBGPPolicyApply(params operations.PostConfigBgpPolicyApplyParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy Apply %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyConfig cmn.GoBGPPolicyApply

	bgpPolicyConfig.NeighIPAddress = params.Attr.IPAddress
	bgpPolicyConfig.PolicyType = params.Attr.PolicyType
	bgpPolicyConfig.Polices = params.Attr.Policies
	bgpPolicyConfig.RouteAction = params.Attr.RouteAction
	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyConfig : %v\n", bgpPolicyConfig)
	_, err := ApiHooks.NetGoBGPPolicyApplyAdd(&bgpPolicyConfig)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteBGPPolicyApply(params operations.DeleteConfigBgpPolicyApplyParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] BGP Policy Apply %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var bgpPolicyConfig cmn.GoBGPPolicyApply

	bgpPolicyConfig.NeighIPAddress = params.Attr.IPAddress
	bgpPolicyConfig.PolicyType = params.Attr.PolicyType
	bgpPolicyConfig.Polices = params.Attr.Policies
	bgpPolicyConfig.RouteAction = "" // No need RouteAction for delete
	tk.LogIt(tk.LogDebug, "[API] GoBGP bgpPolicyConfig : %v\n", bgpPolicyConfig)
	_, err := ApiHooks.NetGoBGPPolicyApplyDel(&bgpPolicyConfig)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
