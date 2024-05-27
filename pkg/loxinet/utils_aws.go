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

func AWSGetInstanceVPCInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "mac",
	})
	if err != nil {
		return "", err
	}

	mac, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}

	vpcPath := fmt.Sprintf("network/interfaces/macs/%s/vpc-id", string(mac))
	resp2, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: vpcPath,
	})
	if err != nil {
		return "", err
	}
	vpc, err := io.ReadAll(resp2.Content)
	if err != nil {
		return "", err
	}

	return string(vpc), nil
}

func AWSGetInstanceAvailabilityZone() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*2))
	defer cancel()
	resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "placement/availability-zone",
	})
	if err != nil {
		return "", err
	}

	az, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}

	return string(az), nil
}

func AWSPrepVIPNetwork() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*5))
	defer cancel()

	filterStr := "tag:loxiType"
	output, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{Name: &filterStr, Values: []string{"loxilb-eni"}},
		},
	})
	if err != nil {
		fmt.Printf("no loxiType intf found \n")
		return err
	}

	subnets := []string{}
	for _, intf := range output.NetworkInterfaces {
		subnets = append(subnets, *intf.SubnetId)
		_, err := ec2Client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: intf.NetworkInterfaceId})
		if err != nil {
			fmt.Printf("failed to delete intf (%s)\n", *intf.NetworkInterfaceId)
			return err
		}
	}

	for _, subnet := range subnets {
		_, err := ec2Client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{SubnetId: &subnet})
		if err != nil {
			fmt.Printf("failed to delete subnet (%s)\n", subnet)
			return err
		}
	}

	vpcID, err := AWSGetInstanceVPCInfo()
	if err != nil {
		fmt.Printf("failed to find vpcid for instance\n")
		return nil
	}

	fmt.Printf("vpcid for instance:%s\n", vpcID)

	azName, err := AWSGetInstanceAvailabilityZone()
	if err != nil {
		fmt.Printf("failed to find az for instance %v\n", vpcID)
		return nil
	}
	fmt.Printf("AZ for instance:%s\n", azName)

	instanceID, err := AWSGetInstanceIDInfo()
	if err != nil {
		fmt.Printf("failed to find instanceID for instance %v\n", vpcID)
		return nil
	}

	cidrBlock := "123.123.123.0/28"
	loxilbSubNetKey := "loxiType"
	loxilbSubNetKeyVal := "loxilb-subnet"
	subnetTag := types.Tag{Key: &loxilbSubNetKey, Value: &loxilbSubNetKeyVal}
	subnetTags := []types.Tag{subnetTag}
	subOutput, err := ec2Client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:            &vpcID,
		AvailabilityZone: &azName,
		CidrBlock:        &cidrBlock,
		TagSpecifications: []types.TagSpecification{{ResourceType: types.ResourceTypeSubnet,
			Tags: subnetTags},
		},
	})
	if err != nil {
		fmt.Printf("failed to create subnet for loxilb instance %v\n", vpcID)
		return nil
	}

	intfDesc := "loxilb-eni"
	loxilbIntfKey := "loxiType"
	loxilbIntfKeyVal := "loxilb-eni"
	intfTag := types.Tag{Key: &loxilbIntfKey, Value: &loxilbIntfKeyVal}
	intfTags := []types.Tag{intfTag}
	intfOutput, err := ec2Client.CreateNetworkInterface(ctx, &ec2.CreateNetworkInterfaceInput{
		SubnetId:    subOutput.Subnet.SubnetId,
		Description: &intfDesc,
		TagSpecifications: []types.TagSpecification{{ResourceType: types.ResourceTypeNetworkInterface,
			Tags: intfTags},
		},
	})
	if err != nil {
		fmt.Printf("failed to create interface for loxilb instance %v\n", vpcID)
		return nil
	}

	fmt.Printf("Create interface (%s) for loxilb instance %v\n", *intfOutput.NetworkInterface.NetworkInterfaceId, vpcID)

	devIdx := int32(1)
	ec2Client.AttachNetworkInterface(ctx, &ec2.AttachNetworkInterfaceInput{DeviceIndex: &devIdx,
		InstanceId:         &instanceID,
		NetworkInterfaceId: intfOutput.NetworkInterface.NetworkInterfaceId,
	})

	return nil
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

	allowReassign := true
	input := &ec2.AssignPrivateIpAddressesInput{
		NetworkInterfaceId: &ni,
		PrivateIpAddresses: []string{vIP.String()},
		AllowReassignment:  &allowReassign,
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

func AWSUpdatePrivateIP(vIP net.IP, add bool) error {
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

func AWSPrivateIpMapper(vip net.IP) (net.IP, error) {
	return vip, nil
}
