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
	apiserver "loxilb/api"
	cmn "loxilb/common"
	tk "loxilb/loxilib"
	nlp "loxilb/loxinlp"
	opts "loxilb/options"
	"net"
	"sync"
	"time"
)

const (
	ROOT_ZONE = "root"
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

func loxiNetTicker() {
	for {
		select {
		case <-mh.tDone:
			return
		case t := <-mh.ticker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			mh.zn.ZoneTicker()
		}
	}
}

var mh loxiNetH

func loxiNetInit() {

	tk.LogItInit("/var/log/loxilb.log", tk.LOG_DEBUG, true)

	mh.tDone = make(chan bool)
	mh.ticker = time.NewTicker(10 * time.Second)
	mh.wg.Add(1)
	go loxiNetTicker()

	mh.dpEbpf = DpEbpfInit()
	mh.dp = DpBrokerInit(mh.dpEbpf)
	mh.zn = ZoneInit()
	mh.zn.ZoneAdd(ROOT_ZONE)
	mh.zr, _ = mh.zn.Zonefind(ROOT_ZONE)
	if mh.zr == nil {
		tk.LogIt(tk.LOG_ERROR, "Root zone not found\n")
		return
	}

	if opts.Opts.NoNlp == false {
		nlp.NlpRegister(NetApiInit())
		nlp.NlpInit()
	}

	if opts.Opts.Bgp {
		mh.bgp = GoBgpInit()
	}

	if opts.Opts.NoApi == false {
		apiserver.RegisterApiHooks(NetApiInit())
		go apiserver.RunApiServer()
	}

}

func loxiNetRun() {
	mh.wg.Wait()
}

func LoxiNetMain() {
	fmt.Printf("loxilb - start\n")

	loxiNetInit()

	// Test stub code -- To be removed
	if false {
		lbServ := cmn.LbServiceArg{ServIP: "20.20.20.1", ServPort: 2020, Proto: "tcp", Sel: cmn.LB_SEL_RR}
		lbEps := []cmn.LbEndPointArg{
			{
				EpIP:   "31.31.31.1",
				EpPort: 5001,
				Weight: 1,
			},
			{
				EpIP:   "32.32.32.1",
				EpPort: 5001,
				Weight: 2,
			},
			{
				EpIP:   "17.17.17.1",
				EpPort: 5001,
				Weight: 2,
			},
		}

		mh.zr.Rules.AddNatLbRule(lbServ, lbEps[:])
		//mh.zr.Rules.DeleteNatLbRule(lbServ)

		// Session information
		anTun := cmn.SessTun{TeID: 1, Addr: net.IP{10, 10, 10, 56}} // An TeID, gNBIP
		cnTun := cmn.SessTun{TeID: 1, Addr: net.IP{10, 10, 10, 59}} // Cn TeID, MyIP

		_, err := mh.zr.Sess.SessAdd("user1", net.IP{88, 88, 88, 88}, anTun, cnTun)
		if err != nil {
			fmt.Printf("Failed to add session\n")
		}

		// Add ULCL classifier
		_, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr: net.IP{32, 32, 32, 1}, Qfi: 11})
		//_, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr:net.IP{9,9,9,9}, Qfi:1})

		mh.zr.Sess.USess2String(&mh)
	}

	loxiNetRun()
}
