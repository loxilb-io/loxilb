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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/loxilb-io/loxilb/api/loxinlp"
	cmn "github.com/loxilb-io/loxilb/common"
	utils "github.com/loxilb-io/loxilb/pkg/utils"
	tk "github.com/loxilb-io/loxilib"
	probing "github.com/prometheus-community/pro-bing"
)

// error codes
const (
	RuleErrBase = iota - 7000
	RuleUnknownServiceErr
	RuleUnknownEpErr
	RuleExistsErr
	RuleAllocErr
	RuleNotExistsErr
	RuleEpCountErr
	RuleTupleErr
	RuleArgsErr
	RuleEpNotExistErr
)

type ruleTMatch uint

// rm tuples
const (
	RmPort ruleTMatch = 1 << iota
	RmL2Src
	RmL2Dst
	RmVlanID
	RmL3Src
	RmL3Dst
	RmL4Src
	RmL4Dst
	RmL4Prot
	RmInL2Src
	RmInL2Dst
	RmInL3Src
	RmInL3Dst
	RmInL4Src
	RmInL4Dst
	RmInL4Port
	RmMax
)

// constants
const (
	MaxNatEndPoints            = 32
	DflLbaInactiveTries        = 2         // Default number of inactive tries before LB arm is turned off
	MaxDflLbaInactiveTries     = 100       // Max number of inactive tries before LB arm is turned off
	DflLbaCheckTimeout         = 10        // Default timeout for checking LB arms
	DflHostProbeTimeout        = 60        // Default probe timeout for end-point host
	InitHostProbeTimeout       = 15        // Initial probe timeout for end-point host
	MaxHostProbeTime           = 24 * 3600 // Max possible host health check duration
	LbDefaultInactiveTimeout   = 4 * 60    // Default inactive timeout for established sessions
	LbDefaultInactiveNSTimeout = 20        // Default inactive timeout for non-session oriented protocols
	LbMaxInactiveTimeout       = 24 * 3600 // Maximum inactive timeout for established sessions
	MaxEndPointCheckers        = 4         // Maximum helpers to check endpoint health
	EndPointCheckerDuration    = 2         // Duration at which ep-helpers will run
	MaxEndPointSweeps          = 20        // Maximum end-point sweeps per round
	VIPSweepDuration           = 30        // Duration of periodic VIP maintenance
	DefaultPersistTimeOut      = 10800     // Default persistent LB session timeout
)

type ruleTType uint

// rt types
const (
	RtEm ruleTType = iota + 1
	RtMf
)

type rule8Tuple struct {
	val   uint8
	valid uint8
}

type rule16Tuple struct {
	val   uint16
	valid uint16
}

type rule32Tuple struct {
	val   uint32
	valid uint32
}

type rule64Tuple struct {
	val   uint64
	valid uint64
}

type ruleIPTuple struct {
	addr net.IPNet
}

type ruleMacTuple struct {
	addr  [6]uint8
	valid [6]uint8
}

type ruleStringTuple struct {
	val string
}

type ruleTuples struct {
	port     ruleStringTuple
	l2Src    ruleMacTuple
	l2Dst    ruleMacTuple
	vlanID   rule16Tuple
	l3Src    ruleIPTuple
	l3Dst    ruleIPTuple
	l4Prot   rule8Tuple
	l4Src    rule16Tuple
	l4Dst    rule16Tuple
	tunID    rule32Tuple
	inL2Src  ruleMacTuple
	inL2Dst  ruleMacTuple
	inL3Src  ruleIPTuple
	inL3Dst  ruleIPTuple
	inL4Prot rule8Tuple
	inL4Src  rule16Tuple
	inL4Dst  rule16Tuple
	pref     uint16
	path     string
}

type ruleTActType uint

// possible actions for a rt-entry
const (
	RtActDrop ruleTActType = iota + 1
	RtActFwd
	RtActTrap
	RtActRedirect
	RtActDnat
	RtActSnat
	RtActFullNat
	RtActFullProxy
)

// possible types of end-point probe
const (
	HostProbePing        = "ping"
	HostProbeConnectTCP  = "tcp"
	HostProbeConnectUDP  = "udp"
	HostProbeConnectSCTP = "sctp"
	HostProbeHTTP        = "http"
	HostProbeHTTPS       = "https"
	HostProbeNone        = "none"
)

type epHostOpts struct {
	inActTryThr       int
	probeType         string
	probeReq          string
	probeResp         string
	probeDuration     uint32
	currProbeDuration uint32
	probePort         uint16
	probeActivated    bool
}

type epHost struct {
	epKey        string
	hostName     string
	ruleCount    int
	inactive     bool
	initProberOn bool
	sT           time.Time
	avgDelay     time.Duration
	minDelay     time.Duration
	maxDelay     time.Duration
	hID          uint8
	inActTries   int
	opts         epHostOpts
}

type ruleNatEp struct {
	xIP           net.IP
	rIP           net.IP
	xPort         uint16
	weight        uint8
	inActTries    int
	inActive      bool
	noService     bool
	chkVal        bool
	stat          ruleStat
	foldEndPoints []ruleNatEp
	foldRuleKey   string
}

type ruleNatSIP struct {
	sIP net.IP
}

type ruleNatActs struct {
	mode      cmn.LBMode
	sel       cmn.EpSelect
	endPoints []ruleNatEp
}

type ruleFwOpt struct {
	rdrMirr  string
	rdrPort  string
	fwMark   uint32
	record   bool
	snatIP   string
	snatPort uint16
}

type ruleFwOpts struct {
	op  ruleTActType
	opt ruleFwOpt
}

type ruleTAct interface{}

type ruleAct struct {
	actType ruleTActType
	action  ruleTAct
}

type ruleStat struct {
	bytes   uint64
	packets uint64
}

type ruleProbe struct {
	actChk     bool
	prbType    string
	prbPort    uint16
	prbReq     string
	prbResp    string
	prbTimeo   uint32
	prbRetries int
}

type ruleEnt struct {
	zone     *Zone
	ruleNum  uint64
	sync     DpStatusT
	tuples   ruleTuples
	ci       string
	hChk     ruleProbe
	managed  bool
	bgp      bool
	addrRslv bool
	sT       time.Time
	iTO      uint32
	pTO      uint32
	act      ruleAct
	privIP   net.IP
	secIP    []ruleNatSIP
	stat     ruleStat
	name     string
	secMode  cmn.LBSec
	locIPs   map[string]struct{}
}

type ruleTable struct {
	tableType  ruleTType
	tableMatch ruleTMatch
	eMap       map[string]*ruleEnt
	rArr       [RtMaximumLbs]*ruleEnt
	pMap       []*ruleEnt
	Mark       *tk.Counter
}

type ruleTableType uint

// rt types
const (
	RtFw ruleTableType = iota + 1
	RtLB
	RtMax
)

// rule specific loxilb constants
const (
	RtMaximumFw4s = (8 * 1024)
	RtMaximumLbs  = (2 * 1024)
)

// RuleCfg - tunable parameters related to inactive rules
type RuleCfg struct {
	RuleInactTries   int
	RuleInactChkTime int
}

type epChecker struct {
	hChk *time.Ticker
	tD   chan bool
}

type vipElem struct {
	ref  int
	pVIP net.IP
}

// RuleH - context container
type RuleH struct {
	zone       *Zone
	cfg        RuleCfg
	tables     [RtMax]ruleTable
	epMap      map[string]*epHost
	vipMap     map[string]*vipElem
	epCs       [MaxEndPointCheckers]epChecker
	wg         sync.WaitGroup
	lepHID     uint8
	epMx       sync.RWMutex
	rootCAPool *x509.CertPool
	tlsCert    tls.Certificate
	vipST      time.Time
}

// RulesInit - initialize the Rules subsystem
func RulesInit(zone *Zone) *RuleH {
	var nRh = new(RuleH)
	nRh.zone = zone

	nRh.cfg.RuleInactChkTime = DflLbaCheckTimeout
	nRh.cfg.RuleInactTries = DflLbaInactiveTries

	nRh.vipMap = make(map[string]*vipElem)
	nRh.epMap = make(map[string]*epHost)
	nRh.tables[RtFw].tableMatch = RmMax - 1
	nRh.tables[RtFw].tableType = RtMf
	nRh.tables[RtFw].eMap = make(map[string]*ruleEnt)
	nRh.tables[RtFw].Mark = tk.NewCounter(1, RtMaximumFw4s)

	nRh.tables[RtLB].tableMatch = RmL3Dst | RmL4Dst | RmL4Prot
	nRh.tables[RtLB].tableType = RtEm
	nRh.tables[RtLB].eMap = make(map[string]*ruleEnt)
	nRh.tables[RtLB].Mark = tk.NewCounter(1, RtMaximumLbs)

	for i := 0; i < MaxEndPointCheckers; i++ {
		nRh.epCs[i].tD = make(chan bool)
		nRh.epCs[i].hChk = time.NewTicker(EndPointCheckerDuration * time.Second)
		go epTicker(nRh, i)
	}
	rootCAPool, err := x509.SystemCertPool()
	if err == nil {
		nRh.rootCAPool = rootCAPool
	} else {
		nRh.rootCAPool = x509.NewCertPool()
	}
	rootCACertile := cmn.CertPath + cmn.CACertFileName

	// Check if there exist a common CA certificate
	if exists := utils.FileExists(rootCACertile); exists {

		rootCA, err := os.ReadFile(rootCACertile)
		if err != nil {
			tk.LogIt(tk.LogError, "RootCA cert load failed : %v\n", err)
		} else {
			nRh.rootCAPool.AppendCertsFromPEM(rootCA)
			tk.LogIt(tk.LogDebug, "RootCA cert loaded\n")
		}
	}

	certFile := cmn.CertPath + cmn.PrivateCertName
	keyFile := cmn.CertPath + cmn.PrivateKeyName

	certExists := utils.FileExists(certFile)
	keyExists := utils.FileExists(keyFile)

	if certExists == true && keyExists == true {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			tk.LogIt(tk.LogError, "Error loading loxilb certificate %s and key file %s",
				certFile, keyFile)
		}
		nRh.tlsCert = cert
	}
	nRh.wg.Add(MaxEndPointCheckers)
	nRh.vipST = time.Now()

	return nRh
}

func (r *ruleTuples) ruleMkKeyCompliance(match ruleTMatch) {
	if match&RmPort != RmPort {
		r.port.val = ""
	}
	if match&RmL2Src != RmL2Src {
		for i := 0; i < 6; i++ {
			r.l2Src.addr[i] = 0
			r.l2Src.valid[i] = 0
		}
	}
	if match&RmL2Dst != RmL2Dst {
		for i := 0; i < 6; i++ {
			r.l2Dst.addr[i] = 0
			r.l2Dst.valid[i] = 0
		}
	}
	if match&RmVlanID != RmVlanID {
		r.vlanID.val = 0
		r.vlanID.valid = 0
	}
	if match&RmL3Src != RmL3Src {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.l3Src.addr = *dst
	}
	if match&RmL3Dst != RmL3Dst {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.l3Dst.addr = *dst
	}
	if match&RmL4Prot != RmL4Prot {
		r.l4Prot.val = 0
		r.l4Prot.valid = 0
	}
	if match&RmL4Src != RmL4Src {
		r.l4Src.val = 0
		r.l4Src.valid = 0
	}
	if match&RmL4Dst != RmL4Dst {
		r.l4Dst.val = 0
		r.l4Dst.valid = 0
	}

	if match&RmInL2Src != RmInL2Src {
		for i := 0; i < 6; i++ {
			r.inL2Src.addr[i] = 0
			r.inL2Src.valid[i] = 0
		}
	}
	if match&RmInL2Dst != RmInL2Dst {
		for i := 0; i < 6; i++ {
			r.inL2Dst.addr[i] = 0
			r.inL2Dst.valid[i] = 0
		}
	}
	if match&RmInL3Src != RmInL3Src {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.inL3Src.addr = *dst
	}
	if match&RmInL3Dst != RmInL3Dst {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.inL3Dst.addr = *dst
	}
	if match&RmInL4Port != RmInL4Port {
		r.inL4Prot.val = 0
		r.inL4Prot.valid = 0
	}
	if match&RmInL4Src != RmInL4Src {
		r.inL4Src.val = 0
		r.inL4Src.valid = 0
	}
	if match&RmInL4Dst != RmInL4Dst {
		r.inL4Dst.val = 0
		r.inL4Dst.valid = 0
	}
}

