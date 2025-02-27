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
	"time"

	nlp "github.com/loxilb-io/loxilb/api/loxinlp"
	cmn "github.com/loxilb-io/loxilb/common"
	bfd "github.com/loxilb-io/loxilb/pkg/proto"
	utils "github.com/loxilb-io/loxilb/pkg/utils"
	tk "github.com/loxilb-io/loxilib"
)

// error codes for cluster module
const (
	CIErrBase = iota - 90000
	CIModErr
	CIStateErr
)

const (
	defaultClusterSubnet  = "10.252.0.0/16"
	defaultCluster6Subnet = "fd55:e81c:146f:66b5::/64"
	ClusterNetID          = 999
)

// ClusterInstance - Struct for Cluster Instance information
type ClusterInstance struct {
	State    int
	StateStr string
	Vip      net.IP
}

// ClusterNode - Struct for Cluster Node Information
type ClusterNode struct {
	Addr   net.IP
	Egress bool
	Status DpStatusT
}

// CIKAArgs - Struct for cluster BFD args
type CIKAArgs struct {
	SpawnKa  bool
	RemoteIP net.IP
	SourceIP net.IP
	Interval int64
	CSubnet  string
	CSubnet6 string
	CDev     string
}

// CIStateH - Cluster context handler
type CIStateH struct {
	SpawnKa     bool
	RemoteIP    net.IP
	SourceIP    net.IP
	Interval    int64
	ClusterMap  map[string]*ClusterInstance
	StateMap    map[string]int
	NodeMap     map[string]*ClusterNode
	Bs          *bfd.Struct
	ClusterNet  string
	ClusterNet6 string
	ClusterGw   string
	ClusterGw6  string
	ClusterIf   string
	OGw         []string
	OGw6        []string
	OSrc        string
	OSrc6       string
	initRules   bool
	initRules6  bool
}

func (ch *CIStateH) BFDSessionNotify(instance string, _ string, ciState string) {
	var sm cmn.HASMod

	sm.Instance = instance
	sm.State = ciState
	sm.Vip = net.ParseIP("0.0.0.0")
	tk.LogIt(tk.LogInfo, "ci-change instance %s - state %s vip %v\n", instance, ciState, sm.Vip)
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	ch.CIStateUpdate(sm)
}

func (ch *CIStateH) startBFDProto(bfdSessConfigArgs bfd.ConfigArgs) {

	if ch.Bs == nil {
		return
	}

	mh.dp.WaitXsyncReady("ka")
	// We need some cool-off period for loxilb to self sync-up in the cluster
	time.Sleep(KAInitTiVal * time.Second)

	txInterval := uint32(bfd.BFDDflSysTXIntervalUs)
	if ch.Interval != 0 && ch.Interval >= bfd.BFDMinSysTXIntervalUs {
		txInterval = uint32(ch.Interval)
	}

	err := ch.Bs.BFDAddRemote(bfdSessConfigArgs, ch)
	if err != nil {
		tk.LogIt(tk.LogCritical, "KA - Cant add BFD remote: %s\n", err.Error())
		//os.Exit(1)
		return
	}
	tk.LogIt(tk.LogInfo, "KA - Added BFD remote %s:%s:%vus\n", ch.RemoteIP.String(), ch.SourceIP.String(), txInterval)
}

// CITicker - Periodic ticker for Cluster module
func (ch *CIStateH) CITicker() {
	// Nothing to do currently
}

// CISpawn - Spawn CI application
func (ch *CIStateH) CISpawn() {
	bs := bfd.StructNew(3784)
	ch.Bs = bs
	if _, err := os.Stat("/etc/loxilb/BFDconfig.txt"); !errors.Is(err, os.ErrNotExist) {
		nlp.ApplyBFDConfig()
		return
	}

	if ch.SpawnKa {
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: ch.RemoteIP.String(), SourceIP: ch.SourceIP.String(),
			Port: cmn.BFDPort, Interval: bfd.BFDMinSysTXIntervalUs, Multi: cmn.BFDDefRetryCount, Instance: cmn.CIDefault}
		go ch.startBFDProto(bfdSessConfigArgs)
	}
}

// CIStateGetInst - routine to get HA state
func (ch *CIStateH) CIStateGetInst(inst string) (string, error) {

	if ci, ok := ch.ClusterMap[inst]; ok {
		return ci.StateStr, nil
	}

	return cmn.CIUnDefStateString, errors.New("not found")
}

