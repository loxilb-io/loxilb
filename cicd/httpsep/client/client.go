package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

func main() {
	help := flag.Bool("help", false, "Prints help")
	host := flag.String("host", "localhost", "Server IP:port")
	caCertFile := flag.String("cacert", "", "Server's CA certificate")
	pcert := flag.String("cert", "", "Client's private certificate")
	pkey := flag.String("key", "", "Clients's private key")
	flag.Parse()

	usage := `usage:
	
    client -host <serverIP:port> -cacert <serverCACertFile> [-cert <privateCertFile> -key <privateKeyFile> -help]
	
Options:
  -help       Optional, Prints this message
  -host    Mandatory, the server's IP:port
  -cacert     Mandatory, Server's CA certificate
  -cert       Optional, Clients's private certificate
  -key        Optional, Client's private key
 `

	if *help == true {
		fmt.Println(usage)
		return
	}

	if *host == "" || *caCertFile == "" {
		log.Fatalf("host and cacert are mandatory:\n%s", usage)
	}

	var cert tls.Certificate
	var err error
	if *pcert != "" && *pkey != "" {
		cert, err = tls.LoadX509KeyPair(*pcert, *pkey)
		if err != nil {
			log.Fatalf("Error loading client cert file %s and client key file %s", *pcert, *pkey)
			return
		}
	}

	caCert, err := ioutil.ReadFile(*caCertFile)
	if err != nil {
		log.Fatalf("Error opening server's CACert file %s(%s)", *caCertFile, err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		},
	}

	client := http.Client{Transport: t, Timeout: 5 * time.Second}
	urlStr := "https://" + *host
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		log.Fatalf("Error creating new http request : %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		switch e := err.(type) {
		case *url.Error:
			log.Fatalf("url.Error : %s", e)
		default:
			log.Fatalf("Unexpected error : %s", err)
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		log.Fatalf("Error in reading resp: %s", err)
	}

	fmt.Printf("%s", body)
}
