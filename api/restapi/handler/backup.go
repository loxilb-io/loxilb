/*
 * Copyright (c) 2025 LoxiLB Authors
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

type DumpFile struct {
	Lbrule    []cmn.LbRuleMod   `json:"loadbalancer,omitempty"`
	Cluster   []cmn.HASMod      `json:"cluster,omitempty"`
	Endpoint  []cmn.EndPointMod `json:"endpoint,omitempty"`
	Firewall  []cmn.FwRuleMod   `json:"firewall,omitempty"`
	Mirror    []cmn.MirrMod     `json:"mirror,omitempty"`
	Policy    []cmn.PolMod      `json:"policy,omitempty"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version"`
}

// API to download a specific log archive
func ConfigGetExport(params operations.GetConfigExportParams, principal any) middleware.Responder {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("loxilb-config-%s.json", timestamp)
	if filename == "" {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Filename is required")}
	}

	filePath := filepath.Join(os.TempDir(), filename)
	file, err := os.Create(filePath)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("File not found")}
	}

	// Dump current configuration to the file
	exportConfig, errRes := DumpConfiguration()
	if errRes != nil {
		os.Remove(filePath) // Clean up on error
		return errRes
	}
	// Convert to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(exportConfig, "", "  ")
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to marshal JSON: " + err.Error())}
	}
	// Write JSON data to file
	_, err = file.Write(jsonData)
	if err != nil {
		file.Close()
		os.Remove(filePath) // Clean up on error
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to write JSON to file: " + err.Error())}
	}

	// Reset file pointer to beginning for reading
	_, err = file.Seek(0, 0)
	if err != nil {
		file.Close()
		os.Remove(filePath)
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to reset file pointer: " + err.Error())}
	}

	// Check if the file is empty
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get file info")}
	}
	if fileInfo.Size() == 0 {
		file.Close()
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("File is empty")}
	}

	// Set headers and send the file
	return middleware.ResponderFunc(func(w http.ResponseWriter, _ runtime.Producer) {
		defer file.Close()
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		bytesCopied, err := io.Copy(w, file)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to copy file content: %s, error: %v\n", filePath, err)
		} else {
			tk.LogIt(tk.LogDebug, "Successfully copied %d bytes from file: %s\n", bytesCopied, filePath)
		}
	})
}

func ConfigPostImport(params operations.PostConfigImportParams, principal any) middleware.Responder {
	if params.Configuration == nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("No configuration file provided")}
	}

	// Make backup of current configuration
	backupTimestamp := time.Now().Format("20060102-150405")
	backupFilename := fmt.Sprintf("loxilb-config-backup-%s.json", backupTimestamp)
	backupFilePath := filepath.Join(os.TempDir(), backupFilename)
	exportConfig, errRes := DumpConfiguration()
	if errRes != nil {
		return errRes
	}

	// Write the backup to a file
	backupFile, err := os.Create(backupFilePath)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to create backup file: " + err.Error())}
	}
	defer backupFile.Close()

	jsonData, err := json.MarshalIndent(exportConfig, "", "  ")
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to marshal JSON for backup: " + err.Error())}
	}

	_, err = backupFile.Write(jsonData)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to write to backup file: " + err.Error())}
	}
	tk.LogIt(tk.LogInfo, "Backup of current configuration saved to %s\n", backupFilePath)

	// Read the uploaded file
	fileData, err := io.ReadAll(params.Configuration)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to read configuration file: " + err.Error())}
	}
	var importData DumpFile
	err = json.Unmarshal(fileData, &importData)
	if err != nil {
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Invalid JSON format: " + err.Error())}
	}

	// Clear existing configurations before importing new ones
	// Delete all existing LB rules using the exported data
	msg, err := DeleteAllConfiguration(jsonData)
	if err != nil {
		tk.LogIt(tk.LogError, "Failed to delete existing configuration: %v\n", err)
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to delete existing configuration: " + err.Error())}
	} else if msg != "" {
		tk.LogIt(tk.LogWarning, "Failed to delete existing configuration: %v but still on going\n", msg)
	}

	// Add new configurations from the imported data
	err = AddImportConfiguration(&importData)
	if err != nil {
		tk.LogIt(tk.LogError, "Failed to import configuration: %v\n", err)
		return &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to import configuration: " + err.Error())}
	}

	return &ResultResponse{Result: "Success"}
}

func DumpConfiguration() (map[string]any, *ErrorResponse) {
	// Get info about the configurations
	// Get LB rules information
	lbrules, err := ApiHooks.NetLbRuleGet()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get LB rules: " + err.Error())}
	}
	// Get Cluster configuration
	clusterConfig, err := ApiHooks.NetCIStateGet()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get Cluster config: " + err.Error())}
	}

	// Get Endpoint configuration
	endpointConfig, err := ApiHooks.NetEpHostGet()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get Endpoint config: " + err.Error())}
	}

	// Get Firewall configuration
	// Exclude rules with Mark 0x40000000 (SrcChkFwMark)
	firewallConfig, err := GetFirewallConfig()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get Firewall config: " + err.Error())}
	}

	// Get Mirror configuration
	mirrorConfig, err := ApiHooks.NetMirrorGet()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get Mirror config: " + err.Error())}
	}

	// Get Policy configuration
	policyConfig, err := ApiHooks.NetPolicerGet()
	if err != nil {
		return nil, &ErrorResponse{Payload: ResultErrorResponseErrorMessage("Failed to get Policy config: " + err.Error())}
	}

	// Create export configuration structure
	exportConfig := map[string]any{
		"timestamp":    time.Now().Format(time.RFC3339),
		"version":      cmn.Version,
		"loadbalancer": lbrules,
		"cluster":      clusterConfig,
		"endpoint":     endpointConfig,
		"firewall":     firewallConfig,
		"mirror":       mirrorConfig,
		"policy":       policyConfig,
	}
	return exportConfig, nil
}

func GetFirewallConfig() ([]cmn.FwRuleMod, error) {
	var returnList []cmn.FwRuleMod
	fwRules, err := ApiHooks.NetFwRuleGet()
	if err != nil {
		return nil, err
	}
	for _, fw := range fwRules {
		if fw.Opts.Mark&0x40000000 != 0 {
			continue
		}
		returnList = append(returnList, fw)
	}

	return returnList, nil
}

func AddImportConfiguration(importData *DumpFile) error {

	// Process Load Balancer configurations
	for _, lb := range importData.Lbrule {
		_, err := ApiHooks.NetLbRuleAdd(&lb)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add LB rule: %v\n", err)
			return err
		}
	}

	// Process Cluster configurations
	for _, cluster := range importData.Cluster {
		_, err := ApiHooks.NetCIStateMod(&cluster)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Cluster config: %v\n", err)
			return err
		}
	}

	// Process Endpoint configurations
	for _, ep := range importData.Endpoint {
		_, err := ApiHooks.NetEpHostAdd(&ep)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Endpoint config: %v\n", err)
			return err
		}
	}

	// Process Firewall configurations
	for _, fw := range importData.Firewall {
		_, err := ApiHooks.NetFwRuleAdd(&fw)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Firewall rule: %v\n", err)
			return err
		}
	}

	// Process Mirror configurations
	for _, mirr := range importData.Mirror {
		_, err := ApiHooks.NetMirrorAdd(&mirr)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Mirror config: %v\n", err)
			return err
		}
	}

	// Process Policy configurations
	for _, pol := range importData.Policy {
		_, err := ApiHooks.NetPolicerAdd(&pol)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to add Policy config: %v\n", err)
			return err
		}
	}
	return nil
}

func DeleteAllConfiguration(jsonData []byte) (string, error) {
	var exportConfig DumpFile
	err := json.Unmarshal(jsonData, &exportConfig)
	if err != nil {
		return "", err
	}
	for _, lb := range exportConfig.Lbrule {
		_, err := ApiHooks.NetLbRuleDel(&lb)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete LB rule: %v\n", err)
			return err.Error(), nil
		}
	}
	for _, fw := range exportConfig.Firewall {
		_, err := ApiHooks.NetFwRuleDel(&fw)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete Firewall rule: %v\n", err)
			return err.Error(), nil

		}
	}
	for _, mirr := range exportConfig.Mirror {
		_, err := ApiHooks.NetMirrorDel(&mirr)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete Mirror config: %v\n", err)
			return err.Error(), nil
		}
	}
	for _, pol := range exportConfig.Policy {
		_, err := ApiHooks.NetPolicerDel(&pol)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete Policy config: %v\n", err)
			return err.Error(), nil
		}
	}
	for _, ep := range exportConfig.Endpoint {
		_, err := ApiHooks.NetEpHostDel(&ep)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to delete Endpoint config: %v\n", err)
			return err.Error(), nil
		}
	}
	// Note : Cluster configurations are not deleted
	return "", nil
}