// CIInit - routine to initialize Cluster context
func CIInit(args CIKAArgs) *CIStateH {
	var nCIh = new(CIStateH)
	nCIh.StateMap = make(map[string]int)
	nCIh.StateMap[cmn.CIMasterStateString] = cmn.CIStateMaster
	nCIh.StateMap[cmn.CIBackupStateString] = cmn.CIStateBackup
	nCIh.StateMap[cmn.CIFaultStateString] = cmn.CIStateConflict
	nCIh.StateMap[cmn.CIStopStateString] = cmn.CIStateNotDefined
	nCIh.StateMap[cmn.CIUnDefStateString] = cmn.CIStateNotDefined
	nCIh.SpawnKa = args.SpawnKa
	nCIh.RemoteIP = args.RemoteIP
	nCIh.SourceIP = args.SourceIP
	nCIh.Interval = args.Interval
	nCIh.ClusterMap = make(map[string]*ClusterInstance)

	if _, ok := nCIh.ClusterMap[cmn.CIDefault]; !ok {
		ci := &ClusterInstance{State: cmn.CIStateNotDefined,
			StateStr: cmn.CIUnDefStateString,
			Vip:      net.IPv4zero,
		}
		nCIh.ClusterMap[cmn.CIDefault] = ci
		if mh.bgp != nil {
			mh.bgp.UpdateCIState(cmn.CIDefault, ci.State, ci.Vip)
		}
	}

	nCIh.NodeMap = make(map[string]*ClusterNode)

	if args.CDev != "" {
		tk.LogIt(tk.LogInfo, "cluster-dev name %s\n", args.CDev)
		_, err := net.InterfaceByName(args.CDev)
		if err != nil {
			tk.LogIt(tk.LogError, "cluster-dev name error\n")
			os.Exit(1)
			return nil
		}
		clusterCIDR := defaultClusterSubnet
		if args.CSubnet != "" {
			clusterCIDR = args.CSubnet
		}

		clusterCIDR6 := defaultCluster6Subnet
		if args.CSubnet6 != "" {
			clusterCIDR6 = args.CSubnet6
		}

		ip, _, err := net.ParseCIDR(clusterCIDR)
		if err != nil {
			tk.LogIt(tk.LogError, "ClusterIP address invalid %s\n", clusterCIDR)
			return nil
		}

		ip6, _, err := net.ParseCIDR(clusterCIDR6)
		if err != nil {
			tk.LogIt(tk.LogError, "ClusterIP6 address invalid %s\n", clusterCIDR6)
			return nil
		}

		ifIP, err := utils.GetIfaceIPAddr(args.CDev)
		if err != nil || ifIP == nil {
			tk.LogIt(tk.LogError, "No IP address found in cluster-dev\n")
			return nil
		}

		ifIP6, _ := utils.GetIfaceIP6Addr(args.CDev)
		if ifIP6 == nil {
			tk.LogIt(tk.LogError, "No IP6 address found in cluster-dev\n")
			ifIP6 = ip6
			ifIP6[len(ifIP6)-1]++
		}

		ip[len(ip)-2] = ifIP[len(ifIP)-2]
		ip[len(ip)-1] = ifIP[len(ifIP)-1]

		ip6[len(ip)-2] = ifIP6[len(ifIP6)-2]
		ip6[len(ip)-1] = ifIP6[len(ifIP6)-1]

		clusterIfName := fmt.Sprintf("vxlan%d", ClusterNetID)

		if nlp.AddVxLANBridgeNoHook(ClusterNetID, args.CDev) < 0 {
			tk.LogIt(tk.LogError, "Failed to created Cluster Network\n")
			return nil
		}

		nlp.DelAddrNoHook(ip.String()+"/16", clusterIfName)
		if nlp.AddAddrNoHook(ip.String()+"/16", clusterIfName) < 0 {
			tk.LogIt(tk.LogError, "Failed to add Cluster Addr %s:%s\n", ip.String(), clusterIfName)
			return nil
		}

		nlp.DelAddrNoHook(ip6.String()+"/16", clusterIfName)
		if nlp.AddAddrNoHook(ip6.String()+"/64", clusterIfName) < 0 {
			tk.LogIt(tk.LogError, "Failed to add Cluster Addr %s:%s\n", ip6.String(), clusterIfName)
			nCIh.ClusterNet6 = ""
		} else {
			nCIh.ClusterNet6 = clusterCIDR6
		}

		gw := make(net.IP, len(ip))
		copy(gw, ip)
		gw[len(gw)-1] = 254

		gw6 := make(net.IP, len(ip6))
		copy(gw6, ip6)
		gw6[len(gw6)-1] = 254

		nCIh.ClusterIf = args.CDev
		nCIh.ClusterNet = clusterCIDR
		nCIh.ClusterNet6 = clusterCIDR6
		nCIh.ClusterGw = gw.String()
		nCIh.ClusterGw6 = gw6.String()

		nCIh.OGw, nCIh.OSrc, _ = nlp.GetRouteNoHook("8.8.8.8")
		nCIh.OGw6, nCIh.OSrc6, _ = nlp.GetRouteNoHook("2001:4860:4860::8888")

		tk.LogIt(tk.LogInfo, "Cluster IP address %s GW %s oGW %v oSrc %v \n", ip.String(), nCIh.ClusterGw, nCIh.OGw, nCIh.OSrc)
		tk.LogIt(tk.LogInfo, "Cluster IP6 address %s GW6 %s oGw6 %v oSrc6 %v\n", ip6.String(), nCIh.ClusterGw, nCIh.OGw6, nCIh.OSrc6)

	}

	return nCIh
}

