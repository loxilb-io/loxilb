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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	utils "github.com/loxilb-io/loxilb/pkg/utils"
	tk "github.com/loxilb-io/loxilib"
	nl "github.com/vishvananda/netlink"
)

var (
	imdsClient  *imds.Client
	ec2Client   *ec2.Client
	vpcID       string
	instanceID  string
	azName      string
	awsCIDRnet  *net.IPNet
	loxiEniID   string
	intfENIName string
	setDFLRoute bool
)

func AWSGetInstanceIDInfo(ctx context.Context) (string, error) {
	resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "instance-id",
	})
	if err != nil {
		return "", err
	}
	defer resp.Content.Close()

	instanceID, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}

	return string(instanceID), nil
}

func AWSGetInstanceVPCInfo(ctx context.Context) (string, error) {
	resp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "mac",
	})
	if err != nil {
		return "", err
	}
	defer resp.Content.Close()

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
	defer resp2.Content.Close()

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
	defer resp.Content.Close()

	az, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", err
	}

	return string(az), nil
}

func AWSPrepDFLRoute() error {

	if !setDFLRoute {
		return nil
	}

	if intfENIName == "" {
		tk.LogIt(tk.LogError, "failed to get ENI intf name (%s)\n", intfENIName)
		return nil
	}

	_, defaultDst, _ := net.ParseCIDR("0.0.0.0/0")
	gw := awsCIDRnet.IP.Mask(awsCIDRnet.Mask)
	gw[3]++

	if false {
		link, err := nl.LinkByName(intfENIName)
		if err != nil {
			tk.LogIt(tk.LogError, "failed to get ENI link (%s)\n", intfENIName)
			return err
		}

		nl.RouteDel(&nl.Route{
			Dst: defaultDst,
		})
		err = nl.RouteAdd(&nl.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        gw,
			Dst:       defaultDst,
		})
		if err != nil {
			tk.LogIt(tk.LogError, "failed to set default gw %s\n", gw.String())
			return err
		}
	} else {
		link, err := nl.LinkByName(intfENIName)
		if err != nil {
			tk.LogIt(tk.LogError, "failed to get ENI link (%s)\n", intfENIName)
			return err
		}

		mh.zr.Rt.RtDelete(*defaultDst, RootZone)

		ra := RtAttr{HostRoute: false, Ifi: link.Attrs().Index, IfRoute: false}
		na := []RtNhAttr{{gw, link.Attrs().Index}}
		_, err = mh.zr.Rt.RtAdd(*defaultDst, RootZone, ra, na)
		if err != nil {
			tk.LogIt(tk.LogError, "failed to set loxidefault gw %s\n", gw.String())
			return err
		}
		utils.ArpResolver(tk.IPtonl(gw))
	}
	setDFLRoute = false
	return nil
}

