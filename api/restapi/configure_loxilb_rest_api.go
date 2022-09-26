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
package restapi

import (
	"crypto/tls"
	"net/http"
	"os"

	opts "github.com/loxilb-io/loxilb/options"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"

	"github.com/loxilb-io/loxilb/api/restapi/handler"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
)

//go:generate swagger generate server --target ../../api --name LoxilbRestAPI --spec ../swagger.yml --principal interface{}

func configureFlags(api *operations.LoxilbRestAPIAPI) {
	api.CommandLineOptionsGroups = append(api.CommandLineOptionsGroups,
		swag.CommandLineOptionsGroup{
			Options: &opts.Opts,
		},
	)

}

func configureAPI(api *operations.LoxilbRestAPIAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// api.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	// Load balancer add and delete and get
	api.PostConfigLoadbalancerHandler = operations.PostConfigLoadbalancerHandlerFunc(handler.ConfigPostLoadbalancer)
	api.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoHandler = operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoHandlerFunc(handler.ConfigDeleteLoadbalancer)
	api.GetConfigLoadbalancerAllHandler = operations.GetConfigLoadbalancerAllHandlerFunc(handler.ConfigGetLoadbalancer)
	api.DeleteConfigLoadbalancerAllHandler = operations.DeleteConfigLoadbalancerAllHandlerFunc(handler.ConfigDeleteAllLoadbalancer)

	// Conntrack get
	api.GetConfigConntrackAllHandler = operations.GetConfigConntrackAllHandlerFunc(handler.ConfigGetConntrack)

	// Port get
	api.GetConfigPortAllHandler = operations.GetConfigPortAllHandlerFunc(handler.ConfigGetPort)

	// route add and delete
	api.PostConfigRouteHandler = operations.PostConfigRouteHandlerFunc(handler.ConfigPostRoute)
	api.DeleteConfigRouteDestinationIPNetIPAddressMaskHandler = operations.DeleteConfigRouteDestinationIPNetIPAddressMaskHandlerFunc(handler.ConfigDeleteRoute)
	api.GetConfigRouteAllHandler = operations.GetConfigRouteAllHandlerFunc(handler.ConfigGetRoute)

	// Session, SessionUlCl Add and delete
	api.PostConfigSessionHandler = operations.PostConfigSessionHandlerFunc(handler.ConfigPostSession)
	api.PostConfigSessionulclHandler = operations.PostConfigSessionulclHandlerFunc(handler.ConfigPostSessionUlCl)
	api.DeleteConfigSessionIdentIdentHandler = operations.DeleteConfigSessionIdentIdentHandlerFunc(handler.ConfigDeleteSession)
	api.DeleteConfigSessionulclIdentIdentUlclAddressIPAddressHandler = operations.DeleteConfigSessionulclIdentIdentUlclAddressIPAddressHandlerFunc(handler.ConfigDeleteSessionUlCl)
	api.GetConfigSessionAllHandler = operations.GetConfigSessionAllHandlerFunc(handler.ConfigGetSession)
	api.GetConfigSessionulclAllHandler = operations.GetConfigSessionulclAllHandlerFunc(handler.ConfigGetSessionUlCl)

	// Policy Add, Delete and Get
	api.PostConfigPolicyHandler = operations.PostConfigPolicyHandlerFunc(handler.ConfigPostPolicy)
	api.DeleteConfigPolicyIdentIdentHandler = operations.DeleteConfigPolicyIdentIdentHandlerFunc(handler.ConfigDeletePolicy)
	api.GetConfigPolicyAllHandler = operations.GetConfigPolicyAllHandlerFunc(handler.ConfigGetPolicy)

	// IPv4 add And Delete
	api.PostConfigIpv4addressHandler = operations.PostConfigIpv4addressHandlerFunc(handler.ConfigPostIPv4Address)
	api.DeleteConfigIpv4addressIPAddressMaskDevIfNameHandler = operations.DeleteConfigIpv4addressIPAddressMaskDevIfNameHandlerFunc(handler.ConfigDeleteIPv4Address)
	api.GetConfigIpv4addressAllHandler = operations.GetConfigIpv4addressAllHandlerFunc(handler.ConfigGetIPv4Address)

	// Mirror Add and Delete
	api.PostConfigMirrorHandler = operations.PostConfigMirrorHandlerFunc(handler.ConfigPostMirror)
	api.DeleteConfigMirrorIdentIdentHandler = operations.DeleteConfigMirrorIdentIdentHandlerFunc(handler.ConfigDeleteMirror)
	api.GetConfigMirrorAllHandler = operations.GetConfigMirrorAllHandlerFunc(handler.ConfigGetMirror)

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {
		os.Exit(0)
	}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix".
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation.
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
