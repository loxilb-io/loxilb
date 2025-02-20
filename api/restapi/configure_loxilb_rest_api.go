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

	opts "github.com/loxilb-io/loxilb/options"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"

	"github.com/loxilb-io/loxilb/api/restapi/handler"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	"github.com/loxilb-io/loxilb/api/restapi/operations/auth"
	"github.com/loxilb-io/loxilb/api/restapi/operations/users"
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
	// Applies when the "Authorization" header is set
	api.BearerAuthAuth = handler.BearerAuthAuth
	// Set your custom authorizer if needed. Default one is security.Authorized()
	api.APIAuthorizer = handler.Authorized()

	// Load balancer add and delete and get
	api.PostConfigLoadbalancerHandler = operations.PostConfigLoadbalancerHandlerFunc(handler.ConfigPostLoadbalancer)
	api.DeleteConfigLoadbalancerHosturlHosturlExternalipaddressIPAddressPortPortProtocolProtoHandler = operations.DeleteConfigLoadbalancerHosturlHosturlExternalipaddressIPAddressPortPortProtocolProtoHandlerFunc(handler.ConfigDeleteLoadbalancer)
	api.DeleteConfigLoadbalancerHosturlHosturlExternalipaddressIPAddressPortPortPortmaxPortmaxProtocolProtoHandler = operations.DeleteConfigLoadbalancerHosturlHosturlExternalipaddressIPAddressPortPortPortmaxPortmaxProtocolProtoHandlerFunc(handler.ConfigDeleteLoadbalancerPortRange)
	api.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoHandler = operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortProtocolProtoHandlerFunc(handler.ConfigDeleteLoadbalancerWithoutPath)
	api.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortPortmaxPortmaxProtocolProtoHandler = operations.DeleteConfigLoadbalancerExternalipaddressIPAddressPortPortPortmaxPortmaxProtocolProtoHandlerFunc(handler.ConfigDeleteLoadbalancerPortRangeWithoutPath)
	api.GetConfigLoadbalancerAllHandler = operations.GetConfigLoadbalancerAllHandlerFunc(handler.ConfigGetLoadbalancer)
	api.DeleteConfigLoadbalancerAllHandler = operations.DeleteConfigLoadbalancerAllHandlerFunc(handler.ConfigDeleteAllLoadbalancer)
	api.DeleteConfigLoadbalancerNameLbNameHandler = operations.DeleteConfigLoadbalancerNameLbNameHandlerFunc(handler.ConfigDeleteLoadbalancerByName)

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

	// Status
	api.GetStatusProcessHandler = operations.GetStatusProcessHandlerFunc(handler.ConfigGetProcess)
	api.GetStatusDeviceHandler = operations.GetStatusDeviceHandlerFunc(handler.ConfigGetDevice)
	api.GetStatusFilesystemHandler = operations.GetStatusFilesystemHandlerFunc(handler.ConfigGetFileSystem)

	// VLAN
	api.GetConfigVlanAllHandler = operations.GetConfigVlanAllHandlerFunc(handler.ConfigGetVLAN)
	api.PostConfigVlanHandler = operations.PostConfigVlanHandlerFunc(handler.ConfigPostVLAN)
	api.DeleteConfigVlanVlanIDHandler = operations.DeleteConfigVlanVlanIDHandlerFunc(handler.ConfigDeleteVLAN)

	// VLAN MEMBER
	api.PostConfigVlanVlanIDMemberHandler = operations.PostConfigVlanVlanIDMemberHandlerFunc(handler.ConfigPostVLANMember)
	api.DeleteConfigVlanVlanIDMemberIfNameTaggedTaggedHandler = operations.DeleteConfigVlanVlanIDMemberIfNameTaggedTaggedHandlerFunc(handler.ConfigDeleteVLANMember)

	// VxLAN
	api.GetConfigTunnelVxlanAllHandler = operations.GetConfigTunnelVxlanAllHandlerFunc(handler.ConfigGetVxLAN)
	api.PostConfigTunnelVxlanHandler = operations.PostConfigTunnelVxlanHandlerFunc(handler.ConfigPostVxLAN)
	api.DeleteConfigTunnelVxlanVxlanIDHandler = operations.DeleteConfigTunnelVxlanVxlanIDHandlerFunc(handler.ConfigDeleteVxLAN)

	//VxLAN Peer
	api.PostConfigTunnelVxlanVxlanIDPeerHandler = operations.PostConfigTunnelVxlanVxlanIDPeerHandlerFunc(handler.ConfigPostVxLANPeer)
	api.DeleteConfigTunnelVxlanVxlanIDPeerPeerIPHandler = operations.DeleteConfigTunnelVxlanVxlanIDPeerPeerIPHandlerFunc(handler.ConfigDeleteVxLANPeer)

	// Neighbor
	api.PostConfigNeighborHandler = operations.PostConfigNeighborHandlerFunc(handler.ConfigPostNeighbor)
	api.DeleteConfigNeighborIPAddressDevIfNameHandler = operations.DeleteConfigNeighborIPAddressDevIfNameHandlerFunc(handler.ConfigDeleteNeighbor)
	api.GetConfigNeighborAllHandler = operations.GetConfigNeighborAllHandlerFunc(handler.ConfigGetNeighbor)

	// FDB
	api.GetConfigFdbAllHandler = operations.GetConfigFdbAllHandlerFunc(handler.ConfigGetFDB)
	api.PostConfigFdbHandler = operations.PostConfigFdbHandlerFunc(handler.ConfigPostFDB)
	api.DeleteConfigFdbMacAddressDevIfNameHandler = operations.DeleteConfigFdbMacAddressDevIfNameHandlerFunc(handler.ConfigDeleteFDB)

	// Cluster Instance
	api.GetConfigCistateAllHandler = operations.GetConfigCistateAllHandlerFunc(handler.ConfigGetCIState)
	api.PostConfigCistateHandler = operations.PostConfigCistateHandlerFunc(handler.ConfigPostCIState)

	// BFD
	api.GetConfigBfdAllHandler = operations.GetConfigBfdAllHandlerFunc(handler.ConfigGetBFDSession)
	api.PostConfigBfdHandler = operations.PostConfigBfdHandlerFunc(handler.ConfigPostBFDSession)
	api.DeleteConfigBfdRemoteIPRemoteIPHandler = operations.DeleteConfigBfdRemoteIPRemoteIPHandlerFunc(handler.ConfigDeleteBFDSession)

	// Firewall
	api.GetConfigFirewallAllHandler = operations.GetConfigFirewallAllHandlerFunc(handler.ConfigGetFW)
	api.PostConfigFirewallHandler = operations.PostConfigFirewallHandlerFunc(handler.ConfigPostFW)
	api.DeleteConfigFirewallHandler = operations.DeleteConfigFirewallHandlerFunc(handler.ConfigDeleteFW)

	// EndPoint
	api.GetConfigEndpointAllHandler = operations.GetConfigEndpointAllHandlerFunc(handler.ConfigGetEndPoint)
	api.PostConfigEndpointHandler = operations.PostConfigEndpointHandlerFunc(handler.ConfigPostEndPoint)
	api.DeleteConfigEndpointEpipaddressIPAddressHandler = operations.DeleteConfigEndpointEpipaddressIPAddressHandlerFunc(handler.ConfigDeleteEndPoint)
	api.PostConfigEndpointhoststateHandler = operations.PostConfigEndpointhoststateHandlerFunc(handler.ConfigPostEndPointHostState)

	// Params
	api.PostConfigParamsHandler = operations.PostConfigParamsHandlerFunc(handler.ConfigPostParams)
	api.GetConfigParamsHandler = operations.GetConfigParamsHandlerFunc(handler.ConfigGetParams)

	// Prometheus
	api.GetMetricsHandler = operations.GetMetricsHandlerFunc(handler.ConfigGetPrometheusCounter)
	api.GetConfigMetricsHandler = operations.GetConfigMetricsHandlerFunc(handler.ConfigGetPrometheusOption)
	api.PostConfigMetricsHandler = operations.PostConfigMetricsHandlerFunc(handler.ConfigPostPrometheus)
	api.DeleteConfigMetricsHandler = operations.DeleteConfigMetricsHandlerFunc(handler.ConfigDeletePrometheus)

	// BGP Peer
	api.GetConfigBgpNeighAllHandler = operations.GetConfigBgpNeighAllHandlerFunc(handler.ConfigGetBGPNeigh)
	api.PostConfigBgpGlobalHandler = operations.PostConfigBgpGlobalHandlerFunc(handler.ConfigPostBGPGlobal)
	api.PostConfigBgpNeighHandler = operations.PostConfigBgpNeighHandlerFunc(handler.ConfigPostBGPNeigh)
	api.DeleteConfigBgpNeighIPAddressHandler = operations.DeleteConfigBgpNeighIPAddressHandlerFunc(handler.ConfigDeleteBGPNeigh)

	// BGP Policy Defined set
	api.GetConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameHandler = operations.GetConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameHandlerFunc(handler.ConfigGetBGPPolicyDefinedSetGet)
	api.PostConfigBgpPolicyDefinedsetsDefinesetTypeHandler = operations.PostConfigBgpPolicyDefinedsetsDefinesetTypeHandlerFunc(handler.ConfigPostBGPPolicyDefinedsets)
	api.DeleteConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameHandler = operations.DeleteConfigBgpPolicyDefinedsetsDefinesetTypeTypeNameHandlerFunc(handler.ConfigDeleteBGPPolicyDefinedsets)

	// BGP Policy Definitions
	api.PostConfigBgpPolicyDefinitionsHandler = operations.PostConfigBgpPolicyDefinitionsHandlerFunc(handler.ConfigPostBGPPolicyDefinitions)
	api.DeleteConfigBgpPolicyDefinitionsPolicyNameHandler = operations.DeleteConfigBgpPolicyDefinitionsPolicyNameHandlerFunc(handler.ConfigDeleteBGPPolicyDefinitions)
	api.GetConfigBgpPolicyDefinitionsAllHandler = operations.GetConfigBgpPolicyDefinitionsAllHandlerFunc(handler.ConfigGetBGPPolicyDefinitions)

	// BGP Policy Apply
	api.PostConfigBgpPolicyApplyHandler = operations.PostConfigBgpPolicyApplyHandlerFunc(handler.ConfigPostBGPPolicyApply)
	api.DeleteConfigBgpPolicyApplyHandler = operations.DeleteConfigBgpPolicyApplyHandlerFunc(handler.ConfigDeleteBGPPolicyApply)

	// Metrics
	api.GetMetricsFlowcountHandler = operations.GetMetricsFlowcountHandlerFunc(handler.ConfigGetFlowCount)
	api.GetMetricsLbrulecountHandler = operations.GetMetricsLbrulecountHandlerFunc(handler.ConfigGetLbRuleCount)
	api.GetMetricsNewflowcountHandler = operations.GetMetricsNewflowcountHandlerFunc(handler.ConfigGetNewFlowCount)
	api.GetMetricsRequestcountHandler = operations.GetMetricsRequestcountHandlerFunc(handler.ConfigGetRequestCount)
	api.GetMetricsErrorcountHandler = operations.GetMetricsErrorcountHandlerFunc(handler.ConfigGetErrorCount)
	api.GetMetricsProcessedtrafficHandler = operations.GetMetricsProcessedtrafficHandlerFunc(handler.ConfigGetProcessedTraffic)
	api.GetMetricsLbprocessedtrafficHandler = operations.GetMetricsLbprocessedtrafficHandlerFunc(handler.ConfigGetLbProcessedTraffic)
	api.GetMetricsEpdisttrafficHandler = operations.GetMetricsEpdisttrafficHandlerFunc(handler.ConfigGetEpDistTraffic)
	api.GetMetricsServicedisttrafficHandler = operations.GetMetricsServicedisttrafficHandlerFunc(handler.ConfigGetServiceDistTraffic)
	api.GetMetricsFwdropsHandler = operations.GetMetricsFwdropsHandlerFunc(handler.ConfigGetFwDrops)
	api.GetMetricsReqcountperclientHandler = operations.GetMetricsReqcountperclientHandlerFunc(handler.ConfigGetReqCounterPerClient)
	api.GetMetricsHostcountHandler = operations.GetMetricsHostcountHandlerFunc(handler.ConfigGetHostCount)

	// Log
	api.GetLogsHandler = operations.GetLogsHandlerFunc(handler.ConfigGetLogs)
	api.GetLogArchivesHandler = operations.GetLogArchivesHandlerFunc(handler.ConfigGetLogArchives)
	api.GetLogArchivesFilenameHandler = operations.GetLogArchivesFilenameHandlerFunc(handler.ConfigGetLogArchivesFilename)

	// Nodegraph
	api.GetNodegraphAllHandler = operations.GetNodegraphAllHandlerFunc(handler.ConfigGetNodeGraph)
	api.GetNodegraphServiceHandler = operations.GetNodegraphServiceHandlerFunc(handler.ConfigGetNodeGraphService)

	// Version
	api.GetVersionHandler = operations.GetVersionHandlerFunc(handler.ConfigGetVersion)

	// It works only if the UserServiceEnable option is enabled.
	if opts.Opts.UserServiceEnable {
		// login logout api
		api.AuthPostAuthLoginHandler = auth.PostAuthLoginHandlerFunc(handler.AuthPostLogin)
		api.AuthPostAuthLogoutHandler = auth.PostAuthLogoutHandlerFunc(handler.AuthPostLogout)

		// Users API
		api.UsersGetAuthUsersHandler = users.GetAuthUsersHandlerFunc(handler.UsersGetUsers)
		api.UsersPostAuthUsersHandler = users.PostAuthUsersHandlerFunc(handler.UsersPostUsers)
		api.UsersDeleteAuthUsersIDHandler = users.DeleteAuthUsersIDHandlerFunc(handler.UsersDeleteUsers)
		api.UsersPutAuthUsersIDHandler = users.PutAuthUsersIDHandlerFunc(handler.UsersPutUsers)
	}

	if opts.Opts.Oauth2Enable {
		// OAuth2 API
		handler.InitOAuthConfigs()
		api.AuthGetOauthProviderHandler = auth.GetOauthProviderHandlerFunc(handler.AuthGetOauthProvider)
		api.AuthGetOauthProviderCallbackHandler = auth.GetOauthProviderCallbackHandlerFunc(handler.AuthGetOauthProviderCallback)
	}

	api.PreServerShutdown = func() {}
	api.ServerShutdown = func() {}

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
	// User service is disabled, so we need to set a valid token for the Authorization header.
	if !opts.Opts.UserServiceEnable {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set a any token for the Authorization header.
			if r.Header.Get("Authorization") == "" {
				r.Header.Set("Authorization", "valid")
			}
			handler.ServeHTTP(w, r)
		})
	}
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
