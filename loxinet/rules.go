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
	cmn "loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"net"
	"sort"
	"time"
)

const (
	RULE_ERR_BASE = iota - ZONE_BASE_ERR - 1000
	RULE_UNK_SERV_ERR
	RULE_UNK_EP_ERR
	RULE_EXISTS_ERR
	RULE_ALLOC_ERR
	RULE_NOT_EXIST_ERR
	RULE_EP_COUNT_ERR
)

type ruleTMatch uint

const (
	RM_PORT ruleTMatch = 1 << iota
	RM_L2SRC
	RM_L2DST
	RM_VLANID
	RM_L3SRC
	RM_L3DST
	RM_L4SRC
	RM_L4DST
	RM_L4PROT
	RM_INL2SRC
	RM_INL2DST
	RM_INL3SRC
	RM_INL3DST
	RM_INL4SRC
	RM_INL4DST
	RM_INL4PROT
	RM_MAX
)

const (
	MAX_NAT_EPS   = 16
	MAX_LBA_INACT = 3  // Default number of inactive tries before LB arm is turned off
	LBA_CHK_TIMEO = 20 // Default timeout for checking LB arms
)

type ruleTType uint

const (
	RT_EM ruleTType = iota + 1
	RT_MF
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

type ruleTuples struct {
	port     rule32Tuple
	l2Src    ruleMacTuple
	l2Dst    ruleMacTuple
	vlanId   rule16Tuple
	l3Src    ruleIPTuple
	l3Dst    ruleIPTuple
	l4Prot   rule8Tuple
	l4Src    rule16Tuple
	l4Dst    rule16Tuple
	tunId    rule32Tuple
	inL2Src  ruleMacTuple
	inL2Dst  ruleMacTuple
	inL3Src  ruleIPTuple
	inL3Dst  ruleIPTuple
	inL4Prot rule8Tuple
	inL4Src  rule16Tuple
	inL4Dst  rule16Tuple
}

type ruleTActType uint

const (
	RT_ACT_DROP ruleTActType = iota + 1
	RT_ACT_FWD
	RT_ACT_REDIRECT
	RT_ACT_DNAT
	RT_ACT_SNAT
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
	sel       cmn.EpSelect
	endPoints []ruleNatEp
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

const (
	RT_ACL ruleTableType = iota + 1
	RT_LB
	RT_MAX
)

const (
	RT_MAX_ACL = (8 * 1024)
	RT_MAX_LB  = (2 * 1024)
)

type RuleCfg struct {
	RuleInactTries   int
	RuleInactChkTime int
}

type RuleH struct {
	Zone   *Zone
	Cfg    RuleCfg
	Tables [RT_MAX]ruleTable
}

func RulesInit(zone *Zone) *RuleH {
	var nRh = new(RuleH)
	nRh.Zone = zone

	nRh.Cfg.RuleInactChkTime = LBA_CHK_TIMEO
	nRh.Cfg.RuleInactTries = MAX_LBA_INACT

	nRh.Tables[RT_ACL].tableMatch = RM_MAX - 1
	nRh.Tables[RT_ACL].tableType = RT_MF
	nRh.Tables[RT_ACL].HwMark = tk.NewCounter(1, RT_MAX_ACL)

	nRh.Tables[RT_LB].tableMatch = RM_L3DST | RM_L4DST | RM_L4PROT
	nRh.Tables[RT_LB].tableType = RT_EM
	nRh.Tables[RT_LB].eMap = make(map[string]*ruleEnt)
	nRh.Tables[RT_LB].HwMark = tk.NewCounter(1, RT_MAX_LB)

	return nRh
}

func (r *ruleTuples) ruleMkKeyCompliance(match ruleTMatch) {
	if match&RM_PORT != RM_PORT {
		r.port.val = 0
		r.port.valid = 0
	}
	if match&RM_L2SRC != RM_L2SRC {
		for i := 0; i < 6; i++ {
			r.l2Src.addr[i] = 0
			r.l2Src.valid[i] = 0
		}
	}
	if match&RM_L2DST != RM_L2DST {
		for i := 0; i < 6; i++ {
			r.l2Dst.addr[i] = 0
			r.l2Dst.valid[i] = 0
		}
	}
	if match&RM_VLANID != RM_VLANID {
		r.vlanId.val = 0
		r.vlanId.valid = 0
	}
	if match&RM_L3SRC != RM_L3SRC {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.l3Src.addr = *dst
	}
	if match&RM_L3DST != RM_L3DST {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.l3Dst.addr = *dst
	}
	if match&RM_L4PROT != RM_L4PROT {
		r.l4Prot.val = 0
		r.l4Prot.valid = 0
	}
	if match&RM_L4SRC != RM_L4SRC {
		r.l4Src.val = 0
		r.l4Src.valid = 0
	}
	if match&RM_L4DST != RM_L4DST {
		r.l4Dst.val = 0
		r.l4Dst.valid = 0
	}

	if match&RM_INL2SRC != RM_INL2SRC {
		for i := 0; i < 6; i++ {
			r.inL2Src.addr[i] = 0
			r.inL2Src.valid[i] = 0
		}
	}
	if match&RM_INL2DST != RM_INL2DST {
		for i := 0; i < 6; i++ {
			r.inL2Dst.addr[i] = 0
			r.inL2Dst.valid[i] = 0
		}
	}
	if match&RM_INL3SRC != RM_INL3SRC {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.inL3Src.addr = *dst
	}
	if match&RM_INL3DST != RM_INL3DST {
		_, dst, _ := net.ParseCIDR("0.0.0.0/0")
		r.inL3Dst.addr = *dst
	}
	if match&RM_INL4PROT != RM_INL4PROT {
		r.inL4Prot.val = 0
		r.inL4Prot.valid = 0
	}
	if match&RM_INL4SRC != RM_INL4SRC {
		r.inL4Src.val = 0
		r.inL4Src.valid = 0
	}
	if match&RM_INL4DST != RM_INL4DST {
		r.inL4Dst.val = 0
		r.inL4Dst.valid = 0
	}
}

func (r *ruleTuples) ruleKey() string {
	var ks string

	ks = fmt.Sprintf("%d", r.port.val&r.port.valid)
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
	ks += fmt.Sprintf("%d", r.vlanId.val&r.vlanId.valid)
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

	if r.port.valid != 0 {
		ks = fmt.Sprintf("inp-%d,", r.port.val&r.port.valid)
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

	if r.vlanId.valid != 0 {
		ks += fmt.Sprintf("vid-%d,", r.vlanId.val&r.vlanId.valid)
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

	if a.actType == RT_ACT_DROP {
		ks += fmt.Sprintf("%s", "drop")
	} else if a.actType == RT_ACT_DNAT ||
		a.actType == RT_ACT_SNAT {
		if a.actType == RT_ACT_SNAT {
			ks += fmt.Sprintf("%s", "do-snat:")
		} else {
			ks += fmt.Sprintf("%s", "do-dnat:")
		}

		switch na := a.action.(type) {
		case *ruleNatActs:
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

func (R *RuleH) Rules2Json() ([]byte, error) {
	var t cmn.LbServiceArg
	var eps []cmn.LbEndPointArg
	var ret cmn.LbRuleMod
	var bret []byte
	for _, data := range R.Tables[RT_LB].eMap {
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

func (R *RuleH) GetNatLbRule() ([]cmn.LbRuleMod, error) {
	var res []cmn.LbRuleMod

	for _, data := range R.Tables[RT_LB].eMap {
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

func (R *RuleH) AddNatLbRule(serv cmn.LbServiceArg, servEndPoints []cmn.LbEndPointArg) (int, error) {
	var natActs ruleNatActs
	var ipProto uint8

	service := serv.ServIP + "/32"
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return RULE_UNK_SERV_ERR, errors.New("malformed service")
	}

	if len(servEndPoints) <= 0 || len(servEndPoints) > MAX_NAT_EPS {
		return RULE_EP_COUNT_ERR, errors.New("too many or no endpoints")
	}

	if serv.Proto == "icmp" && serv.ServPort != 0 {
		return RULE_UNK_SERV_ERR, errors.New("malformed service")
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
		return RULE_UNK_SERV_ERR, errors.New("malformed service proto")
	}

	natActs.sel = serv.Sel
	for _, k := range servEndPoints {
		service = k.EpIP + "/32"
		_, pNetAddr, err := net.ParseCIDR(service)
		if err != nil {
			return RULE_UNK_EP_ERR, errors.New("malformed lb end-point")
		}
		if serv.Proto == "icmp" && k.EpPort != 0 {
			return RULE_UNK_SERV_ERR, errors.New("malformed service")
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

	eRule := R.Tables[RT_LB].eMap[rt.ruleKey()]

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
			return RULE_EXISTS_ERR, errors.New("lb rule exits")
		}

		eRule.act.action.(*ruleNatActs).sel = natActs.sel
		eRule.act.action.(*ruleNatActs).endPoints = eEps
		eRule.sT = time.Now()
		tk.LogIt(tk.LOG_DEBUG, "Nat LB Rule Updated %v\n", eRule)
		eRule.DP(DP_CREATE)

		return 0, nil
	}

	r := new(ruleEnt)
	r.tuples = rt
	r.zone = R.Zone
	r.act.actType = RT_ACT_DNAT
	r.act.action = &natActs
	r.ruleNum, err = R.Tables[RT_LB].HwMark.GetCounter()
	if err != nil {
		return RULE_ALLOC_ERR, errors.New("rule num allocation fail")
	}
	r.sT = time.Now()
	// Per LB end-point health-check is supposed to be handled at CCM,
	// but it certain cases like stand-alone mode, loxilb can do its own
	// lb end-point health monitoring
	r.ActChk = false

	ruleKeys := r.tuples.String()
	ruleEps := r.act.String()
	tk.LogIt(tk.LOG_DEBUG, "Nat LB Rule Added %d:%s-%s\n",
		r.ruleNum, ruleKeys, ruleEps)

	R.Tables[RT_LB].eMap[rt.ruleKey()] = r

	r.DP(DP_CREATE)

	return 0, nil
}

func (R *RuleH) DeleteNatLbRule(serv cmn.LbServiceArg) (int, error) {
	var ipProto uint8

	service := serv.ServIP + "/32"
	_, sNetAddr, err := net.ParseCIDR(service)
	if err != nil {
		return RULE_UNK_SERV_ERR, errors.New("malformed service")
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
		return RULE_UNK_SERV_ERR, errors.New("malformed service proto")
	}

	l4prot := rule8Tuple{ipProto, 0xff}
	l3dst := ruleIPTuple{*sNetAddr}
	l4dst := rule16Tuple{serv.ServPort, 0xffff}
	rt := ruleTuples{l3Dst: l3dst, l4Prot: l4prot, l4Dst: l4dst}

	rule := R.Tables[RT_LB].eMap[rt.ruleKey()]
	if rule == nil {
		return RULE_NOT_EXIST_ERR, errors.New("No such rule")
	}

	defer R.Tables[RT_LB].HwMark.PutCounter(rule.ruleNum)

	delete(R.Tables[RT_LB].eMap, rt.ruleKey())

	ruleKeys := rule.tuples.String()
	ruleEps := rule.act.String()
	tk.LogIt(tk.LOG_DEBUG, "Nat LB Rule Deleted %s-%s\n", ruleKeys, ruleEps)

	rule.DP(DP_REMOVE)

	return 0, nil
}

func (R *RuleH) RulesSync() {
	var sType string
	var rChg bool
	now := time.Now()
	for _, rule := range R.Tables[RT_LB].eMap {
		ruleKeys := rule.tuples.String()
		ruleActs := rule.act.String()
		tk.LogIt(tk.LOG_DEBUG, "%d:%s,%s pc %v bc %v \n",
			rule.ruleNum, ruleKeys, ruleActs,
			rule.stat.packets, rule.stat.bytes)
		rule.DP(DP_STATS_GET)

		if rule.ActChk == false {
			continue
		}

		rChg = false

		// Check if we need to check health of LB arms
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
					//fmt.Printf("rule arm probe %s:%s %v\n", sType, sName, sOk)
					if sOk == false {
						if n.inActTries <= R.Cfg.RuleInactTries {
							np.inActTries++
							if np.inActTries > R.Cfg.RuleInactTries {
								np.inActive = true
								rChg = true
								tk.LogIt(tk.LOG_DEBUG, "LB Rule Arm Inactive - %s:%s\n", sType, sName)
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
			tk.LogIt(tk.LOG_DEBUG, "LB Rule Updated %d:%s,%s\n", rule.ruleNum, ruleKeys, ruleActs)
			rule.DP(DP_CREATE)
		}

	}
}

func (R *RuleH) RulesTicker() {
	R.RulesSync()
}

func (R *RuleH) RuleDestructAll() {
	var lbs cmn.LbServiceArg
	for _, r := range R.Tables[RT_LB].eMap {
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
	return
}

func (r *ruleEnt) Nat2DP(work DpWorkT) int {

	nWork := new(NatDpWorkQ)

	nWork.Work = work
	nWork.Status = &r.Sync
	nWork.ZoneNum = r.zone.ZoneNum
	nWork.ServiceIP = r.tuples.l3Dst.addr.IP.Mask(r.tuples.l3Dst.addr.Mask)
	nWork.L4Port = r.tuples.l4Dst.val
	nWork.Proto = r.tuples.l4Prot.val
	nWork.HwMark = r.ruleNum

	if r.act.actType == RT_ACT_DNAT {
		nWork.NatType = DP_DNAT
	} else if r.act.actType == RT_ACT_SNAT {
		nWork.NatType = DP_SNAT
	}

	switch at := r.act.action.(type) {
	case *ruleNatActs:
		switch {
		case at.sel == cmn.LB_SEL_RR:
			nWork.EpSel = EP_RR
		case at.sel == cmn.LB_SEL_HASH:
			nWork.EpSel = EP_HASH
		case at.sel == cmn.LB_SEL_PRIO:
			nWork.EpSel = EP_PRIO
		default:
			nWork.EpSel = EP_RR
		}
		for _, k := range at.endPoints {
			var ep NatEP

			ep.xIP = k.xIP
			ep.xPort = k.xPort
			ep.weight = k.weight
			ep.inActive = k.inActive

			nWork.endPoints = append(nWork.endPoints, ep)
		}
		break
	default:
		return -1
	}

	mh.dp.ToDpCh <- nWork

	return 0
}

func (r *ruleEnt) DP(work DpWorkT) int {

	if work == DP_TABLE_GET {
		nTable := new(TableDpWorkQ)
		nTable.Work = DP_TABLE_GET
		nTable.Name = MAP_NAME_CT4
		mh.dp.ToDpCh <- nTable
		return 0
	}

	if work == DP_STATS_GET {
		nStat := new(StatDpWorkQ)
		nStat.Work = work
		nStat.HwMark = uint32(r.ruleNum)
		nStat.Name = MAP_NAME_NAT4
		nStat.Bytes = &r.stat.bytes
		nStat.Packets = &r.stat.packets

		mh.dp.ToDpCh <- nStat
		return 0
	}

	if r.act.actType == RT_ACT_DNAT ||
		r.act.actType == RT_ACT_SNAT {
		return r.Nat2DP(work)
	}

	return -1
}