// CIDestroy - routine to destroy Cluster context
func (ch *CIStateH) CIDestroy() {

	if ch.ClusterIf != "" {
		tk.LogIt(tk.LogError, "cluster-dev name\n")
		_, err := net.InterfaceByName(ch.ClusterIf)
		if err != nil {
			tk.LogIt(tk.LogError, "cluster-dev name error\n")
			return
		}

		clusterCIDR := ch.ClusterNet
		clusterCIDR6 := ch.ClusterNet6

		ip, _, err := net.ParseCIDR(clusterCIDR)
		if err != nil {
			tk.LogIt(tk.LogError, "ClusterIP address invalid %s\n", clusterCIDR)
			return
		}

		ip6, _, err := net.ParseCIDR(clusterCIDR6)
		if err != nil {
			tk.LogIt(tk.LogError, "ClusterIP6 address invalid %s\n", clusterCIDR6)
			return
		}

		ifIP, err := utils.GetIfaceIPAddr(ch.ClusterIf)
		if err != nil || ifIP == nil {
			tk.LogIt(tk.LogError, "No IP address found in cluster-dev\n")
			return
		}

		ifIP6, _ := utils.GetIfaceIP6Addr(ch.ClusterIf)
		if ifIP6 == nil {
			tk.LogIt(tk.LogError, "No IP6 address found in cluster-dev\n")
			ifIP6 = ip6
			ifIP6[len(ifIP6)-1]++
		}

		ip[len(ip)-2] = ifIP[len(ifIP)-2]
		ip[len(ip)-1] = ifIP[len(ifIP)-1]

		ip6[len(ip)-2] = ifIP6[len(ifIP6)-2]
		ip6[len(ip)-1] = ifIP6[len(ifIP6)-1]

		clusterIfName := fmt.Sprintf("vxlan%d", ClusterNetID)

		if nlp.DelAddrNoHook(ip.String()+"/16", clusterIfName) < 0 {
			tk.LogIt(tk.LogError, "Failed to delete Cluster Addr %s:%s\n", ip.String(), clusterIfName)
		}

		if nlp.DelAddrNoHook(ip6.String()+"/64", clusterIfName) < 0 {
			tk.LogIt(tk.LogError, "Failed to delete Cluster Addr %s:%s\n", ip6.String(), clusterIfName)
		}

		tk.LogIt(tk.LogInfo, "Cluster IP address %s deleted\n", ip.String())
		tk.LogIt(tk.LogInfo, "Cluster IP6 address %s deleted\n", ip6.String())

		if nlp.DelVxLANNoHook(ClusterNetID) < 0 {
			tk.LogIt(tk.LogError, "Failed to delete Cluster Network\n")
		}

		if len(mh.has.OGw) > 0 {
			nlp.DelRouteNoHook("0.0.0.0/0")
			for _, gw := range mh.has.OGw {
				nlp.AddRouteNoHook("0.0.0.0/0", gw, "static")
			}
		}
		if len(mh.has.OGw6) > 0 {
			nlp.DelRouteNoHook("::/0")
			for _, gw := range mh.has.OGw6 {
				nlp.AddRouteNoHook("::/0", gw, "static")
			}
		}
	}
}

