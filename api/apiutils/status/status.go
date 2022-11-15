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

package status

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/loxilb-io/loxilb/api/models"
)

// ProcessInfoGet- Get Process Status using "top -bn 1" command
func ProcessInfoGet() []*models.ProcessInfoEntry {
	var result []*models.ProcessInfoEntry
	result = make([]*models.ProcessInfoEntry, 0)
	app := "top"

	arg0 := "-bn"
	arg1 := "1"

	cmd := exec.Command(app, arg0, arg1)
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
	}
	topResult := string(stdout)
	// Print the output
	for _, processInfos := range strings.Split(topResult, "\n")[7:] {
		tmpProcess := strings.Fields(processInfos)
		if len(tmpProcess) == 12 {
			//tmpResult := new(models.ProcessInfoEntry)
			tmpResult := &models.ProcessInfoEntry{
				Pid:          tmpProcess[0],
				User:         tmpProcess[1],
				Priority:     tmpProcess[2],
				Nice:         tmpProcess[3],
				VirtMemory:   tmpProcess[4],
				ResidentSize: tmpProcess[5],
				SharedMemory: tmpProcess[6],
				Status:       tmpProcess[7],
				CPUUsage:     tmpProcess[8],
				MemoryUsage:  tmpProcess[9],
				Time:         tmpProcess[10],
				Command:      tmpProcess[11],
			}

			result = append(result, tmpResult)
		}

	}

	return result
}

// DeviceInfoGet - Get Device Status
func DeviceInfoGet() (*models.DeviceInfoEntry, error) {
	result := new(models.DeviceInfoEntry)
	machineIDFile := "/etc/machine-id"
	bootIDFile := "/proc/sys/kernel/random/boot_id"
	OSFile := "/etc/issue"
	versionFile := "/proc/version_signature"
	hostnameFile := "/etc/hostname"
	uptimeFile := "/proc/uptime"
	// Get File data
	machinID, err := os.ReadFile(machineIDFile)
	if err != nil {
		return nil, err
	}
	result.MachineID = string(machinID)

	bootID, err := os.ReadFile(bootIDFile)
	if err != nil {
		return nil, err
	}
	result.BootID = string(bootID)

	OS, err := os.ReadFile(OSFile)
	if err != nil {
		return nil, err
	}
	result.OS = string(OS)

	kernelVersion, err := os.ReadFile(versionFile)
	if err != nil {
		return nil, err
	}
	result.Kernel = string(kernelVersion)

	hostname, err := os.ReadFile(hostnameFile)
	if err != nil {
		return nil, err
	}
	result.HostName = string(hostname)
	uptime, err := os.ReadFile(uptimeFile)
	if err != nil {
		return nil, err
	}
	result.Uptime = string(uptime)

	app := "uname"
	arg0 := "-p"
	cmd := exec.Command(app, arg0)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	result.Architecture = string(stdout)
	return result, nil
}

// Get FileSystem Status
func FileSystemInfoGet() ([]*models.FileSystemInfoEntry, error) {
	var result []*models.FileSystemInfoEntry
	result = make([]*models.FileSystemInfoEntry, 0)
	app := "df"
	arg0 := "-hT"
	cmd := exec.Command(app, arg0)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	dfResult := string(stdout)
	// Print the output
	for _, dfInfos := range strings.Split(dfResult, "\n")[7:] {
		tmpdf := strings.Fields(dfInfos)
		if len(tmpdf) == 7 {
			tmpResult := &models.FileSystemInfoEntry{
				FileSystem: tmpdf[0],
				Type:       tmpdf[1],
				Size:       tmpdf[2],
				Used:       tmpdf[3],
				Avail:      tmpdf[4],
				UsePercent: tmpdf[5],
				MountedOn:  tmpdf[6],
			}

			result = append(result, tmpResult)
		}

	}

	return result, nil
}
