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
	"fmt"
	"os"
	"sync"
	"time"

	apiserver "github.com/loxilb-io/loxilb/api"
	nlp "github.com/loxilb-io/loxilb/api/loxinlp"
	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
)

const (
	RootZone = "root"
)

const (
	LoxinetTiVal = 10
)

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
	wg     sync.WaitGroup
	bgp    *GoBgpH
}

func (mh *loxiNetH) NodeWalker(b string) {
	tk.LogIt(tk.LOG_DEBUG, "%s\n", b)
}

func (mh *loxiNetH) NodeDat2Str(d tk.TrieData) string {
	return ""
}

func (mh *loxiNetH) TrieNodeWalker(b string) {
	tk.LogIt(tk.LOG_DEBUG, "%s", b)
}

// This ticker routine runs every LOXINET_TIVAL seconds
func loxiNetTicker() {
	for {
		select {
		case <-mh.tDone:
			return
		case t := <-mh.ticker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			// Do any housekeeping activities for security zones
			mh.zn.ZoneTicker()
		}
	}
}

var mh loxiNetH

func loxiNetInit() {

	// Initialize logger and specify the log file
	logfile := fmt.Sprintf("%s%s.log", "/var/log/loxilb", os.Getenv("HOSTNAME"))
	tk.LogItInit(logfile, tk.LOG_DEBUG, true)

	mh.tDone = make(chan bool)
	mh.ticker = time.NewTicker(LoxinetTiVal * time.Second)
	mh.wg.Add(1)
	go loxiNetTicker()

	// Initialize the ebpf datapath subsystem
	mh.dpEbpf = DpEbpfInit()
	mh.dp = DpBrokerInit(mh.dpEbpf)

	// Initialize the security zone subsystem
	mh.zn = ZoneInit()

	// Add a root zone by default
	mh.zn.ZoneAdd(RootZone)
	mh.zr, _ = mh.zn.Zonefind(RootZone)
	if mh.zr == nil {
		tk.LogIt(tk.LOG_ERROR, "root zone not found\n")
		return
	}

	// Initialize the nlp subsystem
	if opts.Opts.NoNlp == false {
		nlp.NlpRegister(NetApiInit())
		nlp.NlpInit()
	}

	// Initialize goBgp client
	if opts.Opts.Bgp {
		mh.bgp = GoBgpInit()
	}

	// Initialize and spawn the api server subsystem
	if opts.Opts.NoApi == false {
		apiserver.RegisterApiHooks(NetApiInit())
		go apiserver.RunApiServer()
	}
}

// This routine will not return
func loxiNetRun() {
	mh.wg.Wait()
}

// Main routine of loxinet
func LoxiNetMain() {
	loxiNetInit()
	loxiNetRun()
}