func AWSPrepVIPNetwork() error {
	if awsCIDRnet == nil {
		return nil
	}

	setDFLRoute = true

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*30))
	defer cancel()

	subnets := []string{}
	loxilbKey := "loxiType"
	loxilbIfKeyVal := "loxilb-eni"
	loxilbSubNetKeyVal := "loxilb-subnet"
	filterStr := fmt.Sprintf("%s:%s", "tag", loxilbKey)

	output, err := ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{Name: &filterStr, Values: []string{loxilbIfKeyVal}},
		},
	})

	if err != nil || (output != nil && len(output.NetworkInterfaces) <= 0) {
		tk.LogIt(tk.LogError, "no loxiType intf found\n")
		subnetOutput, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			Filters: []types.Filter{
				{Name: &filterStr, Values: []string{loxilbSubNetKeyVal}},
			},
		})
		if err == nil {
			for _, subnet := range subnetOutput.Subnets {
				subnets = append(subnets, *subnet.SubnetId)
			}
		}
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

	cidrBlock := awsCIDRnet.String()
	vpcFilterStr := "cidr-block-association.cidr-block"
	vpcOut, err := ec2Client.DescribeVpcs(ctx3, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{Name: &vpcFilterStr, Values: []string{cidrBlock}},
		},
	})

	if err != nil {
		tk.LogIt(tk.LogError, "DescribeVpcs failed (%s)\n", err)
		return err
	}
	if len(vpcOut.Vpcs) >= 1 {
		dissAssoc := false
		for _, vpc := range vpcOut.Vpcs {
			if vpc.VpcId != nil && *vpc.VpcId != vpcID {
				for _, cbAs := range vpc.CidrBlockAssociationSet {
					if cbAs.CidrBlockState != nil && cbAs.CidrBlockState.State == types.VpcCidrBlockStateCodeAssociated &&
						cbAs.CidrBlock != nil && *cbAs.CidrBlock == cidrBlock {
						// CIDR is not in the current VPC. There should be no attached subnets/interfaces at this point
						_, err := ec2Client.DisassociateVpcCidrBlock(ctx3, &ec2.DisassociateVpcCidrBlockInput{AssociationId: cbAs.AssociationId})
						if err != nil {
							tk.LogIt(tk.LogError, "cidrBlock (%s) dissassociate failed in VPC %s:%s\n", cidrBlock, *vpcOut.Vpcs[0].VpcId, err)
							return err
						} else {
							tk.LogIt(tk.LogInfo, "cidrBlock (%s) dissassociated from VPC %s\n", cidrBlock, *vpcOut.Vpcs[0].VpcId)
						}
						dissAssoc = true
						break
					}
				}
			}
		}

		if dissAssoc {
			// Reassociate this CIDR block
			_, err := ec2Client.AssociateVpcCidrBlock(ctx,
				&ec2.AssociateVpcCidrBlockInput{VpcId: &vpcID, CidrBlock: &cidrBlock})
			if err != nil {
				tk.LogIt(tk.LogError, "cidrBlock (%s) associate failed in VPC %s:%s\n", cidrBlock, vpcID, err)
				return err
			} else {
				tk.LogIt(tk.LogError, "cidrBlock (%s) associated to VPC %s\n", cidrBlock, vpcID)
			}
		}
	}

	ointfs, err := net.Interfaces()
	if err != nil {
		tk.LogIt(tk.LogError, "failed to get sys ifs\n")
		return err
	}

	subnetTag := types.Tag{Key: &loxilbKey, Value: &loxilbSubNetKeyVal}
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

	sgList, err := AWSImdsGetSecurityGroups(ctx3)
	if err != nil {
		tk.LogIt(tk.LogWarning, "failed to get instance security groups: %s\n", err.Error())
	}
	intfDesc := "loxilb-eni"
	loxilbIntfKey := "loxiType"
	loxilbIntfKeyVal := "loxilb-eni"
	intfTag := types.Tag{Key: &loxilbIntfKey, Value: &loxilbIntfKeyVal}
	intfTags := []types.Tag{intfTag}
	intfOutput, err := ec2Client.CreateNetworkInterface(ctx3, &ec2.CreateNetworkInterfaceInput{
		SubnetId:    subOutput.Subnet.SubnetId,
		Description: &intfDesc,
		Groups:      sgList,
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

	sourceDestCheck := false
	_, err = ec2Client.ModifyNetworkInterfaceAttribute(ctx3, &ec2.ModifyNetworkInterfaceAttributeInput{
		NetworkInterfaceId: intfOutput.NetworkInterface.NetworkInterfaceId,
		SourceDestCheck:    &types.AttributeBooleanValue{Value: &sourceDestCheck},
	})
	if err != nil {
		tk.LogIt(tk.LogError, "failed to modify interface(disable source/dest check):%s\n", err.Error())
	}

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

			ones, _ := awsCIDRnet.Mask.Size()
			subStr := fmt.Sprintf("/%d", ones)
			Address, err := nl.ParseAddr(loxiEniPrivIP + subStr)
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
	} else {
		intfENIName = newIntfName
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
		cidr.Content.Close()
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
		tk.LogIt(tk.LogError, "failed to load cloud config\n")
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	vpcID, err = AWSGetInstanceVPCInfo(ctx)
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find vpcid for instance\n")
		return err
	}

	azName, err = AWSGetInstanceAvailabilityZone(ctx)
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find az for instance %v:%s\n", vpcID, err)
		return err
	}

	instanceID, err = AWSGetInstanceIDInfo(ctx)
	if err != nil {
		tk.LogIt(tk.LogError, "failed to find instanceID for instance %v:%s\n", vpcID, err)
		return err
	}

	tk.LogIt(tk.LogInfo, "AWS API init - instance %s vpc %s az %s\n", instanceID, vpcID, instanceID)
	return nil
}

func AWSImdsGetSecurityGroups(ctx context.Context) ([]string, error) {
	macResp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: "mac",
	})
	if err != nil {
		return nil, err
	}
	defer macResp.Content.Close()

	macByte, err := io.ReadAll(macResp.Content)
	if err != nil {
		return nil, err
	}

	sgResp, err := imdsClient.GetMetadata(ctx, &imds.GetMetadataInput{
		Path: fmt.Sprintf("network/interfaces/macs/%s/security-group-ids", string(macByte)),
	})
	if err != nil {
		return nil, err
	}
	defer sgResp.Content.Close()

	sgByte, err := io.ReadAll(sgResp.Content)
	if err != nil {
		return nil, err
	}

	sgList := strings.Split(string(sgByte), "\n")
	return sgList, nil
}
