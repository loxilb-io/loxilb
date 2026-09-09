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
	"net"
	"sync"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	opts "github.com/loxilb-io/loxilb/options"
	"github.com/loxilb-io/loxilb/pkg/utils"
	tk "github.com/loxilb-io/loxilib"
	nl "github.com/vishvananda/netlink"
)

// vipAdvDevWarn - the configured device is reported once, not once per sweep
var vipAdvDevWarn sync.Once

// vipAdvCtx - state shared by one pass over the VIP map
//
// Its only job is to make sure a sweep takes at most one main table dump per
// address family, and only if some VIP actually gets that far. Nothing here
// outlives the pass: the interface a VIP is advertised on is resolved fresh
// every time, so a routing change is picked up on the next sweep instead of
// being remembered.
//
// A nil ctx is valid and means "no pass to share with" - the one-off callers
// (rule add, cluster state sync, rule delete) go through that path and take a
// dump of their own if they need one.
type vipAdvCtx struct {
	dumped [2]bool
	routes [2][]nl.Route
}

func vipAdvFamilyIdx(v6 bool) int {
	if v6 {
		return 1
	}
	return 0
}

// dumpMainRoutes - the main table for one address family
//
// netlink filters the dump to the main table already, so the VIP's own host
// route is not in here: that one lives in the local table. The answer is
// therefore the same before and after the VIP is bound.
func dumpMainRoutes(v6 bool) []nl.Route {
	family := nl.FAMILY_V4
	if v6 {
		family = nl.FAMILY_V6
	}

	routes, err := nl.RouteList(nil, family)
	if err != nil {
		tk.LogIt(tk.LogError, "vip-adv: main table dump failed: %v\n", err)
		return nil
	}

	return routes
}

// mainRoutes - the shared dump for this pass, taken on first use
func (c *vipAdvCtx) mainRoutes(v6 bool) []nl.Route {
	if c == nil {
		return dumpMainRoutes(v6)
	}

	idx := vipAdvFamilyIdx(v6)
	if !c.dumped[idx] {
		c.routes[idx] = dumpMainRoutes(v6)
		c.dumped[idx] = true
	}

	return c.routes[idx]
}

// vipAdvIfaIf - the port whose subnet contains the VIP
//
// For IPv6 this is also where a bound VIP is found, since its /128 makes the
// port it sits on match. IfaAdd does not mark such an address secondary, so
// the scan below does not skip it.
func (R *RuleH) vipAdvIfaIf(IP net.IP) string {
	v6 := !tk.IsNetIPv4(IP.String())

	for _, ifa := range R.zone.L3.IfaMap {
		if ifa.Key.Obj == "lo" {
			continue
		}

		for _, ifaEnt := range ifa.Ifas {
			if ifaEnt.Secondary {
				continue
			}
			if v6 != tk.IsNetIPv6(ifaEnt.IfaNet.IP.String()) {
				continue
			}
			if ifaEnt.IfaNet.Contains(IP) {
				return ifa.Key.Obj
			}
		}
	}

	return ""
}

// vipAdvTrieIf - the egress port the route trie has for the VIP
//
// The trie only ever holds routes over links loxilb registered as ports, and
// RtAdd keeps rule VIP host routes out of it, so this answer is neither
// polluted by the VIP's own binding nor by links loxilb does not forward on.
//
// A default route match is refused. The trie holds one entry per prefix and
// the last event to arrive wins, so which of several default routes it ends up
// with is arbitrary - the loxilb route model carries no metric to break the
// tie. Falling through to the main table dump gets the kernel's priorities.
func (R *RuleH) vipAdvTrieIf(IP net.IP) string {
	var err int
	var pfx *net.IPNet
	var tDat tk.TrieData

	if tk.IsNetIPv4(IP.String()) {
		err, pfx, tDat = R.zone.Rt.Trie4.FindTrie(IP.String())
	} else {
		err, pfx, tDat = R.zone.Rt.Trie6.FindTrie(IP.String())
	}

	if err != 0 || pfx == nil {
		return ""
	}
	if ones, _ := pfx.Mask.Size(); ones == 0 {
		return ""
	}

	switch rtn := tDat.(type) {
	case *Neigh:
		if rtn != nil && rtn.OifPort != nil {
			return rtn.OifPort.Name
		}
	case *int:
		if p := R.zone.Ports.PortFindByOSID(*rtn); p != nil {
			return p.Name
		}
	}

	return ""
}

