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
	"github.com/loxilb-io/loxilb/api/apiutils/status"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetProcess(params operations.GetStatusProcessParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	//var result []*models.ProcessInfoEntry
	process := status.ProcessInfoGet()

	return operations.NewGetStatusProcessOK().WithPayload(&operations.GetStatusProcessOKBody{ProcessAttr: process})
}

func ConfigGetDevice(params operations.GetStatusDeviceParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	// Get Conntrack informations
	res, err := status.DeviceInfoGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return operations.NewGetStatusDeviceOK().WithPayload(res)
}

func ConfigGetFileSystem(params operations.GetStatusFilesystemParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Status %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := status.FileSystemInfoGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return operations.NewGetStatusFilesystemOK().WithPayload(&operations.GetStatusFilesystemOKBody{FilesystemAttr: res})
}