func (r *ruleTuples) ruleKey() string {
	ks := ""
	if r.path != "" {
		ks += r.path
	}
	ks += fmt.Sprintf("%s", r.port.val)
	ks += fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		r.l2Dst.addr[0]&r.l2Dst.valid[0],
		r.l2Dst.addr[1]&r.l2Dst.valid[1],
		r.l2Dst.addr[2]&r.l2Dst.valid[2],
		r.l2Dst.addr[3]&r.l2Dst.valid[3],
		r.l2Dst.addr[4]&r.l2Dst.valid[4],
		r.l2Dst.addr[5]&r.l2Dst.valid[5])
	ks += fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		r.l2Src.addr[0]&r.l2Src.valid[0],
		r.l2Src.addr[1]&r.l2Src.valid[1],
		r.l2Src.addr[2]&r.l2Src.valid[2],
		r.l2Src.addr[3]&r.l2Src.valid[3],
		r.l2Src.addr[4]&r.l2Src.valid[4],
		r.l2Src.addr[5]&r.l2Src.valid[5])
	ks += fmt.Sprintf("%d", r.vlanID.val&r.vlanID.valid)
	ks += fmt.Sprintf("%s", r.l3Dst.addr.String())
	ks += fmt.Sprintf("%s", r.l3Src.addr.String())
	ks += fmt.Sprintf("%d", r.l4Prot.val&r.l4Prot.valid)

	if r.l4Src.valid == 0xffff {
		ks += fmt.Sprintf("%d", r.l4Src.val&r.l4Src.valid)
	} else {
		ks += fmt.Sprintf("%d%d", r.l4Src.val, r.l4Src.valid)
	}

	if r.l4Dst.valid == 0xffff {
		ks += fmt.Sprintf("%d", r.l4Dst.val&r.l4Dst.valid)
	} else {
		ks += fmt.Sprintf("%d%d", r.l4Dst.val, r.l4Dst.valid)
	}

	ks += fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		r.inL2Dst.addr[0]&r.inL2Dst.valid[0],
		r.inL2Dst.addr[1]&r.inL2Dst.valid[1],
		r.inL2Dst.addr[2]&r.inL2Dst.valid[2],
		r.inL2Dst.addr[3]&r.inL2Dst.valid[3],
		r.inL2Dst.addr[4]&r.inL2Dst.valid[4],
		r.inL2Dst.addr[5]&r.inL2Dst.valid[5])
	ks += fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		r.inL2Src.addr[0]&r.inL2Src.valid[0],
		r.inL2Src.addr[1]&r.inL2Src.valid[1],
		r.inL2Src.addr[2]&r.inL2Src.valid[2],
		r.inL2Src.addr[3]&r.inL2Src.valid[3],
		r.inL2Src.addr[4]&r.inL2Src.valid[4],
		r.inL2Src.addr[5]&r.inL2Src.valid[5])

	ks += fmt.Sprintf("%s", r.inL3Dst.addr.String())
	ks += fmt.Sprintf("%s", r.inL3Src.addr.String())
	ks += fmt.Sprintf("%d", r.inL4Prot.val&r.inL4Prot.valid)
	ks += fmt.Sprintf("%d", r.inL4Src.val&r.inL4Src.valid)
	ks += fmt.Sprintf("%d", r.inL4Dst.val&r.inL4Dst.valid)
	ks += fmt.Sprintf("%d", r.pref)
	return ks
}

func checkValidMACTuple(mt ruleMacTuple) bool {
	if mt.valid[0] != 0 ||
		mt.valid[1] != 0 ||
		mt.valid[2] != 0 ||
		mt.valid[3] != 0 ||
		mt.valid[4] != 0 ||
		mt.valid[5] != 0 {
		return true
	}
	return false
}

func (r *ruleTuples) String() string {

	ks := ""
	if r.path != "" {
		ks += fmt.Sprintf("%s:", r.path)
	}

	if r.port.val != "" {
		ks += fmt.Sprintf("inp-%s,", r.port.val)
	}

	if checkValidMACTuple(r.l2Dst) {
		ks += fmt.Sprintf("dmac-%02x:%02x:%02x:%02x:%02x:%02x,",
			r.l2Dst.addr[0]&r.l2Dst.valid[0],
			r.l2Dst.addr[1]&r.l2Dst.valid[1],
			r.l2Dst.addr[2]&r.l2Dst.valid[2],
			r.l2Dst.addr[3]&r.l2Dst.valid[3],
			r.l2Dst.addr[4]&r.l2Dst.valid[4],
			r.l2Dst.addr[5]&r.l2Dst.valid[5])
	}

	if checkValidMACTuple(r.l2Src) {
		ks += fmt.Sprintf("smac-%02x:%02x:%02x:%02x:%02x:%02x",
			r.l2Src.addr[0]&r.l2Src.valid[0],
			r.l2Src.addr[1]&r.l2Src.valid[1],
			r.l2Src.addr[2]&r.l2Src.valid[2],
			r.l2Src.addr[3]&r.l2Src.valid[3],
			r.l2Src.addr[4]&r.l2Src.valid[4],
			r.l2Src.addr[5]&r.l2Src.valid[5])
	}

	if r.vlanID.valid != 0 {
		ks += fmt.Sprintf("vid-%d,", r.vlanID.val&r.vlanID.valid)
	}

	if r.l3Dst.addr.String() != "<nil>" {
		ks += fmt.Sprintf("dst-%s,", r.l3Dst.addr.String())
	}

	if r.l3Src.addr.String() != "<nil>" {
		ks += fmt.Sprintf("src-%s,", r.l3Src.addr.String())
	}

	if r.l4Prot.valid != 0 {
		ks += fmt.Sprintf("proto-%d,", r.l4Prot.val&r.l4Prot.valid)
	}

	if r.l4Dst.valid != 0 {
		ks += fmt.Sprintf("dport-%d,", r.l4Dst.val&r.l4Dst.valid)
	}

	if r.l4Src.valid != 0 {
		ks += fmt.Sprintf("sport-%d,", r.l4Src.val&r.l4Src.valid)
	}

	if checkValidMACTuple(r.inL2Dst) {
		ks += fmt.Sprintf("idmac-%02x:%02x:%02x:%02x:%02x:%02x,",
			r.inL2Dst.addr[0]&r.inL2Dst.valid[0],
			r.inL2Dst.addr[1]&r.inL2Dst.valid[1],
			r.inL2Dst.addr[2]&r.inL2Dst.valid[2],
			r.inL2Dst.addr[3]&r.inL2Dst.valid[3],
			r.inL2Dst.addr[4]&r.inL2Dst.valid[4],
			r.inL2Dst.addr[5]&r.inL2Dst.valid[5])
	}

	if checkValidMACTuple(r.inL2Src) {
		ks += fmt.Sprintf("ismac-%02x:%02x:%02x:%02x:%02x:%02x,",
			r.inL2Src.addr[0]&r.inL2Src.valid[0],
			r.inL2Src.addr[1]&r.inL2Src.valid[1],
			r.inL2Src.addr[2]&r.inL2Src.valid[2],
			r.inL2Src.addr[3]&r.inL2Src.valid[3],
			r.inL2Src.addr[4]&r.inL2Src.valid[4],
			r.inL2Src.addr[5]&r.inL2Src.valid[5])
	}

	if r.inL3Dst.addr.String() != "<nil>" {
		ks += fmt.Sprintf("idst-%s,", r.inL3Dst.addr.String())
	}

	if r.inL3Src.addr.String() != "<nil>" {
		ks += fmt.Sprintf("isrc-%s,", r.inL3Src.addr.String())
	}

	if r.inL4Prot.valid != 0 {
		ks += fmt.Sprintf("iproto-%d,", r.inL4Prot.val&r.inL4Prot.valid)
	}

	if r.inL4Dst.valid != 0 {
		ks += fmt.Sprintf("idport-%d,", r.inL4Dst.val&r.inL4Dst.valid)
	}

	if r.inL4Src.valid != 0 {
		ks += fmt.Sprintf("isport-%d,", r.inL4Src.val&r.inL4Src.valid)
	}

	return ks
}

func (a *ruleAct) String() string {
	var ks string

	if a.actType == RtActDrop {
		ks += fmt.Sprintf("%s", "drop")
	} else if a.actType == RtActFwd {
		ks += fmt.Sprintf("%s", "allow")
	} else if a.actType == RtActTrap {
		ks += fmt.Sprintf("%s", "trap")
	} else if a.actType == RtActDnat ||
		a.actType == RtActSnat ||
		a.actType == RtActFullNat ||
		a.actType == RtActFullProxy {
		if a.actType == RtActSnat {
			ks += fmt.Sprintf("%s", "do-snat:")
		} else if a.actType == RtActDnat {
			ks += fmt.Sprintf("%s", "do-dnat:")
		} else if a.actType == RtActFullProxy {
			ks += fmt.Sprintf("%s", "do-fullproxy:")
		} else {
			ks += fmt.Sprintf("%s", "do-fullnat:")
		}

		switch na := a.action.(type) {
		case *ruleNatActs:
			if na.mode == cmn.LBModeOneArm {
				ks += fmt.Sprintf("%s", "onearm:")
			} else if na.mode == cmn.LBModeHostOneArm {
				ks += fmt.Sprintf("%s", "armhost:")
			}
			for _, n := range na.endPoints {
				if len(n.foldEndPoints) > 0 {
					for _, nf := range n.foldEndPoints {
						ks += fmt.Sprintf("feip-%s,fep-%d,fw-%d,",
							nf.xIP.String(), nf.xPort, nf.weight)
						if nf.inActive || nf.noService {
							ks += fmt.Sprintf("dead|")
						} else {
							ks += fmt.Sprintf("alive|")
						}
					}
				} else {
					ks += fmt.Sprintf("eip-%s,ep-%d,w-%d,",
						n.xIP.String(), n.xPort, n.weight)
					if n.inActive || n.noService {
						ks += fmt.Sprintf("dead|")
					} else {
						ks += fmt.Sprintf("alive|")
					}
				}
			}
		case *ruleFwOpts:
			if a.actType == RtActSnat {
				ks += fmt.Sprintf("%s:%d", na.opt.snatIP, na.opt.snatPort)
			}
		}
	}

	return ks
}

// Rules2Json - output all rules into json and write to the byte array
func (R *RuleH) Rules2Json() ([]byte, error) {
	var t cmn.LbServiceArg
	var eps []cmn.LbEndPointArg
	var ret cmn.LbRuleMod
	var bret []byte
	for _, data := range R.tables[RtLB].eMap {
		// Make Service Arguments
		t.ServIP = data.tuples.l3Dst.addr.IP.String()
		if data.tuples.l4Prot.val == 6 {
			t.Proto = "tcp"
		} else if data.tuples.l4Prot.val == 17 {
			t.Proto = "udp"
		} else if data.tuples.l4Prot.val == 1 {
			t.Proto = "icmp"
		} else if data.tuples.l4Prot.val == 132 {
			t.Proto = "sctp"
		} else {
			return nil, errors.New("malformed service proto")
		}
		t.ServPort = data.tuples.l4Dst.val
		t.Sel = data.act.action.(*ruleNatActs).sel
		t.Mode = data.act.action.(*ruleNatActs).mode

		// Make Endpoints
		tmpEp := data.act.action.(*ruleNatActs).endPoints
		for _, ep := range tmpEp {
			eps = append(eps, cmn.LbEndPointArg{
				EpIP:   ep.xIP.String(),
				EpPort: ep.xPort,
				Weight: ep.weight,
			})
		}
		// Make LB rule
		ret.Serv = t
		ret.Eps = eps

		js, err := json.Marshal(ret)
		if err != nil {
			return nil, err
		}
		bret = append(bret, js...)
		fmt.Printf("js: %v\n", js)
		fmt.Println(string(js))
	}

	return bret, nil
}

// GetNatLbRule - get all rules and pack them into a cmn.LbRuleMod slice
func (R *RuleH) GetNatLbRule() ([]cmn.LbRuleMod, error) {
	var res []cmn.LbRuleMod

	for _, data := range R.tables[RtLB].eMap {
		var ret cmn.LbRuleMod
		// Make Service Arguments
		ret.Serv.ServIP = data.tuples.l3Dst.addr.IP.String()
		if data.tuples.l4Prot.val == 6 {
			ret.Serv.Proto = "tcp"
		} else if data.tuples.l4Prot.val == 17 {
			ret.Serv.Proto = "udp"
		} else if data.tuples.l4Prot.val == 1 {
			ret.Serv.Proto = "icmp"
		} else if data.tuples.l4Prot.val == 132 {
			ret.Serv.Proto = "sctp"
		} else if data.tuples.l4Prot.val == 0 {
			ret.Serv.Proto = "none"
		} else {
			return []cmn.LbRuleMod{}, errors.New("malformed service proto")
		}
		ret.Serv.ServPort = data.tuples.l4Dst.val
		ret.Serv.Sel = data.act.action.(*ruleNatActs).sel
		ret.Serv.Mode = data.act.action.(*ruleNatActs).mode
		ret.Serv.Monitor = data.hChk.actChk
		ret.Serv.InactiveTimeout = data.iTO
		ret.Serv.Bgp = data.bgp
		ret.Serv.BlockNum = data.tuples.pref
		ret.Serv.Managed = data.managed
		ret.Serv.Security = data.secMode
		ret.Serv.ProbeType = data.hChk.prbType
		ret.Serv.ProbePort = data.hChk.prbPort
		ret.Serv.ProbeReq = data.hChk.prbReq
		ret.Serv.ProbeResp = data.hChk.prbResp
		ret.Serv.Name = data.name
		ret.Serv.HostUrl = data.tuples.path
		if data.act.actType == RtActSnat {
			ret.Serv.Snat = true
		}

		for _, sip := range data.secIP {
			ret.SecIPs = append(ret.SecIPs, cmn.LbSecIPArg{SecIP: sip.sIP.String()})
		}

		data.DP(DpStatsGetImm)

		// Make Endpoints
		tmpEp := data.act.action.(*ruleNatActs).endPoints
		for _, ep := range tmpEp {
			state := "active"
			if ep.noService {
				state = "inactive"
			}

			if ep.inActive {
				continue
			}

			counterStr := fmt.Sprintf("%v:%v", ep.stat.packets, ep.stat.bytes)

			ret.Eps = append(ret.Eps, cmn.LbEndPointArg{
				EpIP:     ep.xIP.String(),
				EpPort:   ep.xPort,
				Weight:   ep.weight,
				State:    state,
				Counters: counterStr,
			})
		}
		// Make LB rule
		res = append(res, ret)
	}

	return res, nil
}

