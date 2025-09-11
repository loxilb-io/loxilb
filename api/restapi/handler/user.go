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
	"time"

	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations/users"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// UsersPostUsers function
// This function is used to add a new user
func UsersPostUsers(params users.PostAuthUsersParams) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: User  %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var user cmn.User
	if params.User.Username != nil {
		user.Username = *params.User.Username
	}
	if params.User.Password != nil {
		user.Password = *params.User.Password
	}
	user.CreatedAt = time.Now()
	user.Role = params.User.Role
	_, err := ApiHooks.NetUserAdd(&user)
	if err != nil {
		tk.LogIt(tk.LogDebug, "api: Error occur : %v\n", err.Error())
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}
	return &ResultResponse{Result: "Success"}
}

// UsersDeleteUsers function
// This function is used to delete a user
func UsersDeleteUsers(params users.DeleteAuthUsersIDParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: User %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	err := ApiHooks.NetUserDel(int(params.ID))
	if err != nil {
		tk.LogIt(tk.LogDebug, "api: Error occur : %v\n", err.Error())
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}
	return &ResultResponse{Result: "Success"}
}

// UsersGetUsers function
// This function is used to get all users
func UsersGetUsers(params users.GetAuthUsersParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: User %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	res, err := ApiHooks.NetUserGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "api: Error occur : %v\n", err.Error())
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}

	// Convert to swagger model
	var result []*models.User
	result = make([]*models.User, 0)
	for _, user := range res {
		var tmpUser models.User
		// ID match
		tmpUser.ID = int64(user.ID)
		tmpUser.Username = &user.Username
		tmpUser.Password = &user.Password
		tmpUser.CreatedAt = user.CreatedAt.Format(time.RFC3339)

		result = append(result, &tmpUser)
	}

	return users.NewGetAuthUsersOK().WithPayload(result)
}

// UsersPutUsers function
// This function is used to update a user
func UsersPutUsers(params users.PutAuthUsersIDParams, principal interface{}) middleware.Responder {
	tk.LogIt(tk.LogTrace, "api: User %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	var user cmn.User
	if params.User.Username != nil {
		user.Username = *params.User.Username
	}
	if params.User.Password != nil {
		user.Password = *params.User.Password
	}
	user.ID = int(params.ID)
	err := ApiHooks.NetUserUpdate(&user)
	if err != nil {
		tk.LogIt(tk.LogDebug, "api: Error occur : %v\n", err.Error())
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage(err.Error())}
	}
	return &ResultResponse{Result: "Success"}
}
