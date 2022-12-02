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
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostPolicy(params operations.PostConfigPolicyParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Policy %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var polMod cmn.PolMod

	// Ident Setting
	if params.Attr.PolicyIdent != "" {
		polMod.Ident = params.Attr.PolicyIdent
	}

	// Info Setting
	if params.Attr.PolicyInfo != nil {
		polMod.Info.ColorAware = params.Attr.PolicyInfo.ColorAware
		polMod.Info.CommittedBlkSize = uint64(params.Attr.PolicyInfo.CommittedBlkSize)
		polMod.Info.CommittedInfoRate = uint64(params.Attr.PolicyInfo.CommittedInfoRate)
		polMod.Info.ExcessBlkSize = uint64(params.Attr.PolicyInfo.ExcessBlkSize)
		polMod.Info.PeakInfoRate = uint64(params.Attr.PolicyInfo.PeakInfoRate)
		polMod.Info.PolType = int(params.Attr.PolicyInfo.Type)
	}

	// Target Setting
	if params.Attr.TargetObject != nil {
		polMod.Target.PolObjName = params.Attr.TargetObject.PolObjName
		polMod.Target.AttachMent = cmn.PolObjType(params.Attr.TargetObject.Attachment)
	}

	tk.LogIt(tk.LogDebug, "[API] polMod : %v\n", polMod)
	_, err := ApiHooks.NetPolicerAdd(&polMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeletePolicy(params operations.DeleteConfigPolicyIdentIdentParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Policy %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var polMod cmn.PolMod

	polMod.Ident = params.Ident

	tk.LogIt(tk.LogDebug, "[API] polMod : %v\n", polMod)
	_, err := ApiHooks.NetPolicerDel(&polMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetPolicy(params operations.GetConfigPolicyAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Policy %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetPolicerGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}

	var result []*models.PolicyEntry
	result = make([]*models.PolicyEntry, 0)
	for _, policy := range res {
		var tmpPol models.PolicyEntry
		var tmpInfo models.PolicyEntryPolicyInfo
		var tmpTarget models.PolicyEntryTargetObject
		// ID match
		tmpPol.PolicyIdent = policy.Ident
		// Info match
		tmpInfo.ColorAware = policy.Info.ColorAware
		tmpInfo.CommittedBlkSize = int64(policy.Info.CommittedBlkSize)
		tmpInfo.CommittedInfoRate = int64(policy.Info.CommittedInfoRate)
		tmpInfo.ExcessBlkSize = int64(policy.Info.ExcessBlkSize)
		tmpInfo.PeakInfoRate = int64(policy.Info.PeakInfoRate)
		tmpInfo.Type = int64(policy.Info.PolType)
		// Target match
		tmpTarget.Attachment = int64(policy.Target.AttachMent)
		tmpTarget.PolObjName = policy.Target.PolObjName

		// Assign policy info and target
		tmpPol.PolicyInfo = &tmpInfo
		tmpPol.TargetObject = &tmpTarget
		result = append(result, &tmpPol)
	}

	return operations.NewGetConfigPolicyAllOK().WithPayload(&operations.GetConfigPolicyAllOKBody{PolAttr: result})
}
