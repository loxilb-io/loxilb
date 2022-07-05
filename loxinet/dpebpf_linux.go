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

/*
#include <stdio.h>
#include <stdlib.h>
#include <stddef.h>
#include <stdbool.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <assert.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <net/if.h>
#include <pthread.h>
#include "../ebpf/kernel/loxilb_libdp.h"
int bpf_map_get_next_key(int fd, const void *key, void *next_key);
int bpf_map_lookup_elem(int fd, const void *key, void *value);
#cgo CFLAGS:  -I./../ebpf/libbpf/src/ -I./../ebpf/common
#cgo LDFLAGS: -L. -L./../ebpf/kernel -L./../ebpf/libbpf/src/build/usr/lib64/ -Wl,-rpath=./ebpf/libbpf/src/build/usr/lib64/ -lloxilbdp -lbpf -lelf -lz
*/
import "C"
import (
    "fmt"
    "errors"
    "net"
    "syscall"
    "time"
    "unsafe"
    tk "loxilb/loxilib"
)

const (
    EBPF_ERR_BASE = iota - 50000
    EBPF_ERR_PORTPROP_ADD
    EBPF_ERR_PORTPROP_DEL
    EBPF_ERR_EBFP_LOAD
    EBPF_ERR_EBFP_UNLOAD
    EBPF_ERR_L2ADDR_ADD
    EBPF_ERR_L2ADDR_DEL
    EBPF_ERR_TMAC_ADD
    EBPF_ERR_TMAC_DEL
    EBPF_ERR_NH_ADD
    EBPF_ERR_NH_DEL
    EBPF_ERR_RT4_ADD
    EBPF_ERR_RT4_DEL
    EBPF_ERR_NAT4_ADD
    EBPF_ERR_NAT4_DEL
    EBPF_ERR_WQ_UNK
)

type (
    sActValue   C.struct_dp_cmn_act
    intfMapKey  C.struct_intf_key
    intfMapDat  C.struct_dp_intf_tact
    intfSetIfi  C.struct_dp_intf_tact_set_ifi
    sMacKey     C.struct_dp_smac_key
    dMacKey     C.struct_dp_dmac_key
    dMacMapDat  C.struct_dp_dmac_tact
    l2VlanAct   C.struct_dp_l2vlan_act
    tMacKey     C.struct_dp_tmac_key
    tMacDat     C.struct_dp_tmac_tact
    rtNhAct     C.struct_dp_rt_nh_act
    nhKey       C.struct_dp_nh_key
    nhDat       C.struct_dp_nh_tact
    rtL2NhAct   C.struct_dp_rt_l2nh_act
    rtVxL2NhAct C.struct_dp_rt_l2vxnh_act
    rt4Key      C.struct_dp_rtv4_key
    rtDat       C.struct_dp_rt_tact
    rtL3NhAct   C.struct_dp_rt_nh_act
    nat4Key     C.struct_dp_natv4_key
    nat4Acts    C.struct_dp_natv4_tacts
    nxfrmAct    C.struct_mf_xfrm_inf
)

type DpEbpfH struct {
    ticker *time.Ticker
    tDone  chan bool
    tbN    int
}

func dpEbpfTicker() {
    tbls := []int{int(C.LL_DP_RTV4_STATS_MAP),
        int(C.LL_DP_TMAC_STATS_MAP),
        int(C.LL_DP_BD_STATS_MAP),
        int(C.LL_DP_TX_BD_STATS_MAP),
        int(C.LL_DP_ACLV4_STATS_MAP)}
    tLen := len(tbls)

    for {
        if mh.dpEbpf == nil {
            continue
        }
        select {
        case <-mh.dpEbpf.tDone:
            return
        case t := <-mh.dpEbpf.ticker.C:
            sel := mh.dpEbpf.tbN % tLen
            tk.LogIt(-1, "DP Tick at for selector %v:%d\n", t, sel)

            // For every tick collect stats for an eBPF map
            // This routine caches stats in a local statsDB
            C.llb_collect_table_stats(C.int(tbls[sel]))

            // Age any entries related to Conntrack
            // Conntrack entries also use ACL entries for fast-forwarding
            // which might also get aged out in this process
            C.llb_age_table_entries(C.LL_DP_CTV4_MAP)
            mh.dpEbpf.tbN++
        }
    }
}

