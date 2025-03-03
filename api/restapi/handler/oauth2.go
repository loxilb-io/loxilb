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
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations/auth"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
)

var (
	stateTokens      = make(map[string]time.Time)
	stateTokensMutex sync.Mutex
	StateTokenTTL    = 10 * time.Minute
)

// GenerateStateToken generates a secure state token and stores it with an expiration time.
// GenerateStateToken generates a random state token, encodes it in URL-safe
// base64, and stores it with an expiration time. The state token is used to
// prevent CSRF attacks during OAuth authentication flows. It returns the
// generated state token as a string. If there is an error generating the
// random bytes, the function will panic.
func GenerateStateToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // This should never happen
	}
	stateToken := base64.URLEncoding.EncodeToString(b)

	// Store the state token with an expiration time
	stateTokensMutex.Lock()
	stateTokens[stateToken] = time.Now().Add(StateTokenTTL)
	stateTokensMutex.Unlock()

	return stateToken
}

// ValidateStateToken validates the state token and removes it from the store if valid.
// ValidateStateToken checks if the provided state token exists and is not expired.
// If the token is valid, it removes it from the store to prevent reuse and returns true.
// If the token does not exist or is expired, it returns false.
func ValidateStateToken(token string) bool {
	stateTokensMutex.Lock()
	defer stateTokensMutex.Unlock()

	expirationTime, exists := stateTokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expirationTime) {
		delete(stateTokens, token)
		return false
	}

	// Token is valid, remove it from the store to prevent reuse
	delete(stateTokens, token)
	return true
}

// CleanupExpiredStateTokens removes expired state tokens from the store.
// CleanupExpiredStateTokens iterates through the stored state tokens and removes
// any that have expired. This helps to keep the state token store clean and
// prevents memory leaks.
func CleanupExpiredStateTokens() {
	stateTokensMutex.Lock()
	defer stateTokensMutex.Unlock()

	now := time.Now()
	for token, expirationTime := range stateTokens {
		if now.After(expirationTime) {
			delete(stateTokens, token)
		}
	}
}

func fetchUserInfo(client *http.Client, provider string) (map[string]interface{}, error) {
	var url string
	switch provider {
	case "github":
		url = "https://api.github.com/user"
	case "facebook":
		url = "https://graph.facebook.com/me?fields=id,email"
	default:
		url = "https://www.googleapis.com/oauth2/v2/userinfo"
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return userInfo, nil
}

var OAuthConfigs = map[string]*oauth2.Config{
	"google": {
		ClientID:     "",
		ClientSecret: "",
		RedirectURL:  "",
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	},
	"github": {
		ClientID:     "",
		ClientSecret: "",
		RedirectURL:  "",
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	},
}

func InitOAuthConfigs() {
	googleConfig := OAuthConfigs["google"]
	googleConfig.ClientID = opts.Opts.Oauth2GoogleClientID
	googleConfig.ClientSecret = opts.Opts.Oauth2GoogleClientSecret
	googleConfig.RedirectURL = opts.Opts.Oauth2GoogleRedirectURL
	OAuthConfigs["google"] = googleConfig

	githubConfig := OAuthConfigs["github"]
	githubConfig.ClientID = opts.Opts.Oauth2GithubClientID
	githubConfig.ClientSecret = opts.Opts.Oauth2GithubClientSecret
	githubConfig.RedirectURL = opts.Opts.Oauth2GithubRedirectURL
	OAuthConfigs["github"] = githubConfig
}

// AuthGetOauthProvider function
// This function is used to get the OAuth provider
func AuthGetOauthProvider(params auth.GetOauthProviderParams) middleware.Responder {

	provider := params.Provider
	oauthConfig, exists := OAuthConfigs[provider]

	if !exists {
		return auth.NewGetOauthProviderBadRequest().WithPayload(&models.OauthErrorResponse{
			Message: "Invalid OAuth provider",
		})
	}

	state := GenerateStateToken() // Generate a secure state token
	tk.LogIt(tk.LogTrace, "Generated state token for OAuth login:%v\n", state)

	// Can't extract the redirect URL from the OAuth config, so we set it here
	authURL := oauthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("access_type", "offline"),
		oauth2.SetAuthURLParam("prompt", "consent"), // Forces a new refresh token even if already authorized
		oauth2.SetAuthURLParam("redirect_uri", oauthConfig.RedirectURL),
	)
	//authURL := oauthConfig.AuthCodeURL(state, oauth2.SetAuthURLParam("redirect_uri", oauthConfig.RedirectURL))

	// Return a redirect response
	return middleware.ResponderFunc(func(w http.ResponseWriter, _ runtime.Producer) {
		http.Redirect(w, params.HTTPRequest, authURL, http.StatusTemporaryRedirect)
	})
}