// CIAddClusterRoute - routine to add a cluster route
func (ch *CIStateH) CIAddClusterRoute(dest string, add bool) {

	if tk.IsNetIPv4(dest) && dest != mh.has.ClusterGw ||
		tk.IsNetIPv6(dest) && dest != mh.has.ClusterGw6 {
		return
	}

	if add {
		found := false
		if tk.IsNetIPv4(dest) {
			gws, _, _ := nlp.GetRouteNoHook("8.8.8.8")
			for _, gw := range gws {
				if gw == dest {
					found = true
					break
				}
			}
			fmt.Printf("gws = %v: dest %s\n", gws, dest)
			if !found {
				nlp.DelRouteNoHook("0.0.0.0/0")
				nlp.AddRouteNoHook("0.0.0.0/0", dest, "static")
				fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet, DstIP: "0.0.0.0/0"}
				_, err := mh.zr.Rules.DeleteFwRule(fwarg)
				if err != nil {
					tk.LogIt(tk.LogError, "Failed to delete egress snat for cluster %s\n", mh.has.ClusterNet)
				}

			}
		} else {
			found = false
			gws, _, _ := nlp.GetRouteNoHook("2001:4860:4860::8888")
			for _, gw := range gws {
				if gw == dest {
					found = true
					break
				}
			}
			if !found {
				nlp.DelRouteNoHook("::/0")
				nlp.AddRouteNoHook("::/0", dest, "static")
				fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet6, DstIP: "::/0"}
				_, err := mh.zr.Rules.DeleteFwRule(fwarg)
				if err != nil {
					tk.LogIt(tk.LogError, "Failed to delete egress snat for cluster %s\n", mh.has.ClusterNet6)
				}
			}
		}
	} else {
		found := false
		if tk.IsNetIPv4(dest) {
			gws, _, _ := nlp.GetRouteNoHook("8.8.8.8")
			for _, gw := range gws {
				if gw == dest {
					found = true
					break
				}
			}
			if found {
				nlp.DelRouteNoHook("0.0.0.0/0")
				for i, gw := range mh.has.OGw {
					nlp.AddRouteNoHook("0.0.0.0/0", gw, "static")
					if i == 0 {
						fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet, DstIP: "0.0.0.0/0"}
						fwOpts := cmn.FwOptArg{DoSnat: true, OnDefault: true, ToIP: mh.has.OSrc}
						_, err := mh.zr.Rules.AddFwRule(fwarg, fwOpts)
						if err != nil {
							tk.LogIt(tk.LogError, "Failed to create egress snat for cluster %s:%s\n", mh.has.ClusterNet, err)
						}
					}
				}
			} else if !ch.initRules {
				for i := range mh.has.OGw {
					if i == 0 {
						fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet, DstIP: "0.0.0.0/0"}
						fwOpts := cmn.FwOptArg{DoSnat: true, OnDefault: true, ToIP: mh.has.OSrc}
						_, err := mh.zr.Rules.AddFwRule(fwarg, fwOpts)
						if err != nil {
							tk.LogIt(tk.LogError, "Failed to create egress snat for cluster %s:%s\n", mh.has.ClusterNet, err)
						}
					}
				}
				ch.initRules = true
			}
		} else {
			found = false
			gws, _, _ := nlp.GetRouteNoHook("2001:4860:4860::8888")
			for _, gw := range gws {
				if gw == dest {
					found = true
					break
				}
			}
			if found {
				nlp.DelRouteNoHook("::/0")
				for i, gw := range mh.has.OGw6 {
					nlp.AddRouteNoHook("::", gw, "static")
					if i == 0 {
						fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet6, DstIP: "0.0.0.0/0"}
						fwOpts := cmn.FwOptArg{DoSnat: true, OnDefault: true, ToIP: mh.has.OSrc6}
						_, err := mh.zr.Rules.AddFwRule(fwarg, fwOpts)
						if err != nil {
							tk.LogIt(tk.LogError, "Failed to create egress snat for cluster %s:%s\n", mh.has.ClusterNet6, err)
						}
					}
				}
			} else if !ch.initRules6 {
				for i := range mh.has.OGw6 {
					if i == 0 {
						fwarg := cmn.FwRuleArg{SrcIP: mh.has.ClusterNet6, DstIP: "0.0.0.0/0"}
						fwOpts := cmn.FwOptArg{DoSnat: true, OnDefault: true, ToIP: mh.has.OSrc6}
						_, err := mh.zr.Rules.AddFwRule(fwarg, fwOpts)
						if err != nil {
							tk.LogIt(tk.LogError, "Failed to create egress snat for cluster %s:%s\n", mh.has.ClusterNet6, err)
						}
					}
				}
				ch.initRules6 = true
			}
		}
	}
}

