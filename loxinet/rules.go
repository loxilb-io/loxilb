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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

// error codes
const (
	RuleErrBase = iota - ZoneBaseErr - 1000
	RuleUnknownServiceErr
	RuleUnknownEpErr
	RuleExistsErr
	RuleAllocErr
	RuleNotExistsErr
	RuleEpCountErr
	RuleTupleErr
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
	MaxNatEndPoints     = 16
	MaxLbaInactiveTries = 3  // Default number of inactive tries before LB arm is turned off
	LbaCheckTimeout     = 20 // Default timeout for checking LB arms
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
)

type ruleNatEp struct {
	xIP        net.IP
	xPort      uint16
	weight     uint8
	inActTries int
	inActive   bool
	Mark       bool
}

type ruleNatActs struct {
	mode      cmn.LBMode
	sel       cmn.EpSelect
	endPoints []ruleNatEp
}

type ruleFwOpt struct {
	rdrMirr string
	rdrPort string
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

type ruleEnt struct {
	zone    *Zone
	ruleNum int
	Sync    DpStatusT
	tuples  ruleTuples
	ActChk  bool
	sT      time.Time
	act     ruleAct
	stat    ruleStat
}

type ruleTable struct {
	tableType  ruleTType
	tableMatch ruleTMatch
	eMap       map[string]*ruleEnt
	pMap       []*ruleEnt
	HwMark     *tk.Counter
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

// RuleH - context container
type RuleH struct {
	Zone   *Zone
	Cfg    RuleCfg
	Tables [RtMax]ruleTable
}

// RulesInit - initialize the Rules subsystem
func RulesInit(zone *Zone) *RuleH {
	var nRh = new(RuleH)
	nRh.Zone = zone

	nRh.Cfg.RuleInactChkTime = LbaCheckTimeout
	nRh.Cfg.RuleInactTries = MaxLbaInactiveTries

	nRh.Tables[RtFw].tableMatch = RmMax - 1
	nRh.Tables[RtFw].tableType = RtMf
	nRh.Tables[RtFw].eMap = make(map[string]*ruleEnt)
	nRh.Tables[RtFw].HwMark = tk.NewCounter(1, RtMaximumFw4s)

	nRh.Tables[RtLB].tableMatch = RmL3Dst | RmL4Dst | RmL4Prot
	nRh.Tables[RtLB].tableType = RtEm
	nRh.Tables[RtLB].eMap = make(map[string]*ruleEnt)
	nRh.Tables[RtLB].HwMark = tk.NewCounter(1, RtMaximumLbs)

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
	var ks string

	ks = fmt.Sprintf("%s", r.port.val)
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
	ks += fmt.Sprintf("%d", r.l4Src.val&r.l4Src.valid)
	ks += fmt.Sprintf("%d", r.l4Dst.val&r.l4Dst.valid)

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
	var ks string

	if r.port.val != "" {
		ks = fmt.Sprintf("inp-%s,", r.port.val)
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
		a.actType == RtActFullNat {
		if a.actType == RtActSnat {
			ks += fmt.Sprintf("%s", "do-snat:")
		} else if a.actType == RtActDnat {
			ks += fmt.Sprintf("%s", "do-dnat:")
		} else {
			ks += fmt.Sprintf("%s", "do-fullnat:")
		}

		switch na := a.action.(type) {
		case *ruleNatActs:
			if na.mode == cmn.LBModeOneArm {
				ks += fmt.Sprintf("%s", "onearm:")
			}
			for _, n := range na.endPoints {
				ks += fmt.Sprintf("eip-%s,ep-%d,w-%d,",
					n.xIP.String(), n.xPort, n.weight)
				if n.inActive {
					ks += fmt.Sprintf("dead|")
				} else {
					ks += fmt.Sprintf("alive|")
				}
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
	for _, data := range R.Tables[RtLB].eMap {
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
		t.Mode = int32(data.act.action.(*ruleNatActs).mode)

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

	for _, data := range R.Tables[RtLB].eMap {
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
		} else {
			return []cmn.LbRuleMod{}, errors.New("malformed service proto")
		}
		ret.Serv.ServPort = data.tuples.l4Dst.val
		ret.Serv.Sel = data.act.action.(*ruleNatActs).sel
		ret.Serv.Mode = int32(data.act.action.(*ruleNatActs).mode)
		 
		// Make Endpoints
		tmpEp := data.act.action.(*ruleNatActs).endPoints
		for _, ep := range tmpEp {

			ret.Eps = append(ret.Eps, cmn.LbEndPointArg{
				EpIP:   ep.xIP.String(),
				EpPort: ep.xPort,
				Weight: ep.weight,
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

// AddNatLbRule - Add a service LB nat rule. The service details are passed in serv argument,
// and end-point information is passed in the slice servEndPoints. On success,
// it will return 0 and nil error, else appropriate return code and error string will be set
func (R *RuleH) AddNatLbRule(serv cmn.LbServiceArg, servEndPoints []cmn.LbEndPointArg) (int, error) {
	var natActs ruleNatActs
	var ipProto uint8

	// Vaildate service args
	service := serv.ServIP + "/32"
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return RuleUnknownServiceErr, errors.New("malformed-service error")
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
	} else {
		return RuleUnknownServiceErr, errors.New("malformed-proto error")
	}

	natActs.sel = serv.Sel
	natActs.mode = cmn.LBMode(serv.Mode)

	for _, k := range servEndPoints {
		service = k.EpIP + "/32"
		_, pNetAddr, err := net.ParseCIDR(service)
		if err != nil {
			return RuleUnknownEpErr, errors.New("malformed-lbep error")
		}
		if serv.Proto == "icmp" && k.EpPort != 0 {
			return RuleUnknownServiceErr, errors.New("malformed-service error")
		}
		ep := ruleNatEp{pNetAddr.IP, k.EpPort, k.Weight, 0, false, false}
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
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst}

	eRule := R.Tables[RtLB].eMap[rt.ruleKey()]

	if eRule != nil {
		// If a NAT rule already exists, we try not reschuffle the order of the end-points.
		// We will try to append the new end-points at the end, while marking any other end-points
		// not in the new list as inactive
		var ruleChg bool = false
		eEps := eRule.act.action.(*ruleNatActs).endPoints
		for i, eEp := range eEps {
			for j, nEp := range natActs.endPoints {
				if eEp.xIP.Equal(nEp.xIP) &&
					eEp.xPort == nEp.xPort &&
					eEp.weight == nEp.weight {
					e := &eEps[i]
					n := &natActs.endPoints[j]
					if eEp.inActive {
						e.inActive = false
					}
					e.Mark = true
					n.Mark = true
					break
				}
			}
		}

		for i, nEp := range natActs.endPoints {
			n := &natActs.endPoints[i]
			if nEp.Mark == false {
				ruleChg = true
				n.Mark = true
				eEps = append(eEps, *n)
			}
		}

		for i, eEp := range eEps {
			e := &eEps[i]
			if eEp.Mark == false {
				ruleChg = true
				e.inActive = true
			}
			e.Mark = false
		}

		if ruleChg == false {
			return RuleExistsErr, errors.New("lbrule-exists error")
		}

		// Update the rule
		eRule.act.action.(*ruleNatActs).sel = natActs.sel
		eRule.act.action.(*ruleNatActs).endPoints = eEps
		eRule.act.action.(*ruleNatActs).mode = natActs.mode

		eRule.sT = time.Now()
		tk.LogIt(tk.LogDebug, "nat lb-rule updated - %s:%s\n", eRule.tuples.String(), eRule.act.String())
		eRule.DP(DpCreate)

		return 0, nil
	}

	r := new(ruleEnt)
	r.tuples = rt
	r.zone = R.Zone
	if serv.Mode == int32(cmn.LBModeFullNAT) || serv.Mode == int32(cmn.LBModeOneArm) {
		r.act.actType = RtActFullNat
		// For full-nat mode, it is necessary to do own lb end-point health monitoring
		r.ActChk = true
	} else {
		r.act.actType = RtActDnat
		// Per LB end-point health-check is supposed to be handled at CCM,
		// but it certain cases like stand-alone mode, loxilb can do its own
		// lb end-point health monitoring
		r.ActChk = false
	}
	r.act.action = &natActs
	r.ruleNum, err = R.Tables[RtLB].HwMark.GetCounter()
	if err != nil {
		tk.LogIt(tk.LogError, "nat lb-rule - %s:%s hwm error\n", eRule.tuples.String(), eRule.act.String())
		return RuleAllocErr, errors.New("rule-hwm error")
	}
	r.sT = time.Now()

	tk.LogIt(tk.LogDebug, "nat lb-rule added - %d:%s-%s\n", r.ruleNum, r.tuples.String(), r.act.String())

	R.Tables[RtLB].eMap[rt.ruleKey()] = r

	r.DP(DpCreate)

	return 0, nil
}

// DeleteNatLbRule - Delete a service LB nat rule. The service details are passed in serv argument.
// On success, it will return 0 and nil error, else appropriate return code and
// error string will be set
func (R *RuleH) DeleteNatLbRule(serv cmn.LbServiceArg) (int, error) {
	var ipProto uint8

	service := serv.ServIP + "/32"
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
	} else {
		return RuleUnknownServiceErr, errors.New("malformed-proto error")
	}

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst}

	rule := R.Tables[RtLB].eMap[rt.ruleKey()]
	if rule == nil {
		return RuleNotExistsErr, errors.New("no-rule error")
	}

	defer R.Tables[RtLB].HwMark.PutCounter(rule.ruleNum)

	delete(R.Tables[RtLB].eMap, rt.ruleKey())

	tk.LogIt(tk.LogDebug, "nat lb-rule deleted %s-%s\n", rule.tuples.String(), rule.act.String())

	rule.DP(DpRemove)

	return 0, nil
}

// GetFwRule - get all Fwrules and pack them into a cmn.FwRuleMod slice
func (R *RuleH) GetFwRule() ([]cmn.FwRuleMod, error) {
	var res []cmn.FwRuleMod

	for _, data := range R.Tables[RtFw].eMap {
		var ret cmn.FwRuleMod
		// Make Fw Arguments
		ret.Rule.DstIP = data.tuples.l3Dst.addr.String()
		ret.Rule.SrcIP = data.tuples.l3Src.addr.String()
		ret.Rule.DstPortMin = data.tuples.l4Dst.valid
		ret.Rule.DstPortMin = data.tuples.l4Dst.val
		ret.Rule.SrcPortMin = data.tuples.l4Src.valid
		ret.Rule.SrcPortMin = data.tuples.l4Src.val
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
		}

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
			l4dst = rule16Tuple{fwRule.SrcPortMin, 0xffff}
		}
	} else {
		l4dst = rule16Tuple{fwRule.DstPortMax, fwRule.DstPortMin}
	}
	inport := ruleStringTuple{fwRule.InPort}
	rt := ruleTuples{l3Src: l3src, l3Dst: l3dst, l4Prot: l4prot,
		l4Src: l4src, l4Dst: l4dst, port: inport, pref: fwRule.Pref}

	eFw := R.Tables[RtFw].eMap[rt.ruleKey()]

	if eFw != nil {
		// If a FW rule already exists
		return RuleExistsErr, errors.New("fwrule-exists error")
	}

	r := new(ruleEnt)
	r.tuples = rt
	r.zone = R.Zone

	/* Default is drop */
	fwOpts.op = RtActDrop

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
	}

	r.act.action = &fwOpts
	r.ruleNum, err = R.Tables[RtFw].HwMark.GetCounter()
	if err != nil {
		tk.LogIt(tk.LogError, "fw-rule - %s:%s mark error\n", eFw.tuples.String(), eFw.act.String())
		return RuleAllocErr, errors.New("rule-mark error")
	}
	r.sT = time.Now()

	tk.LogIt(tk.LogDebug, "fw-rule added - %d:%s-%s\n", r.ruleNum, r.tuples.String(), r.act.String())

	R.Tables[RtFw].eMap[rt.ruleKey()] = r

	r.DP(DpCreate)

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

	if fwRule.Proto != 0 {
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
			l4dst = rule16Tuple{fwRule.SrcPortMin, 0xffff}
		}
	} else {
		l4dst = rule16Tuple{fwRule.DstPortMax, fwRule.DstPortMin}
	}
	inport := ruleStringTuple{fwRule.InPort}
	rt := ruleTuples{l3Src: l3src, l3Dst: l3dst, l4Prot: l4prot, l4Src: l4src, l4Dst: l4dst, port: inport}

	rule := R.Tables[RtFw].eMap[rt.ruleKey()]
	if rule == nil {
		return RuleNotExistsErr, errors.New("no-rule error")
	}

	defer R.Tables[RtFw].HwMark.PutCounter(rule.ruleNum)

	delete(R.Tables[RtFw].eMap, rt.ruleKey())

	tk.LogIt(tk.LogDebug, "fw-rule deleted %s-%s\n", rule.tuples.String(), rule.act.String())

	rule.DP(DpRemove)

	return 0, nil
}

// RulesSync - This is periodic ticker routine which does two main things :
// 1. Syncs rule statistics counts
// 2. Check health of lb-rule end-points
func (R *RuleH) RulesSync() {
	var sType string
	var rChg bool
	now := time.Now()
	for _, rule := range R.Tables[RtLB].eMap {
		ruleKeys := rule.tuples.String()
		ruleActs := rule.act.String()
		if rule.Sync != 0 {
			rule.DP(DpCreate)
		}
		rule.DP(DpStatsGet)
		tk.LogIt(tk.LogDebug, "%d:%s,%s pc %v bc %v \n",
			rule.ruleNum, ruleKeys, ruleActs,
			rule.stat.packets, rule.stat.bytes)

		if rule.ActChk == false {
			continue
		}

		rChg = false

		// Check if we need to check health of LB endpoints
		if time.Duration(now.Sub(rule.sT).Seconds()) >= time.Duration(R.Cfg.RuleInactChkTime) {
			switch na := rule.act.action.(type) {
			case *ruleNatActs:
				if rule.tuples.l4Prot.val == 6 {
					sType = "tcp"
				} else if rule.tuples.l4Prot.val == 17 {
					sType = "udp"
				} else if rule.tuples.l4Prot.val == 1 {
					sType = "icmp"
				} else if rule.tuples.l4Prot.val == 132 {
					sType = "sctp"
				} else {
					break
				}

				for idx, n := range na.endPoints {
					np := &na.endPoints[idx]
					sName := fmt.Sprintf("%s:%d", n.xIP.String(), n.xPort)
					sOk := tk.L4ServiceProber(sType, sName)
					if sOk == false {
						if n.inActTries <= R.Cfg.RuleInactTries {
							np.inActTries++
							if np.inActTries > R.Cfg.RuleInactTries {
								np.inActive = true
								rChg = true
								tk.LogIt(tk.LogDebug, "nat lb-rule inactive ep - %s:%s\n", sType, sName)
							}
						}
					} else {
						if n.inActive {
							np.inActive = false
							np.inActTries = 0
							rChg = true
						}
					}
				}
			}
			rule.sT = now
		}

		if rChg {
			tk.LogIt(tk.LogDebug, "nat lb-Rule updated %d:%s,%s\n", rule.ruleNum, ruleKeys, ruleActs)
			rule.DP(DpCreate)
		}

	}

	for _, rule := range R.Tables[RtFw].eMap {
		ruleKeys := rule.tuples.String()
		ruleActs := rule.act.String()
		if rule.Sync != 0 {
			rule.DP(DpCreate)
		}
		rule.DP(DpStatsGet)
		tk.LogIt(tk.LogDebug, "%d:%s,%s pc %v bc %v \n",
			rule.ruleNum, ruleKeys, ruleActs,
			rule.stat.packets, rule.stat.bytes)
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
	for _, r := range R.Tables[RtLB].eMap {
		lbs.ServIP = r.tuples.l3Dst.addr.IP.String()
		if r.tuples.l4Dst.val == 6 {
			lbs.Proto = "tcp"
		} else if r.tuples.l4Dst.val == 1 {
			lbs.Proto = "icmp"
		} else if r.tuples.l4Dst.val == 17 {
			lbs.Proto = "udp"
		} else if r.tuples.l4Dst.val == 132 {
			lbs.Proto = "sctp"
		} else {
			continue
		}

		lbs.ServPort = r.tuples.l4Dst.val

		R.DeleteNatLbRule(lbs)
	}
	for _, r := range R.Tables[RtFw].eMap {
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

// Nat2DP - Sync state of nat-rule entity to data-path
func (r *ruleEnt) Nat2DP(work DpWorkT) int {
	var mode cmn.LBMode

	nWork := new(NatDpWorkQ)

	nWork.Work = work
	nWork.Status = &r.Sync
	nWork.ZoneNum = r.zone.ZoneNum
	nWork.ServiceIP = r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
	nWork.L4Port = r.tuples.l4Dst.val
	nWork.Proto = r.tuples.l4Prot.val
	nWork.HwMark = r.ruleNum

	if r.act.actType == RtActDnat {
		nWork.NatType = DpDnat
	} else if r.act.actType == RtActSnat {
		nWork.NatType = DpSnat
	} else if r.act.actType == RtActFullNat {
		nWork.NatType = DpFullNat
	} else {
		return -1
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
		default:
			nWork.EpSel = EpRR
		}
		mode = at.mode
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
				for j < MaxNatEndPoints {
					idx := small[v%k]
					oEp := &at.endPoints[idx]
					neps[j].xIP = oEp.xIP
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
				ep.XPort = e.xPort
				ep.Weight = e.weight
				ep.InActive = e.inActive
				nWork.endPoints = append(nWork.endPoints, ep)
			}
		} else {
			for _, k := range at.endPoints {
				var ep NatEP

				ep.XIP = k.xIP
				ep.XPort = k.xPort
				ep.Weight = k.weight
				ep.InActive = k.inActive

				nWork.endPoints = append(nWork.endPoints, ep)
			}
		}
		break
	default:
		return -1
	}

	if nWork.NatType == DpFullNat {
		for idx := range nWork.endPoints {
			ep := &nWork.endPoints[idx]
			if mode == cmn.LBModeOneArm {
			    e, sip := r.zone.L3.IfaSelectAny(ep.XIP)
			    if e != 0 {
				    r.Sync = DpCreateErr
				    return -1
			    }
			    ep.RIP = sip 
			} else {
			    ep.RIP = r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
			}
		}
	} else {
		for idx := range nWork.endPoints {
			ep := &nWork.endPoints[idx]
			ep.RIP = net.IPv4(0, 0, 0, 0)
		}
	}

	mh.dp.ToDpCh <- nWork

	return 0
}

// Fw2DP - Sync state of fw-rule entity to data-path
func (r *ruleEnt) Fw2DP(work DpWorkT) int {

	nWork := new(FwDpWorkQ)

	nWork.Work = work
	nWork.Status = &r.Sync
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
			r.Sync = DpChangeErr
			return -1
		}
		nWork.Port = uint16(port.PortNo)
	}
	nWork.Proto = r.tuples.l4Prot.val
	nWork.HwMark = r.ruleNum
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
				r.Sync = DpChangeErr
				return -1
			}
			nWork.FwVal1 = uint16(port.PortNo)
		case RtActTrap:
			nWork.FwType = DpFwTrap
		default:
			nWork.FwType = DpFwDrop
		}
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
		r.act.actType == RtActFullNat {
		isNat = true
	}

	if work == DpMapGet {
		nTable := new(TableDpWorkQ)
		nTable.Work = DpMapGet
		nTable.Name = MapNameCt4
		mh.dp.ToDpCh <- nTable
		return 0
	}

	if work == DpStatsGet {
		nStat := new(StatDpWorkQ)
		nStat.Work = work
		nStat.HwMark = uint32(r.ruleNum)
		if isNat == true {
			nStat.Name = MapNameNat4
		} else {
			nStat.Name = MapNameFw4
		}
		nStat.Bytes = &r.stat.bytes
		nStat.Packets = &r.stat.packets

		mh.dp.ToDpCh <- nStat
		return 0
	}

	if isNat == true {
		return r.Nat2DP(work)
	} 

	return r.Fw2DP(work)

}
