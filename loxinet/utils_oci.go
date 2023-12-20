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

package loxinet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/example/helpers"
	"io"
	"net"
	"net/http"
)

type ociInterfaces struct {
	PrivateIp       string `json:"privateIP"`
	SubnetCidrBlock string `json:"subnetCidrBlock"`
	VirtualRouterIp string `json:"virtualRouterIp"`
	VlanTag         int    `json:"vlanTag"`
	VnicId          string `json:"vnicId"`
}

func OCICreatePrivateIp(vnStr string, vIP net.IP) error {

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)

	displayName := fmt.Sprintf("loxilb-%s", vIP.String())
	req := core.CreatePrivateIpRequest{CreatePrivateIpDetails: core.CreatePrivateIpDetails{VnicId: common.String(vnStr),
		DisplayName: common.String(displayName),
		IpAddress:   common.String(vIP.String())}}

	_, err = client.CreatePrivateIp(context.Background(), req)
	helpers.FatalIfError(err)

	return err
}

func OCIGetPrivateIpID(vnStr string, vIP net.IP) (string, error) {

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)

	req := core.ListPrivateIpsRequest{VnicId: common.String(vnStr)}

	resp, err := client.ListPrivateIps(context.Background(), req)
	helpers.FatalIfError(err)

	displayName := fmt.Sprintf("loxilb-%s", vIP.String())

	for _, item := range resp.Items {
		if *item.DisplayName == displayName {
			return *item.Id, nil
		}
	}

	return "", errors.New("ipID not found")
}

func OCIDeletePrivateIp(ipIDStr string) error {

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)

	req := core.DeletePrivateIpRequest{PrivateIpId: common.String(ipIDStr)}

	resp, err := client.DeletePrivateIp(context.Background(), req)
	helpers.FatalIfError(err)

	fmt.Println(resp)
	return err
}

func getOCIInterfaces(vIP net.IP) (string, error) {
	var ociIfs []ociInterfaces
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/opc/v2/vnics/", nil)
	if err != nil {
		return "", errors.New("oci-api failure")
	}
	req.Header.Set("Authorization", "Bearer Oracle")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("oci-api call failure")
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("oci-api no body")
	}
	err = json.Unmarshal(bodyText, &ociIfs)
	if err != nil {
		return "", errors.New("oci-api json parse failure")
	}

	for _, ociIF := range ociIfs {
		_, ipn, err := net.ParseCIDR(ociIF.SubnetCidrBlock)
		if err != nil {
			continue
		}
		if ipn.Contains(vIP) {
			return ociIF.VnicId, nil
		}
	}

	return "", errors.New("oci-api no such interface")
}

func OCIUpdatePrivateIp(vIP net.IP, add bool) error {
	vnStr, err := getOCIInterfaces(vIP)
	if err != nil {
		return err
	}

	ipID, err := OCIGetPrivateIpID(vnStr, vIP)
	if err == nil && ipID != "" {
		err1 := OCIDeletePrivateIp(ipID)
		if !add {
			return err1
		}
	}

	if !add {
		return err
	}

	return OCICreatePrivateIp(vnStr, vIP)
}