// validateXlateEPWeights - validate and adjust weights if necessary
func validateXlateEPWeights(servEndPoints []cmn.LbEndPointArg) (int, error) {
	sum := 0
	for _, se := range servEndPoints {
		sum += int(se.Weight)
	}

	if sum > 100 {
		return -1, errors.New("malformed-weight error")
	} else if sum < 100 {
		rem := (100 - sum) / len(servEndPoints)
		for idx := range servEndPoints {
			pSe := &servEndPoints[idx]
			pSe.Weight += uint8(rem)
		}
	}

	return 0, nil
}

func (R *RuleH) modNatEpHost(r *ruleEnt, endpoints []ruleNatEp, doAddOp bool, liveCheckEn bool) {
	var hopts epHostOpts
	pType := ""
	pPort := uint16(0)
	if r.hChk.prbRetries == 0 {
		hopts.inActTryThr = DflLbaInactiveTries
	} else {
		hopts.inActTryThr = r.hChk.prbRetries
	}
	if r.hChk.prbTimeo == 0 {
		hopts.probeDuration = DflHostProbeTimeout
	} else {
		hopts.probeDuration = r.hChk.prbTimeo
	}
	for _, nep := range endpoints {
		if r.tuples.l4Prot.val == 6 {
			pType = HostProbeConnectTCP
			pPort = nep.xPort
		} else if r.tuples.l4Prot.val == 17 {
			pType = HostProbeConnectUDP
			pPort = nep.xPort
		} else if r.tuples.l4Prot.val == 1 {
			pType = HostProbePing
		} else if r.tuples.l4Prot.val == 132 {
			pType = HostProbeConnectSCTP
			pPort = nep.xPort
		} else {
			pType = HostProbePing
		}

		if r.hChk.prbType != "" {
			// If probetype is specified as a part of rule,
			// override per end-point liveness settings
			hopts.probeType = r.hChk.prbType
			hopts.probePort = r.hChk.prbPort
			hopts.probeReq = r.hChk.prbReq
			hopts.probeResp = r.hChk.prbResp
		} else {
			hopts.probeType = pType
			hopts.probePort = pPort
		}

		if mh.pProbe == true || liveCheckEn {
			hopts.probeActivated = true
		}

		epKey := makeEPKey(nep.xIP.String(), pType, pPort)

		if doAddOp {
			if nep.inActive != true {
				R.AddEPHost(false, nep.xIP.String(), epKey, hopts)
			}
		} else {
			R.DeleteEPHost(false, epKey, nep.xIP.String(), hopts.probeType, hopts.probePort)
		}
	}
}

// GetNatLbRuleByID - Get a NAT rule by its identifier
func (R *RuleH) GetNatLbRuleByID(ruleID uint32) *ruleEnt {
	if ruleID < RtMaximumLbs {
		return R.tables[RtLB].rArr[ruleID]
	}

	return nil
}

// GetNatLbRuleByServArgs - Get a NAT rule by its service args
func (R *RuleH) GetNatLbRuleByServArgs(serv cmn.LbServiceArg) *ruleEnt {
	var ipProto uint8
	service := ""
	if tk.IsNetIPv4(serv.ServIP) {
		service = serv.ServIP + "/32"
	} else {
		service = serv.ServIP + "/128"
	}
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return nil
	}

	if serv.Proto == "tcp" {
		ipProto = 6
	} else if serv.Proto == "udp" {
		ipProto = 17
	} else if serv.Proto == "icmp" {
		ipProto = 1
	} else if serv.Proto == "sctp" {
		ipProto = 132
	} else if serv.Proto == "none" {
		ipProto = 0
	} else {
		return nil
	}

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: serv.BlockNum, path: serv.HostUrl}
	return R.tables[RtLB].eMap[rt.ruleKey()]
}

// GetNatLbRuleSecIPs - Get secondary IPs for SCTP NAT rule by its service args
func (R *RuleH) GetNatLbRuleSecIPs(serv cmn.LbServiceArg) []string {
	var ipProto uint8
	var ips []string
	service := ""
	if tk.IsNetIPv4(serv.ServIP) {
		service = serv.ServIP + "/32"
	} else {
		service = serv.ServIP + "/128"
	}
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return nil
	}

	if serv.Proto == "sctp" {
		ipProto = 132
	} else {
		return nil
	}

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: serv.BlockNum, path: serv.HostUrl}
	if R.tables[RtLB].eMap[rt.ruleKey()] != nil {
		for _, ip := range R.tables[RtLB].eMap[rt.ruleKey()].secIP {
			ips = append(ips, ip.sIP.String())
		}
	}
	return ips
}

func (R *RuleH) electEPSrc(r *ruleEnt) bool {
	var sip net.IP
	var e int
	chg := false
	mode := "default"
	addrRslv := false

	switch na := r.act.action.(type) {
	case *ruleNatActs:
		{
			for idx := range na.endPoints {
				np := &na.endPoints[idx]

				if np.foldRuleKey != "" {
					fr := R.tables[RtLB].eMap[np.foldRuleKey]
					if fr == nil || fr.addrRslv {
						addrRslv = true
						continue
					}
				}
				sip = np.rIP
				if na.mode == cmn.LBModeOneArm || na.mode == cmn.LBModeHostOneArm {
					mode = "onearm"
					e, sip, _ = R.zone.L3.IfaSelectAny(np.xIP, true)
					if e != 0 {
						tk.LogIt(tk.LogDebug, "Failed to find suitable source for %s\n", np.xIP.String())
						addrRslv = true
					}
					if np.xIP.Equal(sip) {
						sip = net.IPv4(0, 0, 0, 0)
					}
				} else if na.mode == cmn.LBModeFullNAT {
					mode = "fullnat"
					if !mh.has.IsCIKAMode() {
						sip = r.RuleVIP2PrivIP()
						if np.xIP.Equal(sip) {
							sip = net.IPv4(0, 0, 0, 0)
						} else if utils.IsIPHostAddr(np.xIP.String()) {
							sip = net.IPv4(0, 0, 0, 0)
						}
					} else {
						vip, err := mh.has.CIVipGet(r.ci)
						if err == nil {
							tk.LogIt(tk.LogDebug, "vip for %s: %s\n", r.ci, vip.String())
							sip = vip
						} else {
							tk.LogIt(tk.LogError, "vip for %s not found \n", r.ci)
							addrRslv = true
						}
					}
				} else {
					serviceIP := r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
					if tk.IsNetIPv6(serviceIP.String()) && tk.IsNetIPv4(np.xIP.String()) {
						e, sip, _ = r.zone.L3.IfaSelectAny(np.xIP, false)
						if e != 0 {
							addrRslv = true
						}
					} else {
						sip = net.IPv4(0, 0, 0, 0)
					}
				}

				if !np.rIP.Equal(sip) {
					np.rIP = sip
					chg = true
					tk.LogIt(tk.LogDebug, "%s:suitable source for %s: %s\n", mode, np.xIP.String(), np.rIP.String())
				}
			}
		}
	}
	r.addrRslv = addrRslv
	return chg
}

func (R *RuleH) mkHostAssocs(r *ruleEnt) bool {
	chg := false
	curLocIPS := make(map[string]int)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return chg
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// check if IPv4 or IPv6 is not nil
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				if tk.IsNetIPv4(ipnet.IP.String()) && r.tuples.l3Dst.addr.IP.String() != ipnet.IP.String() {
					if _, found := curLocIPS[ipnet.IP.String()]; !found {
						curLocIPS[ipnet.IP.String()] = 0
					}
				}
			}
		}
	}

	for locIP := range r.locIPs {
		if _, found := curLocIPS[locIP]; found {
			curLocIPS[locIP]++
		} else {
			curLocIPS[locIP] = -1
		}
	}

	for clocIP, exists := range curLocIPS {
		if exists == 0 {
			chg = true
			r.locIPs[clocIP] = struct{}{}
			tk.LogIt(tk.LogInfo, "%s: added loc %s\n", r.tuples.String(), clocIP)
		} else if exists < 0 {
			chg = true
			delete(r.locIPs, clocIP)
			tk.LogIt(tk.LogInfo, "%s: deleted loc %s\n", r.tuples.String(), clocIP)
		}
	}

	return chg
}

func (R *RuleH) syncEPHostState2Rule(rule *ruleEnt, checkNow bool) bool {
	var sType string
	rChg := false
	if checkNow || time.Duration(time.Now().Sub(rule.sT).Seconds()) >= time.Duration(R.cfg.RuleInactChkTime) {
		switch na := rule.act.action.(type) {
		case *ruleNatActs:
			if rule.tuples.l4Prot.val == 6 {
				sType = HostProbeConnectTCP
			} else if rule.tuples.l4Prot.val == 17 {
				sType = HostProbeConnectUDP
			} else if rule.tuples.l4Prot.val == 1 {
				sType = HostProbePing
			} else if rule.tuples.l4Prot.val == 132 {
				sType = HostProbeConnectSCTP
			} else {
				return rChg
			}

			for idx, n := range na.endPoints {
				sOk := R.IsEPHostActive(makeEPKey(n.xIP.String(), sType, n.xPort))
				np := &na.endPoints[idx]
				if sOk == false {
					if np.noService == false {
						np.noService = true
						rChg = true
						tk.LogIt(tk.LogDebug, "nat lb-rule service-down ep - %s:%s\n", sType, n.xIP.String())
					}
				} else {
					if n.noService {
						np.noService = false
						np.inActTries = 0
						rChg = true
						tk.LogIt(tk.LogDebug, "nat lb-rule service-up ep - %s:%s\n", sType, n.xIP.String())
					}
				}
			}
			rule.sT = time.Now()
		}
	}

	return rChg
}

// foldRecursiveEPs - Check if this rule's key matches endpoint of another rule.
// If so, replace that rule's endpoints to this rule's endpoints
func (R *RuleH) foldRecursiveEPs(r *ruleEnt) {

	for _, tr := range R.tables[RtLB].eMap {
		switch atr := r.act.action.(type) {
		case *ruleNatActs:
			for i := range atr.endPoints {
				rep := &atr.endPoints[i]
				service := ""
				if tk.IsNetIPv4(rep.xIP.String()) {
					service = rep.xIP.String() + "/32"
				} else {
					service = rep.xIP.String() + "/128"
				}
				_, sNetAddr, err := net.ParseCIDR(service)
				if err != nil {
					continue
				}
				l4prot := rule8Tuple{r.tuples.l4Prot.val, 0xff}
				l3dst := ruleIPTuple{*sNetAddr}
				l4dst := rule16Tuple{rep.xPort, 0xffff}
				rtk := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: r.tuples.pref}
				if rtk.ruleKey() == tr.tuples.ruleKey() {
					rep.foldEndPoints = tr.act.action.(*ruleNatActs).endPoints
					rep.foldRuleKey = tr.tuples.ruleKey()
				}
			}
		}

		switch at := tr.act.action.(type) {
		case *ruleNatActs:
			if r.act.action.(*ruleNatActs).sel != at.sel || r.act.action.(*ruleNatActs).sel == cmn.LbSelPrio {
				continue
			}
			fold := false
			for i := range at.endPoints {
				ep := &at.endPoints[i]
				service := ""
				if tk.IsNetIPv4(ep.xIP.String()) {
					service = ep.xIP.String() + "/32"
				} else {
					service = ep.xIP.String() + "/128"
				}
				_, sNetAddr, err := net.ParseCIDR(service)
				if err != nil {
					continue
				}

				l4prot := rule8Tuple{r.tuples.l4Prot.val, 0xff}
				l3dst := ruleIPTuple{*sNetAddr}
				l4dst := rule16Tuple{ep.xPort, 0xffff}
				rtk := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: r.tuples.pref}
				if r.tuples.ruleKey() == rtk.ruleKey() {
					ep.foldEndPoints = r.act.action.(*ruleNatActs).endPoints
					ep.foldRuleKey = r.tuples.ruleKey()
					fold = true
				}
				if fold {
					tr.DP(DpCreate)
					tk.LogIt(tk.LogDebug, "nat lb-rule folded - %d:%s-%s\n", tr.ruleNum, tr.tuples.String(), tr.act.String())
				}
			}
		}
	}
}

