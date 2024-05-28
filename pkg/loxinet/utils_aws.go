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
	nl "github.com/vishvananda/netlink"
)

var (
	imdsClient *imds.Client
	ec2Client  *ec2.Client
	vpcID      string
	instanceID string
	azName     string
	awsCIDRnet *net.IPNet
	loxiEniID  string
)

func AWSGetInstanceIDInfo(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
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

func AWSGetInstanceAvailabilityZone(ctx context.Context) (string, error) {
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
	if awsCIDRnet == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*30))
	defer cancel()

	subnets := []string{}
	filterStr := "tag:loxiType"
	output, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{Name: &filterStr, Values: []string{"loxilb-eni"}},
		},
	})
	if err != nil {
		tk.LogIt(tk.LogError, "no loxiType intf found\n")
	} else {
		for _, intf := range output.NetworkInterfaces {
			subnets = append(subnets, *intf.SubnetId)
			if intf.Attachment != nil {
				force := true
				_, err := ec2Client.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{AttachmentId: intf.Attachment.AttachmentId, Force: &force})
				if err != nil {
					tk.LogIt(tk.LogError, "failed to detach intf (%s):%s\n", *intf.NetworkInterfaceId, err)
					return err
				}
			}
			loop := 20
			for loop > 0 {
				ctx2, cancel2 := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
				_, err2 := ec2Client.DeleteNetworkInterface(ctx2, &ec2.DeleteNetworkInterfaceInput{NetworkInterfaceId: intf.NetworkInterfaceId})
				cancel2()
				if err2 != nil {
					tk.LogIt(tk.LogError, "failed to delete intf (%s):%s\n", *intf.NetworkInterfaceId, err2)
					time.Sleep(2 * time.Second)
					loop--
					if loop <= 0 {
						return err2
					}
					continue
				}
				break
			}
		}
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Duration(time.Second*30))
	defer cancel3()
	for _, subnet := range subnets {
		_, err := ec2Client.DeleteSubnet(ctx3, &ec2.DeleteSubnetInput{SubnetId: &subnet})
		if err != nil {
			tk.LogIt(tk.LogError, "failed to delete subnet (%s):%s\n", subnet, err)
			return err
		}
	}

	ointfs, err := net.Interfaces()
	if err != nil {
		tk.LogIt(tk.LogError, "failed to get sys ifs\n")
		return err
	}

	cidrBlock := awsCIDRnet.String()
	loxilbSubNetKey := "loxiType"
	loxilbSubNetKeyVal := "loxilb-subnet"
	subnetTag := types.Tag{Key: &loxilbSubNetKey, Value: &loxilbSubNetKeyVal}
	subnetTags := []types.Tag{subnetTag}
	subOutput, err := ec2Client.CreateSubnet(ctx3, &ec2.CreateSubnetInput{
		VpcId:            &vpcID,
		AvailabilityZone: &azName,
		CidrBlock:        &cidrBlock,
		TagSpecifications: []types.TagSpecification{{ResourceType: types.ResourceTypeSubnet,
			Tags: subnetTags},
		},
	})
	if err != nil {
		tk.LogIt(tk.LogError, "failed to create subnet for loxilb instance %v:%s\n", vpcID, err)
		return nil
	}

	intfDesc := "loxilb-eni"
	loxilbIntfKey := "loxiType"
	loxilbIntfKeyVal := "loxilb-eni"
	intfTag := types.Tag{Key: &loxilbIntfKey, Value: &loxilbIntfKeyVal}
	intfTags := []types.Tag{intfTag}
	intfOutput, err := ec2Client.CreateNetworkInterface(ctx3, &ec2.CreateNetworkInterfaceInput{
		SubnetId:    subOutput.Subnet.SubnetId,
		Description: &intfDesc,
		TagSpecifications: []types.TagSpecification{{ResourceType: types.ResourceTypeNetworkInterface,
			Tags: intfTags},
		},
	})
	if err != nil {
		tk.LogIt(tk.LogError, "failed to create interface for loxilb instance %v:%s\n", vpcID, err)
		return nil
	}

	loxiEniPrivIP := *intfOutput.NetworkInterface.PrivateIpAddress
	loxiEniID = *intfOutput.NetworkInterface.NetworkInterfaceId

	tk.LogIt(tk.LogInfo, "Created interface (%s:%s) for loxilb instance %v\n", *intfOutput.NetworkInterface.NetworkInterfaceId, loxiEniPrivIP, vpcID)

	devIdx := int32(1)
	aniOut, err := ec2Client.AttachNetworkInterface(ctx3, &ec2.AttachNetworkInterfaceInput{DeviceIndex: &devIdx,
		InstanceId:         &instanceID,
		NetworkInterfaceId: intfOutput.NetworkInterface.NetworkInterfaceId,
	})
	if err != nil {
		tk.LogIt(tk.LogError, "failed to attach interface for loxilb instance %v:%s\n", vpcID, err)
		return nil
	}

	tk.LogIt(tk.LogInfo, "Attached interface (%d) for loxilb instance %v\n", *aniOut.NetworkCardIndex, vpcID)

	tryCount := 0
	newIntfName := ""