func DpEbpfInit() *DpEbpfH {
    C.loxilb_main()

    // Make sure to unload eBPF programs at init time
    ifList, err := net.Interfaces()
    if err != nil {
        return nil
    }

    for _, intf := range ifList {
        if intf.Name == "llb0" {
            continue
        }
        tk.LogIt(tk.LOG_INFO, "unloading ebp :%s\n", intf.Name)
        ifStr := C.CString(intf.Name)
        section := C.CString(string(C.XDP_LL_SEC_DEFAULT))
        C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 1)
        C.free(unsafe.Pointer(ifStr))
        C.free(unsafe.Pointer(section))
    }

    ne := new(DpEbpfH)
    ne.tDone = make(chan bool)
    ne.ticker = time.NewTicker(25 * time.Second)

    go dpEbpfTicker()

    return ne
}

func loadEbpfPgm(name string) int {
    ifStr := C.CString(name)
    section := C.CString(string(C.XDP_LL_SEC_DEFAULT))
    ret := C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 0)
    C.free(unsafe.Pointer(ifStr))
    C.free(unsafe.Pointer(section))
    return int(ret)
}

func unLoadEbpfPgm(name string) int {
    ifStr := C.CString(name)
    section := C.CString(string(C.XDP_LL_SEC_DEFAULT))
    ret := C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 1)
    C.free(unsafe.Pointer(ifStr))
    C.free(unsafe.Pointer(section))
    return int(ret)
}

func getPtrOffset(ptr unsafe.Pointer, size uintptr) unsafe.Pointer {
    return unsafe.Pointer(uintptr(ptr) + size)
}

func osPortIsRunning(portName string) bool {
    sfd, err := syscall.Socket(syscall.AF_INET,
        syscall.SOCK_DGRAM,
        syscall.IPPROTO_IP)
    if err != nil {
        tk.LogIt(tk.LOG_ERROR, "Error %s", err)
        return false
    }

    ifstr := C.CString(portName)
    ifr_struct := make([]byte, 32)
    C.memcpy(unsafe.Pointer(&ifr_struct[0]), unsafe.Pointer(ifstr), 16)

    r0, _, err := syscall.Syscall(syscall.SYS_IOCTL,
        uintptr(sfd),
        syscall.SIOCGIFFLAGS,
        uintptr(unsafe.Pointer(&ifr_struct[0])))
    if r0 != 0 {
        C.free(unsafe.Pointer(ifstr))
        syscall.Close(sfd)
        tk.LogIt(tk.LOG_ERROR, "Error %s", err)
        return false
    }

    C.free(unsafe.Pointer(ifstr))
    syscall.Close(sfd)

    var flags uint16
    C.memcpy(unsafe.Pointer(&flags), unsafe.Pointer(&ifr_struct[16]), 2)

    if flags&syscall.IFF_RUNNING != 0 {
        return true
    }

    return false
}

