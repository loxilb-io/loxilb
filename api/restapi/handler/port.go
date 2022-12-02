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
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigGetPort(params operations.GetConfigPortAllParams) middleware.Responder {
	tk.LogIt(tk.LogDebug, "[API] Port %s API called. url : %s\n", params.HTTPRequest.Method, params.HTTPRequest.URL)

	// Get Port informations
	ports, err := ApiHooks.NetPortGet()
	if err != nil {
		tk.LogIt(tk.LogDebug, "[API] Error occur : %v\n", err)
		return &ResultResponse{Result: err.Error()}
	}
	var result []*models.PortEntry
	result = make([]*models.PortEntry, 0)
	for _, port := range ports {
		var tmpPort models.PortEntry
		var tmpSw models.PortEntryPortSoftwareInformation
		var tmpHw models.PortEntryPortHardwareInformation
		var tmpStat models.PortEntryPortStatisticInformation
		var tmpL2 models.PortEntryPortL2Information
		var tmpL3 models.PortEntryPortL3Information

		// Port common part
		tmpPort.DataplaneSync = int64(port.Sync)
		tmpPort.PortName = port.Name
		tmpPort.Zone = port.Zone
		tmpPort.PortNo = int64(port.PortNo)

		// SoftwareInfo
		tmpSw.OsID = int64(port.SInfo.OsID)
		tmpSw.PortType = int64(port.SInfo.PortType)
		tmpSw.PortProp = int64(port.SInfo.PortProp)
		tmpSw.PortActive = port.SInfo.PortActive
		tmpSw.BpfLoaded = port.SInfo.BpfLoaded

		// HardwareInfo
		tmpHw.Link = port.HInfo.Link
		tmpHw.Master = port.HInfo.Master
		tmpHw.Mtu = int64(port.HInfo.Mtu)
		tmpHw.Real = port.HInfo.Real
		tmpHw.State = port.HInfo.State
		tmpHw.TunnelID = int64(port.HInfo.TunID)
		tmpHw.MacAddress = port.HInfo.MacAddrStr
		tmpHw.RawMacAddress = make([]int64, 6)
		for i, bt := range port.HInfo.MacAddr {
			tmpHw.RawMacAddress[i] = int64(bt)
		}

		// Statistics
		tmpStat.RxBytes = int64(port.Stats.RxBytes)
		tmpStat.RxErrors = int64(port.Stats.RxError)
		tmpStat.RxPackets = int64(port.Stats.RxPackets)
		tmpStat.TxBytes = int64(port.Stats.TxBytes)
		tmpStat.TxErrors = int64(port.Stats.TxError)
		tmpStat.TxPackets = int64(port.Stats.TxPackets)

		// L3 info
		tmpL3.IPV4Address = port.L3.Ipv4Addrs
		tmpL3.IPV6Address = port.L3.Ipv6Addrs
		tmpL3.Routed = port.L3.Routed

		// L2 info
		tmpL2.IsPvid = port.L2.IsPvid
		tmpL2.Vid = int64(port.L2.Vid)

		tmpPort.PortSoftwareInformation = &tmpSw
		tmpPort.PortHardwareInformation = &tmpHw
		tmpPort.PortStatisticInformation = &tmpStat
		tmpPort.PortL2Information = &tmpL2
		tmpPort.PortL3Information = &tmpL3

		result = append(result, &tmpPort)
	}
	return operations.NewGetConfigPortAllOK().WithPayload(&operations.GetConfigPortAllOKBody{PortAttr: result})
}
