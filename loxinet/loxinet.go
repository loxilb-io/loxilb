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
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	apiserver "github.com/loxilb-io/loxilb/api"
	nlp "github.com/loxilb-io/loxilb/api/loxinlp"
	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
)

// string constant representing root security zone
const (
	RootZone = "root"
)

// constants
const (
	LoxinetTiVal   = 10
	GoBGPInitTiVal = 20
	KAInitTiVal    = 30
)

// utility variables
const (
	MkfsScript     = "/usr/local/sbin/mkllb_bpffs"
	BpfFsCheckFile = "/opt/loxilb/dp/bpf/intf_map"
)

// IterIntf - interface implementation to iterate various loxinet
// subsystems entitities
type IterIntf interface {
	NodeWalker(b string)
}

type loxiNetH struct {
	dpEbpf *DpEbpfH
	dp     *DpH
	zn     *ZoneH
	zr     *Zone
	mtx    sync.RWMutex
	ticker *time.Ticker
	tDone  chan bool
	sigCh  chan os.Signal
	wg     sync.WaitGroup
	bgp    *GoBgpH
	has    *CIStateH
	logger *tk.Logger
	ready  bool
}

// NodeWalker - an implementation of node walker interface
func (mh *loxiNetH) NodeWalker(b string) {
	tk.LogIt(tk.LogDebug, "%s\n", b)
}

func logString2Level(logStr string) tk.LogLevelT {
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

func kaString2Mode(kaStr string) (bool, bool) {
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

// ParamSet - Set Loxinet Params
func (mh *loxiNetH) ParamSet(param cmn.ParamMod) (int, error) {
	logLevel := logString2Level(param.LogLevel)

	if mh.logger != nil {
		mh.logger.LogItSetLevel(logLevel)
	}
	return 0, nil
}

// ParamGet - Get Loxinet Params
func (mh *loxiNetH) ParamGet(param *cmn.ParamMod) (int, error) {
	logLevel := "n/a"
	switch mh.logger.CurrLogLevel {
	case tk.LogDebug:
		logLevel = "debug"
	case tk.LogInfo:
		logLevel = "info"
	case tk.LogError:
		logLevel = "error"
	case tk.LogNotice:
		logLevel = "notice"
	case tk.LogWarning:
		logLevel = "warning"
	case tk.LogAlert:
		logLevel = "alert"
	case tk.LogCritical:
		logLevel = "critical"
	case tk.LogEmerg:
		logLevel = "emergency"
	default:
		param.LogLevel = logLevel
		return -1, errors.New("unknown log level")
	}

	param.LogLevel = logLevel
	return 0, nil
}

// loxiNetTicker - this ticker routine runs every LOXINET_TIVAL seconds
func loxiNetTicker() {
	for {
		select {
		case <-mh.tDone:
			return
		case <-mh.sigCh:
			var ws syscall.WaitStatus
			var ru syscall.Rusage
			wpid := 1
			try := 0
			for wpid >= 0 && try < 100 {
				wpid, _ = syscall.Wait4(-1, &ws, syscall.WNOHANG, &ru)
				try++
			}
		case t := <-mh.ticker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			// Do any housekeeping activities for security zones
			mh.zn.ZoneTicker()
			mh.has.CITicker()
		}
	}
}

var mh loxiNetH

func loxiNetInit() {
	spawnKa, kaMode := kaString2Mode(opts.Opts.Ka)
	clusterMode := false
	if opts.Opts.ClusterNodes != "none" {
		clusterMode = true
	}

	// Initialize logger and specify the log file
	logfile := fmt.Sprintf("%s%s.log", "/var/log/loxilb", os.Getenv("HOSTNAME"))
	logLevel := logString2Level(opts.Opts.LogLevel)
	mh.logger = tk.LogItInit(logfile, logLevel, true)

	// It is important to make sure loxilb's eBPF filesystem
	// is in place and mounted to make sure maps are pinned properly
	if FileExists(BpfFsCheckFile) == false {
		if FileExists(MkfsScript) {
			RunCommand(MkfsScript, true)
		}
	}

	mh.sigCh = make(chan os.Signal, 5)
	signal.Notify(mh.sigCh, os.Interrupt, syscall.SIGCHLD)

	mh.tDone = make(chan bool)
	mh.ticker = time.NewTicker(LoxinetTiVal * time.Second)
	mh.wg.Add(1)
	go loxiNetTicker()

	// Initialize the ebpf datapath subsystem
	mh.dpEbpf = DpEbpfInit(clusterMode)
	mh.dp = DpBrokerInit(mh.dpEbpf)

	// Initialize the security zone subsystem
	mh.zn = ZoneInit()

	// Add a root zone by default
	mh.zn.ZoneAdd(RootZone)

	mh.zr, _ = mh.zn.Zonefind(RootZone)
	if mh.zr == nil {
		tk.LogIt(tk.LogError, "root zone not found\n")
		return
	}

	// Initialize goBgp client
	if opts.Opts.Bgp {
		mh.bgp = GoBgpInit()
	}

	// Initialize the nlp subsystem
	if opts.Opts.NoNlp == false {
		nlp.NlpRegister(NetAPIInit())
		nlp.NlpInit()
	}

	// Initialize and spawn the api server subsystem
	if opts.Opts.NoApi == false {
		apiserver.RegisterAPIHooks(NetAPIInit())
		go apiserver.RunAPIServer()
	}

	mh.has = CIInit(spawnKa, kaMode)
	// Add cluster nodes if specified
	if clusterMode {
		cNodes := strings.Split(opts.Opts.ClusterNodes, ",")
		for _, cNode := range cNodes {
			addr := net.ParseIP(cNode)
			if addr == nil {
				continue
			}
			mh.has.ClusterNodeAdd(cmn.CluserNodeMod{Addr: addr})
		}
	}
	mh.ready = true
}

// loxiNetRun - This routine will not return
func loxiNetRun() {
	mh.wg.Wait()
}

// Main -  main routine of loxinet
func Main() {
	loxiNetInit()
	loxiNetRun()
}
