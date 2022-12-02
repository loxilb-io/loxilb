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

	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostMirror(params operations.PostConfigMirrorParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Mirror %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	var MirrMod cmn.MirrMod

	// Ident Setting
	if params.Attr.MirrorIdent != "" {
		MirrMod.Ident = params.Attr.MirrorIdent
	}

	// Info Setting
	if params.Attr.MirrorInfo != nil {
		MirrMod.Info.MirrPort = params.Attr.MirrorInfo.Port
		MirrMod.Info.MirrRip = net.ParseIP(params.Attr.MirrorInfo.RemoteIP)
		MirrMod.Info.MirrSip = net.ParseIP(params.Attr.MirrorInfo.SourceIP)
		MirrMod.Info.MirrTid = uint32(params.Attr.MirrorInfo.TunnelID)
		MirrMod.Info.MirrType = int(params.Attr.MirrorInfo.Type)
		MirrMod.Info.MirrVlan = int(params.Attr.MirrorInfo.Vlan)
	}

	// Target Setting
	if params.Attr.TargetObject != nil {
		MirrMod.Target.MirrObjName = params.Attr.TargetObject.MirrObjName
		MirrMod.Target.AttachMent = cmn.MirrObjType(params.Attr.TargetObject.Attachment)
	}

	tk.LogIt(tk.LogDebug, "[API] MirrMod : %v\n", MirrMod)
	_, err := ApiHooks.NetMirrorAdd(&MirrMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteMirror(params operations.DeleteConfigMirrorIdentIdentParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Mirror %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var MirrMod cmn.MirrMod

	MirrMod.Ident = params.Ident

	tk.LogIt(tk.LogDebug, "[API] MirrMod : %v\n", MirrMod)
	_, err := ApiHooks.NetMirrorDel(&MirrMod)
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigGetMirror(params operations.GetConfigMirrorAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Mirror %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetMirrorGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.MirrorGetEntry
	result = make([]*models.MirrorGetEntry, 0)
	for _, Mirror := range res {
		var tmpMirr models.MirrorGetEntry
		var tmpInfo models.MirrorGetEntryMirrorInfo
		var tmpTarget models.MirrorGetEntryTargetObject
		// ID match
		tmpMirr.MirrorIdent = Mirror.Ident
		// Info match
		tmpInfo.Type = int64(Mirror.Info.MirrType)
		tmpInfo.Port = Mirror.Info.MirrPort
		if Mirror.Info.MirrRip != nil {
			tmpInfo.RemoteIP = Mirror.Info.MirrRip.String()
		}
		if Mirror.Info.MirrSip != nil {
			tmpInfo.SourceIP = Mirror.Info.MirrSip.String()
		}
		tmpInfo.TunnelID = int64(Mirror.Info.MirrTid)
		tmpInfo.Vlan = int64(Mirror.Info.MirrVlan)
		// Target match
		tmpTarget.Attachment = int64(Mirror.Target.AttachMent)
		tmpTarget.MirrObjName = Mirror.Target.MirrObjName

		// Sync match
		helperSync := int64(Mirror.Sync)
		tmpMirr.Sync = &helperSync
		// Assign Mirror info and target
		tmpMirr.MirrorInfo = &tmpInfo
		tmpMirr.TargetObject = &tmpTarget
		result = append(result, &tmpMirr)
	}

	return operations.NewGetConfigMirrorAllOK().WithPayload(&operations.GetConfigMirrorAllOKBody{MirrAttr: result})
}
