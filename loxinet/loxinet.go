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
	apiserver "github.com/loxilb-io/loxilb/api"
	nlp "github.com/loxilb-io/loxilb/api/loxinlp"
	prometheus "github.com/loxilb-io/loxilb/api/prometheus"
	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
	"net"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"sync"
	"syscall"
	"time"
)

// string constant representing root security zone
const (
	RootZone = "root"
)

// constants
const (
	LoxinetTiVal   = 10
	GoBGPInitTiVal = 5
	KAInitTiVal    = 35
)

// utility variables
const (
	MkfsScript     = "/usr/local/sbin/mkllb_bpffs"
	BpfFsCheckFile = "/opt/loxilb/dp/bpf/intf_map"
)

type loxiNetH struct {
	dpEbpf     *DpEbpfH
	dp         *DpH
	zn         *ZoneH
	zr         *Zone
	mtx        sync.RWMutex
	ticker     *time.Ticker
	tDone      chan bool
	sigCh      chan os.Signal
	wg         sync.WaitGroup
	bgp        *GoBgpH
	sumDis     bool
	pProbe     bool
	has        *CIStateH
	logger     *tk.Logger
	ready      bool
	self       int
	rssEn      bool
	eHooks     bool
	cloudLabel string
	pFile      *os.File
}

// NodeWalker - an implementation of node walker interface
func (mh *loxiNetH) NodeWalker(b string) {
	tk.LogIt(tk.LogDebug, "%s\n", b)
}

// ParamSet - Set Loxinet Params
func (mh *loxiNetH) ParamSet(param cmn.ParamMod) (int, error) {
	logLevel := LogString2Level(param.LogLevel)

	if mh.logger != nil {
		mh.logger.LogItSetLevel(logLevel)
	}

	DpEbpfSetLogLevel(logLevel)

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
func loxiNetTicker(bgpPeerMode bool) {
	for {
		select {
		case <-mh.tDone:
			return
		case sig := <-mh.sigCh:
			if sig == syscall.SIGCHLD {
				var ws syscall.WaitStatus
				var ru syscall.Rusage
				wpid := 1
				try := 0
				for wpid >= 0 && try < 100 {
					wpid, _ = syscall.Wait4(-1, &ws, syscall.WNOHANG, &ru)
					try++
				}
			} else if sig == syscall.SIGHUP {
				tk.LogIt(tk.LogCritical, "SIGHUP received\n")
				pprof.StopCPUProfile()
			} else if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				tk.LogIt(tk.LogCritical, "Shutdown on sig %v\n", sig)
				// TODO - More subsystem cleanup TBD
				mh.zr.Rules.RuleDestructAll()
				if !bgpPeerMode {
					mh.dpEbpf.DpEbpfUnInit()
				}
				apiserver.ApiServerShutOk()
			}
		case t := <-mh.ticker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			if !bgpPeerMode {
				// Do any housekeeping activities for security zones
				mh.zn.ZoneTicker()
				mh.has.CITicker()
			}
		}
	}
}

var mh loxiNetH

