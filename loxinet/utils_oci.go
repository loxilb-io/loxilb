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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	tk "github.com/loxilb-io/loxilib"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/example/helpers"
	"io"
	"net"
	"net/http"
	"time"
)

type ociInterfaces struct {
	PrivateIp       string `json:"privateIP"`
	SubnetCidrBlock string `json:"subnetCidrBlock"`
	VirtualRouterIp string `json:"virtualRouterIp"`
	VlanTag         int    `json:"vlanTag"`
	VnicId          string `json:"vnicId"`
}

type ociInstInfo struct {
	CompartmentId string `json:"compartmentId"`
	Id            string `json:"id"`
}

var (
	CompartMentID string
	VcnId         string
	InstanceID    string
	Client        *core.VirtualNetworkClient
)

func OCICreatePrivateIp(vnStr string, vIP net.IP) error {

	displayName := fmt.Sprintf("loxilb-%s", vIP.String())
	req := core.CreatePrivateIpRequest{CreatePrivateIpDetails: core.CreatePrivateIpDetails{VnicId: common.String(vnStr),
		DisplayName: common.String(displayName),
		IpAddress:   common.String(vIP.String())}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	_, err := Client.CreatePrivateIp(ctx, req)
	return err
}

func OCIGetPrivateIpID(vnStr string, vIP net.IP) (string, error) {

	req := core.ListPrivateIpsRequest{VnicId: common.String(vnStr)}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	resp, err := Client.ListPrivateIps(ctx, req)
	if err != nil {
		return "", err
	}

	displayName := fmt.Sprintf("loxilb-%s", vIP.String())

	for _, item := range resp.Items {
		if *item.DisplayName == displayName {
			return *item.Id, nil
		}
	}

	return "", errors.New("ipID not found")
}

func OCIGetPrivateIpIDByIP(vIP net.IP) (string, error) {

	req := core.ListSubnetsRequest{
		//VcnId:        common.String(VcnId),
		CompartmentId: common.String(CompartMentID),
		Limit:         common.Int(100)}

	resp, err := Client.ListSubnets(context.Background(), req)
	if err != nil {
		return "", err
	}

	for _, subnet := range resp.Items {
		req1 := core.ListPrivateIpsRequest{IpAddress: common.String(vIP.String()),
			SubnetId: common.String(*subnet.Id)}
		resp1, err := Client.ListPrivateIps(context.Background(), req1)
		if err != nil {
			return "", err
		}

		displayName := fmt.Sprintf("loxilb-%s", vIP.String())

		for _, item := range resp1.Items {
			if *item.DisplayName == displayName {
				return *item.Id, nil
			}
		}
	}

	return "", errors.New("ipID by IP not found")
}

func OCIDeletePrivateIp(ipIDStr string) error {

	req := core.DeletePrivateIpRequest{PrivateIpId: common.String(ipIDStr)}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	resp, err := Client.DeletePrivateIp(ctx, req)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	req = req.WithContext(ctx)
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

func getOCIInstanceInfo() (string, string) {
	var info ociInstInfo
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://169.254.169.254/opc/v2/instance/", nil)
	if err != nil {
		return "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer Oracle")
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}
	err = json.Unmarshal(bodyText, &info)
	if err != nil {
		return "", ""
	}

	return info.CompartmentId, info.Id
}

func OCIUpdatePrivateIp(vIP net.IP, add bool) error {
	vnStr, err := getOCIInterfaces(vIP)
	if err != nil {
		return err
	}

	ipID, err := OCIGetPrivateIpIDByIP(vIP)
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

func OCIApiInit() error {
	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(common.DefaultConfigProvider())
	helpers.FatalIfError(err)

	Client = &client
	CompartMentID, InstanceID = getOCIInstanceInfo()
	if CompartMentID == "" || InstanceID == "" {
		helpers.FatalIfError(errors.New("no instance info"))
	}
	tk.LogIt(tk.LogInfo, "oci api init\n")
	return err
}