// CIStateGet - routine to get HA state
func (ch *CIStateH) CIStateGet() ([]cmn.HASMod, error) {
	var res []cmn.HASMod

	for i, s := range ch.ClusterMap {
		var temp cmn.HASMod
		temp.Instance = i
		temp.State = s.StateStr
		temp.Vip = s.Vip
		res = append(res, temp)
	}
	return res, nil
}

// CIVipGet - routine to get HA state
func (ch *CIStateH) CIVipGet(inst string) (net.IP, error) {
	if ci, ok := ch.ClusterMap[inst]; ok {
		if ci.Vip != nil && !ci.Vip.IsUnspecified() {
			return ci.Vip, nil
		}
	}
	return net.IPv4zero, errors.New("not found")
}

// IsCIKAMode - routine to get KA mode
func (ch *CIStateH) IsCIKAMode() bool {
	return false
}

// CIStateUpdate - routine to update cluster state
func (ch *CIStateH) CIStateUpdate(cm cmn.HASMod) (int, error) {

	if _, ok := ch.ClusterMap[cm.Instance]; !ok {
		ch.ClusterMap[cm.Instance] = &ClusterInstance{State: cmn.CIStateNotDefined,
			StateStr: cmn.CIUnDefStateString,
			Vip:      net.IPv4zero}
		tk.LogIt(tk.LogDebug, "[CLUSTER] New Instance %s created\n", cm.Instance)
	}

	ci, found := ch.ClusterMap[cm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] New Instance %s find error\n", cm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	if ci.StateStr == cm.State {
		return ci.State, nil
	}

	if _, ok := ch.StateMap[cm.State]; ok {
		tk.LogIt(tk.LogDebug, "[CLUSTER] Instance %s Current State %s Updated State: %s VIP : %s\n",
			cm.Instance, ci.StateStr, cm.State, cm.Vip.String())
		ci.StateStr = cm.State
		ci.State = ch.StateMap[cm.State]
		ci.Vip = cm.Vip

		if mh.bgp != nil {
			mh.bgp.UpdateCIState(cm.Instance, ci.State, ci.Vip)
		}
		go mh.zr.Rules.RulesSyncToClusterState(cm.Instance, cm.State)
		return ci.State, nil
	}

	tk.LogIt(tk.LogError, "[CLUSTER] Invalid State: %s\n", cm.State)
	return ci.State, errors.New("invalid cluster-state")

}

// ClusterNodeAdd - routine to update cluster nodes
func (ch *CIStateH) ClusterNodeAdd(node cmn.ClusterNodeMod) (int, error) {

	cNode := ch.NodeMap[node.Addr.String()]

	if cNode != nil {
		return -1, errors.New("existing cnode")
	}

	cNode = new(ClusterNode)
	cNode.Addr = node.Addr
	cNode.Egress = node.Egress
	ch.NodeMap[node.Addr.String()] = cNode

	cNode.DP(DpCreate)

	return 0, nil
}

// ClusterNodeDelete - routine to delete cluster node
func (ch *CIStateH) ClusterNodeDelete(node cmn.ClusterNodeMod) (int, error) {

	cNode := ch.NodeMap[node.Addr.String()]

	if cNode == nil {
		return -1, errors.New("no such cnode")
	}

	delete(ch.NodeMap, node.Addr.String())

	cNode.DP(DpRemove)
	return 0, nil
}