// AuthGetOauthProviderCallback function
// This function is used to get the OAuth provider callback
func AuthGetOauthProviderCallback(params auth.GetOauthProviderCallbackParams) middleware.Responder {
	var response models.OauthLoginResponse

	provider := params.Provider
	oauthConfig, exists := OAuthConfigs[provider]
	if !exists {
		return auth.NewGetOauthProviderCallbackBadRequest().WithPayload(&models.OauthErrorResponse{
			Message: "Invalid OAuth provider",
		})
	}

	state := params.State
	if !ValidateStateToken(state) { // Validate the state token
		return auth.NewGetOauthProviderCallbackBadRequest().WithPayload(&models.OauthErrorResponse{
			Message: "Invalid state token",
		})
	}

	code := params.Code
	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: "Token exchange failed",
		})
	}

	client := oauthConfig.Client(context.Background(), token)
	userInfo, err := fetchUserInfo(client, provider)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: "Failed to get user info",
		})
	}

	var oauthName, email, oauthID string

	if provider == "google" {
		email = userInfo["email"].(string)
		oauthName = userInfo["name"].(string)
		oauthID = userInfo["id"].(string)
	} else if provider == "github" {
		// Fetch emails explicitly from GitHub API
		emailsResp, err := client.Get("https://api.github.com/user/emails")
		if err != nil {
			return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
				Message: "Failed to get GitHub emails",
			})
		}
		defer emailsResp.Body.Close()

		var emails []map[string]interface{}
		if err := json.NewDecoder(emailsResp.Body).Decode(&emails); err != nil {
			return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
				Message: "Failed to parse GitHub emails",
			})
		}

		for _, e := range emails {
			if primary, ok := e["primary"].(bool); ok && primary {
				email = e["email"].(string)
				break
			}
		}

		// Fetch user details from GitHub API
		userResp, err := client.Get("https://api.github.com/user")
		if err == nil {
			defer userResp.Body.Close()
			var userDetail map[string]interface{}
			if err := json.NewDecoder(userResp.Body).Decode(&userDetail); err == nil {
				oauthName, _ = userDetail["name"].(string)
				oauthID, _ = userDetail["id"].(string)
			}
		}
	}

	tk.LogIt(tk.LogInfo, "Oauth User %s logged-in name: %s Refresh token %s\n", email, oauthName, token.RefreshToken)
	loginToken, valid, err := ApiHooks.NetOauthUserTokenStore(email, token.AccessToken, token.RefreshToken, token.Expiry)

	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	if valid {
		response.Token = loginToken
		response.ID = oauthID
		response.Expiresin = int64(token.Expiry.Second())
		response.Refreshtoken = token.RefreshToken
	}
	return auth.NewGetOauthProviderCallbackOK().WithPayload(&response)
}

// RefreshAccessToken refreshes the access token using the provided refresh token.
// It returns a fresh access token or an error if the refresh fails.
func RefreshAccessToken(refreshToken string, provider string) (*oauth2.Token, error) {
	// Get the OAuth config for the specified provider
	oauthConfig, exists := OAuthConfigs[provider]
	if !exists {
		return nil, errors.New("Invalid OAuth provider")
	}

	// Create a new oauth2.Token with the refresh token
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	// Use the OAuth config to refresh the token
	newToken, err := oauthConfig.TokenSource(context.Background(), token).Token()
	if err != nil {
		return nil, errors.New("failed to refresh access token: " + err.Error())
	}

	// Return the new access token
	return newToken, nil
}

// RefreshTokenHandler refreshes the access token using a provided refresh token and OAuth provider.
// It returns the new access token and its expiration time, or an error if the refresh fails.
func RefreshTokenHandler(params auth.GetOauthProviderTokenParams) middleware.Responder {
	// Assume the refresh_token is passed in some way (e.g., from a user's session or request)
	accessToken := params.Token // You'll need to extract this from the request or session
	refreshToken := params.Refreshtoken

	// Assume the provider is either "google" or "github"
	provider := params.Provider

	cached, err := ApiHooks.NetOauthValidateAllTokens(accessToken, refreshToken)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: %v", err),
		})
	}

	cacheStrings, ok := cached.(string)
	if !ok {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: invalid cache"),
		})
	}

	cacheValues := strings.Split("|", cacheStrings)
	if len(cacheValues) != 3 {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: invalid cache format"),
		})
	}

	userEmail := cacheValues[0]

	// Refresh the access token using the refresh token
	newToken, err := RefreshAccessToken(refreshToken, provider)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: %v", err),
		})
	}

	err = ApiHooks.NetOauthDeleteToken(accessToken)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: %v", err),
		})
	}

	tk.LogIt(tk.LogInfo, "Oauth User %s token refreshed\n", userEmail)
	loginToken, _, err := ApiHooks.NetOauthUserTokenStore(userEmail, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry)
	if err != nil {
		return auth.NewGetOauthProviderCallbackInternalServerError().WithPayload(&models.OauthErrorResponse{
			Message: fmt.Sprintf("Failed to refresh access token: %v", err),
		})
	}

	// Return the fresh access token
	return auth.NewGetOauthProviderTokenOK().WithPayload(&models.OauthTokenResponse{
		Token:     loginToken,
		Expiresin: int64(newToken.Expiry.Second()),
	})
}
