/*
 * Copyright (c) 2025 LoxiLB Authors
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
	"strings"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/apiutils/cors"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

// ConfigGetCors retrieves the list of Cors entries
func ConfigGetCors(params operations.GetConfigCorsAllParams, principal interface{}) middleware.Responder {
	// Get Cors rules
	tk.LogIt(tk.LogTrace, "api: Cors %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res := cors.GetCORSManager().GetOrigin()
	corsList := make([]string, 0, len(res))
	for cors := range res {
		if strings.TrimSpace(cors) == "" {
			msg := "Cors URL cannot be empty"
			return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
		}
		cors = strings.TrimSpace(cors)
		if cors == "*" {
			return operations.NewGetConfigCorsAllOK().WithPayload(&operations.GetConfigCorsAllOKBody{CorsAttr: []string{"*"}})
		}
		corsList = append(corsList, cors)
		tk.LogIt(tk.LogDebug, "Cors URL: %s\n", cors)
	}
	return operations.NewGetConfigCorsAllOK().WithPayload(&operations.GetConfigCorsAllOKBody{CorsAttr: corsList})
}

// ConfigPostCors adds or updates Cors entries
func ConfigPostCors(params operations.PostConfigCorsParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: Cors %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	for _, corsurl := range params.Attr.Cors {
		if strings.TrimSpace(corsurl) == "" {
			msg := "Cors URL cannot be empty"
			return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
		}
		corsurl = strings.TrimSpace(corsurl)
		if corsurl == "*" {
			msg := "failed to add Cors URL: wildcard '*' is not allowed"
			return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
		}
		// Add or update the Cors URL
		corsManager := cors.GetCORSManager()
		err := corsManager.AddOrigin(corsurl)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Cors URL: %s\n", corsurl)
			msg := "Failed to add Cors URL: " + err.Error()
			return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
		}
		// Log the successful addition
		tk.LogIt(tk.LogDebug, "Cors URL: %s added successfully\n", corsurl)
	}

	return &ResultResponse{Result: "Success"}
}

// ConfigDeleteCors deletes a Cors entry
func ConfigDeleteCors(params operations.DeleteConfigCorsCorsURLParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: Cors %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	if strings.TrimSpace(params.CorsURL) == "" {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Cors URL cannot be empty")}
	}
	params.CorsURL = strings.TrimSpace(params.CorsURL)
	if params.CorsURL == "*" {
		msg := "failed to delete Cors URL: wildcard '*' is not allowed"
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
	}
	// Remove the Cors URL
	corsManager := cors.GetCORSManager()
	err := corsManager.RemoveOrigin(params.CorsURL)
	if err != nil {
		tk.LogIt(tk.LogError, "Failed to delete Cors URL: %s\n", params.CorsURL)
		msg := "Failed to delete Cors URL: " + err.Error()
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(msg)}
	}
	// Log the successful deletion
	tk.LogIt(tk.LogDebug, "Cors URL: %s deleted successfully\n", params.CorsURL)
	return &ResultResponse{Result: "Success"}
}