func DpPortPropMod(w *PortDpWorkQ) int {
    var txK C.uint
    var txV C.uint
    var setIfi *intfSetIfi

    key := new(intfMapKey)
    key.ing_vid = C.ushort(tk.Htons(uint16(w.IngVlan)))
    key.ifindex = C.uint(w.OsPortNum)

    txK = C.uint(w.PortNum)

    if w.Work == DP_CREATE {

        if w.LoadEbpf != "" {
            lRet := loadEbpfPgm(w.LoadEbpf)
            if lRet != 0 {
                tk.LogIt(tk.LOG_ERROR, "Error in loading ebpf prog for IFidx %d\n", w.PortNum)
                return EBPF_ERR_EBFP_LOAD
            }
        }
        data := new(intfMapDat)
        C.memset(unsafe.Pointer(data), 0, C.sizeof_struct_dp_intf_tact)
        data.ca.act_type = C.DP_SET_IFI
        setIfi = (*intfSetIfi)(getPtrOffset(unsafe.Pointer(data),
            C.sizeof_struct_dp_cmn_act))

        setIfi.xdp_ifidx = C.ushort(w.PortNum)
        setIfi.zone = C.ushort(w.SetZoneNum)

        setIfi.bd = C.ushort(uint16(w.SetBD))
        setIfi.mirr = C.ushort(w.SetMirr)
        setIfi.polid = C.ushort(w.SetPol)

        ret := C.llb_add_table_elem(C.LL_DP_INTF_MAP,
                                    unsafe.Pointer(key),
                                    unsafe.Pointer(data))

        if ret != 0 {
            tk.LogIt(tk.LOG_ERROR, "error adding in intf map %d vlan %d\n", w.OsPortNum, w.IngVlan)
            return EBPF_ERR_PORTPROP_ADD
        }

        tk.LogIt(tk.LOG_DEBUG, "intf map added idx %d vlan %d\n", w.OsPortNum, w.IngVlan)
        txV = C.uint(w.OsPortNum)
        ret = C.llb_add_table_elem(C.LL_DP_TX_INTF_MAP,
            unsafe.Pointer(&txK),
            unsafe.Pointer(&txV))
        if ret != 0 {
            C.llb_del_table_elem(C.LL_DP_INTF_MAP,
                unsafe.Pointer(key))
            tk.LogIt(tk.LOG_ERROR, "[EBPF PORT] Error adding in Intf TX map\n")
            return EBPF_ERR_PORTPROP_ADD
        }
        tk.LogIt(tk.LOG_DEBUG, "[EBPF PORT] TX Intf map added %d\n", w.PortNum)
        return 0
    } else if w.Work == DP_REMOVE {

        C.llb_del_table_elem(C.LL_DP_TX_INTF_MAP,
            unsafe.Pointer(&txK))

        C.llb_del_table_elem(C.LL_DP_INTF_MAP,
            unsafe.Pointer(key))

        if w.LoadEbpf != "" {
            lRet := unLoadEbpfPgm(w.LoadEbpf)
            if lRet != 0 {
                tk.LogIt(tk.LOG_ERROR, "[EBPF PORT] Error in unloading ebpf prog for IFidx %d\n", w.OsPortNum)
                return EBPF_ERR_EBFP_LOAD
            }
            tk.LogIt(tk.LOG_DEBUG, "[EBPF PORT] ebpf prog for IFidx %d unloaded\n", w.OsPortNum)
        }

        return 0
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpPortPropAdd(w *PortDpWorkQ) int {
    fmt.Println(*w)
    return DpPortPropMod(w)
}

func (e *DpEbpfH) DpPortPropDel(w *PortDpWorkQ) int {
    fmt.Println(*w)
    return DpPortPropMod(w)
}

func DpL2AddrMod(w *L2AddrDpWorkQ) int {
    var l2va *l2VlanAct

    skey := new(sMacKey)
    C.memcpy(unsafe.Pointer(&skey.smac[0]), unsafe.Pointer(&w.l2Addr[0]), 6)
    skey.bd = C.ushort((uint16(w.BD)))

    dkey := new(dMacKey)
    C.memcpy(unsafe.Pointer(&dkey.dmac[0]), unsafe.Pointer(&w.l2Addr[0]), 6)
    dkey.bd = C.ushort((uint16(w.BD)))

    if w.Work == DP_CREATE {
        sdat := new(sActValue)
        sdat.act_type = C.DP_SET_NOP

        ddat := new(dMacMapDat)
        C.memset(unsafe.Pointer(ddat), 0, C.sizeof_struct_dp_dmac_tact)

        if w.Tun == 0 {
            l2va = (*l2VlanAct)(getPtrOffset(unsafe.Pointer(ddat),
                C.sizeof_struct_dp_cmn_act))
            if w.Tagged != 0 {
                ddat.ca.act_type = C.DP_SET_ADD_L2VLAN
                l2va.vlan = C.ushort(tk.Htons(uint16(w.BD)))
                l2va.oport = C.ushort(w.PortNum)
            } else {
                ddat.ca.act_type = C.DP_SET_RM_L2VLAN
                l2va.vlan = C.ushort(tk.Htons(uint16(w.BD)))
                l2va.oport = C.ushort(w.PortNum)
            }
        }

        ret := C.llb_add_table_elem(C.LL_DP_SMAC_MAP,
            unsafe.Pointer(skey),
            unsafe.Pointer(sdat))
        if ret != 0 {
            return EBPF_ERR_L2ADDR_ADD
        }

        if w.Tun == 0 {
            ret = C.llb_add_table_elem(C.LL_DP_DMAC_MAP,
                unsafe.Pointer(dkey),
                unsafe.Pointer(ddat))
            if ret != 0 {
                C.llb_del_table_elem(C.LL_DP_SMAC_MAP, unsafe.Pointer(skey))
                return EBPF_ERR_L2ADDR_ADD
            }
        }

        return 0
    } else if w.Work == DP_REMOVE {

        C.llb_del_table_elem(C.LL_DP_SMAC_MAP, unsafe.Pointer(skey))

        if w.Tun == 0 {
            C.llb_del_table_elem(C.LL_DP_DMAC_MAP, unsafe.Pointer(dkey))
        }

        return 0
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpL2AddrAdd(w *L2AddrDpWorkQ) int {
    fmt.Println(*w)
    return DpL2AddrMod(w)
}

func (e *DpEbpfH) DpL2AddrDel(w *L2AddrDpWorkQ) int {
    fmt.Println(*w)
    return DpL2AddrMod(w)
}

func DpRouterMacMod(w *RouterMacDpWorkQ) int {

    key := new(tMacKey)
    C.memcpy(unsafe.Pointer(&key.mac[0]), unsafe.Pointer(&w.l2Addr[0]), 6)
    switch {
    case w.TunType == DP_TUN_VXLAN:
        key.tun_type = C.LLB_TUN_VXLAN
    case w.TunType == DP_TUN_GRE:
        key.tun_type = C.LLB_TUN_GRE
    case w.TunType == DP_TUN_GTP:
        key.tun_type = C.LLB_TUN_GTP
    case w.TunType == DP_TUN_STT:
        key.tun_type = C.LLB_TUN_STT
    }

    key.tunnel_id = C.uint(w.TunId)

    if w.Work == DP_CREATE {
        dat := new(sActValue)
        if w.TunId != 0 {
            if w.NhNum == 0 {
                dat.act_type = C.DP_SET_RM_VXLAN
                rtNhAct := (*rtNhAct)(getPtrOffset(unsafe.Pointer(dat),
                                    C.sizeof_struct_dp_cmn_act))
                C.memset(unsafe.Pointer(rtNhAct), 0, C.sizeof_struct_dp_rt_nh_act)
                rtNhAct.nh_num = 0
                rtNhAct.tid = 0
                rtNhAct.bd = C.ushort(w.BD)
            } else {
                /* No need for tunnel ID in case of Access side */
                key.tunnel_id = 0
                key.tun_type = 0
                dat.act_type = C.DP_SET_RT_TUN_NH
                rtNhAct := (*rtNhAct)(getPtrOffset(unsafe.Pointer(dat),
                                      C.sizeof_struct_dp_cmn_act))
                C.memset(unsafe.Pointer(rtNhAct), 0, C.sizeof_struct_dp_rt_nh_act)

                rtNhAct.nh_num = C.ushort(w.NhNum)
                tid := ((w.TunId << 8) & 0xffffff00)
                rtNhAct.tid = C.uint(tk.Htonl(tid))
            }
        } else {
            dat.act_type = C.DP_SET_L3_EN
        }

        ret := C.llb_add_table_elem(C.LL_DP_TMAC_MAP,
            unsafe.Pointer(key),
            unsafe.Pointer(dat))

        if ret != 0 {
            return EBPF_ERR_TMAC_ADD
        }

        return 0
    } else if w.Work == DP_REMOVE {

        C.llb_del_table_elem(C.LL_DP_TMAC_MAP, unsafe.Pointer(key))
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpRouterMacAdd(w *RouterMacDpWorkQ) int {
    fmt.Println(*w)
    return DpRouterMacMod(w)
}

func (e *DpEbpfH) DpRouterMacDel(w *RouterMacDpWorkQ) int {
    fmt.Println(*w)
    return DpRouterMacMod(w)
}

func DpNextHopMod(w *NextHopDpWorkQ) int {
    var act *rtL2NhAct
    var vxAct *rtVxL2NhAct

    key := new(nhKey)
    key.nh_num = C.uint(w.nextHopNum)

    if w.Work == DP_CREATE {
        dat := new(nhDat)
        C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_nh_tact)
        if !w.resolved {
            dat.ca.act_type = C.DP_SET_DROP
        } else {
            if w.tunNh {
                fmt.Printf("Setting tunNh %x\n", key.nh_num)
                dat.ca.act_type = C.DP_SET_NEIGH_VXLAN
                vxAct = (*rtVxL2NhAct)(getPtrOffset(unsafe.Pointer(dat),
                                       C.sizeof_struct_dp_cmn_act))

                ipAddr := tk.IPtonl(w.rIP)
                vxAct.l3t.rip = C.uint(ipAddr)
                vxAct.l3t.sip = C.uint(tk.IPtonl(w.sIP))
                tid := ((w.tunID << 8) & 0xffffff00)
                vxAct.l3t.tid = C.uint(tk.Htonl(tid))

                fmt.Printf("rip 0x%x sip 0x%x 0x%x\n", vxAct.l3t.sip, vxAct.l3t.rip, vxAct.l3t.tid)

                act = (*rtL2NhAct)(&vxAct.l2nh)
                C.memcpy(unsafe.Pointer(&act.dmac[0]), unsafe.Pointer(&w.dstAddr[0]), 6)
                C.memcpy(unsafe.Pointer(&act.smac[0]), unsafe.Pointer(&w.srcAddr[0]), 6)
                act.bd = C.ushort(w.BD)
            } else {
                dat.ca.act_type = C.DP_SET_NEIGH_L2
                act = (*rtL2NhAct)(getPtrOffset(unsafe.Pointer(dat),
                    C.sizeof_struct_dp_cmn_act))
                C.memcpy(unsafe.Pointer(&act.dmac[0]), unsafe.Pointer(&w.dstAddr[0]), 6)
                C.memcpy(unsafe.Pointer(&act.smac[0]), unsafe.Pointer(&w.srcAddr[0]), 6)
                act.bd = C.ushort(w.BD)
                act.rnh_num = C.ushort(w.nNextHopNum)
            }
        }

        ret := C.llb_add_table_elem(C.LL_DP_NH_MAP,
                                    unsafe.Pointer(key),
                                    unsafe.Pointer(dat))
        if ret != 0 {
            return EBPF_ERR_NH_ADD
        }
        return 0
    } else if w.Work == DP_REMOVE {
        dat := new(nhDat)
        C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_nh_tact)
        //C.llb_del_table_elem(C.LL_DP_NH_MAP, unsafe.Pointer(key))
        // eBPF array elements cant be delete. Instead we just reset it
        C.llb_add_table_elem(C.LL_DP_NH_MAP,
            unsafe.Pointer(key),
            unsafe.Pointer(dat))
        return 0
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpNextHopAdd(w *NextHopDpWorkQ) int {
    fmt.Println(*w)
    return DpNextHopMod(w)
}

func (e *DpEbpfH) DpNextHopDel(w *NextHopDpWorkQ) int {
    fmt.Println(*w)
    return DpNextHopMod(w)
}

func DpRouteMod(w *RouteDpWorkQ) int {
    var act *rtL3NhAct
    var kPtr *[6]uint8

    key := new(rt4Key)

    len, _ := w.Dst.Mask.Size()
    len += 16 /* 16-bit ZoneNum + prefix-len */
    key.l.prefixlen = C.uint(len)
    kPtr = (*[6]uint8)(getPtrOffset(unsafe.Pointer(key),
                       C.sizeof_struct_bpf_lpm_trie_key))

    kPtr[0] = uint8(w.ZoneNum >> 8 & 0xff)
    kPtr[1] = uint8(w.ZoneNum & 0xff)
    kPtr[2] = uint8(w.Dst.IP[0])
    kPtr[3] = uint8(w.Dst.IP[1])
    kPtr[4] = uint8(w.Dst.IP[2])
    kPtr[5] = uint8(w.Dst.IP[3])

    if w.Work == DP_CREATE {
        dat := new(rtDat)
        C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_rt_tact)

        if w.NHwMark >= 0 {
            dat.ca.act_type = C.DP_SET_RT_NHNUM
            act = (*rtL3NhAct)(getPtrOffset(unsafe.Pointer(dat),
                C.sizeof_struct_dp_cmn_act))
            act.nh_num = C.ushort(w.NHwMark)
        } else {
            dat.ca.act_type = C.DP_SET_TOCP
        }

        if w.RtHwMark > 0 {
            dat.ca.cidx = C.uint(w.RtHwMark)
        }

        ret := C.llb_add_table_elem(C.LL_DP_RTV4_MAP,
            unsafe.Pointer(key),
            unsafe.Pointer(dat))
        if ret != 0 {
            return EBPF_ERR_RT4_ADD
        }
        return 0
    } else if w.Work == DP_REMOVE {
        C.llb_del_table_elem(C.LL_DP_RTV4_MAP, unsafe.Pointer(key))

        if w.RtHwMark > 0 {
            C.llb_clear_table_stats(C.LL_DP_RTV4_STATS_MAP, C.uint(w.RtHwMark))
        }
        return 0
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpRouteAdd(w *RouteDpWorkQ) int {
    fmt.Println(*w)
    return DpRouteMod(w)
}

func (e *DpEbpfH) DpRouteDel(w *RouteDpWorkQ) int {
    fmt.Println(*w)
    return DpRouteMod(w)
}

func DpNatLbRuleMod(w *NatDpWorkQ) int {

    key := new(nat4Key)

    key.daddr = C.uint(tk.IPtonl(w.ServiceIP))
    key.dport = C.ushort(tk.Htons(w.L4Port))
    key.l4proto = C.uchar(w.Proto)
    key.zone = C.ushort(w.ZoneNum)

    if w.Work == DP_CREATE {
        dat := new(nat4Acts)
        C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_natv4_tacts)
        if w.NatType == DP_SNAT {
            dat.ca.act_type = C.DP_SET_SNAT
        } else if w.NatType == DP_DNAT {
            dat.ca.act_type = C.DP_SET_DNAT
        } else {
            return EBPF_ERR_NAT4_ADD
        }

        switch {
        case w.EpSel == EP_RR:
            dat.sel_type = C.NAT_LB_SEL_RR
        case w.EpSel == EP_HASH:
            dat.sel_type = C.NAT_LB_SEL_HASH
        /* Currently not implemented in DP */
        /*case w.EpSel == EP_PRIO:
          dat.sel_type = C.NAT_LB_SEL_PRIO*/
        default:
            dat.sel_type = C.NAT_LB_SEL_RR
        }
        dat.ca.cidx = C.uint(w.HwMark)

        nxfa := (*nxfrmAct)(unsafe.Pointer(&dat.nxfrms[0]))

        for _, k := range w.endPoints {
            nxfa.wprio = C.ushort(k.weight)
            nxfa.nat_xport = C.ushort(tk.Htons(k.xPort))
            nxfa.nat_xip = C.uint(tk.IPtonl(k.xIP))

            if k.inActive {
                nxfa.inactive = 1
            }

            nxfa = (*nxfrmAct)(getPtrOffset(unsafe.Pointer(nxfa),
                C.sizeof_struct_mf_xfrm_inf))
        }

        // Any unused end-points should be marked inactive
        for i := len(w.endPoints); i < C.LLB_MAX_NXFRMS; i++ {
            nxfa := (*nxfrmAct)(unsafe.Pointer(&dat.nxfrms[i]))
            nxfa.inactive = 1
        }

        dat.nxfrm = C.uint(len(w.endPoints))

        ret := C.llb_add_table_elem(C.LL_DP_NAT4_MAP,
                                    unsafe.Pointer(key),
                                    unsafe.Pointer(dat))

        if ret != 0 {
            return EBPF_ERR_TMAC_ADD
        }

        return 0
    } else if w.Work == DP_REMOVE {

        C.llb_del_table_elem(C.LL_DP_NAT4_MAP, unsafe.Pointer(key))
    }

    return EBPF_ERR_WQ_UNK
}

func (e *DpEbpfH) DpNatLbRuleAdd(w *NatDpWorkQ) int {
    fmt.Println(*w)
    return DpNatLbRuleMod(w)
}

func (e *DpEbpfH) DpNatLbRuleDel(w *NatDpWorkQ) int {
    fmt.Println(*w)
    return DpNatLbRuleMod(w)
}

func (e *DpEbpfH) DpStat(w *StatDpWorkQ) int {
    var packets, bytes uint64
    var tbl []int
    switch {
    case w.Name == MAP_NAME_NAT4:
        tbl = append(tbl, int(C.LL_DP_NAT4_MAP))
        break
    case w.Name == MAP_NAME_BD:
        tbl = append(tbl, int(C.LL_DP_BD_STATS_MAP), int(C.LL_DP_TX_BD_STATS_MAP))
        break
    case w.Name == MAP_NAME_RXBD:
        tbl = append(tbl, int(C.LL_DP_BD_STATS_MAP))
        break
    case w.Name == MAP_NAME_TXBD:
        tbl = append(tbl, int(C.LL_DP_TX_BD_STATS_MAP))
        break
    case w.Name == MAP_NAME_RT4:
        tbl = append(tbl, int(C.LL_DP_RTV4_MAP))
        break
    default:
        return EBPF_ERR_WQ_UNK
    }

    if w.Work == DP_STATS_GET {
        var b C.longlong
        var p C.longlong

        packets = 0
        bytes = 0

        for _, t := range tbl {

            ret := C.llb_fetch_table_stats_cached(C.int(t), C.uint(w.HwMark),
                                                  (unsafe.Pointer(&b)), unsafe.Pointer(&p))
            if ret != 0 {
                return EBPF_ERR_TMAC_ADD
            }

            packets += uint64(p)
            bytes += uint64(b)
        }

        if packets != 0 && bytes != 0 {
            *w.Packets = uint64(packets)
            *w.Bytes = uint64(bytes)
        }
    } else if w.Work == DP_STATS_CLR {
        for _, t := range tbl {
            C.llb_clear_table_stats(C.int(t), C.uint(w.HwMark))
        }
    }

    return 0
}

func convDPCt2GoObj(ctKey *C.struct_dp_ctv4_key, ctDat *C.struct_dp_ctv4_dat) *DpCtInfo {
    ct := new(DpCtInfo)

    ct.dip = tk.NltoIP(uint32(ctKey.daddr))
    ct.sip = tk.NltoIP(uint32(ctKey.saddr))
    ct.dport = tk.Ntohs(uint16(ctKey.dport))
    ct.sport = tk.Ntohs(uint16(ctKey.sport))

    p := uint8(ctKey.l4proto)
    switch {
    case p == 1:
        ct.proto = "icmp"
        i := (*C.ct_icmp_pinf_t)(unsafe.Pointer(&ctDat.pi))
        switch {
        case i.state&C.CT_ICMP_DUNR != 0:
            ct.cState = "dest-unr"
        case i.state&C.CT_ICMP_TTL != 0:
            ct.cState = "ttl-exp"
        case i.state&C.CT_ICMP_RDR != 0:
            ct.cState = "icmp-redir"
        case i.state == C.CT_ICMP_CLOSED:
            ct.cState = "closed"
        case i.state == C.CT_ICMP_REQS:
            ct.cState = "req-sent"
        case i.state == C.CT_ICMP_REPS:
            ct.cState = "bidir"
        }
    case p == 6:
        ct.proto = "tcp"
        t := (*C.ct_tcp_pinf_t)(unsafe.Pointer(&ctDat.pi))
        switch {
        case t.state == C.CT_TCP_CLOSED:
            ct.cState = "closed"
        case t.state == C.CT_TCP_SS:
            ct.cState = "sync-sent"
        case t.state == C.CT_TCP_SA:
            ct.cState = "sync-ack"
        case t.state == C.CT_TCP_EST:
            ct.cState = "est"
        case t.state == C.CT_TCP_ERR:
            ct.cState = "err"
        case t.state == C.CT_TCP_CW:
            ct.cState = "closed-wait"
        default:
            ct.cState = "fini"
        }
    case p == 17:
        ct.proto = "udp"
        u := (*C.ct_udp_pinf_t)(unsafe.Pointer(&ctDat.pi))
        switch {
        case u.state == C.CT_UDP_CNI:
            ct.cState = "closed"
        case u.state == C.CT_UDP_UEST:
            ct.cState = "udp-uni"
        case u.state == C.CT_UDP_EST:
            ct.cState = "udp-est"
        default:
            ct.cState = "unk"
        }
    case p == 132:
        ct.proto = "sctp"
    default:
        ct.proto = fmt.Sprintf("%d", p)
    }

    if ctDat.xi.nat_flags == C.LLB_NAT_DST ||
        ctDat.xi.nat_flags == C.LLB_NAT_SRC {
        var xip net.IP

        xip = append(xip, uint8(ctDat.xi.nat_xip&0xff))
        xip = append(xip, uint8(ctDat.xi.nat_xip>>8&0xff))
        xip = append(xip, uint8(ctDat.xi.nat_xip>>16&0xff))
        xip = append(xip, uint8(ctDat.xi.nat_xip>>24&0xff))

        port := tk.Ntohs(uint16(ctDat.xi.nat_xport))

        if ctDat.xi.nat_flags == C.LLB_NAT_DST {
            ct.cAct = fmt.Sprintf("dnat-%s:%d:w%d", xip.String(), port, ctDat.xi.wprio)
        } else if ctDat.xi.nat_flags == C.LLB_NAT_SRC {
            ct.cAct = fmt.Sprintf("snat-%s:%d:w%d", xip.String(), port, ctDat.xi.wprio)
        }
    }

    return ct
}

func (e *DpEbpfH) DpTableGet(w *TableDpWorkQ) (error, DpRetT) {
    var tbl int


    if w.Work != DP_TABLE_GET {
        return errors.New("unknown work type"),  EBPF_ERR_WQ_UNK
    }

    switch {
    case w.Name == MAP_NAME_CT4:
        tbl = C.LL_DP_ACLV4_MAP
    default:
        return errors.New("unknown work type"), EBPF_ERR_WQ_UNK
    }

    if tbl == C.LL_DP_ACLV4_MAP {
        ctMap := make(map[string]*DpCtInfo)
        var n int = 0
        var key *C.struct_dp_ctv4_key = nil
        nextKey := new(C.struct_dp_ctv4_key)
        var tact C.struct_dp_aclv4_tact
        var act *C.struct_dp_ctv4_dat

        fd := C.llb_tb2fd(C.int(tbl))

        for C.bpf_map_get_next_key(C.int(fd), (unsafe.Pointer)(key), (unsafe.Pointer)(nextKey)) == 0 {
            ctKey := (*C.struct_dp_ctv4_key)(unsafe.Pointer(nextKey))

            if C.bpf_map_lookup_elem(C.int(fd), (unsafe.Pointer)(nextKey), (unsafe.Pointer)(&tact)) != 0 {
                continue
            }

            act = &tact.ctd

            if act.dir == C.CT_DIR_IN {
                goCt4Ent := convDPCt2GoObj(ctKey, act)
                ctMap[goCt4Ent.Key()] = goCt4Ent
            }
            key = nextKey
            n++
        }
        return nil, ctMap
    }

    return errors.New("unknown work type"), EBPF_ERR_WQ_UNK
}
