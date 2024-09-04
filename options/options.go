package options

import (
	"github.com/jessevdk/go-flags"
)

var Opts struct {
	Bgp               bool           `short:"b" long:"bgp" description:"Connect and Sync with GoBGP server"`
	Ka                string         `short:"k" long:"ka" description:"KeepAlive/BFD RemoteIP:SourceIP:Interval" default:"none"`
	Version           bool           `short:"v" long:"version" description:"Show loxilb version"`
	NoAPI             bool           `short:"a" long:"api" description:"Run Rest API server"`
	NoNlp             bool           `short:"n" long:"nonlp" description:"Do not register with nlp"`
	Host              string         `long:"host" description:"the IP to listen on" default:"0.0.0.0" env:"HOST"`
	Port              int            `long:"port" description:"the port to listen on for insecure connections" default:"11111" env:"PORT"`
	TLS               bool           `long:"tls" description:"enable TLS " env:"TLS"`
	TLSHost           string         `long:"tls-host" description:"the IP to listen on for tls, when not specified it's the same as --host" env:"TLS_HOST"`
	TLSPort           int            `long:"tls-port" description:"the port to listen on for secure connections" default:"8091" env:"TLS_PORT"`
	TLSCertificate    flags.Filename `long:"tls-certificate" description:"the certificate to use for secure connections" default:"/opt/loxilb/cert/server.crt" env:"TLS_CERTIFICATE"`
	TLSCertificateKey flags.Filename `long:"tls-key" description:"the private key to use for secure connections" default:"/opt/loxilb/cert/server.key" env:"TLS_PRIVATE_KEY"`
	ClusterNodes      string         `long:"cluster" description:"Comma-separated list of cluter-node IP Addresses" default:"none"`
	ClusterSelf       int            `long:"self" description:"annonation of self in cluster" default:"0"`
	LogLevel          string         `long:"loglevel" description:"One of debug,info,error,warning,notice,critical,emergency,alert" default:"debug"`
	CPUProfile        string         `long:"cpuprofile" description:"Enable cpu profiling and specify file to use" default:"none" env:"CPUPROF"`
	Prometheus        bool           `short:"p" long:"prometheus" description:"Run prometheus thread"`
	CSumDisable       bool           `long:"disable-csum" description:"Disable checksum update(experimental)"`
	PassiveEPProbe    bool           `long:"passive-probe" description:"Enable passive liveness probes(experimental)"`
	RssEnable         bool           `long:"rss-enable" description:"Enable rss optimization(experimental)"`
	EgrHooks          bool           `long:"egr-hooks" description:"Enable eBPF egress hooks(experimental)"`
	BgpPeerMode       bool           `short:"r" long:"peer" description:"Run loxilb with goBGP only, no Datapath"`
	BlackList         string         `long:"blacklist" description:"Regex string of blacklisted ports" default:"none"`
	RPC               string         `long:"rpc" description:"RPC mode for syncing - netrpc or grpc" default:"netrpc"`
	K8sAPI            string         `long:"k8s-api" description:"Enable k8s watcher(experimental)" default:"none"`
	IPVSCompat        bool           `long:"ipvs-compat" description:"Enable ipvs-compat(experimental)"`
	FallBack          bool           `long:"fallback" description:"Fallback to system default networking(experimental)"`
	LocalSockPolicy   bool           `long:"localsockpolicy" description:"support local socket policies (experimental)"`
	SockMapSupport    bool           `long:"sockmapsupport" description:"Support sockmap based L4 proxying (experimental)"`
	Cloud             string         `long:"cloud" description:"cloud type if any e.g aws,ncloud" default:"on-prem"`
	CloudCIDRBlock    string         `long:"cloudcidrblock" description:"cloud implementations need VIP cidr blocks(experimental)"`
	CloudInstance     string         `long:"cloudinstance" description:"instance-name to distinguish instance sets running in a same cloud-region"`
	ConfigPath        string         `long:"config-path" description:"Config file path" default:"/etc/loxilb/"`
	ProxyModeOnly     bool           `long:"proxyonlymode" description:"Run loxilb in proxy mode only, no Datapath"`
	WhiteList         string         `long:"whitelist" description:"Regex string of whitelisted interface(experimental)" default:"none"`
}
