package options

import (
	"github.com/jessevdk/go-flags"
)

var Opts struct {
	Bgp               bool           `short:"b" long:"bgp" description:"Connect and Sync with GoBGP server"`
	Ka                string         `short:"k" long:"ka" description:"One of in,out"`
	Version           bool           `short:"v" long:"version" description:"Show loxilb version"`
	NoApi             bool           `short:"a" long:"api" description:"Run Rest API server"`
	NoNlp             bool           `short:"n" long:"nonlp" description:"Do not register with nlp"`
	Host              string         `long:"host" description:"the IP to listen on" default:"0.0.0.0" env:"HOST"`
	Port              int            `long:"port" description:"the port to listen on for insecure connections" default:"11111" env:"PORT"`
	TLSHost           string         `long:"tls-host" description:"the IP to listen on for tls, when not specified it's the same as --host" env:"TLS_HOST"`
	TLSPort           int            `long:"tls-port" description:"the port to listen on for secure connections" default:"8091" env:"TLS_PORT"`
	TLSCertificate    flags.Filename `long:"tls-certificate" description:"the certificate to use for secure connections" default:"/opt/loxilb/cert/server.crt" env:"TLS_CERTIFICATE"`
	TLSCertificateKey flags.Filename `long:"tls-key" description:"the private key to use for secure connections" default:"/opt/loxilb/cert/server.key" env:"TLS_PRIVATE_KEY"`
	ClusterNodes      string         `long:"cluster" description:"Comma-separated list of cluter-node IP Addresses" default:"none"`
	ClusterSelf       int            `long:"self" description:"annonation of self in cluster" default:"0"`
	LogLevel          string         `long:"loglevel" description:"One of debug,info,error,warning,notice,critical,emergency,alert" default:"debug"`
}