// vipAdvMainTableIf - longest prefix match over a main table dump
//
// Candidates whose egress is not a loxilb port are dropped before the match.
// loxilb only attaches its datapath to registered ports, so a VIP advertised
// out of any other link would pull traffic the datapath never sees. The trie
// steps get this for free; a raw kernel dump does not.
func (R *RuleH) vipAdvMainTableIf(IP net.IP, ctx *vipAdvCtx) string {
	v6 := !tk.IsNetIPv4(IP.String())
	cands := utils.MainTableEgressCands(IP, ctx.mainRoutes(v6), nil)

	best := pickEgressCand(cands, func(ifIndex int) bool {
		return R.zone.Ports.PortFindByOSID(ifIndex) != nil
	})
	if best < 0 {
		return ""
	}

	return cands[best].IfName
}

// pickEgressCand - index of the winning candidate, -1 if none is eligible
//
// Longest prefix first, lowest priority to break a tie, and only among links
// isPort accepts. The port test comes before the match rather than after it, so
// a more specific route over a link loxilb does not forward on steps aside for
// the next best one instead of failing the whole resolution.
func pickEgressCand(cands []utils.EgressCand, isPort func(ifIndex int) bool) int {
	best := -1

	for i := range cands {
		if !isPort(cands[i].IfIndex) {
			continue
		}
		if best < 0 ||
			cands[i].PrefixLen > cands[best].PrefixLen ||
			(cands[i].PrefixLen == cands[best].PrefixLen && cands[i].Priority < cands[best].Priority) {
			best = i
		}
	}

	return best
}

// VipAdvIf - the interface a VIP should be advertised on, "" if none resolves
//
// Resolution order, stopping at the first answer:
//
//  1. IPv6 only, the port carrying the /128. The kernel answers a neighbour
//     solicitation only on a link that holds the target address, so for IPv6
//     where the address sits is where it has to be advertised.
//  2. the route trie's egress port, unless it matched a default route
//  3. IPv4 only, the port whose subnet contains the VIP
//  4. longest prefix match over a main table dump
//
// Whatever comes back is checked once more against the interface filter, so a
// link that cannot carry a well formed ARP or NA never reaches a frame builder
// no matter which step produced it.
func (R *RuleH) VipAdvIf(IP net.IP, ctx *vipAdvCtx) string {
	if ifName := R.vipAdvConfIf(); ifName != "" {
		return ifName
	}

	v6 := !tk.IsNetIPv4(IP.String())
	ifName := ""

	if v6 {
		ifName = R.vipAdvIfaIf(IP)
	}
	if ifName == "" {
		ifName = R.vipAdvTrieIf(IP)
	}
	if ifName == "" && !v6 {
		ifName = R.vipAdvIfaIf(IP)
	}
	if ifName == "" {
		ifName = R.vipAdvMainTableIf(IP, ctx)
	}

	if ifName == "" || ifName == "lo" {
		return ""
	}
	if !utils.AdvIfUsableByName(ifName) {
		// The caller reports the resolution failure once, on the transition.
		// This line only says which link was rejected, so it stays at debug.
		tk.LogIt(tk.LogDebug, "vip-adv: %s - %s cannot carry an advertisement\n", IP.String(), ifName)
		return ""
	}

	return ifName
}

// vipAdvConfIf - the configured advertise interface, "" when there is none to use
//
// The escape hatch for the setups automatic resolution cannot get right:
// multi-homed hosts where the client facing link is not the one the routing
// table points at, ECMP, and policy routing. A configured device that loxilb
// has not registered as a port is refused for the same reason step 4 refuses
// one, and resolution carries on as if nothing were configured.
func (R *RuleH) vipAdvConfIf() string {
	dev := opts.Opts.VIPAdvDev
	if dev == "" {
		return ""
	}

	if R.zone.Ports.PortFindByName(dev) == nil {
		vipAdvDevWarn.Do(func() {
			tk.LogIt(tk.LogError, "vip-adv: configured dev %s is not a loxilb port, ignored\n", dev)
		})
		return ""
	}
	if !utils.AdvIfUsableByName(dev) {
		vipAdvDevWarn.Do(func() {
			tk.LogIt(tk.LogError, "vip-adv: configured dev %s cannot carry an advertisement, ignored\n", dev)
		})
		return ""
	}

	return dev
}