// unFoldRecursiveEPs - Check if this rule's key matches endpoint of another rule.
// If so, replace that rule's original endpoint
func (R *RuleH) unFoldRecursiveEPs(r *ruleEnt) {

	selPolicy := cmn.LbSelRr
	switch at := r.act.action.(type) {
	case *ruleNatActs:
		selPolicy = at.sel
	}

	for _, tr := range R.tables[RtLB].eMap {
		if tr == r {
			continue
		}
		switch atr := r.act.action.(type) {
		case *ruleNatActs:
			for i := range atr.endPoints {
				rep := &atr.endPoints[i]
				if rep.foldRuleKey == tr.tuples.ruleKey() {
					rep.foldEndPoints = nil
					rep.foldRuleKey = ""
				}
			}
		}
		switch at := tr.act.action.(type) {
		case *ruleNatActs:
			if selPolicy != at.sel || selPolicy == cmn.LbSelPrio {
				continue
			}
			for i := range at.endPoints {
				ep := &at.endPoints[i]
				if r.tuples.ruleKey() == ep.foldRuleKey {
					ep.foldEndPoints = nil
					ep.foldRuleKey = ""
					tr.DP(DpCreate)
					tk.LogIt(tk.LogDebug, "nat lb-rule unfolded - %d:%s-%s\n", tr.ruleNum, tr.tuples.String(), tr.act.String())
				}
			}
		}
	}
}

// addVIPSys - system specific operations for VIPs of a LB rule
func (R *RuleH) addVIPSys(r *ruleEnt) {
	if r.act.actType != RtActSnat && !strings.Contains(r.name, "ipvs") && !strings.Contains(r.name, "static") {
		R.AddRuleVIP(r.tuples.l3Dst.addr.IP, r.RuleVIP2PrivIP())

		// Take care of any secondary VIPs
		for _, sVIP := range r.secIP {
			R.AddRuleVIP(sVIP.sIP, sVIP.sIP)
		}
	}
}

func getLBArms(oldEps []ruleNatEp, newEps []ruleNatEp, oper cmn.LBOp) (bool, []ruleNatEp) {
	var retEps []ruleNatEp
	ruleChg := false
	found := false

	for i, eEp := range oldEps {
		for j, nEp := range newEps {
			if eEp.xIP.Equal(nEp.xIP) &&
				eEp.xPort == nEp.xPort {
				e := &oldEps[i]
				n := &newEps[j]
				if eEp.inActive && oper != cmn.LBOPDetach {
					ruleChg = true
					e.inActive = false
				}
				if e.weight != nEp.weight {
					ruleChg = true
					e.weight = nEp.weight
				}
				e.chkVal = true
				n.chkVal = true
				found = true
				break
			}
		}
	}

	// Remove LB arms from an existing LB
	if oper == cmn.LBOPDetach {
		if !found {
			return false, oldEps
		}
		for i := range oldEps {
			e := &oldEps[i]
			if !e.chkVal {
				retEps = append(retEps, *e)
			}
		}
		return true, retEps
	}

	retEps = oldEps

	// Attach LB arms to an existing LB
	for i, nEp := range newEps {
		n := &newEps[i]
		if !nEp.chkVal {
			ruleChg = true
			n.chkVal = true
			retEps = append(retEps, *n)
		}
	}

	for i, eEp := range retEps {
		e := &retEps[i]
		if !eEp.chkVal && oper == cmn.LBOPAdd {
			ruleChg = true
			e.inActive = true
		}
		e.chkVal = false
	}
	return ruleChg, retEps
}

// AddNatLbRule - Add a service LB nat rule. The service details are passed in serv argument,
// and end-point information is passed in the slice servEndPoints. On success,
// it will return 0 and nil error, else appropriate return code and error string will be set
func (R *RuleH) AddNatLbRule(serv cmn.LbServiceArg, servSecIPs []cmn.LbSecIPArg, servEndPoints []cmn.LbEndPointArg) (int, error) {
	var natActs ruleNatActs
	var nSecIP []ruleNatSIP
	var ipProto uint8
	var privIP net.IP

	// Validate service args
	service := ""
	if tk.IsNetIPv4(serv.ServIP) {
		service = serv.ServIP + "/32"
	} else {
		service = serv.ServIP + "/128"
	}
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return RuleUnknownServiceErr, errors.New("malformed-service error")
	}

	privIP = nil
	if serv.PrivateIP != "" {
		privIP = net.ParseIP(serv.PrivateIP)
		if privIP == nil {
			return RuleUnknownServiceErr, errors.New("malformed-service privateIP error")
		}
	}

	// Validate inactivity timeout
	if serv.InactiveTimeout > LbMaxInactiveTimeout {
		return RuleArgsErr, errors.New("service-args error")
	} else if serv.InactiveTimeout == 0 {
		serv.InactiveTimeout = LbDefaultInactiveTimeout
		if serv.Proto != "tcp" && serv.Proto != "sctp" {
			serv.InactiveTimeout = LbDefaultInactiveNSTimeout
		}
	}

	// Validate liveness probetype and port
	if serv.ProbeType != "" {
		if serv.ProbeType != HostProbeConnectSCTP &&
			serv.ProbeType != HostProbeConnectTCP &&
			serv.ProbeType != HostProbeConnectUDP &&
			serv.ProbeType != HostProbePing &&
			serv.ProbeType != HostProbeNone {
			return RuleArgsErr, errors.New("malformed-service-ptype error")
		}

		if (serv.ProbeType == HostProbeConnectSCTP ||
			serv.ProbeType == HostProbeConnectTCP ||
			serv.ProbeType == HostProbeConnectUDP) &&
			(serv.ProbePort == 0) {
			return RuleArgsErr, errors.New("malformed-service-pport error")
		}

		if (serv.ProbeType == HostProbeNone || serv.ProbeType == HostProbePing) &&
			(serv.ProbePort != 0) {
			return RuleArgsErr, errors.New("malformed-service-pport error")
		}

		// Override monitor flag to true if certain conditions meet
		if serv.ProbeType != HostProbeNone {
			serv.Monitor = true
		}
	} else if serv.ProbePort != 0 {
		return RuleArgsErr, errors.New("malformed-service-pport error")
	}

	// Currently support a maximum of MAX_NAT_EPS
	if len(servEndPoints) <= 0 || len(servEndPoints) > MaxNatEndPoints {
		return RuleEpCountErr, errors.New("endpoints-range error")
	}

	// For ICMP service, non-zero port can't be specified
	if serv.Proto == "icmp" && serv.ServPort != 0 {
		return RuleUnknownServiceErr, errors.New("malformed-service error")
	}

	if serv.Proto == "tcp" {
		ipProto = 6
	} else if serv.Proto == "udp" {
		ipProto = 17
	} else if serv.Proto == "icmp" {
		ipProto = 1
	} else if serv.Proto == "sctp" {
		ipProto = 132
	} else if serv.Proto == "none" {
		ipProto = 0
	} else {
		return RuleUnknownServiceErr, errors.New("malformed-proto error")
	}

	if serv.Proto != "sctp" && len(servSecIPs) > 0 {
		return RuleArgsErr, errors.New("secondaryIP-args error")
	}

	if len(servSecIPs) > 3 {
		return RuleArgsErr, errors.New("secondaryIP-args len error")
	}

	activateProbe := false

	for _, k := range servSecIPs {
		pNetAddr := net.ParseIP(k.SecIP)
		if pNetAddr == nil {
			return RuleUnknownServiceErr, errors.New("malformed-secIP error")
		}
		if tk.IsNetIPv4(serv.ServIP) && tk.IsNetIPv6(k.SecIP) {
			return RuleUnknownServiceErr, errors.New("malformed-secIP nat46 error")
		}
		sip := ruleNatSIP{pNetAddr}
		nSecIP = append(nSecIP, sip)
	}

	sort.SliceStable(nSecIP, func(i, j int) bool {
		a := tk.IPtonl(nSecIP[i].sIP)
		b := tk.IPtonl(nSecIP[j].sIP)
		return a < b
	})

	if serv.Mode == cmn.LBModeHostOneArm && !sNetAddr.IP.IsUnspecified() {
		tk.LogIt(tk.LogInfo, "nat lb-rule %s-%v-%s hostarm needs unspec VIP\n", serv.ServIP, serv.ServPort, serv.Proto)
		return RuleArgsErr, errors.New("hostarm-args error")
	}

	natActs.sel = serv.Sel
	natActs.mode = cmn.LBMode(serv.Mode)

	if natActs.mode == cmn.LBModeOneArm || natActs.mode == cmn.LBModeFullNAT || natActs.mode == cmn.LBModeHostOneArm || serv.Monitor {
		activateProbe = true
	}

	for _, k := range servEndPoints {
		pNetAddr := net.ParseIP(k.EpIP)
		xNetAddr := net.IPv4(0, 0, 0, 0)
		if pNetAddr == nil {
			return RuleUnknownEpErr, errors.New("malformed-lbep error")
		}
		if tk.IsNetIPv4(serv.ServIP) && tk.IsNetIPv6(k.EpIP) {
			return RuleUnknownServiceErr, errors.New("malformed-service nat46 error")
		}
		if serv.Proto == "icmp" && k.EpPort != 0 {
			return RuleUnknownServiceErr, errors.New("malformed-service error")
		}

		if natActs.mode == cmn.LBModeDSR && k.EpPort != serv.ServPort {
			return RuleUnknownServiceErr, errors.New("malformed-service dsr-port error")
		}
		ep := ruleNatEp{pNetAddr, xNetAddr, k.EpPort, k.Weight, 0, false, false, false, ruleStat{0, 0}, nil, ""}
		natActs.endPoints = append(natActs.endPoints, ep)
	}

	sort.SliceStable(natActs.endPoints, func(i, j int) bool {
		a := tk.IPtonl(natActs.endPoints[i].xIP)
		b := tk.IPtonl(natActs.endPoints[j].xIP)
		return a < b
	})

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: serv.BlockNum, path: serv.HostUrl}

	eRule := R.tables[RtLB].eMap[rt.ruleKey()]

	if eRule != nil {
		if !reflect.DeepEqual(eRule.secIP, nSecIP) {
			return RuleUnknownServiceErr, errors.New("secIP modify error")
		}
		// If a NAT rule already exists, we try not reschuffle the order of the end-points.
		// We will try to append the new end-points at the end, while marking any other end-points
		// not in the new list as inactive
		ruleChg, retEps := getLBArms(eRule.act.action.(*ruleNatActs).endPoints, natActs.endPoints, serv.Oper)

		if eRule.hChk.prbType != serv.ProbeType || eRule.hChk.prbPort != serv.ProbePort ||
			eRule.hChk.prbReq != serv.ProbeReq || eRule.hChk.prbResp != serv.ProbeResp ||
			eRule.pTO != serv.PersistTimeout || eRule.act.action.(*ruleNatActs).sel != natActs.sel ||
			eRule.act.action.(*ruleNatActs).mode != natActs.mode {
			ruleChg = true
		}

		if !ruleChg {
			return RuleExistsErr, errors.New("lbrule-exists error")
		}

		if eRule.secMode != serv.Security {
			return RuleExistsErr, errors.New("lbrule-exist error: cant modify rule security mode")
		}

		if len(retEps) == 0 {
			tk.LogIt(tk.LogDebug, "nat lb-rule %s has no-endpoints: to be deleted\n", eRule.tuples.String())
			return R.DeleteNatLbRule(serv)
		}

		if eRule.act.action.(*ruleNatActs).mode == cmn.LBModeFullProxy && natActs.mode != cmn.LBModeFullProxy ||
			eRule.act.action.(*ruleNatActs).mode != cmn.LBModeFullProxy && natActs.mode == cmn.LBModeFullProxy {
			return RuleExistsErr, errors.New("lbrule-exist error: cant modify fullproxy rule mode")
		}

		if eRule.act.action.(*ruleNatActs).mode == cmn.LBModeFullProxy {
			eRule.DP(DpRemove)
		}

		// Update the rule
		eRule.hChk.prbType = serv.ProbeType
		eRule.hChk.prbPort = serv.ProbePort
		eRule.hChk.prbReq = serv.ProbeReq
		eRule.hChk.prbResp = serv.ProbeResp
		eRule.hChk.prbRetries = serv.ProbeRetries
		eRule.hChk.prbTimeo = serv.ProbeTimeout
		eRule.pTO = serv.PersistTimeout
		eRule.act.action.(*ruleNatActs).sel = natActs.sel
		eRule.act.action.(*ruleNatActs).endPoints = retEps
		eRule.act.action.(*ruleNatActs).mode = natActs.mode
		// Managed flag can't be modified on the fly
		// eRule.managed = serv.Managed

		if !serv.Snat {
			R.modNatEpHost(eRule, retEps, true, activateProbe)
			R.electEPSrc(eRule)
		}

		eRule.sT = time.Now()
		eRule.iTO = serv.InactiveTimeout
		tk.LogIt(tk.LogDebug, "nat lb-rule updated - %s:%s\n", eRule.tuples.String(), eRule.act.String())
		eRule.DP(DpCreate)

		return 0, nil
	} else if serv.Oper == cmn.LBOPDetach {
		tk.LogIt(tk.LogInfo, "nat lb-rule %s-%v-%s does not exist\n", serv.ServIP, serv.ServPort, serv.Proto)
		return RuleNotExistsErr, errors.New("lbrule not-exists error")
	}

	r := new(ruleEnt)
	r.tuples = rt
	r.zone = R.zone
	r.name = serv.Name
	if serv.Snat {
		r.act.actType = RtActSnat
	} else if serv.Mode == cmn.LBModeFullNAT || serv.Mode == cmn.LBModeOneArm || serv.Mode == cmn.LBModeHostOneArm {
		r.act.actType = RtActFullNat
	} else if serv.Mode == cmn.LBModeFullProxy {
		r.act.actType = RtActFullProxy
	} else {
		r.act.actType = RtActDnat
	}
	r.managed = serv.Managed
	r.secIP = nSecIP
	r.secMode = serv.Security
	// Per LB end-point health-check is supposed to be handled at kube-loxilb/CCM,
	// but it certain cases like stand-alone mode, loxilb can do its own
	// lb end-point health monitoring
	r.hChk.prbType = serv.ProbeType
	r.hChk.prbPort = serv.ProbePort
	r.hChk.prbReq = serv.ProbeReq
	r.hChk.prbResp = serv.ProbeResp
	r.hChk.prbRetries = serv.ProbeRetries
	r.hChk.prbTimeo = serv.ProbeTimeout
	r.hChk.actChk = serv.Monitor

	r.act.action = &natActs
	r.ruleNum, err = R.tables[RtLB].Mark.GetCounter()
	if err != nil {
		tk.LogIt(tk.LogError, "nat lb-rule - %s:%s hwm error\n", r.tuples.String(), r.act.String())
		return RuleAllocErr, errors.New("rule-hwm error")
	}
	r.sT = time.Now()
	r.iTO = serv.InactiveTimeout
	r.bgp = serv.Bgp
	r.ci = cmn.CIDefault
	r.privIP = privIP
	r.pTO = 0
	if serv.Sel == cmn.LbSelRrPersist {
		if serv.PersistTimeout == 0 || serv.PersistTimeout > 24*60*60 {
			r.pTO = DefaultPersistTimeOut
		} else {
			r.pTO = serv.PersistTimeout
		}
	}
	r.locIPs = make(map[string]struct{})

	if !serv.Snat {
		R.foldRecursiveEPs(r)
		R.modNatEpHost(r, natActs.endPoints, true, activateProbe)
		R.electEPSrc(r)
		if serv.Mode == cmn.LBModeHostOneArm {
			R.mkHostAssocs(r)
		}
	}

	tk.LogIt(tk.LogDebug, "nat lb-rule added - %d:%s-%s\n", r.ruleNum, r.tuples.String(), r.act.String())

	R.tables[RtLB].eMap[rt.ruleKey()] = r
	if r.ruleNum < RtMaximumLbs {
		R.tables[RtLB].rArr[r.ruleNum] = r
	}
	R.addVIPSys(r)
	r.DP(DpCreate)

	return 0, nil
}

