/*
 * Copyright (c) 2023 NetLOX Inc
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

package loxinet

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	tk "github.com/loxilb-io/loxilib"
)

var (
	nClient *NcloudClient
)

type NcloudConfig struct {
	AccessKey string
	SecretKey string
}

type NcloudClient struct {
	config    *NcloudConfig
	client    *http.Client
	serverURL string
}

func (n *NcloudClient) NcloudGetMetadataInterfaceID() (string, error) {
	metadataURL := "http://169.254.169.254"
	urls := "/latest/meta-data/networkInterfaceNoList/0"
	req, err := http.NewRequest(http.MethodGet, metadataURL+urls, nil)
	if err != nil {
		return "", err
	}

	n.setHeaders(req, urls)
	res, err := n.client.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(resBody)), nil
}

func (n *NcloudClient) NcloudCreatePrivateIp(ni string, vIP net.IP) error {
	urls := fmt.Sprintf("%s?networkInterfaceNo=%s&secondaryIpList.1=%s&allowReassign=true&responseFormatType=json", "/vserver/v2/assignSecondaryIps", ni, vIP.String())
	req, err := http.NewRequest(http.MethodGet, n.serverURL+urls, nil)
	if err != nil {
		return err
	}

	n.setHeaders(req, urls)
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	type AssignSecondaryIpsResponse struct {
		ReturnMessage string `json:"returnMessage"`
	}
	type ncloudResponse struct {
		AssignSecondaryIpsResponse AssignSecondaryIpsResponse `json:"assignSecondaryIpsResponse"`
	}

	checkReturn := ncloudResponse{}
	if err := json.Unmarshal(respBody, &checkReturn); err != nil {
		return err
	}

	if checkReturn.AssignSecondaryIpsResponse.ReturnMessage != "success" {
		return fmt.Errorf(string(respBody))
	}

	return nil
}

func (n *NcloudClient) NcloudDeletePrivateIp(ni string, vIP net.IP) error {
	urls := fmt.Sprintf("%s?networkInterfaceNo=%s&secondaryIpList.1=%s&responseFormatType=json", "/vserver/v2/unassignSecondaryIps", ni, vIP.String())
	req, err := http.NewRequest(http.MethodGet, n.serverURL+urls, nil)
	if err != nil {
		return err
	}

	n.setHeaders(req, urls)
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	type UnassignSecondaryIpsResponse struct {
		ReturnMessage string `json:"returnMessage"`
	}
	type ncloudResponse struct {
		UnassignSecondaryIpsResponse UnassignSecondaryIpsResponse `json:"unassignSecondaryIpsResponse"`
	}

	checkReturn := ncloudResponse{}
	if err := json.Unmarshal(respBody, &checkReturn); err != nil {
		return err
	}

	if checkReturn.UnassignSecondaryIpsResponse.ReturnMessage != "success" {
		return fmt.Errorf(string(respBody))
	}

	return nil
}

func (n *NcloudClient) NcloudUpdatePrivateIp(vIP net.IP, add bool) error {
	if n.checkNcloudCredential(); n.config == nil {
		return fmt.Errorf("failed to load Ncloud credential")
	}

	niID, err := n.NcloudGetMetadataInterfaceID()
	if err != nil {
		tk.LogIt(tk.LogError, "NCloud get instance failed: %v\n", err)
		return err
	}

	if !add {
		return n.NcloudDeletePrivateIp(niID, vIP)
	}

	return n.NcloudCreatePrivateIp(niID, vIP)
}

func (n *NcloudClient) getUnixMilliTimeString() string {
	currentTime := time.Now().UnixMilli()
	return strconv.FormatInt(currentTime, 10)
}

func (n *NcloudClient) createSignature(method string, urls string, timestamp string) string {
	message := fmt.Sprintf("%s %s\n%s\n%s", method, urls, timestamp, n.config.AccessKey)

	hmac256 := hmac.New(sha256.New, []byte(n.config.SecretKey))
	hmac256.Write([]byte(message))
	hmacSum := hmac256.Sum(nil)

	return base64.StdEncoding.EncodeToString(hmacSum)
}

func (n *NcloudClient) setHeaders(req *http.Request, urls string) {
	timestamp := n.getUnixMilliTimeString()
	signature := n.createSignature(http.MethodGet, urls, timestamp)
	req.Header.Set("x-ncp-apigw-timestamp", timestamp)
	req.Header.Set("x-ncp-iam-access-key", n.config.AccessKey)
	req.Header.Set("x-ncp-apigw-signature-v2", signature)
}

func (n *NcloudClient) checkNcloudCredential() {
	if n.config != nil {
		return
	}

	cfg, err := loadDefaultConfig()
	if err != nil {
		tk.LogIt(tk.LogInfo, "failed to get NCloud credential")
		return
	}
	n.config = cfg
}

func NcloudApiInit() error {
	cfg, err := loadDefaultConfig()
	if err != nil {
		tk.LogIt(tk.LogInfo, "failed to get NCloud credential. error: %s", err.Error())
	}

	// Using the Config value, create the DynamoDB client
	nClient = newFromConfig(cfg)

	tk.LogIt(tk.LogInfo, "NCloud API init\n")
	return nil
}

func newFromConfig(cfg *NcloudConfig) *NcloudClient {
	return &NcloudClient{
		config:    cfg,
		client:    &http.Client{},
		serverURL: "https://ncloud.apigw.ntruss.com",
	}
}

func loadDefaultConfig() (*NcloudConfig, error) {
	var ncloudConfig NcloudConfig

	user, err := user.Current()
	if err != nil {
		return nil, err
	}

	path := user.HomeDir + "/.ncloud/credential"
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	credentials := strings.Split(string(b), "\n")
	for _, credential := range credentials {
		keyValues := strings.Split(credential, "=")
		if strings.Contains(keyValues[0], "ncloud_access_key_id") {
			ncloudConfig.AccessKey = strings.TrimSpace(keyValues[1])
		} else if strings.Contains(keyValues[0], "ncloud_secret_access_key") {
			ncloudConfig.SecretKey = strings.TrimSpace(keyValues[1])
		}
	}
	if ncloudConfig.AccessKey == "" || ncloudConfig.SecretKey == "" {
		return nil, fmt.Errorf("failed to get access key or secret key")
	}

	return &ncloudConfig, nil
}