// vipAdvBurst - one running advertisement burst
type vipAdvBurst struct {
	cancel context.CancelFunc
}

// vipAdvBurstSched - at most one advertisement burst per cluster instance
//
// A burst belongs to the transition that started it. The next transition of
// the same instance, whichever way it goes, replaces it: a promotion starts a
// fresh burst, a demotion just stops the old one, so a master that steps down
// mid-burst does not keep claiming the VIPs for the peer that took them.
type vipAdvBurstSched struct {
	mx     sync.Mutex
	active map[string]*vipAdvBurst
}

// arm - drop the burst running for inst, if any, and start run in its place
//
// run may be nil, which only does the drop. run gets a context that is
// cancelled when it is replaced, and is expected to return promptly on it.
func (s *vipAdvBurstSched) arm(inst string, run func(ctx context.Context)) {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.active == nil {
		s.active = make(map[string]*vipAdvBurst)
	}
	if b := s.active[inst]; b != nil {
		b.cancel()
		delete(s.active, inst)
	}
	if run == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &vipAdvBurst{cancel: cancel}
	s.active[inst] = b

	go func() {
		defer func() {
			cancel()
			s.mx.Lock()
			if s.active[inst] == b {
				delete(s.active, inst)
			}
			s.mx.Unlock()
		}()
		run(ctx)
	}()
}

// vipAdvTarget - a VIP and the interface it is advertised on
type vipAdvTarget struct {
	ip    net.IP
	iface string
}

// vipAdvBurstOnState - repeat the advertisement after a MASTER transition
//
// The transition sends one gratuitous ARP or NA per VIP, and the next one is
// the sweep's, up to VIPSweepDuration later. One frame is easy to lose - the
// link may still be coming up, the switch may still be learning - and until
// the sweep every neighbour keeps its stale entry and forwards to the old
// master. So send a few more, VIPAdvRepeatInterval apart, the way keepalived
// does with garp_master_repeat. Rule adds while already master are not
// repeated: there is no stale entry to overwrite, the first ARP request
// resolves the VIP on its own.
//
// Whatever the new state, a burst still running for the instance is dropped
// first. The repeats are advertisements only: the bind, cloud hook and
// neighbour cleanup happened on the transition and are not redone.
func (R *RuleH) vipAdvBurstOnState(inst, ciStateStr string) {
	repeat := opts.Opts.VIPAdvRepeat
	if ciStateStr != cmn.CIMasterStateString || repeat <= 0 {
		R.vipAdvRep.arm(inst, nil)
		return
	}

	tk.LogIt(tk.LogInfo, "lb-rule vip adv - %s master, repeating %d times\n", inst, repeat)

	R.vipAdvRep.arm(inst, func(ctx context.Context) {
		for i := 0; i < repeat; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(VIPAdvRepeatInterval * time.Second):
			}

			targets := R.vipAdvBurstTargets(inst)
			if len(targets) == 0 {
				return
			}
			for _, t := range targets {
				if ctx.Err() != nil {
					return
				}
				tk.LogIt(tk.LogDebug, "lb-rule vip %s - repeat %d/%d on %s\n", t.ip.String(), i+1, repeat, t.iface)
				vipAdvSend(t.ip, t.iface)
			}
		}
	})
}

// vipAdvBurstTargets - the VIPs of inst and where each goes, nil once inst is
// no longer master
//
// Resolved fresh under the lock, like the sweep does, so a routing change
// between repeats is followed rather than remembered. The frames themselves
// go out after the lock is dropped.
func (R *RuleH) vipAdvBurstTargets(inst string) []vipAdvTarget {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()

	if ciState, _ := mh.has.CIStateGetInst(inst); ciState != cmn.CIMasterStateString {
		return nil
	}

	advCtx := new(vipAdvCtx)
	targets := make([]vipAdvTarget, 0, len(R.vipMap))
	for vip, vipElem := range R.vipMap {
		if vipElem.inst != inst {
			continue
		}
		ip := vipElem.pVIP
		if ip == nil {
			ip = net.ParseIP(vip)
		}
		if ip == nil || ip.IsUnspecified() {
			continue
		}
		iface := R.VipAdvIf(ip, advCtx)
		if iface == "" {
			continue
		}
		targets = append(targets, vipAdvTarget{ip: ip, iface: iface})
	}

	return targets
}
