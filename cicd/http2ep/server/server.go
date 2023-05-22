package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	var rootCACert []byte
	var err error
	var caCertPool *x509.CertPool
	authType := tls.NoClientCert

	help := flag.Bool("help", false, "prints help")
	host := flag.String("host", "", "server name")
	port := flag.String("port", "8080", "The https port, defaults to 9000")
	cert := flag.String("cert", "", "server's certificate")
	caCert := flag.String("cacert", "", "Client's CA certificate")
	key := flag.String("key", "", "server's private key")
	strict := flag.Bool("strict", false, "true if strict client authentication is required")
	flag.Parse()

	usage := `usage:
	
	server -cert <CertFile> -key <PrivateKey> -cacert <caCert> [-port <port> -strict <true/false> -help]
	
	Options:
	-help       Prints usage
	-host       Mandatory, server's name
	-cert       Mandatory, server's certificate
	-key        Mandatory, server's private key
	-cacert     Optional, client's CA certificate if strict check is required
	-port       Optional, server's https listening port
	-strict     Optional, true if clien't strict authentication is required`
	args := os.Args[1:]
	if len(args) == 0 || *help == true {
		fmt.Println(usage)
		return
	}

	if *strict == true {
		rootCACert, err = ioutil.ReadFile(*caCert)
		if err != nil {
			log.Fatal("Error loading CA cert : ", err)
		}
		caCertPool = x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(rootCACert)
		authType = tls.RequireAndVerifyClientCert
	}

	server := &http.Server{
		Addr: ":" + *port,
		TLSConfig: &tls.Config{
			ClientAuth: authType,
			ClientCAs:  caCertPool,
		},
	}

	http.HandleFunc("/health", func(res http.ResponseWriter, req *http.Request) {
		//log.Printf("Received %s request for host %s from IP address %s",
		//req.Method, req.Host, req.RemoteAddr)
		resp := fmt.Sprintf("OK")
		res.Write([]byte(resp))
		//log.Printf("OK\n")
	})
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		//log.Printf("Received %s request for host %s from IP address %s",
		//req.Method, req.Host, req.RemoteAddr)
		resp := fmt.Sprintf("%s:%s", req.Proto, *host)
		res.Write([]byte(resp))
		//log.Printf("OK\n")
	})
	server.ListenAndServeTLS(*cert, *key)
}