// deleteVIPSys - system specific operations for deleting VIPs of a LB rule
func (R *RuleH) deleteVIPSys(r *ruleEnt) {
	if r.act.actType != RtActSnat && !strings.Contains(r.name, "ipvs") && !strings.Contains(r.name, "static") {
		R.DeleteRuleVIP(r.tuples.l3Dst.addr.IP)

		// Take care of any secondary VIPs
		for _, sVIP := range r.secIP {
			R.DeleteRuleVIP(sVIP.sIP)
		}
	}
}

// DeleteNatLbRule - Delete a service LB nat rule. The service details are passed in serv argument.
// On success, it will return 0 and nil error, else appropriate return code and
// error string will be set
func (R *RuleH) DeleteNatLbRule(serv cmn.LbServiceArg) (int, error) {
	var ipProto uint8

	service := ""
	if tk.IsNetIPv4(serv.ServIP) {
		service = serv.ServIP + "/32"
	} else {
		service = serv.ServIP + "/128"
	}
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return RuleUnknownServiceErr, errors.New("malformed-service error")
	}

	if serv.Proto == "tcp" {
		ipProto = 6
	} else if serv.Proto == "udp" {
		ipProto = 17
	} else if serv.Proto == "icmp" {
		ipProto = 1
	} else if serv.Proto == "sctp" {
		ipProto = 132
	} else if serv.Proto == "none" {
		ipProto = 0
	} else {
		return RuleUnknownServiceErr, errors.New("malformed-proto error")
	}

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst, pref: serv.BlockNum, path: serv.HostUrl}

	rule := R.tables[RtLB].eMap[rt.ruleKey()]
	if rule == nil {
		return RuleNotExistsErr, errors.New("no-rule error")
	}

	defer R.tables[RtLB].Mark.PutCounter(rule.ruleNum)

	eEps := rule.act.action.(*ruleNatActs).endPoints
	activatedProbe := false
	if rule.act.action.(*ruleNatActs).mode == cmn.LBModeOneArm || rule.act.action.(*ruleNatActs).mode == cmn.LBModeFullNAT || rule.act.action.(*ruleNatActs).mode == cmn.LBModeHostOneArm || rule.hChk.actChk {
		activatedProbe = true
	}
	if rule.act.actType != RtActSnat {
		R.modNatEpHost(rule, eEps, false, activatedProbe)
		R.unFoldRecursiveEPs(rule)
	}

	delete(R.tables[RtLB].eMap, rt.ruleKey())
	if rule.ruleNum < RtMaximumLbs {
		R.tables[RtLB].rArr[rule.ruleNum] = nil
	}

	R.deleteVIPSys(rule)

	tk.LogIt(tk.LogDebug, "nat lb-rule deleted %s-%s\n", rule.tuples.String(), rule.act.String())

	rule.DP(DpRemove)

	return 0, nil
}

// GetFwRule - get all Fwrules and pack them into a cmn.FwRuleMod slice
func (R *RuleH) GetFwRule() ([]cmn.FwRuleMod, error) {
	var res []cmn.FwRuleMod

	for _, data := range R.tables[RtFw].eMap {
		var ret cmn.FwRuleMod
		// Make Fw Arguments
		ret.Rule.DstIP = data.tuples.l3Dst.addr.String()
		ret.Rule.SrcIP = data.tuples.l3Src.addr.String()
		if data.tuples.l4Dst.valid == 0xffff {
			ret.Rule.DstPortMin = data.tuples.l4Dst.val
		} else {
			ret.Rule.DstPortMin = data.tuples.l4Dst.valid
		}
		ret.Rule.DstPortMax = data.tuples.l4Dst.val
		if data.tuples.l4Src.valid == 0xffff {
			ret.Rule.SrcPortMin = data.tuples.l4Src.val
		} else {
			ret.Rule.SrcPortMin = data.tuples.l4Src.valid
		}

		ret.Rule.SrcPortMax = data.tuples.l4Src.val
		ret.Rule.Proto = data.tuples.l4Prot.val
		ret.Rule.InPort = data.tuples.port.val
		ret.Rule.Pref = data.tuples.pref

		// Make Fw Opts
		fwOpts := data.act.action.(*ruleFwOpts)
		if fwOpts.op == RtActFwd {
			ret.Opts.Allow = true
		} else if fwOpts.op == RtActDrop {
			ret.Opts.Drop = true
		} else if fwOpts.op == RtActRedirect {
			ret.Opts.Rdr = true
			ret.Opts.RdrPort = fwOpts.opt.rdrPort
		} else if fwOpts.op == RtActTrap {
			ret.Opts.Trap = true
		} else if fwOpts.op == RtActSnat {
			ret.Opts.DoSnat = true
			ret.Opts.ToIP = fwOpts.opt.snatIP
			ret.Opts.ToPort = uint16(fwOpts.opt.snatPort)
		}
		if fwOpts.op != RtActSnat {
			ret.Opts.Mark = fwOpts.opt.fwMark
		}
		ret.Opts.Record = fwOpts.opt.record

		data.Fw2DP(DpStatsGetImm)
		ret.Opts.Counter = fmt.Sprintf("%v:%v", data.stat.packets, data.stat.bytes)

		// Make FwRule
		res = append(res, ret)
	}

	return res, nil
}

// AddFwRule - Add a firewall rule. The rule details are passed in fwRule argument
// it will return 0 and nil error, else appropriate return code and error string will be set
func (R *RuleH) AddFwRule(fwRule cmn.FwRuleArg, fwOptArgs cmn.FwOptArg) (int, error) {
	var fwOpts ruleFwOpts
	var l4src rule16Tuple
	var l4dst rule16Tuple
	var l4prot rule8Tuple

	// Validate rule args
	_, dNetAddr, err := net.ParseCIDR(fwRule.DstIP)
	if err != nil {
		return RuleTupleErr, errors.New("malformed-rule error")
	}

	_, sNetAddr, err := net.ParseCIDR(fwRule.SrcIP)
	if err != nil {
		return RuleTupleErr, errors.New("malformed-rule error")
	}

	l3dst := ruleIPTuple{*dNetAddr}
	l3src := ruleIPTuple{*sNetAddr}

	if fwRule.Proto == 0 {
		l4prot = rule8Tuple{0, 0}
	} else {
		l4prot = rule8Tuple{fwRule.Proto, 0xff}
	}

	if fwRule.SrcPortMax == fwRule.SrcPortMin {
		if fwRule.SrcPortMin == 0 {
			l4src = rule16Tuple{0, 0}
		} else {
			l4src = rule16Tuple{fwRule.SrcPortMin, 0xffff}
		}
	} else {
		l4src = rule16Tuple{fwRule.SrcPortMax, fwRule.SrcPortMin}
	}
	if fwRule.DstPortMax == fwRule.DstPortMin {
		if fwRule.DstPortMin == 0 {
			l4dst = rule16Tuple{0, 0}
		} else {
			l4dst = rule16Tuple{fwRule.DstPortMin, 0xffff}
		}
	} else {
		l4dst = rule16Tuple{fwRule.DstPortMax, fwRule.DstPortMin}
	}
	inport := ruleStringTuple{fwRule.InPort}
	rt := ruleTuples{l3Src: l3src, l3Dst: l3dst, l4Prot: l4prot,
		l4Src: l4src, l4Dst: l4dst, port: inport, pref: fwRule.Pref}

	eFw := R.tables[RtFw].eMap[rt.ruleKey()]

	if eFw != nil {
		// If a FW rule already exists
		return RuleExistsErr, errors.New("fwrule-exists error")
	}

	r := new(ruleEnt)
	r.tuples = rt
	r.zone = R.zone

	/* Default is drop */
	fwOpts.op = RtActDrop
	fwOpts.opt.fwMark = fwOptArgs.Mark
	fwOpts.opt.record = fwOptArgs.Record

	if fwOptArgs.Allow {
		r.act.actType = RtActFwd
		fwOpts.op = RtActFwd
	} else if fwOptArgs.Drop {
		r.act.actType = RtActDrop
		fwOpts.op = RtActDrop
	} else if fwOptArgs.Rdr {
		r.act.actType = RtActRedirect
		fwOpts.op = RtActRedirect
		fwOpts.opt.rdrPort = fwOptArgs.RdrPort
	} else if fwOptArgs.Trap {
		r.act.actType = RtActTrap
		fwOpts.op = RtActTrap
	} else if fwOptArgs.DoSnat {
		r.act.actType = RtActSnat
		fwOpts.op = RtActSnat
		fwOpts.opt.snatIP = fwOptArgs.ToIP
		fwOpts.opt.snatPort = fwOptArgs.ToPort

		if sIP := net.ParseIP(fwOptArgs.ToIP); sIP == nil {
			return RuleArgsErr, errors.New("malformed-args error")
		}

		if fwOpts.opt.fwMark != 0 {
			return RuleArgsErr, errors.New("malformed-args fwmark !=0 for snat-error")
		}
	}

	r.act.action = &fwOpts
	r.ruleNum, err = R.tables[RtFw].Mark.GetCounter()
	if err != nil {
		tk.LogIt(tk.LogError, "fw-rule - %s:%s mark error\n", r.tuples.String(), r.act.String())
		return RuleAllocErr, errors.New("rule-mark error")
	}
	r.sT = time.Now()

	if fwOptArgs.DoSnat {
		// Create SNAT Rule
		var servArg cmn.LbServiceArg
		servArg.ServIP = "0.0.0.0"
		servArg.ServPort = 0
		servArg.Proto = "none"
		servArg.BlockNum = uint16(r.ruleNum) | 0x1000
		servArg.Sel = cmn.LbSelRr
		servArg.Mode = cmn.LBModeDefault
		servArg.Snat = true
		servArg.InactiveTimeout = LbDefaultInactiveTimeout
		servArg.Name = fmt.Sprintf("%s:%s:%d", "snat", fwOpts.opt.snatIP, fwOpts.opt.snatPort)

		snatEP := []cmn.LbEndPointArg{{EpIP: fwOpts.opt.snatIP, EpPort: fwOpts.opt.snatPort}}

		_, err := R.AddNatLbRule(servArg, nil, snatEP)
		if err != nil {
			tk.LogIt(tk.LogError, "fw-rule - %s:%s (%s) snat create error\n", r.tuples.String(), r.act.String(), err)
			return RuleArgsErr, errors.New("rule-snat error")
		}

		fwOpts.opt.fwMark = uint32(uint16((r.ruleNum) | 0x1000))

	}

	tk.LogIt(tk.LogDebug, "fw-rule added - %d:%s-%s\n", r.ruleNum, r.tuples.String(), r.act.String())

	R.tables[RtFw].eMap[rt.ruleKey()] = r

	r.Fw2DP(DpCreate)

	return 0, nil
}