// CIBFDSessionAdd - routine to add BFD session
func (ch *CIStateH) CIBFDSessionAdd(bm cmn.BFDMod) (int, error) {

	if ch.Bs == nil {
		return -1, errors.New("bfd not initialized")
	}

	if bm.Interval != 0 && bm.Interval < bfd.BFDMinSysTXIntervalUs {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD session Interval value too low\n")
		return -1, errors.New("bfd interval too low")
	}

	_, found := ch.ClusterMap[bm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD SU - Cluster Instance %s not found\n", bm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	ip := net.ParseIP(bm.RemoteIP.String())
	if ip == nil {
		return -1, errors.New("remoteIP address malformed")
	}

	if !ch.SpawnKa {
		myIP := net.ParseIP(bm.SourceIP.String())
		if myIP == nil {
			return -1, errors.New("source address malformed")
		}

		tk.LogIt(tk.LogInfo, "[CLUSTER] Cluster Instance %s starting BFD..\n", bm.Instance)
		ch.SpawnKa = true

		ch.RemoteIP = bm.RemoteIP
		ch.SourceIP = bm.SourceIP
		ch.Interval = int64(bm.Interval)
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String(), SourceIP: bm.SourceIP.String(),
			Port: cmn.BFDPort, Interval: uint32(bm.Interval),
			Multi: bm.RetryCount, Instance: bm.Instance}
		go ch.startBFDProto(bfdSessConfigArgs)
	} else {
		bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String(), SourceIP: bm.SourceIP.String(),
			Port: cmn.BFDPort, Interval: uint32(bm.Interval),
			Multi: bm.RetryCount, Instance: bm.Instance}
		err := ch.Bs.BFDAddRemote(bfdSessConfigArgs, ch)
		if err != nil {
			tk.LogIt(tk.LogCritical, "KA - Cant add BFD remote: %s\n", err.Error())
			return -1, err
		}
		tk.LogIt(tk.LogInfo, "KA - BFD remote %s:%s:%vus Added\n", bm.RemoteIP.String(), bm.SourceIP.String(), bm.Interval)
	}
	return 0, nil
}

// CIBFDSessionDel - routine to delete BFD session
func (ch *CIStateH) CIBFDSessionDel(bm cmn.BFDMod) (int, error) {

	if !ch.SpawnKa {
		tk.LogIt(tk.LogError, "[CLUSTER] Cluster Instance %s not running BFD\n", bm.Instance)
		return -1, errors.New("bfd session not running")
	}

	_, found := ch.ClusterMap[bm.Instance]
	if !found {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD SU - Cluster Instance %s not found\n", bm.Instance)
		return -1, errors.New("cluster instance not found")
	}

	bfdSessConfigArgs := bfd.ConfigArgs{RemoteIP: bm.RemoteIP.String()}
	err := ch.Bs.BFDDeleteRemote(bfdSessConfigArgs)
	if err != nil {
		tk.LogIt(tk.LogCritical, "KA - Cant delete BFD remote\n")
		return -1, err
	}
	ch.SpawnKa = false
	tk.LogIt(tk.LogInfo, "KA - BFD remote %s:%s deleted\n", bm.Instance, bm.RemoteIP.String())
	return 0, nil
}

// CIBFDSessionGet - routine to get BFD session info
func (ch *CIStateH) CIBFDSessionGet() ([]cmn.BFDMod, error) {
	if !ch.SpawnKa || ch.Bs == nil {
		tk.LogIt(tk.LogError, "[CLUSTER] BFD sessions not running\n")
		return nil, errors.New("bfd session not running")
	}

	return ch.Bs.BFDGet()
}

// DP - sync state of cluster-node entity to data-path
func (cn *ClusterNode) DP(work DpWorkT) int {

	if cn.Egress {
		if work == DpCreate {
			if !utils.IsIPHostAddr(cn.Addr.String()) {
				ret := nlp.AddVxLANPeerNoHook(ClusterNetID, cn.Addr.String())
				if ret != 0 {
					cn.Status = DpCreateErr
				}
			}
			return 0
		} else {
			if !utils.IsIPHostAddr(cn.Addr.String()) {
				nlp.DelVxLANPeerNoHook(ClusterNetID, cn.Addr.String())
				return 0
			}
		}
	}

	pwq := new(PeerDpWorkQ)
	pwq.Work = work
	pwq.PeerIP = cn.Addr

	pwq.Status = &cn.Status

	mh.dp.ToDpCh <- pwq

	return 0
}