retry:
	nintfs, _ := net.Interfaces()
	if err != nil {
		tk.LogIt(tk.LogError, "failed to get sys ifs\n")
		return err
	}

	for _, nintf := range nintfs {
		found := false
		for _, ointf := range ointfs {
			if nintf.Name == ointf.Name {
				found = true
				break
			}
		}
		if !found {
			tk.LogIt(tk.LogInfo, "aws: new interface config %s\n", nintf.Name)
			link, err := nl.LinkByName(nintf.Name)
			if err != nil {
				tk.LogIt(tk.LogError, "failed to get link (%s)\n", nintf.Name)
			}
			err = nl.LinkSetUp(link)
			if err != nil {
				tk.LogIt(tk.LogError, "failed to set link (%s) up :%s\n", nintf.Name, err)
			}

			err = nl.LinkSetMTU(link, 9000)
			if err != nil {
				tk.LogIt(tk.LogError, "failed to set link (%s) mtu:%s\n", nintf.Name, err)
			}

			Address, err := nl.ParseAddr(loxiEniPrivIP + "/32")
			if err != nil {
				tk.LogIt(tk.LogWarning, "privIP  %s parse fail\n", loxiEniPrivIP)
				return err
			}
			err = nl.AddrAdd(link, Address)
			if err != nil {
				tk.LogIt(tk.LogWarning, "privIP %s:%s add failed\n", loxiEniPrivIP, nintf.Name)
			}
			newIntfName = nintf.Name
		}
	}
	if newIntfName == "" {
		if tryCount < 10 {
			time.Sleep(1 * time.Second)
			tryCount++
			goto retry
		}
	}

	return nil
}

func AWSGetNetworkInterface(ctx context.Context, instanceID string, vIP net.IP) (string, error) {
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

func AWSCreatePrivateIp(ctx context.Context, ni string, vIP net.IP) error {
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

func AWSDeletePrivateIp(ctx context.Context, ni string, vIP net.IP) error {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*30))
	defer cancel()

	var niID string
	var err error
	if awsCIDRnet == nil || loxiEniID == "" {
		niID, err = AWSGetNetworkInterface(ctx, instanceID, vIP)
		if err != nil {
			tk.LogIt(tk.LogError, "AWS get network interface failed: %v\n", err)
			return err
		}
	} else {
		niID = loxiEniID
	}

	if !add {
		return AWSDeletePrivateIp(ctx, niID, vIP)
	}

	return AWSCreatePrivateIp(ctx, niID, vIP)
}

func AWSAssociateElasticIp(vIP, eIP net.IP, add bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	var niID string
	var err error
	if awsCIDRnet == nil || loxiEniID == "" {
		niID, err = AWSGetNetworkInterface(ctx, instanceID, vIP)
		if err != nil {
			tk.LogIt(tk.LogError, "AWS get network interface failed: %v\n", err)
			return err
		}
	} else {
		niID = loxiEniID
	}

	eipID, eipAssociateID, err := AWSGetElasticIpId(ctx, eIP)
	if err != nil {
		tk.LogIt(tk.LogError, "AWS get elastic IP failed: %v\n", err)
		return err
	}
	if !add {
		return AWSDisassociateElasticIpWithInterface(ctx, eipAssociateID, niID)
	}
	return AWSAssociateElasticIpWithInterface(ctx, eipID, niID, vIP)
}

func AWSAssociateElasticIpWithInterface(ctx context.Context, eipID, niID string, privateIP net.IP) error {
	allowReassign := true
	input := &ec2.AssociateAddressInput{
		AllocationId:       &eipID,
		NetworkInterfaceId: &niID,
		AllowReassociation: &allowReassign,
	}
	if privateIP != nil {
		if err := AWSCreatePrivateIp(ctx, niID, privateIP); err != nil {
			return err
		}
		ipstr := privateIP.String()
		input.PrivateIpAddress = &ipstr
	}
	_, err := ec2Client.AssociateAddress(ctx, input)
	return err
}

func AWSDisassociateElasticIpWithInterface(ctx context.Context, eipAssociateID, niID string) error {
	_, err := ec2Client.DisassociateAddress(ctx, &ec2.DisassociateAddressInput{
		AssociationId: &eipAssociateID,
	})
	return err
}

func AWSGetElasticIpId(ctx context.Context, eIP net.IP) (string, string, error) {
	filterStr := "public-ip"
	output, err := ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{
		Filters: []types.Filter{
			{Name: &filterStr, Values: []string{eIP.String()}},
		}},
	)
	if err != nil {
		return "", "", err
	}
	if len(output.Addresses) <= 0 {
		return "", "", fmt.Errorf("not found Elastic IP %s", eIP.String())
	}
	var allocateId, associateId string
	if output.Addresses[0].AllocationId != nil {
		allocateId = *output.Addresses[0].AllocationId
	}
	if output.Addresses[0].AssociationId != nil {
		associateId = *output.Addresses[0].AssociationId
	}
	return allocateId, associateId, nil
}

func AWSApiInit(cloudCIDRBlock string) error {

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	if cloudCIDRBlock != "" {
		_, awsCIDRnet, err = net.ParseCIDR(cloudCIDRBlock)
		if err != nil {
			tk.LogIt(tk.LogError, "failed to parse cloud cidr block %s\n", cloudCIDRBlock)
			return err
		}
	}

	// Using the Config value, create the DynamoDB client
	imdsClient = imds.NewFromConfig(cfg)
	ec2Client = ec2.NewFromConfig(cfg)

	vpcID, err = AWSGetInstanceVPCInfo()
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find vpcid for instance\n")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*10))
	defer cancel()

	azName, err = AWSGetInstanceAvailabilityZone(ctx)
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find az for instance %v:%s\n", vpcID, err)
		return nil
	}

	instanceID, err = AWSGetInstanceIDInfo(ctx)
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find instanceID for instance %v:%s\n", vpcID, err)
		return nil
	}

	tk.LogIt(tk.LogInfo, "AWS API init - instance %s vpc %s az %s\n", instanceID, vpcID, instanceID)
	return nil
}

func AWSPrivateIpMapper(vip net.IP) (net.IP, error) {
	return vip, nil
}