// DeleteFwRule - Delete a firewall rule,
// On success, it will return 0 and nil error, else appropriate return code and
// error string will be set
func (R *RuleH) DeleteFwRule(fwRule cmn.FwRuleArg) (int, error) {
	var l4src rule16Tuple
	var l4dst rule16Tuple
	var l4prot rule8Tuple

	// Vaildate rule args
	_, dNetAddr, err := net.ParseCIDR(fwRule.DstIP)
	if err != nil {
		return RuleTupleErr, errors.New("malformed-rule error")
	}

	_, sNetAddr, err := net.ParseCIDR(fwRule.SrcIP)
	if err != nil {
		return RuleTupleErr, errors.New("malformed-rule error")
	}

	l3dst := ruleIPTuple{*dNetAddr}
	l3src := ruleIPTuple{*sNetAddr}

	if fwRule.Proto == 0 {
		l4prot = rule8Tuple{0, 0}
	} else {
		l4prot = rule8Tuple{fwRule.Proto, 0xff}
	}

	if fwRule.SrcPortMax == fwRule.SrcPortMin {
		if fwRule.SrcPortMin == 0 {
			l4src = rule16Tuple{0, 0}
		} else {
			l4src = rule16Tuple{fwRule.SrcPortMin, 0xffff}
		}
	} else {
		l4src = rule16Tuple{fwRule.SrcPortMax, fwRule.SrcPortMin}
	}
	if fwRule.DstPortMax == fwRule.DstPortMin {
		if fwRule.DstPortMin == 0 {
			l4dst = rule16Tuple{0, 0}
		} else {
			l4dst = rule16Tuple{fwRule.DstPortMin, 0xffff}
		}
	} else {
		l4dst = rule16Tuple{fwRule.DstPortMax, fwRule.DstPortMin}
	}
	inport := ruleStringTuple{fwRule.InPort}
	rt := ruleTuples{l3Src: l3src, l3Dst: l3dst, l4Prot: l4prot, l4Src: l4src, l4Dst: l4dst, port: inport, pref: fwRule.Pref}

	rule := R.tables[RtFw].eMap[rt.ruleKey()]
	if rule == nil {
		return RuleNotExistsErr, errors.New("no-rule error")
	}

	if rule.act.actType == RtActSnat {
		// Delete implicit SNAT Rule

		var servArg cmn.LbServiceArg
		servArg.ServIP = "0.0.0.0"
		servArg.ServPort = 0
		servArg.Proto = "none"
		servArg.BlockNum = uint16(rule.ruleNum) | 0x1000
		servArg.Sel = cmn.LbSelRr
		servArg.Mode = cmn.LBModeDefault
		servArg.Snat = true

		switch fwOpts := rule.act.action.(type) {
		case *ruleFwOpts:
			servArg.Name = fmt.Sprintf("%s:%s:%d", "Masq", fwOpts.opt.snatIP, fwOpts.opt.snatPort)
		}

		_, err := R.DeleteNatLbRule(servArg)
		if err != nil {
			tk.LogIt(tk.LogError, "fw-rule - %s:%s snat delete error\n", rule.tuples.String(), rule.act.String())
		}
	}

	defer R.tables[RtFw].Mark.PutCounter(rule.ruleNum)

	delete(R.tables[RtFw].eMap, rt.ruleKey())

	tk.LogIt(tk.LogDebug, "fw-rule deleted %s-%s\n", rule.tuples.String(), rule.act.String())

	rule.Fw2DP(DpRemove)

	return 0, nil
}

// GetEpHosts - get all end-points and pack them into a cmn.EndPointMod slice
func (R *RuleH) GetEpHosts() ([]cmn.EndPointMod, error) {
	var res []cmn.EndPointMod

	for _, data := range R.epMap {
		var ret cmn.EndPointMod
		// Make end-point
		ret.HostName = data.hostName
		ret.Name = data.epKey
		if !data.opts.probeActivated {
			ret.ProbeType = HostProbeNone
		} else {
			ret.ProbeType = data.opts.probeType
			ret.ProbeDuration = data.opts.probeDuration
			ret.InActTries = data.opts.inActTryThr
		}
		ret.ProbeReq = data.opts.probeReq
		ret.ProbeResp = data.opts.probeResp
		ret.ProbePort = data.opts.probePort
		if ret.ProbeType == HostProbePing {
			ret.MinDelay = fmt.Sprintf("%v", data.minDelay)
			ret.AvgDelay = fmt.Sprintf("%v", data.avgDelay)
			ret.MaxDelay = fmt.Sprintf("%v", data.maxDelay)
		}
		if data.inactive {
			ret.CurrState = "nok"
		} else {
			ret.CurrState = "ok"
		}

		// Append to slice
		res = append(res, ret)
	}

	return res, nil
}

// IsEPHostActive - Check if end-point is active
func (R *RuleH) IsEPHostActive(epKey string) bool {
	ep := R.epMap[epKey]
	if ep == nil {
		return true // Are we sure ??
	}

	return !ep.inactive
}

func validateEPHostOpts(hostName string, args epHostOpts) (int, error) {
	// Validate hostopts
	if net.ParseIP(hostName) == nil {
		return RuleArgsErr, errors.New("host-parse error")
	}

	if args.inActTryThr > MaxDflLbaInactiveTries ||
		args.probeDuration > MaxHostProbeTime {
		return RuleArgsErr, errors.New("host-args error")
	}

	if args.probeType != HostProbePing &&
		args.probeType != HostProbeConnectTCP &&
		args.probeType != HostProbeConnectUDP &&
		args.probeType != HostProbeConnectSCTP &&
		args.probeType != HostProbeHTTP &&
		args.probeType != HostProbeHTTPS &&
		args.probeType != HostProbeNone {
		return RuleArgsErr, errors.New("host-args unknown probe type")
	}

	if (args.probeType == HostProbeConnectTCP ||
		args.probeType == HostProbeConnectUDP ||
		args.probeType == HostProbeConnectSCTP) &&
		args.probePort == 0 {
		return RuleArgsErr, errors.New("host-args unknown probe port")
	}

	return 0, nil
}

func makeEPKey(hostName string, probeType string, probePort uint16) string {
	return hostName + "_" + probeType + "_" + strconv.Itoa(int(probePort))
}

// AddEPHost - Add an end-point host
// name, if present will be used as endpoint key
// It will return 0 and nil error, else appropriate return code and error string will be set
func (R *RuleH) AddEPHost(apiCall bool, hostName string, name string, args epHostOpts) (int, error) {
	var epKey string

	if apiCall && args.probeType != HostProbeNone {
		args.probeActivated = true
	}

	R.epMx.Lock()
	defer R.epMx.Unlock()

	// Validate hostopts
	_, err := validateEPHostOpts(hostName, args)
	if err != nil {
		tk.LogIt(tk.LogError, "Failed to add EP :%s\n", err)
		return RuleArgsErr, err
	}
	// Load CA cert into pool
	if args.probeType == HostProbeHTTPS {
		// Check if there exist a CA certificate particularly for this EP
		rootCACertile := cmn.CertPath + hostName + "/" + cmn.CACertFileName
		if exists := utils.FileExists(rootCACertile); exists {
			rootCA, err := os.ReadFile(rootCACertile)
			if err != nil {
				tk.LogIt(tk.LogError, "RootCA cert load failed : %v", err)
				return RuleArgsErr, errors.New("rootca cert load failed")
			}
			R.rootCAPool.AppendCertsFromPEM(rootCA)
			tk.LogIt(tk.LogDebug, "RootCA cert loaded for %s\n", hostName)
		}
	}
	if name == "" {
		epKey = makeEPKey(hostName, args.probeType, args.probePort)
	} else {
		epKey = name
	}

	ep := R.epMap[epKey]
	if ep != nil {
		if apiCall {
			ep.opts = args
			ep.opts.currProbeDuration = ep.opts.probeDuration
			ep.initProberOn = true
			return 0, nil
		}
		ep.ruleCount++
		return 0, nil
	}

	ep = new(epHost)
	ep.epKey = epKey
	ep.hostName = hostName
	ep.opts = args
	ep.initProberOn = true
	ep.opts.currProbeDuration = ep.opts.probeDuration

	if apiCall != true {
		ep.ruleCount = 1
	}
	if args.probeType != HostProbeConnectUDP {
		ep.hID = R.lepHID % MaxEndPointCheckers
		//ep.sT = time.Now()
		R.lepHID++
	} else {
		ep.hID = 0
	}

	R.epMap[epKey] = ep

	tk.LogIt(tk.LogDebug, "ep-host added %v:%d\n", epKey, ep.hID)

	return 0, nil
}

// DeleteEPHost - Delete an end-point host
// It will return 0 and nil error, else appropriate return code and error string will be set
func (R *RuleH) DeleteEPHost(apiCall bool, name string, hostName string, probeType string, probePort uint16) (int, error) {
	var key string

	R.epMx.Lock()
	defer R.epMx.Unlock()
	if name == "" {
		key = makeEPKey(hostName, probeType, probePort)
	} else {
		key = name
	}
	ep := R.epMap[key]
	if ep == nil {
		return RuleEpNotExistErr, errors.New("host-notfound error")
	}

	if apiCall == false {
		ep.ruleCount--
	}

	if ep.ruleCount > 0 {
		return RuleEpCountErr, errors.New("LB Rule-referred")
	}

	delete(R.epMap, ep.epKey)

	tk.LogIt(tk.LogDebug, "ep-host deleted %v\n", key)

	return 0, nil
}

func (ep *epHost) transitionEPState(currState bool, inactThr int) {
	if currState {
		if ep.inactive {
			ep.inactive = false
			ep.inActTries = 0
			ep.opts.currProbeDuration = ep.opts.probeDuration
			tk.LogIt(tk.LogDebug, "active ep - %s:%s:%d(%v)\n",
				ep.epKey, ep.opts.probeType, ep.opts.probePort, ep.avgDelay)
		}
	} else {
		if ep.inActTries < inactThr {
			ep.inActTries++
			if ep.inActTries >= inactThr {
				if !ep.inactive {
					tk.LogIt(tk.LogDebug, "inactive ep - %s:%s:%d(next try after %ds)\n",
						ep.epKey, ep.opts.probeType, ep.opts.probePort, ep.opts.currProbeDuration)
				}
				ep.inactive = true
			}
		} else {
			ep.inActTries++
			// Inactive eps are moved back
			if ep.opts.currProbeDuration < 3*DflHostProbeTimeout {
				ep.opts.currProbeDuration += 20
			}
			//tk.LogIt(tk.LogDebug, "inactive ep - %s:%s:%d(next try after %ds)\n",
			//	ep.epKey, ep.opts.probeType, ep.opts.probePort, ep.opts.currProbeDuration)
		}
	}
}

