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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// IterIntf - interface implementation to iterate various loxinet
// subsystems entitities
type IterIntf interface {
	NodeWalker(b string)
}

// FileExists - Check if file exists
func FileExists(fname string) bool {
	info, err := os.Stat(fname)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// FileCreate - Create a file
func FileCreate(fname string) int {
	file, e := os.Create(fname)
	if e != nil {
		return -1
	}
	file.Close()
	return 0
}

// IsLoxiAPIActive - Check if API url is active
func IsLoxiAPIActive(url string) bool {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{Timeout: timeout}
	_, e := client.Get(url)
	return e == nil
}

// ReadPIDFile - Read a PID file
func ReadPIDFile(pf string) int {

	if exists := FileExists(pf); !exists {
		return 0
	}

	d, err := ioutil.ReadFile(pf)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(d)))
	if err != nil {
		return 0
	}

	p, err1 := os.FindProcess(int(pid))
	if err1 != nil {
		return 0
	}

	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return 0
	}

	return pid
}

// RunCommand - Run a bash command
func RunCommand(command string, isFatal bool) (int, error) {
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError, "Error in running %s:%s\n", command, err)
		if isFatal {
			os.Exit(1)
		}
		return 0, err
	}

	return 0, nil
}

// LogString2Level - Convert log level in string to LogLevelT
func LogString2Level(logStr string) tk.LogLevelT {
	logLevel := tk.LogDebug
	switch opts.Opts.LogLevel {
	case "info":
		logLevel = tk.LogInfo
	case "error":
		logLevel = tk.LogError
	case "notice":
		logLevel = tk.LogNotice
	case "warning":
		logLevel = tk.LogWarning
	case "alert":
		logLevel = tk.LogAlert
	case "critical":
		logLevel = tk.LogCritical
	case "emergency":
		logLevel = tk.LogEmerg
	default:
		logLevel = tk.LogDebug
	}
	return logLevel
}

// KAString2Mode - Convert ka mode in string opts to spawn/KAMode
func KAString2Mode(kaStr string) (bool, bool) {
	spawnKa := false
	kaMode := false
	switch opts.Opts.Ka {
	case "in":
		spawnKa = true
		kaMode = true
	case "out":
		spawnKa = false
		kaMode = true
	}
	return spawnKa, kaMode
}

// HTTPSProber - Do a https probe for given url
// returns true/false depending on whether probing was successful
func HTTPSProber(urls string, certPool *x509.CertPool, resp string) bool {
	var err error
	var req *http.Request
	var res *http.Response

	timeout := time.Duration(2 * time.Second)
	client := http.Client{Timeout: timeout,
		Transport: &http.Transport{
			IdleConnTimeout: 5 * time.Second,
			TLSClientConfig: &tls.Config{RootCAs: certPool}},
	}
	if req, err = http.NewRequest(http.MethodGet, urls, nil); err != nil {
		return false
	}

	res, err = client.Do(req)
	if err != nil || res.StatusCode != 200 {
		return false
	}
	defer res.Body.Close()
	if resp != "" {
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}
		return string(data) == resp
	}

	return true
}
