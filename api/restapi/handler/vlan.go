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
package handler

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/loxinlp"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

func ConfigPostVLAN(params operations.PostConfigVlanParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVLANNoHook(int(params.Attr.Vid))
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVLAN(params operations.DeleteConfigVlanVlanIDParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVLANNoHook(int(params.VlanID))
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigPostVLANMember(params operations.PostConfigVlanVlanIDMemberParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.AddVLANMemberNoHook(int(params.VlanID), params.Attr.Dev, params.Attr.Tagged)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteVLANMember(params operations.DeleteConfigVlanVlanIDMemberIfNameTaggedTaggedParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] FDB %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
	ret := loxinlp.DelVLANMemberNoHook(int(params.VlanID), params.IfName, params.Tagged)
	if ret != 0 {
		return &ResultResponse{Result: "fail"}
	}
	return &ResultResponse{Result: "Success"}
}

// func ConfigGetVLAN(params operations.GetConfigFDBAllParams) middleware.Responder {
// 	tk.LogIt(tk.LogDebug, "[API] FDB   %s API callded. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)
// 	res, _ := ApiHooks.NetIpv4AddrGet()
// 	var result []*models.FDBGetEntry
// 	result = make([]*models.FDBGetEntry, 0)
// 	for _, ipaddrs := range res {
// 		var tmpResult models.FDBGetEntry
// 		tmpResult.Dev = ipaddrs.Dev
// 		helperSync := int64(ipaddrs.Sync)
// 		tmpResult.Sync = &helperSync
// 		tmpResult.IPAddress = ipaddrs.IP
// 		result = append(result, &tmpResult)
// 	}
// 	return operations.NewGetConfigFDBAllOK().WithPayload(&operations.GetConfigFDBAllOKBody{IPAttr: result})
// }

// func setupDefaultTunnelInterface(netns ns.NetNS) error {
// 	tunName := loxilb.GetVxlanInterfaceName(i.networkConfig.LoxiVxlanID)

// 	return netns.Do(func(_ ns.NetNS) error {
// 		epLink, err := netlink.LinkByName(i.networkConfig.EndpointInterface)
// 		if err != nil {
// 			return fmt.Errorf("failed to get endpoint interface %s: %v", i.networkConfig.EndpointInterface, err)
// 		}

// 		tunLink, err := netlink.LinkByName(tunName)
// 		// If tunnel interface is not found, create new tunnel interface
// 		if err != nil {
// 			klog.Infof("creating tunnel interface %s in namespace.", tunName)
// 			localIP, _, _ := net.ParseCIDR(i.nodeConfig.LocalIP)
// 			newVxlan := &netlink.Vxlan{
// 				LinkAttrs: netlink.LinkAttrs{
// 					Name: loxilb.GetVxlanInterfaceName(i.networkConfig.LoxiVxlanID),
// 					MTU:  i.networkConfig.LoxiInterfaceMTU,
// 				},
// 				SrcAddr:      localIP,
// 				Port:         4789,
// 				VxlanId:      i.networkConfig.LoxiVxlanID,
// 				VtepDevIndex: epLink.Attrs().Index,
// 			}
// 			if err := netlink.LinkAdd(newVxlan); err != nil {
// 				return err
// 			}
// 			time.Sleep(2 * time.Second)

// 			tunLink, err = netlink.LinkByName(tunName)
// 			if err != nil {
// 				return fmt.Errorf("failed to create new tunnel interface %s: %v", tunName, err)
// 			}
// 		}

// 		netlink.LinkSetUp(tunLink)

// 		addrs, err := netlink.AddrList(tunLink, netlink.FAMILY_V4)
// 		if err != nil {
// 			return err
// 		}

// 		tunIP, tunIPN, err := net.ParseCIDR(i.networkConfig.TunnelIP)
// 		if err != nil {
// 			return fmt.Errorf("failed to set tunnel interface %s to IP %s: %v", tunName, i.networkConfig.TunnelIP, err)
// 		}

// 		for _, addr := range addrs {
// 			// tunnel interface has ip address already
// 			if addr.IP.Equal(tunIP) {
// 				return nil
// 			}
// 			// If another IP is detected, it is deleted.
// 			if addr.IPNet.Contains(tunIP) || tunIPN.Contains(addr.IP) {
// 				netlink.AddrDel(tunLink, &addr)
// 			}
// 		}
// 		return netlink.AddrAdd(tunLink,
// 			&netlink.Addr{
// 				IPNet: &net.IPNet{
// 					IP:   tunIP,
// 					Mask: tunIPN.Mask,
// 				},
// 			},
// 		)
// 	})
// }