func (R *RuleH) epCheckNow(ep *epHost) {
	var sType string
	sHint := ""

	inActTryThr := ep.opts.inActTryThr
	if ep.initProberOn {
		inActTryThr = 1
		ep.initProberOn = false
	}

	sName := fmt.Sprintf("%s:%d", ep.hostName, ep.opts.probePort)
	if tk.IsNetIPv6(ep.hostName) {
		sName = fmt.Sprintf("[%s]:%d", ep.hostName, ep.opts.probePort)
	}

	if !ep.opts.probeActivated {
		ep.inactive = false
		ep.inActTries = 0
		return
	}

	if ep.opts.probeType == HostProbeConnectTCP ||
		ep.opts.probeType == HostProbeConnectUDP ||
		ep.opts.probeType == HostProbeConnectSCTP {
		if ep.opts.probeType == HostProbeConnectTCP {
			sType = "tcp"
			ret, sIP, _ := R.zone.L3.IfaSelectAny(net.ParseIP(ep.hostName), true)
			if ret == 0 {
				sHint = sIP.String()
			}
		} else if ep.opts.probeType == HostProbeConnectUDP {
			sType = "udp"
			ret, sIP, _ := R.zone.L3.IfaSelectAny(net.ParseIP(ep.hostName), true)
			if ret == 0 {
				sHint = sIP.String()
			}
		} else {
			sType = "sctp"
			ret, sIP, _ := R.zone.L3.IfaSelectAny(net.ParseIP(ep.hostName), true)
			if ret == 0 {
				sHint = sIP.String()
			}
		}
		sOk := tk.L4ServiceProber(sType, sName, sHint, ep.opts.probeReq, ep.opts.probeResp)
		ep.transitionEPState(sOk, inActTryThr)
	} else if ep.opts.probeType == HostProbePing {
		pinger, err := probing.NewPinger(ep.hostName)
		if err != nil {
			return
		}

		pinger.Count = ep.opts.inActTryThr
		pinger.Size = 100
		pinger.Interval = time.Duration(200000000)
		pinger.Timeout = time.Duration(500000000)
		pinger.SetPrivileged(true)

		//pinger.OnFinish = func(stats *ping.Statistics) {
		//	fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
		//	fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n",
		//		stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		//	fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
		//		stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		//}

		//pinger.OnRecv = func(pkt *probing.Packet) {
		//	fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n",
		//		pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
		//}
		err = pinger.Run()
		if err != nil {
			return
		}

		stats := pinger.Statistics()

		if stats.PacketsRecv != 0 {
			ep.avgDelay = stats.AvgRtt
			ep.minDelay = stats.MinRtt
			ep.maxDelay = stats.MaxRtt
			ep.transitionEPState(true, 1)
		} else {
			ep.avgDelay = time.Duration(0)
			ep.minDelay = time.Duration(0)
			ep.maxDelay = time.Duration(0)
			ep.transitionEPState(false, 1)
		}
		pinger.Stop()
	} else if ep.opts.probeType == HostProbeHTTP {
		var addr net.IP
		if addr = net.ParseIP(ep.hostName); addr == nil {
			// This is already verified
			return
		}

		urlStr := fmt.Sprintf("http://%s:%d/%s", addr.String(), ep.opts.probePort, ep.opts.probeReq)
		sOk := tk.HTTPProber(urlStr)
		ep.transitionEPState(sOk, inActTryThr)
	} else if ep.opts.probeType == HostProbeHTTPS {
		var addr net.IP
		if addr = net.ParseIP(ep.hostName); addr == nil {
			// This is already verified
			return
		}

		urlStr := fmt.Sprintf("https://%s:%d/%s", addr.String(), ep.opts.probePort, ep.opts.probeReq)
		sOk := utils.HTTPSProber(urlStr, R.tlsCert, R.rootCAPool, ep.opts.probeResp)
		//tk.LogIt(tk.LogDebug, "[PROBE] https ep - URL[%s:%s] Resp[%s] %v\n", ep.hostName, urlStr, ep.opts.probeResp, sOk)
		ep.transitionEPState(sOk, inActTryThr)
	} else {
		// TODO
		ep.inactive = false
		ep.inActTries = 0
	}
}

func epTicker(R *RuleH, helper int) {
	epc := R.epCs[helper]

	idx := 0
	tlen := 0
	var run uint32
	run = 0

	for {
		select {
		case <-epc.tD:
			return
		case t := <-epc.hChk.C:
			epHosts := make([]*epHost, 0)
			tk.LogIt(-1, "Tick at %v:%d\n", t, helper)

			R.epMx.Lock()
			if tlen != len(R.epMap) || idx >= len(R.epMap) {
				idx = 0
				tlen = len(R.epMap)
			}
			if idx > 0 {
				idx = 0
				// We restart the sweep from beginning while taking a short break
				// Due to how goLang range works, we would be sweeping eps mostly randomly
				R.epMx.Unlock()
				break
			}
			tidx := 0
			for _, host := range R.epMap {

				if host.hID == uint8(helper) {

					if run%2 == 0 {
						if (host.opts.probeType == HostProbePing && host.avgDelay == 0) || host.inactive ||
							(host.initProberOn && time.Duration(t.Sub(host.sT).Seconds()) >= time.Duration(InitHostProbeTimeout)) {
							epHosts = append(epHosts, host)
						}
					} else {
						if (host.initProberOn && time.Duration(t.Sub(host.sT).Seconds()) >= time.Duration(InitHostProbeTimeout)) ||
							time.Duration(t.Sub(host.sT).Seconds()) >= time.Duration(host.opts.currProbeDuration) {
							epHosts = append(epHosts, host)
						}
					}
					if len(epHosts) >= MaxEndPointSweeps {
						idx = tidx + 1
						break
					}
				}
				tidx++
			}
			R.epMx.Unlock()
			run++

			begin := time.Now()
			for _, eph := range epHosts {
				R.epCheckNow(eph)
				eph.sT = time.Now()
				if time.Duration(eph.sT.Sub(begin).Seconds()) >= EndPointCheckerDuration {
					break
				}
			}
			epHosts = nil
		}
	}
}

// RulesSync - This is periodic ticker routine which does two main things :
// 1. Syncs rule statistics counts
// 2. Check health of lb-rule end-points
func (R *RuleH) RulesSync() {
	rChg := false
	for _, rule := range R.tables[RtLB].eMap {
		ruleKeys := rule.tuples.String()
		ruleActs := rule.act.String()
		rChg = R.electEPSrc(rule)
		rlChg := false
		switch at := rule.act.action.(type) {
		case *ruleNatActs:
			if at.mode == cmn.LBModeHostOneArm {
				rlChg = R.mkHostAssocs(rule)
			}
		}
		if rlChg {
			// Dont support modify currently
			rule.DP(DpRemove)
			rule.DP(DpCreate)
		} else if rule.sync != 0 || rChg {
			rule.DP(DpCreate)
		}

		if !rule.hChk.actChk {
			continue
		}

		rChg = R.syncEPHostState2Rule(rule, false)
		if rChg {
			tk.LogIt(tk.LogDebug, "nat lb-Rule updated %d:%s,%s\n", rule.ruleNum, ruleKeys, ruleActs)
			rule.DP(DpCreate)
		}
	}

	if time.Duration(time.Since(R.vipST).Seconds()) > time.Duration(VIPSweepDuration) {
		for vip, vipElem := range R.vipMap {
			ip := vipElem.pVIP
			if ip == nil {
				ip = net.ParseIP(vip)
			}
			if ip != nil {
				R.AdvRuleVIPIfL2(ip, net.ParseIP(vip))
			}
		}
		R.vipST = time.Now()
	}

	for _, rule := range R.tables[RtFw].eMap {
		//ruleKeys := rule.tuples.String()
		//ruleActs := rule.act.String()
		if rule.sync != 0 {
			rule.Fw2DP(DpCreate)
		}
		//rule.DP(DpStatsGet)
		//tk.LogIt(-1, "%d:%s,%s pc %v bc %v \n",
		//	rule.ruleNum, ruleKeys, ruleActs,
		//	rule.stat.packets, rule.stat.bytes)
	}
}

// RulesTicker - Ticker for all rules
func (R *RuleH) RulesTicker() {
	R.RulesSync()
}

// RuleDestructAll - Destructor routine for all rules
func (R *RuleH) RuleDestructAll() {
	var lbs cmn.LbServiceArg
	var fwr cmn.FwRuleArg
	fmt.Printf("Deleting Rules\n")

	for _, r := range R.tables[RtLB].eMap {
		lbs.ServIP = r.tuples.l3Dst.addr.IP.String()
		fmt.Printf("Deleting %s\n", r.tuples.l3Dst.addr.IP.String())

		if r.tuples.l4Prot.val == 6 {
			lbs.Proto = "tcp"
		} else if r.tuples.l4Prot.val == 1 {
			lbs.Proto = "icmp"
		} else if r.tuples.l4Prot.val == 17 {
			lbs.Proto = "udp"
		} else if r.tuples.l4Prot.val == 132 {
			lbs.Proto = "sctp"
		} else if r.tuples.l4Prot.val == 0 {
			lbs.Proto = "none"
		} else {
			continue
		}

		lbs.ServPort = r.tuples.l4Dst.val
		R.DeleteNatLbRule(lbs)
	}
	for _, r := range R.tables[RtFw].eMap {
		fwr.DstIP = r.tuples.l3Dst.addr.String()
		fwr.SrcIP = r.tuples.l3Src.addr.String()
		if r.tuples.l4Src.valid == 0xffff {
			fwr.SrcPortMin = r.tuples.l4Src.val
			fwr.SrcPortMax = r.tuples.l4Src.val
		} else {
			fwr.SrcPortMin = r.tuples.l4Src.valid
			fwr.SrcPortMax = r.tuples.l4Src.val
		}
		if r.tuples.l4Dst.valid == 0xffff {
			fwr.DstPortMin = r.tuples.l4Dst.val
			fwr.DstPortMax = r.tuples.l4Dst.val
		} else {
			fwr.DstPortMin = r.tuples.l4Dst.valid
			fwr.DstPortMax = r.tuples.l4Dst.val
		}

		fwr.Proto = r.tuples.l4Prot.val
		fwr.InPort = r.tuples.port.val

		R.DeleteFwRule(fwr)
	}
	return
}

// VIP2DP - Sync state of nat-rule for local sock VIP-port rewrite
func (r *ruleEnt) VIP2DP(work DpWorkT) int {
	portMap := make(map[int]struct{})
	if mh.lSockPolicy {
		switch at := r.act.action.(type) {
		case *ruleNatActs:
			for _, ep := range at.endPoints {
				if _, ok := portMap[int(ep.xPort)]; ok {
					continue
				}
				portMap[int(ep.xPort)] = struct{}{}
				nVIPWork := new(SockVIPDpWorkQ)
				nVIPWork.Work = work
				if ep.inActive {
					nVIPWork.Work = DpRemove
				}
				nVIPWork.VIP = r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
				nVIPWork.Port = r.tuples.l4Dst.val
				nVIPWork.RwPort = ep.xPort
				nVIPWork.Status = new(DpStatusT)
				mh.dp.ToDpCh <- nVIPWork
			}
		}
	}
	return 0
}

// Nat2DP - Sync state of nat-rule entity to data-path
func (r *ruleEnt) Nat2DP(work DpWorkT) int {

	if r.addrRslv {
		return -1
	}

	nWork := new(NatDpWorkQ)

	nWork.Work = work
	nWork.Status = &r.sync
	nWork.ZoneNum = r.zone.ZoneNum
	nWork.ServiceIP = r.RuleVIP2PrivIP()
	if r.secMode == cmn.LBServHTTPS {
		nWork.SecMode = DpTermHTTPS
	} else if r.secMode == cmn.LBServE2EHTTPS {
		nWork.SecMode = DpE2EHTTPS
	}
	nWork.L4Port = r.tuples.l4Dst.val
	nWork.Proto = r.tuples.l4Prot.val
	nWork.Mark = int(r.ruleNum)
	nWork.BlockNum = r.tuples.pref
	nWork.InActTo = uint64(r.iTO)
	nWork.PersistTo = uint64(r.pTO)
	nWork.HostURL = r.tuples.path

	if r.act.actType == RtActDnat {
		nWork.NatType = DpDnat
	} else if r.act.actType == RtActSnat {
		nWork.NatType = DpSnat
	} else if r.act.actType == RtActFullNat {
		nWork.NatType = DpFullNat
	} else if r.act.actType == RtActFullProxy {
		nWork.NatType = DpFullProxy
	} else {
		return -1
	}

	mode := cmn.LBModeDefault

	for _, sip := range r.secIP {
		nWork.secIP = append(nWork.secIP, sip.sIP)
	}

	switch at := r.act.action.(type) {
	case *ruleNatActs:
		switch {
		case at.sel == cmn.LbSelRr:
			nWork.EpSel = EpRR
		case at.sel == cmn.LbSelHash:
			nWork.EpSel = EpHash
		case at.sel == cmn.LbSelPrio:
			// Note that internally we use RR to achieve wRR
			nWork.EpSel = EpRR
		case at.sel == cmn.LbSelRrPersist:
			nWork.EpSel = EpRRPersist
		case at.sel == cmn.LbSelLeastConnections:
			nWork.EpSel = EpLeastConn
		case at.sel == cmn.LbSelN2:
			nWork.EpSel = EpN2
		default:
			nWork.EpSel = EpRR
		}
		mode = at.mode
		if mode == cmn.LBModeDSR {
			nWork.DsrMode = true
		}
		nWork.CsumDis = mh.sumDis
		if at.sel == cmn.LbSelPrio {
			j := 0
			k := 0
			var small [MaxNatEndPoints]int
			var neps [MaxNatEndPoints]ruleNatEp
			for i, ep := range at.endPoints {
				if ep.inActive {
					continue
				}
				oEp := &at.endPoints[i]
				sw := (int(ep.weight) * MaxNatEndPoints) / 100
				if sw == 0 {
					small[k] = i
					k++
				}
				for x := 0; x < sw && j < MaxNatEndPoints; x++ {
					neps[j].xIP = oEp.xIP
					neps[j].rIP = oEp.rIP
					neps[j].xPort = oEp.xPort
					neps[j].inActive = oEp.inActive
					neps[j].weight = oEp.weight
					if sw == 1 {
						small[k] = i
						k++
					}
					j++
				}
			}
			if j < MaxNatEndPoints {
				v := 0
				if k == 0 {
					k = len(at.endPoints)
				}
				for j < MaxNatEndPoints {
					idx := small[v%k]
					oEp := &at.endPoints[idx]
					neps[j].xIP = oEp.xIP
					neps[j].rIP = oEp.rIP
					neps[j].xPort = oEp.xPort
					neps[j].inActive = oEp.inActive
					neps[j].weight = oEp.weight
					j++
					v++
				}
			}
			for _, e := range neps {
				var ep NatEP

				ep.XIP = e.xIP
				ep.RIP = e.rIP
				ep.XPort = e.xPort
				ep.Weight = e.weight
				if e.inActive || e.noService {
					ep.InActive = true
				}
				nWork.endPoints = append(nWork.endPoints, ep)
			}
		} else {
			for _, k := range at.endPoints {
				if len(k.foldEndPoints) > 0 {
					for _, kf := range k.foldEndPoints {
						var ep NatEP

						ep.XIP = kf.xIP
						ep.RIP = kf.rIP
						ep.XPort = kf.xPort
						ep.Weight = kf.weight
						if kf.inActive || kf.noService {
							ep.InActive = true
						}

						nWork.endPoints = append(nWork.endPoints, ep)
					}
				} else {
					var ep NatEP

					ep.XIP = k.xIP
					ep.RIP = k.rIP
					ep.XPort = k.xPort
					ep.Weight = k.weight
					if k.inActive || k.noService {
						ep.InActive = true
					}

					nWork.endPoints = append(nWork.endPoints, ep)
				}
			}
		}
	default:
		return -1
	}

	mh.dp.ToDpCh <- nWork
	r.VIP2DP(nWork.Work)

	if mode == cmn.LBModeHostOneArm {
		for locIP := range r.locIPs {
			if sIP := net.ParseIP(locIP); sIP != nil {
				nWork1 := new(NatDpWorkQ)
				*nWork1 = *nWork
				nWork1.ServiceIP = sIP
				mh.dp.ToDpCh <- nWork1
			}
		}
	}

	return 0
}

