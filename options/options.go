package options

import (
	"fmt"
	"strings"

	"github.com/jessevdk/go-flags"
)

var Opts struct {
	Bgp                  bool           `short:"b" long:"bgp" description:"Connect and Sync with GoBGP server"`
	Ka                   string         `short:"k" long:"ka" description:"KeepAlive/BFD RemoteIP:SourceIP:Interval" default:"none"`
	Version              bool           `short:"v" long:"version" description:"Show loxilb version"`
	NoAPI                bool           `short:"a" long:"api" description:"Run Rest API server"`
	NoNlp                bool           `short:"n" long:"nonlp" description:"Do not register with nlp"`
	Host                 string         `long:"host" description:"the IP to listen on" default:"0.0.0.0" env:"HOST"`
	Port                 int            `long:"port" description:"the port to listen on for insecure connections" default:"11111" env:"PORT"`
	TLS                  bool           `long:"tls" description:"enable TLS " env:"TLS"`
	TLSHost              string         `long:"tls-host" description:"the IP to listen on for tls" default:"0.0.0.0" env:"TLS_HOST"`
	TLSPort              int            `long:"tls-port" description:"the port to listen on for secure connections" default:"8091" env:"TLS_PORT"`
	TLSCertificate       flags.Filename `long:"tls-certificate" description:"the certificate to use for secure connections" default:"/opt/loxilb/cert/server.crt" env:"TLS_CERTIFICATE"`
	TLSCertificateKey    flags.Filename `long:"tls-key" description:"the private key to use for secure connections" default:"/opt/loxilb/cert/server.key" env:"TLS_PRIVATE_KEY"`
	ClusterNodes         string         `long:"cluster" description:"Comma-separated list of cluter-node IP Addresses" default:"none"`
	ClusterSelf          int            `long:"self" description:"annonation of self in cluster" default:"0"`
	LogLevel             string         `long:"loglevel" description:"One of trace,debug,info,error,warning,notice,critical,emergency,alert" default:"debug"`
	CPUProfile           string         `long:"cpuprofile" description:"Enable cpu profiling and specify file to use" default:"none" env:"CPUPROF"`
	Prometheus           bool           `short:"p" long:"prometheus" description:"Run prometheus thread"`
	CRC32SumDisable      bool           `long:"disable-crc32" description:"Disable crc32 checksum update(experimental)"`
	PassiveEPProbe       bool           `long:"passive-probe" description:"Enable passive liveness probes(experimental)"`
	RssEnable            bool           `long:"rss-enable" description:"Enable rss optimization(experimental)"`
	EgrHooks             bool           `long:"egr-hooks" description:"Enable eBPF egress hooks(experimental)"`
	BgpPeerMode          bool           `short:"r" long:"peer" description:"Run loxilb with goBGP only, no Datapath"`
	BlackList            string         `long:"blacklist" description:"Regex string of blacklisted ports" default:"none"`
	RPC                  string         `long:"rpc" description:"RPC mode for syncing - netrpc or grpc" default:"netrpc"`
	K8sAPI               string         `long:"k8s-api" description:"Enable k8s watcher(experimental)" default:"none"`
	IPVSCompat           bool           `long:"ipvs-compat" description:"Enable ipvs-compat(experimental)"`
	FallBack             bool           `long:"fallback" description:"Fallback to system default networking(experimental)"`
	LocalSockPolicy      bool           `long:"localsockpolicy" description:"support local socket policies (experimental)"`
	SockMapSupport       bool           `long:"sockmapsupport" description:"Support sockmap based L4 proxying (experimental)"`
	Cloud                string         `long:"cloud" description:"cloud type if any e.g aws,ncloud" default:"on-prem"`
	CloudCIDRBlock       string         `long:"cloudcidrblock" description:"cloud implementations need VIP cidr blocks(experimental)"`
	CloudInstance        string         `long:"cloudinstance" description:"instance-name to distinguish instance sets running in a same cloud-region"`
	ConfigPath           string         `long:"config-path" description:"Config file path" default:"/etc/loxilb/"`
	ProxyModeOnly        bool           `long:"proxyonlymode" description:"Run loxilb in proxy mode only, no Datapath"`
	WhiteList            string         `long:"whitelist" description:"Regex string of whitelisted interface(experimental)" default:"none"`
	ClusterInterface     string         `long:"clusterinterface" description:"cluster interface for egress HA" default:""`
	UserServiceEnable    bool           `long:"userservice" description:"Enable user service for loxilb"`
	DatabaseHost         string         `long:"databasehost" description:"Database host" default:"127.0.0.1"`
	DatabasePort         int            `long:"databaseport" description:"Database port" default:"3306"`
	DatabaseUser         string         `long:"databaseuser" description:"Database user" default:"root"`
	DatabasePasswordPath string         `long:"databasepasswordpath" description:"Database password" default:"/etc/loxilb/mysql_password"`
	DatabaseName         string         `long:"databasename" description:"Database name" default:"loxilb_db"`

	// Oauth2 Options as input arguemtns
	Oauth2Enable   bool   `long:"oauth2" description:"Enable user oauth2 service for loxilb"`
	Oauth2Provider string `long:"oauth2provider" description:"Oauth2 provider names, comma-separated" default:"google"`

	// Oauth2 secure informations
	Oauth2GoogleClientID     string `long:"oauth2google-clientid" description:"Oauth2 google client id" env:"OAUTH2_GOOGLE_CLIENT_ID"`
	Oauth2GoogleClientSecret string `long:"oauth2google-clientsecret" description:"Oauth2 google client secret" env:"OAUTH2_GOOGLE_CLIENT_SECRET"`
	Oauth2GoogleRedirectURL  string `long:"oauth2google-redirecturl" description:"Oauth2 google redirect url" env:"OAUTH2_GOOGLE_REDIRECT_URL"`
	Oauth2GithubClientID     string `long:"oauth2github-clientid" description:"Oauth2 github client id" env:"OAUTH2_GITHUB_CLIENT_ID"`
	Oauth2GithubClientSecret string `long:"oauth2github-clientsecret" description:"Oauth2 github client secret" env:"OAUTH2_GITHUB_CLIENT_SECRET"`
	Oauth2GithubRedirectURL  string `long:"oauth2github-redirecturl" description:"Oauth2 github redirect url" env:"OAUTH2_GITHUB_REDIRECT_URL"`
}

// ValidateOpts checks if the required environment variables are set when Oauth2Enable is true
func ValidateOpts() error {
	// Check if Oauth2Enable is true
	if Opts.Oauth2Enable {
		// Split the Oauth2Provider string into a slice of providers
		providers := strings.Split(Opts.Oauth2Provider, ",")

		// Iterate over each provider and validate the required environment variables
		for _, provider := range providers {
			switch provider {
			case "google":
				if Opts.Oauth2GoogleClientID == "" || Opts.Oauth2GoogleClientSecret == "" || Opts.Oauth2GoogleRedirectURL == "" {
					return fmt.Errorf("Oauth2 google client id, client secret and redirect url are required but not set")
				}
			case "github":
				if Opts.Oauth2GithubClientID == "" || Opts.Oauth2GithubClientSecret == "" || Opts.Oauth2GithubRedirectURL == "" {
					return fmt.Errorf("Oauth2 github client id, client secret and redirect url are required but not set")
				}
			default:
				return fmt.Errorf("unsupported oauth2 provider: %s", provider)
			}
		}
	}
	return nil
}