func loxiNetInit() {
	var rpcMode int

	spawnKa, kaMode := KAString2Mode(opts.Opts.Ka)
	clusterMode := false
	if opts.Opts.ClusterNodes != "none" {
		clusterMode = true
	}

	// Initialize logger and specify the log file
	logfile := fmt.Sprintf("%s%s.log", "/var/log/loxilb", os.Getenv("HOSTNAME"))
	logLevel := LogString2Level(opts.Opts.LogLevel)
	mh.logger = tk.LogItInit(logfile, logLevel, true)

	// Stack trace logger
	defer func() {
		if e := recover(); e != nil {
			tk.LogIt(tk.LogCritical, "%s: %s", e, debug.Stack())
		}
	}()

	// It is important to make sure loxilb's eBPF filesystem
	// is in place and mounted to make sure maps are pinned properly
	if !FileExists(BpfFsCheckFile) {
		if FileExists(MkfsScript) {
			RunCommand(MkfsScript, true)
		}
	}

	mh.cloudLabel = opts.Opts.Cloud
	mh.self = opts.Opts.ClusterSelf
	mh.rssEn = opts.Opts.RssEnable
	mh.eHooks = opts.Opts.EgrHooks
	mh.sumDis = opts.Opts.CSumDisable
	mh.pProbe = opts.Opts.PassiveEPProbe
	mh.sigCh = make(chan os.Signal, 5)
	signal.Notify(mh.sigCh, os.Interrupt, syscall.SIGCHLD, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	if mh.cloudLabel == "oci" {
		OCIApiInit()
	}

	// Check if profiling is enabled
	if opts.Opts.CPUProfile != "none" {
		var err error
		mh.pFile, err = os.Create(opts.Opts.CPUProfile)
		if err != nil {
			tk.LogIt(tk.LogNotice, "profile file create failed\n")
			return
		}
		err = pprof.StartCPUProfile(mh.pFile)
		if err != nil {
			tk.LogIt(tk.LogNotice, "CPU profiler start failed\n")
			return
		}
	}
	if opts.Opts.Rpc == "netrpc" {
		rpcMode = RPCTypeNetRPC
	} else {
		rpcMode = RPCTypeGRPC
	}

	if !opts.Opts.BgpPeerMode {
		// Initialize the ebpf datapath subsystem
		mh.dpEbpf = DpEbpfInit(clusterMode, mh.self, mh.rssEn, mh.eHooks, -1)
		mh.dp = DpBrokerInit(mh.dpEbpf, rpcMode)

		// Initialize the security zone subsystem
		mh.zn = ZoneInit()

		// Add a root zone by default
		mh.zn.ZoneAdd(RootZone)
		mh.zr, _ = mh.zn.Zonefind(RootZone)
		if mh.zr == nil {
			tk.LogIt(tk.LogError, "root zone not found\n")
			return
		}

		// Initialize the clustering subsystem
		mh.has = CIInit(spawnKa, kaMode)
		if clusterMode {
			if opts.Opts.Bgp {
				tk.LogIt(tk.LogInfo, "init-wait cluster mode\n")
				time.Sleep(10 * time.Second)
			}
			// Add cluster nodes if specified
			cNodes := strings.Split(opts.Opts.ClusterNodes, ",")
			for _, cNode := range cNodes {
				addr := net.ParseIP(cNode)
				if addr == nil {
					continue
				}
				mh.has.ClusterNodeAdd(cmn.ClusterNodeMod{Addr: addr})
			}
		}
	} else {
		// If bgp peer mode is enable then bgp flag has to be set by default
		opts.Opts.Bgp = true
		//opts.Opts.NoNlp = true
		opts.Opts.NoPrometheus = true
	}

	// Initialize goBgp client
	if opts.Opts.Bgp {
		mh.bgp = GoBgpInit(opts.Opts.BgpPeerMode)
	}

	// Initialize and spawn the api server subsystem
	if !opts.Opts.NoAPI {
		apiserver.RegisterAPIHooks(NetAPIInit(opts.Opts.BgpPeerMode))
		go apiserver.RunAPIServer()
		apiserver.WaitAPIServerReady()
	}

	// Initialize the nlp subsystem
	if !opts.Opts.NoNlp {
		nlp.NlpRegister(NetAPIInit(opts.Opts.BgpPeerMode))
		nlp.NlpInit(opts.Opts.BgpPeerMode, opts.Opts.BlackList)
	}

	// Initialize the Prometheus subsystem
	if !opts.Opts.NoPrometheus {
		prometheus.PrometheusRegister(NetAPIInit(opts.Opts.BgpPeerMode))
		prometheus.Init()
	}

	if !opts.Opts.BgpPeerMode {
		// Spawn CI maintenance application
		mh.has.CISpawn()
	}
	// Initialize the loxinet global ticker(s)
	mh.tDone = make(chan bool)
	mh.ticker = time.NewTicker(LoxinetTiVal * time.Second)
	mh.wg.Add(1)
	go loxiNetTicker(opts.Opts.BgpPeerMode)

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
