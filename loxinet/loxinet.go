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

var version string = "0.0.1"

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
        GoBgpInit()
    }

    if opts.Opts.Api {
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
        ifmac := [6]byte{0x26, 0x7f, 0x65, 0x6c, 0x20, 0x4a}
        mh.zr.Ports.PortAdd("hs1", 2, cmn.PORT_REAL, ROOT_ZONE,
            PortHwInfo{ifmac, true, true, 1500, "", "", 0},
            PortLayer2Info{false, 10})

        ifmac = [6]byte{0xde, 0xdc, 0x1f, 0x62, 0x60, 0x55}
        mh.zr.Ports.PortAdd("hs2", 3, cmn.PORT_REAL, ROOT_ZONE,
            PortHwInfo{ifmac, true, true, 1500, "", "", 0},
            PortLayer2Info{false, 10})

        ifmac = [6]byte{0xc6, 0x28, 0xa0, 0xbb, 0xd4, 0xd3}
        mh.zr.Ports.PortAdd("hs3", 5, cmn.PORT_REAL, ROOT_ZONE,
                PortHwInfo{ifmac, true, true, 1500, "", "", 0},
                PortLayer2Info{false, 10})
        
        ifmac = [6]byte{0xde, 0xdc, 0x1f, 0x62, 0x60, 0x55}
        mh.zr.Ports.PortAdd("vxlan100", 15, cmn.PORT_VXLANBR, ROOT_ZONE,
            PortHwInfo{ifmac, true, true, 1500, "", "hs3", 1000},
            PortLayer2Info{false, 0})

        ifmac = [6]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0xa}
        _, err := mh.zr.Vlans.VlanAdd(100, "vlan100", ROOT_ZONE, 124,
            PortHwInfo{ifmac, true, true, 1500, "", "", 0})

        _, err = mh.zr.Vlans.VlanPortAdd(100, "vxlan100", false)
        if err != nil {
            fmt.Printf("failed to add port %s to vlan %d\n", "vxlan100", 100)
        }

        //mh.zr.Ports.Ports2String(&mh)
        //mh.zr.Vlans.Vlans2String(&mh)

        mh.zr.L3.IfaAdd("hs1", "31.31.31.254/24")
        mh.zr.L3.IfaAdd("hs2", "32.32.32.254/24")

        hwmac, _ := net.ParseMAC("b6:e9:cd:f8:2a:be")
        _, err = mh.zr.Nh.NeighAdd(net.IPv4(31, 31, 31, 1), ROOT_ZONE, NeighAttr{2, 1, hwmac})
        if err != nil {
            fmt.Printf("NHAdd fail 31.31.31.1\n")
        }

        hwmac, _ = net.ParseMAC("d6:3d:ef:62:1d:01")
        _, err = mh.zr.Nh.NeighAdd(net.IPv4(32, 32, 32, 1), ROOT_ZONE, NeighAttr{3, 1, hwmac})
        if err != nil {
            fmt.Printf("NHAdd fail 32.32.32.1\n")
        }

        fdbKey := FdbKey{[6]byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf}, 100}
        fdbAttr := FdbAttr{"vxlan100", net.ParseIP("32.32.32.1"), cmn.FDB_TUN}

        _, err = mh.zr.L2.L2FdbAdd(fdbKey, fdbAttr)

        mh.zr.L3.IfaAdd("vlan100", "1.1.1.1/24")

        hwmac1, _ := net.ParseMAC("0a:0b:0c:0d:0e:0f")
        _, err = mh.zr.Nh.NeighAdd(net.IPv4(1, 1, 1, 2), ROOT_ZONE,
                                   NeighAttr{124, 1, hwmac1})

        route := net.IPv4(8, 8, 8, 8)
        mask := net.CIDRMask(24, 32)
        route = route.Mask(mask)
        ipnet := net.IPNet{IP: route, Mask: mask}
        ra := RtAttr{0, 0, false}
        na := []RtNhAttr{{net.IPv4(32, 32, 32, 1), 3}}
        _, err = mh.zr.Rt.RtAdd(ipnet, ROOT_ZONE, ra, na)
        if err != nil {
            fmt.Printf("Failed to add route")
        }
                                
        lbServ := cmn.LbServiceArg{ServIP: "10.10.10.1", ServPort: 2020, Proto: "tcp", Sel: cmn.LB_SEL_RR}
        lbEps := []cmn.LbEndPointArg{
            {
                EpIP:   "31.31.31.1",
                EpPort: 5001,
                Weight: 1,
            },
            {
                EpIP:   "31.31.31.1",
                EpPort: 5001,
                Weight: 2,
            },
            {
                EpIP:   "31.31.31.1",
                EpPort: 5001,
                Weight: 2,
            },
        }

        mh.zr.Rules.AddNatLbRule(lbServ, lbEps[:])
        //mh.zr.Rules.DeleteNatLbRule(lbServ)

        mh.zr.Ports.PortUpdateProp("hs2", cmn.PORT_PROP_UPP, ROOT_ZONE, true)
        
        mh.zr.Ports.Ports2String(&mh)
        mh.zr.Vlans.Vlans2String(&mh)
        mh.zr.Rt.Rts2String(&mh)

        // Session information 
        anTun := cmn.SessTun{TeID:1, Addr:net.IP{172, 17, 1, 231}} // An TeID, gNBIP
        cnTun := cmn.SessTun{TeID:1, Addr:net.IP{172, 17, 1, 50}}  // Cn TeID, MyIP

        _, err = mh.zr.Sess.SessAdd("user1", net.IP{100, 64, 50, 1}, anTun, cnTun)
        if err != nil {
            fmt.Printf("Failed to add session\n")
        }

        // Add ULCL classifier
        _, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr:net.IP{8,8,8,8}, Qfi:11})
        //_, err = mh.zr.Sess.UlClAddCls("user1", cmn.UlClArg{Addr:net.IP{9,9,9,9}, Qfi:1})

        mh.zr.Sess.USess2String(&mh)
    }

    loxiNetRun()
}