// Fw2DP - Sync state of fw-rule entity to data-path
func (r *ruleEnt) Fw2DP(work DpWorkT) int {

	if work == DpStatsGet || work == DpStatsGetImm {
		nStat := new(StatDpWorkQ)
		nStat.Work = work
		nStat.Mark = uint32(r.ruleNum)
		nStat.Name = MapNameFw4
		nStat.Bytes = &r.stat.bytes
		nStat.Packets = &r.stat.packets

		if work != DpStatsGetImm {
			mh.dp.ToDpCh <- nStat
		} else {
			DpWorkSingle(mh.dp, nStat)
		}
		return 0
	}

	nWork := new(FwDpWorkQ)

	nWork.Work = work
	nWork.Status = &r.sync
	nWork.ZoneNum = r.zone.ZoneNum
	nWork.SrcIP = r.tuples.l3Src.addr
	nWork.DstIP = r.tuples.l3Dst.addr
	if r.tuples.l4Src.valid == 0xffff {
		nWork.L4SrcMin = r.tuples.l4Src.val
		nWork.L4SrcMax = r.tuples.l4Src.val
	} else {
		nWork.L4SrcMin = r.tuples.l4Src.valid
		nWork.L4SrcMax = r.tuples.l4Src.val
	}
	if r.tuples.l4Dst.valid == 0xffff {
		nWork.L4DstMin = r.tuples.l4Dst.val
		nWork.L4DstMax = r.tuples.l4Dst.val
	} else {
		nWork.L4DstMin = r.tuples.l4Dst.valid
		nWork.L4DstMax = r.tuples.l4Dst.val
	}
	if r.tuples.port.val != "" {
		port := r.zone.Ports.PortFindByName(r.tuples.port.val)
		if port == nil {
			r.sync = DpChangeErr
			return -1
		}
		nWork.Port = uint16(port.PortNo)
	}
	nWork.Proto = r.tuples.l4Prot.val
	nWork.Mark = int(r.ruleNum)
	nWork.Pref = r.tuples.pref

	switch at := r.act.action.(type) {
	case *ruleFwOpts:
		switch at.op {
		case RtActFwd:
			nWork.FwType = DpFwFwd
		case RtActDrop:
			nWork.FwType = DpFwDrop
		case RtActRedirect:
			nWork.FwType = DpFwRdr
			port := r.zone.Ports.PortFindByName(at.opt.rdrPort)
			if port == nil {
				r.sync = DpChangeErr
				return -1
			}
			nWork.FwVal1 = uint16(port.PortNo)
		case RtActTrap:
			nWork.FwType = DpFwTrap
		case RtActSnat:
			nWork.FwType = DpFwFwd
		default:
			nWork.FwType = DpFwDrop
		}
		nWork.FwVal2 = at.opt.fwMark
		nWork.FwRecord = at.opt.record
	default:
		return -1
	}

	mh.dp.ToDpCh <- nWork

	return 0
}

// DP - sync state of rule entity to data-path
func (r *ruleEnt) DP(work DpWorkT) int {
	isNat := false

	if r.act.actType == RtActDnat ||
		r.act.actType == RtActSnat ||
		r.act.actType == RtActFullNat ||
		r.act.actType == RtActFullProxy {
		isNat = true
	}

	if work == DpMapGet {
		nTable := new(TableDpWorkQ)
		nTable.Work = DpMapGet
		nTable.Name = MapNameCt4
		mh.dp.ToDpCh <- nTable
		return 0
	}

	if work == DpStatsGet || work == DpStatsGetImm {
		if isNat {
			switch at := r.act.action.(type) {
			case *ruleNatActs:
				numEndPoints := 0
				for i := range at.endPoints {
					nEP := &at.endPoints[i]
					if len(nEP.foldEndPoints) > 0 {
						totBytes := uint64(0)
						totPackets := uint64(0)
						for range nEP.foldEndPoints {
							bytes := uint64(0)
							packets := uint64(0)
							nStat := new(StatDpWorkQ)
							nStat.Work = DpStatsGetImm
							nStat.Mark = (((uint32(r.ruleNum)) & 0xfff) << 4) | (uint32(numEndPoints) & 0xf)
							nStat.Name = MapNameNat4
							nStat.Bytes = &bytes
							nStat.Packets = &packets
							DpWorkSingle(mh.dp, nStat)
							numEndPoints++
							totBytes += bytes
							totPackets += packets
						}
						nEP.stat.bytes = totBytes
						nEP.stat.packets = totPackets
					} else {
						nStat := new(StatDpWorkQ)
						nStat.Work = work
						nStat.Mark = (((uint32(r.ruleNum)) & 0xfff) << 4) | (uint32(numEndPoints) & 0xf)
						nStat.Name = MapNameNat4
						nStat.Bytes = &nEP.stat.bytes
						nStat.Packets = &nEP.stat.packets
						if work == DpStatsGetImm {
							DpWorkSingle(mh.dp, nStat)
						} else {
							mh.dp.ToDpCh <- nStat
						}
						numEndPoints++
					}
				}
			}
		} else {
			nStat := new(StatDpWorkQ)
			nStat.Work = work
			nStat.Mark = uint32(r.ruleNum)
			nStat.Name = MapNameFw4
			nStat.Bytes = &r.stat.bytes
			nStat.Packets = &r.stat.packets

			mh.dp.ToDpCh <- nStat
		}
		return 0
	}

	if isNat {
		return r.Nat2DP(work)
	}

	return r.Fw2DP(work)

}

func (R *RuleH) AdvRuleVIPIfL2(IP net.IP, eIP net.IP) error {
	ciState, _ := mh.has.CIStateGetInst(cmn.CIDefault)
	if ciState == "MASTER" {
		dev := fmt.Sprintf("llb-rule-%s", IP.String())
		ret, _ := R.zone.L3.IfaFind(dev, IP)
		if ret == 0 {
			R.zone.L3.IfaDelete(dev, IP.String()+"/32")
		}
		ev, _, iface := R.zone.L3.IfaSelectAny(IP, false)
		if ev == 0 {
			if !utils.IsIPHostAddr(IP.String()) {
				if mh.cloudHook != nil {
					err := mh.cloudHook.CloudUpdatePrivateIP(IP, eIP, true)
					if err != nil {
						tk.LogIt(tk.LogError, "%s: lb-rule vip %s add failed. err: %v\n", mh.cloudLabel, IP.String(), err)
						return err
					}
				}

				if loxinlp.AddAddrNoHook(IP.String()+"/32", "lo") != 0 {
					tk.LogIt(tk.LogError, "nat lb-rule vip %s:%s add failed\n", IP.String(), "lo")
				} else {
					tk.LogIt(tk.LogInfo, "nat lb-rule vip %s:%s added\n", IP.String(), "lo")
				}
				loxinlp.DelNeighNoHook(IP.String(), "")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			rCh := make(chan int)
			go utils.GratArpReqWithCtx(ctx, rCh, IP, iface)
			select {
			case <-rCh:
				break
			case <-ctx.Done():
				tk.LogIt(tk.LogInfo, "nat lb-rule vip %s - iface %s : GratARP timeout\n", IP.String(), iface)
			}
		}

	} else if ciState != "NOT_DEFINED" {
		if utils.IsIPHostAddr(IP.String()) {
			if loxinlp.DelAddrNoHook(IP.String()+"/32", "lo") != 0 {
				tk.LogIt(tk.LogError, "nat lb-rule vip %s:%s delete failed\n", IP.String(), "lo")
			} else {
				tk.LogIt(tk.LogInfo, "nat lb-rule vip %s:%s deleted\n", IP.String(), "lo")
			}
		}
	} else {
		if _, foundIP := R.zone.L3.IfaAddrLocal(IP); foundIP == nil {
			dev := fmt.Sprintf("llb-rule-%s", IP.String())
			ret, _ := R.zone.L3.IfaFind(dev, IP)
			if ret != 0 {
				_, err := R.zone.L3.IfaAdd(dev, IP.String()+"/32")
				if err != nil {
					fmt.Printf("Failed to add IP : %s:%s\n", dev, err)
				}
			}
		}
	}

	return nil
}

func (R *RuleH) RuleVIPSyncToClusterState() {

	ciState, _ := mh.has.CIStateGetInst(cmn.CIDefault)
	if mh.cloudHook != nil {
		if ciState == "MASTER" {
			mh.cloudHook.CloudPrepareVIPNetWork()
		} else if ciState == "BACKUP" {
			mh.cloudHook.CloudUnPrepareVIPNetWork()
		}
	}

	for vip, vipElem := range R.vipMap {
		ip := vipElem.pVIP
		if ip == nil {
			ip = net.ParseIP(vip)
		}
		if ip != nil {
			R.AdvRuleVIPIfL2(ip, net.ParseIP(vip))
		}
	}
}

func (r *ruleEnt) RuleVIP2PrivIP() net.IP {
	if r.privIP == nil || r.privIP.IsUnspecified() {
		return r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
	} else {
		return r.privIP
	}
}

func (R *RuleH) AddRuleVIP(VIP net.IP, pVIP net.IP) {
	vipEnt := R.vipMap[VIP.String()]
	if vipEnt == nil {
		vipEnt = new(vipElem)
		vipEnt.ref = 1
		vipEnt.pVIP = pVIP
		R.vipMap[VIP.String()] = vipEnt
	} else {
		vipEnt.ref++
	}

	if vipEnt.ref == 1 {
		if pVIP == nil {
			R.AdvRuleVIPIfL2(VIP, VIP)
		} else {
			R.AdvRuleVIPIfL2(pVIP, VIP)
		}
	}
}

func (R *RuleH) DeleteRuleVIP(VIP net.IP) {

	vipEnt := R.vipMap[VIP.String()]
	if vipEnt != nil {
		vipEnt.ref--
	}

	if vipEnt != nil && vipEnt.ref == 0 {
		xVIP := VIP
		if vipEnt.pVIP != nil {
			xVIP = vipEnt.pVIP
		}
		if utils.IsIPHostAddr(xVIP.String()) {
			loxinlp.DelAddrNoHook(xVIP.String()+"/32", "lo")
			if mh.cloudHook != nil {
				err := mh.cloudHook.CloudUpdatePrivateIP(xVIP, VIP, false)
				if err != nil {
					tk.LogIt(tk.LogError, "%s: lb-rule vip %s delete failed. err: %v\n", mh.cloudLabel, xVIP.String(), err)
				}
			}
		}
		dev := fmt.Sprintf("llb-rule-%s", xVIP.String())
		ret, _ := mh.zr.L3.IfaFind(dev, xVIP)
		if ret == 0 {
			mh.zr.L3.IfaDelete(dev, xVIP.String()+"/32")
		}
		delete(R.vipMap, VIP.String())
	}
}

func (R *RuleH) IsIPRuleVIP(IP net.IP) bool {
	if _, found := R.vipMap[IP.String()]; found {
		return true
	}
	return false
}
