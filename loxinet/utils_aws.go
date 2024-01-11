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
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tk "github.com/loxilb-io/loxilib"
)

var (
	imdsClient *imds.Client
	ec2Client  *ec2.Client
)

func AWSGetInstanceIDInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "instance-id",
	})
	if err != nil {
		return "", err
	}

	instanceID, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}

	return string(instanceID), nil
}

func AWSGetNetworkInterface(instanceID string, vIP net.IP) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*5))
	defer cancel()

	filterStr := "attachment.instance-id"
	output, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{Name: &filterStr, Values: []string{instanceID}},
		},
	})
	if err != nil {
		return "", err
	}

	for _, i := range output.NetworkInterfaces {
		path := fmt.Sprintf("network/interfaces/macs/%s/subnet-ipv4-cidr-block", *i.MacAddress)
		cidr, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
			Path: path,
		})
		if err != nil {
			continue
		}

		b, err := io.ReadAll(cidr.Content)
		if err != nil {
			continue
		}

		_, ips, err := net.ParseCIDR(string(b))
		if err != nil {
			continue
		}

		if ips.Contains(vIP) {
			if i.NetworkInterfaceId != nil {
				return *i.NetworkInterfaceId, nil
			}
		}
	}

	return "", errors.New("not found interface")
}

func AWSCreatePrivateIp(ni string, vIP net.IP) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()

	input := &ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId: &ni,
		PrivateIpAddresses: []string{vIP.String()},
	}
	_, err := ec2Client.AssignPrivateIpAddresses(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func AWSDeletePrivateIp(ni string, vIP net.IP) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()

	input := &ec2.UnassignPrivateIpAddressesInput{
		NetworkInterfaceId: &ni,
		PrivateIpAddresses: []string{vIP.String()},
	}
	_, err := ec2Client.UnassignPrivateIpAddresses(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

func AWSUpdatePrivateIp(vIP net.IP, add bool) error {
	instanceID, err := AWSGetInstanceIDInfo()
	if err != nil {
		tk.LogIt(tk.LogError, "AWS get instance failed: %v\n", err)
		return err
	}

	niID, err := AWSGetNetworkInterface(instanceID, vIP)
	if err != nil {
		tk.LogIt(tk.LogError, "AWS get network interface failed: %v\n", err)
		return err
	}

	if !add {
		return AWSDeletePrivateIp(niID, vIP)
	}

	return AWSCreatePrivateIp(niID, vIP)
}

func AWSApiInit() error {
	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// Using the Config value, create the DynamoDB client
	imdsClient = imds.NewFromConfig(cfg)
	ec2Client = ec2.NewFromConfig(cfg)

	tk.LogIt(tk.LogInfo, "AWS API init\n")
	return nil
}
