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
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetPort(params operations.GetConfigPortAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Port %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	// Get Port informations
	ports, err := ApiHooks.NetPortGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	return &PortResponse{PortAttr: ports}
}
