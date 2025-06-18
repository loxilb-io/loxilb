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
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations/auth"
	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
)

// BearerAuthAuth parses and validates a JWT token string.
// It returns the claims contained in the token if it is valid, or an error if the token is invalid or parsing fails.
// But if the UserServiceEnable option is disabled, it will return true.
//
// Parameters:
//   - tokenString: the JWT token string to be validated.
//
// Returns:
//   - bool: the claims contained in the token if it is valid.
//   - error: an error if the token is invalid or parsing fails.
func BearerAuthAuth(tokenString string) (interface{}, error) {
	if opts.Opts.UserServiceEnable {
		// User DB based valaidation
		return ApiHooks.NetUserValidate(tokenString)
	} else if opts.Opts.Oauth2Enable {
		// OAuth2 based validation
		return ApiHooks.NetOauthUserValidate(tokenString)
	} else if opts.Opts.ManualTokenEnable {
		// Manual token based validation
		return ManualTokenValidate(tokenString)
	} else {
		return true, nil
	}
}

// AuthPostLogin function
// This function is used to authenticate the user and generate a token
func AuthPostLogin(params auth.PostAuthLoginParams) middleware.Responder {
	var response models.LoginResponse
	var user cmn.User
	if params.User.Username != nil {
		user.Username = *params.User.Username
	}
	if params.User.Password != nil {
		user.Password = *params.User.Password
	}
	token, valid, err := ApiHooks.NetUserLogin(&user)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	if valid {
		response.Token = token
	}
	return auth.NewPostAuthLoginOK().WithPayload(&response)
}

// AuthPostLogout function
// This function is used to logout the user
func AuthPostLogout(params auth.PostAuthLogoutParams, principal interface{}) middleware.Responder {
	token := params.HTTPRequest.Header.Get("Authorization")
	err := ApiHooks.NetUserLogout(token)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return auth.NewPostAuthLogoutOK()
}

// Authorized function to handle authorization logic
// requests are authorized based on the role of the user
func Authorized() runtime.Authorizer {
	// TODO: Add more roles and permissions logic for oauth users
	if opts.Opts.UserServiceEnable {
		return runtime.AuthorizerFunc(func(param *http.Request, principal interface{}) error {
			permitInfo := principal.(string)
			// Viewer user can only GET requests
			UserNameAndRole := strings.Split(permitInfo, "|")
			if len(UserNameAndRole) != 2 {
				return errors.New("Invalid user info. Please contact the administrator")
			}
			UserRole := UserNameAndRole[1]
			// Viewer user can only GET requests except for logout
			if strings.Contains(UserRole, "viewer") && param.Method == "POST" && param.URL.Path == "/netlox/v1/auth/logout" {
				return nil
			} else if strings.Contains(UserRole, "viewer") && param.Method != "GET" {
				return errors.New("Permission denied")
			}
			return nil
		})
	} else {
		return runtime.AuthorizerFunc(func(_ *http.Request, _ interface{}) error {

			return nil
		})

	}

}

// ManualTokenValidate function
// This function is used to validate the manual token
func ManualTokenValidate(tokenString string) (interface{}, error) {
	manualTokenPath := opts.Opts.ManualTokenPath

	data, err := os.ReadFile(manualTokenPath)
	if err != nil {
		// File not found but return invalid token
		tk.LogIt(tk.LogError, "Manual token file not found: %v\n", err)
		return nil, errors.New("invalid token")
	}
	manualToken := strings.TrimSpace(string(data))
	// Validate the token
	if tokenString == manualToken {
		return true, nil
	}

	return nil, errors.New("invalid token")
}
