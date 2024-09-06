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
#include "../../loxilb-ebpf/kernel/loxilb_libdp.h"
int bpf_map_get_next_key(int fd, const void *key, void *next_key);
int bpf_map_lookup_elem(int fd, const void *key, void *value);
extern void goMapNotiHandler(struct ll_dp_map_notif *);
extern void goProxyEntCollector(struct dp_proxy_ct_ent *);
extern void goLinuxArpResolver(unsigned int);
#cgo CFLAGS:  -I./../../loxilb-ebpf/libbpf/src/ -I./../../loxilb-ebpf/common
#cgo LDFLAGS: -L. -L/lib64 -L./../../loxilb-ebpf/kernel -L./../../loxilb-ebpf/libbpf/src/build/usr/lib64/ -Wl,-rpath=/lib64/ -l:./../../loxilb-ebpf/kernel/libloxilbdp.a -l:./../../loxilb-ebpf/libbpf/src/libbpf.a -lelf -lz -lssl -lcrypto
*/
import "C"
import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	cmn "github.com/loxilb-io/loxilb/common"
	utils "github.com/loxilb-io/loxilb/pkg/utils"
	tk "github.com/loxilb-io/loxilib"
	nlp "github.com/vishvananda/netlink"
)

// This file implements the interface DpHookInterface
// The implementation is specific to loxilb ebpf datapath for linux

// error codes
const (
	EbpfErrBase = iota - 50000
	EbpfErrPortPropAdd
	EbpfErrPortPropDel
	EbpfErrEbpfLoad
	EbpfErrEbpfUnload
	EbpfErrL2AddrAdd
	EbpfErrL2AddrDel
	EbpfErrTmacAdd
	EbpfErrTmacDel
	EbpfErrNhAdd
	EbpfErrNhDel
	EbpfErrRt4Add
	EbpfErrRt4Del
	EbpfErrNat4Add
	EbpfErrNat4Del
	EbpfErrSess4Add
	EbpfErrSess4Del
	EbpfErrPolAdd
	EbpfErrPolDel
	EbpfErrMirrAdd
	EbpfErrMirrDel
	EbpfErrFwAdd
	EbpfErrFwDel
	EbpfErrCtAdd
	EbpfErrCtDel
	EbpfErrSockVIPMod
	EbpfErrSockVIPAdd
	EbpfErrSockVIPDel
	EbpfErrWqUnk
)

// constants
const (
	dpEbpfLinuxTiVal     = 10
	ctGCTiValDefault     = 25
	ctiDeleteSyncRetries = 3
	blkCtiMaxLen         = 8192
	mapNotifierChLen     = 8096
	mapNotifierWorkers   = 1
)

// ebpf table related defines in go
type (
	sActValue  C.struct_dp_cmn_act
	intfMapKey C.struct_intf_key
	intfMapDat C.struct_dp_intf_tact
	intfSetIfi C.struct_dp_intf_tact_set_ifi
	sMacKey    C.struct_dp_smac_key
	dMacKey    C.struct_dp_dmac_key
	dMacMapDat C.struct_dp_dmac_tact
	l2VlanAct  C.struct_dp_l2vlan_act
	tMacKey    C.struct_dp_tmac_key
	tMacDat    C.struct_dp_tmac_tact
	rtNhAct    C.struct_dp_rt_nh_act
	nhKey      C.struct_dp_nh_key
	nhDat      C.struct_dp_nh_tact
	rtL2NhAct  C.struct_dp_rt_l2nh_act
	rtTunNhAct C.struct_dp_rt_tunnh_act
	rt4Key     C.struct_dp_rtv4_key
	rt6Key     C.struct_dp_rtv6_key
	rtDat      C.struct_dp_rt_tact
	rtL3NhAct  C.struct_dp_rt_nh_act
	natKey     C.struct_dp_nat_key
	proxyActs  C.struct_dp_proxy_tacts
	nxfrmAct   C.struct_mf_xfrm_inf
	sess4Key   C.struct_dp_sess4_key
	sessAct    C.struct_dp_sess_tact
	polTact    C.struct_dp_pol_tact
	polAct     C.struct_dp_policer_act
	mirrTact   C.struct_dp_mirr_tact
	fw4Ent     C.struct_dp_fwv4_ent
	portAct    C.struct_dp_rdr_act
	mapNoti    C.struct_ll_dp_map_notif
	vipKey     C.struct_sock_rwr_key
	vipAct     C.struct_sock_rwr_action
	proxtCT    C.struct_dp_proxy_ct_ent
)

var (
	proxyCtInfo []*DpCtInfo
)

// DpEbpfH - context container
type DpEbpfH struct {
	ticker  *time.Ticker
	tDone   chan bool
	trigGC  chan bool
	gcTS    time.Time
	gcTiVal uint
	ctBcast chan bool
	nID     uint
	tbN     uint
	CtSync  bool
	RssEn   bool
	ToMapCh chan interface{}
	ToFinCh [mapNotifierWorkers]chan int
	mtx     sync.RWMutex
	ctMap   map[string]*DpCtInfo
}

// dpEbpfTicker - this ticker routine runs every DpEbpfLinuxTiVal seconds
func dpEbpfTicker() {

	// Stack trace logger
	defer func() {
		if e := recover(); e != nil {
			tk.LogIt(tk.LogCritical, "%s: %s", e, debug.Stack())
		}
		if mh.dp != nil {
			mh.dp.DpHooks.DpEbpfUnInit()
		}
		os.Exit(1)
	}()

	tbls := []int{int(C.LL_DP_RTV4_STATS_MAP),
		int(C.LL_DP_TMAC_STATS_MAP),
		int(C.LL_DP_BD_STATS_MAP),
		int(C.LL_DP_TX_BD_STATS_MAP),
		int(C.LL_DP_SESS4_STATS_MAP),
		int(C.LL_DP_FW4_STATS_MAP)}
	tLen := uint(len(tbls))

	for {
		if mh.dpEbpf == nil {
			continue
		}
		select {
		case <-mh.dpEbpf.tDone:
			return
		case <-mh.dpEbpf.ctBcast:
			tk.LogIt(tk.LogDebug, "CT Bcast\n")
			dpCTMapBcast()
			continue
		case <-mh.dpEbpf.trigGC:
			C.llb_age_map_entries(C.LL_DP_CT_MAP)
			C.llb_age_map_entries(C.LL_DP_FCV4_MAP)
			mh.dpEbpf.gcTS = time.Now()
		case t := <-mh.dpEbpf.ticker.C:
			sel := mh.dpEbpf.tbN % tLen
			tk.LogIt(-1, "DP Tick at for selector %v:%d\n", t, sel)

			// For every tick collect stats for an eBPF map
			// This routine caches stats in a local statsDB
			// which can be collected from a separate gothread
			C.llb_collect_map_stats(C.int(tbls[sel]))

			// Age any entries related to Conntrack
			/* No need to fetch all stats in this fashion */
			//C.llb_collect_map_stats(C.int(C.LL_DP_CT_STATS_MAP))
			/* Per entry stats will be fetched in C.ll_ct_map_ent_has_aged */
			if mh.dpEbpf.gcTiVal == 0 || time.Duration(time.Since(mh.dpEbpf.gcTS).Seconds()) > time.Duration(mh.dpEbpf.gcTiVal) {
				C.llb_age_map_entries(C.LL_DP_CT_MAP)
				C.llb_age_map_entries(C.LL_DP_FCV4_MAP)
				mh.dpEbpf.gcTS = time.Now()
			}

			// This means around 10s from start
			if !mh.dpEbpf.CtSync {
				tk.LogIt(tk.LogDebug, "Get xsync()\n")
				ret := mh.dp.DpXsyncRPC(DpSyncGet, nil)
				if ret == 0 {
					mh.dpEbpf.CtSync = true
				}
			}
			dpCTMapChkUpdates()
			mh.dpEbpf.tbN++
		}
	}
}

// DpEbpfDPLogLevel - Routine to set log level for DP
func DpEbpfDPLogLevel(cfg *C.struct_ebpfcfg, debug tk.LogLevelT) {
	switch debug {
	case tk.LogAlert:
		cfg.loglevel = 5 // LOG_FATAL
	case tk.LogCritical:
		cfg.loglevel = 5 // LOG_FATAL
	case tk.LogError:
		cfg.loglevel = 4 // LOG_ERROR
	case tk.LogWarning:
		cfg.loglevel = 3 // LOG_WARNING
	case tk.LogNotice:
		cfg.loglevel = 3 // LOG_WARNING
	case tk.LogInfo:
		cfg.loglevel = 2 // LOG_INFO
	case tk.LogTrace:
		cfg.loglevel = 0 // LOG_TRACE
	case tk.LogDebug:
	default:
		cfg.loglevel = 1 // LOG_DEBUG
	}
}

// DpEbpfSetLogLevel - Set log level for ebpf subsystem
func DpEbpfSetLogLevel(logLevel tk.LogLevelT) {
	cfg := C.struct_ebpfcfg{loglevel: 1}

	DpEbpfDPLogLevel(&cfg, logLevel)
	C.loxilb_set_loglevel(&cfg)
}

// DpEbpfInit - initialize the ebpf dp subsystem
func DpEbpfInit(clusterEn, rssEn, egrHooks, localSockPolicy, sockMapEn bool, nodeNum int, disBPF bool, logLevel tk.LogLevelT) *DpEbpfH {
	var cfg C.struct_ebpfcfg

	if clusterEn {
		cfg.have_mtrace = 1
	} else {
		cfg.have_mtrace = 0
	}
	if egrHooks {
		cfg.egr_hooks = 1
	} else {
		cfg.egr_hooks = 0
	}
	if localSockPolicy {
		cfg.have_sockrwr = 1
	} else {
		cfg.have_sockrwr = 0
	}
	if sockMapEn {
		cfg.have_sockmap = 1
	} else {
		cfg.have_sockmap = 0
	}

	if disBPF {
		cfg.have_noebpf = 1
	} else {
		cfg.have_noebpf = 0
	}

	cfg.nodenum = C.int(nodeNum)
	cfg.loglevel = 1
	cfg.no_loader = 0

	DpEbpfDPLogLevel(&cfg, logLevel)

	C.loxilb_main(&cfg)

	// Make sure to unload eBPF programs at init time
	ifList, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, intf := range ifList {
		if intf.Name == "llb0" {
			continue
		}
		tk.LogIt(tk.LogInfo, "ebpf unload - %s\n", intf.Name)
		ifStr := C.CString(intf.Name)
		section := C.CString(string(C.TC_LL_SEC_DEFAULT))
		C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 1)
		if rssEn {
			xSection := C.CString(string(C.XDP_LL_SEC_DEFAULT))
			C.llb_dp_link_attach(ifStr, xSection, C.LL_BPF_MOUNT_XDP, 1)
			C.free(unsafe.Pointer(xSection))
		}
		C.free(unsafe.Pointer(ifStr))
		C.free(unsafe.Pointer(section))
	}

	ne := new(DpEbpfH)
	ne.tDone = make(chan bool)
	ne.trigGC = make(chan bool)
	ne.gcTS = time.Now()
	ne.gcTiVal = ctGCTiValDefault
	ne.ToMapCh = make(chan interface{}, mapNotifierChLen)
	for i := 0; i < mapNotifierWorkers; i++ {
		ne.ToFinCh[i] = make(chan int)
	}
	ne.ctBcast = make(chan bool)
	ne.ticker = time.NewTicker(dpEbpfLinuxTiVal * time.Second)
	ne.ctMap = make(map[string]*DpCtInfo)
	ne.RssEn = rssEn
	ne.nID = uint((C.LLB_CT_MAP_ENTRIES / C.LLB_MAX_LB_NODES) * nodeNum)

	go dpEbpfTicker()
	for i := 0; i < mapNotifierWorkers; i++ {
		go dpMapNotifierWorker(ne.ToFinCh[i], ne.ToMapCh)
	}

	return ne
}

// DpEbpfUnInit - uninitialize the ebpf dp subsystem
func (e *DpEbpfH) DpEbpfUnInit() {

	e.tDone <- true
	for i := 0; i < mapNotifierWorkers; i++ {
		e.ToFinCh[i] <- 1
	}

	tk.LogIt(tk.LogInfo, "ebpf uninit : %s\n", debug.Stack())

	// Make sure to unload eBPF programs
	ifList, err := net.Interfaces()
	if err != nil {
		return
	}

	tk.LogIt(tk.LogInfo, "ebpf uninit begin\n")

	for _, intf := range ifList {

		tk.LogIt(tk.LogInfo, "ebpf unload - %s\n", intf.Name)
		ifStr := C.CString(intf.Name)
		section := C.CString(string(C.TC_LL_SEC_DEFAULT))
		C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 1)
		if e.RssEn || intf.Name == "llb0" {
			xSection := C.CString(string(C.XDP_LL_SEC_DEFAULT))
			C.llb_dp_link_attach(ifStr, xSection, C.LL_BPF_MOUNT_XDP, 1)
			C.free(unsafe.Pointer(xSection))
		}
		C.free(unsafe.Pointer(ifStr))
		C.free(unsafe.Pointer(section))
	}

	C.llb_unload_kern_all()
}

func convNetIP2DPv6Addr(addr unsafe.Pointer, goIP net.IP) {
	aPtr := (*C.uchar)(addr)
	for bp := 0; bp < 16; bp++ {
		*aPtr = C.uchar(goIP[bp])
		aPtr = (*C.uchar)(getPtrOffset(unsafe.Pointer(aPtr),
			C.sizeof_uchar))
	}
}

func convDPv6Addr2NetIP(addr unsafe.Pointer) net.IP {
	var goIP net.IP
	aPtr := (*C.uchar)(addr)

	for i := 0; i < 16; i++ {
		goIP = append(goIP, uint8(*aPtr))
		aPtr = (*C.uchar)(getPtrOffset(unsafe.Pointer(aPtr),
			C.sizeof_uchar))
	}
	return goIP
}

// loadEbpfPgm - load loxilb eBPF program to an interface
func (e *DpEbpfH) loadEbpfPgm(name string) int {
	if mh.disBPF {
		return 0
	}
	ifStr := C.CString(name)
	xSection := C.CString(string(C.XDP_LL_SEC_DEFAULT))
	link, err := nlp.LinkByName(name)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[DP] Port %s not found\n", name)
		return -1
	}
	if e.RssEn {
		C.llb_dp_link_attach(ifStr, xSection, C.LL_BPF_MOUNT_XDP, 0)
	}
	section := C.CString(string(C.TC_LL_SEC_DEFAULT))
	C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 0)

	filters, err := nlp.FilterList(link, nlp.HANDLE_MIN_INGRESS)
	if err != nil {
		tk.LogIt(tk.LogWarning, "[DP] Filter on %s not found\n", name)
		return -1
	}
	ret := -1
	for _, f := range filters {
		if t, ok := f.(*nlp.BpfFilter); ok {
			if strings.Contains(t.Name, "tc_packet_func") {
				ret = 0
				break
			}
		}
	}
	C.free(unsafe.Pointer(ifStr))
	C.free(unsafe.Pointer(xSection))
	C.free(unsafe.Pointer(section))
	return int(ret)
}

// unLoadEbpfPgm - unload loxilb eBPF program from an interface
func (e *DpEbpfH) unLoadEbpfPgm(name string) int {
	if mh.disBPF {
		return 0
	}
	ifStr := C.CString(name)
	xSection := C.CString(string(C.XDP_LL_SEC_DEFAULT))

	if e.RssEn {
		C.llb_dp_link_attach(ifStr, xSection, C.LL_BPF_MOUNT_XDP, 1)
	}

	section := C.CString(string(C.XDP_LL_SEC_DEFAULT))
	ret := C.llb_dp_link_attach(ifStr, section, C.LL_BPF_MOUNT_TC, 1)
	C.free(unsafe.Pointer(ifStr))
	C.free(unsafe.Pointer(section))
	C.free(unsafe.Pointer(xSection))
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
		tk.LogIt(tk.LogError, "Error %s", err)
		return false
	}

	ifstr := C.CString(portName)
	ifrStruct := make([]byte, 32)
	C.memcpy(unsafe.Pointer(&ifrStruct[0]), unsafe.Pointer(ifstr), 16)

	r0, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(sfd),
		syscall.SIOCGIFFLAGS,
		uintptr(unsafe.Pointer(&ifrStruct[0])))
	if r0 != 0 {
		C.free(unsafe.Pointer(ifstr))
		syscall.Close(sfd)
		tk.LogIt(tk.LogError, "Error %s", err)
		return false
	}

	C.free(unsafe.Pointer(ifstr))
	syscall.Close(sfd)

	var flags uint16
	C.memcpy(unsafe.Pointer(&flags), unsafe.Pointer(&ifrStruct[16]), 2)

	if flags&syscall.IFF_RUNNING != 0 {
		return true
	}

	return false
}

// DpPortPropMod - routine to work on a ebpf port property request
func (e *DpEbpfH) DpPortPropMod(w *PortDpWorkQ) int {
	var txK C.uint
	var txV C.uint
	var setIfi *intfSetIfi

	// This is a special case
	if w.LoadEbpf == "llb0" {
		w.PortNum = C.LLB_INTERFACES - 1
	}

	key := new(intfMapKey)
	key.ing_vid = C.ushort(tk.Htons(uint16(w.IngVlan)))
	key.ifindex = C.uint(w.OsPortNum)

	txK = C.uint(w.PortNum)

	if w.Work == DpCreate {

		if w.LoadEbpf != "" && w.LoadEbpf != "lo" && w.LoadEbpf != "llb0" {
			lRet := e.loadEbpfPgm(w.LoadEbpf)
			if lRet != 0 {
				tk.LogIt(tk.LogError, "ebpf load - %d error\n", w.PortNum)
				/* Shouldn't exit if the interface is not there, so return -1 and continue*/
				_, err := nlp.LinkByName(w.LoadEbpf)
				if err != nil {
					return -1
				}
				syscall.Exit(1)
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

		if w.Prop&cmn.PortPropUpp == cmn.PortPropUpp {
			setIfi.pprop = C.LLB_DP_PORT_UPP
		}

		ret := C.llb_add_map_elem(C.LL_DP_INTF_MAP, unsafe.Pointer(key), unsafe.Pointer(data))

		if ret != 0 {
			tk.LogIt(tk.LogError, "ebpf intfmap - %d vlan %d error\n", w.OsPortNum, w.IngVlan)
			return EbpfErrPortPropAdd
		}

		tk.LogIt(tk.LogDebug, "ebpf intfmap added - %d vlan %d -> %d\n", w.OsPortNum, w.IngVlan, w.PortNum)

		txV = C.uint(w.OsPortNum)
		ret = C.llb_add_map_elem(C.LL_DP_TX_INTF_MAP, unsafe.Pointer(&txK), unsafe.Pointer(&txV))
		if ret != 0 {
			C.llb_del_map_elem(C.LL_DP_INTF_MAP, unsafe.Pointer(key))
			tk.LogIt(tk.LogError, "ebpf txintfmap - %d error\n", w.OsPortNum)
			return EbpfErrPortPropAdd
		}
		tk.LogIt(tk.LogDebug, "ebpf txintfmap added - %d -> %d\n", w.PortNum, w.OsPortNum)
		return 0
	} else if w.Work == DpRemove {

		// TX_INTF_MAP is array type so we can't delete it
		// Rather we need to zero it out first
		txV = C.uint(0)
		C.llb_add_map_elem(C.LL_DP_TX_INTF_MAP, unsafe.Pointer(&txK), unsafe.Pointer(&txV))
		C.llb_del_map_elem(C.LL_DP_TX_INTF_MAP, unsafe.Pointer(&txK))

		C.llb_del_map_elem(C.LL_DP_INTF_MAP, unsafe.Pointer(key))

		if w.LoadEbpf != "" {
			lRet := e.unLoadEbpfPgm(w.LoadEbpf)
			if lRet != 0 {
				tk.LogIt(tk.LogError, "ebpf unload - ifi %d error\n", w.OsPortNum)
				return EbpfErrEbpfLoad
			}
			tk.LogIt(tk.LogDebug, "ebpf unloaded - ifi %d\n", w.OsPortNum)
		}

		return 0
	}

	return EbpfErrWqUnk
}

// DpPortPropAdd - routine to work on a ebpf port property add
func (e *DpEbpfH) DpPortPropAdd(w *PortDpWorkQ) int {
	return e.DpPortPropMod(w)
}

// DpPortPropDel - routine to work on a ebpf port property delete
func (e *DpEbpfH) DpPortPropDel(w *PortDpWorkQ) int {
	return e.DpPortPropMod(w)
}

// DpL2AddrMod - routine to work on a ebpf l2 addr request
func DpL2AddrMod(w *L2AddrDpWorkQ) int {
	var l2va *l2VlanAct

	skey := new(sMacKey)
	C.memcpy(unsafe.Pointer(&skey.smac[0]), unsafe.Pointer(&w.L2Addr[0]), 6)
	skey.bd = C.ushort((uint16(w.BD)))

	dkey := new(dMacKey)
	C.memcpy(unsafe.Pointer(&dkey.dmac[0]), unsafe.Pointer(&w.L2Addr[0]), 6)
	dkey.bd = C.ushort((uint16(w.BD)))

	if w.Work == DpCreate {
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

		ret := C.llb_add_map_elem(C.LL_DP_SMAC_MAP,
			unsafe.Pointer(skey),
			unsafe.Pointer(sdat))
		if ret != 0 {
			return EbpfErrL2AddrAdd
		}

		if w.Tun == 0 {
			ret = C.llb_add_map_elem(C.LL_DP_DMAC_MAP,
				unsafe.Pointer(dkey),
				unsafe.Pointer(ddat))
			if ret != 0 {
				C.llb_del_map_elem(C.LL_DP_SMAC_MAP, unsafe.Pointer(skey))
				return EbpfErrL2AddrAdd
			}
		}

		return 0
	} else if w.Work == DpRemove {

		C.llb_del_map_elem(C.LL_DP_SMAC_MAP, unsafe.Pointer(skey))

		if w.Tun == 0 {
			C.llb_del_map_elem(C.LL_DP_DMAC_MAP, unsafe.Pointer(dkey))
		}

		return 0
	}

	return EbpfErrWqUnk
}

// DpL2AddrAdd - routine to work on a ebpf l2 addr add
func (e *DpEbpfH) DpL2AddrAdd(w *L2AddrDpWorkQ) int {
	return DpL2AddrMod(w)
}

// DpL2AddrDel - routine to work on a ebpf l2 addr delete
func (e *DpEbpfH) DpL2AddrDel(w *L2AddrDpWorkQ) int {
	return DpL2AddrMod(w)
}

// DpRouterMacMod - routine to work on a ebpf rt-mac change request
func DpRouterMacMod(w *RouterMacDpWorkQ) int {

	key := new(tMacKey)
	C.memcpy(unsafe.Pointer(&key.mac[0]), unsafe.Pointer(&w.L2Addr[0]), 6)
	switch {
	case w.TunType == DpTunVxlan:
		key.tun_type = C.LLB_TUN_VXLAN
	case w.TunType == DpTunGre:
		key.tun_type = C.LLB_TUN_GRE
	case w.TunType == DpTunGtp:
		key.tun_type = C.LLB_TUN_GTP
	case w.TunType == DpTunStt:
		key.tun_type = C.LLB_TUN_STT
	}

	key.tunnel_id = C.uint(w.TunID)

	if w.Work == DpCreate {
		dat := new(tMacDat)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_tmac_tact)
		if w.TunID != 0 {
			if w.NhNum == 0 {
				dat.ca.act_type = C.DP_SET_RM_VXLAN
				rtNhAct := (*rtNhAct)(getPtrOffset(unsafe.Pointer(dat),
					C.sizeof_struct_dp_cmn_act))
				C.memset(unsafe.Pointer(rtNhAct), 0, C.sizeof_struct_dp_rt_nh_act)
				rtNhAct.nh_num[0] = 0
				rtNhAct.tid = 0
				rtNhAct.bd = C.ushort(w.BD)
			} else {
				/* No need for tunnel ID in case of Access side */
				key.tunnel_id = 0
				key.tun_type = 0
				dat.ca.act_type = C.DP_SET_RT_TUN_NH
				rtNhAct := (*rtNhAct)(getPtrOffset(unsafe.Pointer(dat),
					C.sizeof_struct_dp_cmn_act))
				C.memset(unsafe.Pointer(rtNhAct), 0, C.sizeof_struct_dp_rt_nh_act)

				rtNhAct.nh_num[0] = C.ushort(w.NhNum)
				tid := ((w.TunID << 8) & 0xffffff00)
				rtNhAct.tid = C.uint(tk.Htonl(tid))
			}
		} else {
			dat.ca.act_type = C.DP_SET_L3_EN
		}

		ret := C.llb_add_map_elem(C.LL_DP_TMAC_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))

		if ret != 0 {
			if w.Status != nil {
				*w.Status = DpCreateErr
			}
			return EbpfErrTmacAdd
		}

		if w.Status != nil {
			*w.Status = 0
		}

		return 0
	} else if w.Work == DpRemove {

		C.llb_del_map_elem(C.LL_DP_TMAC_MAP, unsafe.Pointer(key))
	}

	return EbpfErrWqUnk
}

// DpRouterMacAdd - routine to work on a ebpf rt-mac add request
func (e *DpEbpfH) DpRouterMacAdd(w *RouterMacDpWorkQ) int {
	return DpRouterMacMod(w)
}

// DpRouterMacDel - routine to work on a ebpf rt-mac delete request
func (e *DpEbpfH) DpRouterMacDel(w *RouterMacDpWorkQ) int {
	return DpRouterMacMod(w)
}

// DpNextHopMod - routine to work on a ebpf next-hop change request
func DpNextHopMod(w *NextHopDpWorkQ) int {
	var act *rtL2NhAct
	var tunAct *rtTunNhAct

	key := new(nhKey)
	key.nh_num = C.uint(w.NextHopNum)

	if w.Work == DpCreate {
		dat := new(nhDat)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_nh_tact)
		if !w.Resolved {
			dat.ca.act_type = C.DP_SET_TOCP
		} else {
			if w.TunNh {
				tk.LogIt(tk.LogDebug, "Setting tunNh 0x%x\n", key.nh_num)
				if w.TunType == DpTunIPIP {
					dat.ca.act_type = C.DP_SET_NEIGH_IPIP
				} else {
					dat.ca.act_type = C.DP_SET_NEIGH_VXLAN
				}
				tunAct = (*rtTunNhAct)(getPtrOffset(unsafe.Pointer(dat),
					C.sizeof_struct_dp_cmn_act))

				ipAddr := tk.IPtonl(w.RIP)
				tunAct.l3t.rip = C.uint(ipAddr)
				tunAct.l3t.sip = C.uint(tk.IPtonl(w.SIP))
				tid := ((w.TunID << 8) & 0xffffff00)
				tunAct.l3t.tid = C.uint(tk.Htonl(tid))

				act = (*rtL2NhAct)(&tunAct.l2nh)
				C.memcpy(unsafe.Pointer(&act.dmac[0]), unsafe.Pointer(&w.DstAddr[0]), 6)
				C.memcpy(unsafe.Pointer(&act.smac[0]), unsafe.Pointer(&w.SrcAddr[0]), 6)
				act.bd = C.ushort(w.BD)
			} else {
				dat.ca.act_type = C.DP_SET_NEIGH_L2
				act = (*rtL2NhAct)(getPtrOffset(unsafe.Pointer(dat),
					C.sizeof_struct_dp_cmn_act))
				C.memcpy(unsafe.Pointer(&act.dmac[0]), unsafe.Pointer(&w.DstAddr[0]), 6)
				C.memcpy(unsafe.Pointer(&act.smac[0]), unsafe.Pointer(&w.SrcAddr[0]), 6)
				act.bd = C.ushort(w.BD)
				act.rnh_num = C.ushort(w.NNextHopNum)
			}
		}

		ret := C.llb_add_map_elem(C.LL_DP_NH_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))
		if ret != 0 {
			return EbpfErrNhAdd
		}
		return 0
	} else if w.Work == DpRemove {
		dat := new(nhDat)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_nh_tact)
		//C.llb_del_table_elem(C.LL_DP_NH_MAP, unsafe.Pointer(key))
		// eBPF array elements cant be deleted. Instead we just reset it
		C.llb_add_map_elem(C.LL_DP_NH_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))
		return 0
	}

	return EbpfErrWqUnk
}

// DpNextHopAdd - routine to work on a ebpf next-hop add request
func (e *DpEbpfH) DpNextHopAdd(w *NextHopDpWorkQ) int {
	return DpNextHopMod(w)
}

// DpNextHopDel - routine to work on a ebpf next-hop delete request
func (e *DpEbpfH) DpNextHopDel(w *NextHopDpWorkQ) int {
	return DpNextHopMod(w)
}

// DpRouteMod - routine to work on a ebpf route change request
func DpRouteMod(w *RouteDpWorkQ) int {
	var mapNum C.int
	var mapSnum C.int
	var act *rtL3NhAct
	var kPtr *[6]uint8
	var key unsafe.Pointer

	if w.ZoneNum == 0 {
		tk.LogIt(tk.LogError, "ZoneNum must be specified\n")
		syscall.Exit(1)
	}

	if tk.IsNetIPv4(w.Dst.IP.String()) {
		key4 := new(rt4Key)

		len, _ := w.Dst.Mask.Size()
		len += 16 /* 16-bit ZoneNum + prefix-len */
		key4.l.prefixlen = C.uint(len)
		kPtr = (*[6]uint8)(getPtrOffset(unsafe.Pointer(key4),
			C.sizeof_struct_bpf_lpm_trie_key))

		kPtr[0] = uint8(w.ZoneNum >> 8 & 0xff)
		kPtr[1] = uint8(w.ZoneNum & 0xff)
		kPtr[2] = uint8(w.Dst.IP[0])
		kPtr[3] = uint8(w.Dst.IP[1])
		kPtr[4] = uint8(w.Dst.IP[2])
		kPtr[5] = uint8(w.Dst.IP[3])
		key = unsafe.Pointer(key4)
		mapNum = C.LL_DP_RTV4_MAP
		mapSnum = C.LL_DP_RTV4_STATS_MAP
	} else {
		key6 := new(rt6Key)

		len, _ := w.Dst.Mask.Size()
		key6.l.prefixlen = C.uint(len)

		k6Ptr := (*C.uchar)(getPtrOffset(unsafe.Pointer(key6),
			C.sizeof_struct_bpf_lpm_trie_key))

		for bp := 0; bp < 16; bp++ {
			*k6Ptr = C.uchar(w.Dst.IP[bp])
			k6Ptr = (*C.uchar)(getPtrOffset(unsafe.Pointer(k6Ptr),
				C.sizeof_uchar))
		}
		key = unsafe.Pointer(key6)
		mapNum = C.LL_DP_RTV6_MAP
		mapSnum = C.LL_DP_RTV6_STATS_MAP
	}

	if w.Work == DpCreate {
		dat := new(rtDat)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_rt_tact)

		if w.NMax > 0 {
			dat.ca.act_type = C.DP_SET_RT_NHNUM
			act = (*rtL3NhAct)(getPtrOffset(unsafe.Pointer(dat),
				C.sizeof_struct_dp_cmn_act))
			act.naps = C.ushort(w.NMax)
			for i := range w.NMark {
				if i < C.DP_MAX_ACTIVE_PATHS {
					act.nh_num[i] = C.ushort(w.NMark[i])
				}
			}
		} else {
			mLen, _ := w.Dst.Mask.Size()
			if mLen == 32 || mLen == 128 {
				dat.ca.act_type = C.DP_SET_TOCP
			} else {
				dat.ca.act_type = C.DP_SET_NOP
			}
		}

		if w.RtMark > 0 {
			dat.ca.cidx = C.uint(w.RtMark)
		}

		ret := C.llb_add_map_elem(mapNum,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))
		if ret != 0 {
			return EbpfErrRt4Add
		}
		return 0
	} else if w.Work == DpRemove {
		C.llb_del_map_elem(mapNum, unsafe.Pointer(key))

		if w.RtMark > 0 {
			C.llb_clear_map_stats(mapSnum, C.uint(w.RtMark))
		}
		return 0
	}

	return EbpfErrWqUnk
}

// DpRouteAdd - routine to work on a ebpf route add request
func (e *DpEbpfH) DpRouteAdd(w *RouteDpWorkQ) int {
	return DpRouteMod(w)
}

// DpRouteDel - routine to work on a ebpf route delete request
func (e *DpEbpfH) DpRouteDel(w *RouteDpWorkQ) int {
	return DpRouteMod(w)
}

// DpNatLbRuleMod - routine to work on a ebpf nat-lb change request
func DpNatLbRuleMod(w *NatDpWorkQ) int {

	key := new(natKey)

	key.mark = C.ushort(w.BlockNum)

	if w.NatType == DpSnat {
		key.mark |= 0x1000
	} else {
		key.daddr = [4]C.uint{0, 0, 0, 0}
		if tk.IsNetIPv4(w.ServiceIP.String()) {
			key.daddr[0] = C.uint(tk.IPtonl(w.ServiceIP))
			key.v6 = 0
		} else {
			convNetIP2DPv6Addr(unsafe.Pointer(&key.daddr[0]), w.ServiceIP)
			key.v6 = 1
		}
		key.mark = C.ushort(w.BlockNum)
		key.dport = C.ushort(tk.Htons(w.L4Port))
		key.l4proto = C.uchar(w.Proto)
		key.zone = C.ushort(w.ZoneNum)
	}

	dat := new(proxyActs)
	C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_proxy_tacts)
	if w.NatType == DpSnat {
		dat.ca.act_type = C.DP_SET_SNAT
	} else if w.NatType == DpDnat || w.NatType == DpFullNat {
		dat.ca.act_type = C.DP_SET_DNAT
	} else if w.NatType == DpFullProxy {
		dat.ca.act_type = C.DP_SET_FULLPROXY
	} else {
		tk.LogIt(tk.LogDebug, "[DP] LB rule %s add[NOK] - EbpfErrNat4Add\n", w.ServiceIP.String())
		return EbpfErrNat4Add
	}

	// seconds to nanoseconds
	dat.ito = C.uint64_t(w.InActTo * 1000000000)
	dat.pto = C.uint64_t(w.PersistTo * 1000000000)
	dat.base_to = 0

	/*dat.npmhh = 2
	dat.pmhh[0] = 0x64646464
	dat.pmhh[1] = 0x65656565*/
	for i, k := range w.secIP {
		dat.pmhh[i] = C.uint(tk.IPtonl(k))
	}
	dat.npmhh = C.uchar(len(w.secIP))

	switch {
	case w.EpSel == EpRR:
		dat.sel_type = C.NAT_LB_SEL_RR
	case w.EpSel == EpHash:
		dat.sel_type = C.NAT_LB_SEL_HASH
	case w.EpSel == EpRRPersist:
		dat.sel_type = C.NAT_LB_SEL_RR_PERSIST
	case w.EpSel == EpLeastConn:
		dat.sel_type = C.NAT_LB_SEL_LC
	case w.EpSel == EpN2:
		dat.sel_type = C.NAT_LB_SEL_N2
	/* Currently not implemented in DP */
	/*case w.EpSel == EP_PRIO:
	  dat.sel_type = C.NAT_LB_SEL_PRIO*/
	default:
		dat.sel_type = C.NAT_LB_SEL_RR
	}
	dat.ca.cidx = C.uint(w.Mark)
	if w.DsrMode {
		dat.ca.oaux = 1
	}

	nxfa := (*nxfrmAct)(unsafe.Pointer(&dat.nxfrms[0]))

	for _, k := range w.endPoints {
		nxfa.wprio = C.uchar(k.Weight)
		nxfa.nat_xport = C.ushort(tk.Htons(k.XPort))
		if tk.IsNetIPv6(k.XIP.String()) {
			convNetIP2DPv6Addr(unsafe.Pointer(&nxfa.nat_xip[0]), k.XIP)

			if tk.IsNetIPv6(k.RIP.String()) {
				convNetIP2DPv6Addr(unsafe.Pointer(&nxfa.nat_rip[0]), k.RIP)
			}
			nxfa.nv6 = 1
		} else {
			nxfa.nat_xip[0] = C.uint(tk.IPtonl(k.XIP))
			nxfa.nat_rip[0] = C.uint(tk.IPtonl(k.RIP))
			nxfa.nv6 = 0
		}

		if k.InActive {
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

	dat.nxfrm = C.ushort(len(w.endPoints))
	if w.CsumDis {
		dat.cdis = 1
	} else {
		dat.cdis = 0
	}

	if w.SecMode == DpTermHTTPS {
		dat.sec_mode = C.SEC_MODE_HTTPS
	} else if w.SecMode == DpE2EHTTPS {
		dat.sec_mode = C.SEC_MODE_HTTPS_E2E
	}

	hostURLStr := C.CString(w.HostURL)
	C.memcpy(unsafe.Pointer(&dat.host_url[0]), unsafe.Pointer(hostURLStr), C.ulong(len(w.HostURL))+1)

	if w.Work == DpCreate {
		ret := C.llb_add_map_elem(C.LL_DP_NAT_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))

		if ret != 0 {
			tk.LogIt(tk.LogDebug, "[DP] LB rule %s add[NOK]\n", w.ServiceIP.String())
			return EbpfErrTmacAdd
		}
		tk.LogIt(tk.LogDebug, "[DP] LB rule %s add[OK]\n", w.ServiceIP.String())
		return 0
	} else if w.Work == DpRemove {
		C.llb_del_map_elem_wval(C.LL_DP_NAT_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))
		return 0
	}

	return EbpfErrWqUnk
}

// DpNatLbRuleAdd - routine to work on a ebpf nat-lb add request
func (e *DpEbpfH) DpNatLbRuleAdd(w *NatDpWorkQ) int {
	ec := DpNatLbRuleMod(w)
	if ec != 0 {
		*w.Status = DpCreateErr
	} else {
		*w.Status = 0
	}
	return ec
}

// DpNatLbRuleDel - routine to work on a ebpf nat-lb delete request
func (e *DpEbpfH) DpNatLbRuleDel(w *NatDpWorkQ) int {
	return DpNatLbRuleMod(w)
}

// DpStat - routine to work on a ebpf map statistics request
func (e *DpEbpfH) DpStat(w *StatDpWorkQ) int {
	var packets, bytes, dropPackets uint64
	var tbl []int
	var polTbl []int
	sync := 0
	switch {
	case w.Name == MapNameNat4:
		tbl = append(tbl, int(C.LL_DP_NAT_STATS_MAP))
		sync = 1
	case w.Name == MapNameBD:
		tbl = append(tbl, int(C.LL_DP_BD_STATS_MAP), int(C.LL_DP_TX_BD_STATS_MAP))
	case w.Name == MapNameRxBD:
		tbl = append(tbl, int(C.LL_DP_BD_STATS_MAP))
	case w.Name == MapNameTxBD:
		tbl = append(tbl, int(C.LL_DP_TX_BD_STATS_MAP))
	case w.Name == MapNameRt4:
		tbl = append(tbl, int(C.LL_DP_RTV4_MAP))
	case w.Name == MapNameULCL:
		tbl = append(tbl, int(C.LL_DP_SESS4_MAP))
	case w.Name == MapNameIpol:
		polTbl = append(polTbl, int(C.LL_DP_POL_MAP))
	case w.Name == MapNameFw4:
		tbl = append(tbl, int(C.LL_DP_FW4_MAP))
	default:
		return EbpfErrWqUnk
	}

	if w.Work == DpStatsGet || w.Work == DpStatsGetImm {
		var b C.longlong
		var p C.longlong

		packets = 0
		bytes = 0
		dropPackets = 0

		if w.Work == DpStatsGetImm {
			sync = 1
		}

		for _, t := range tbl {

			ret := C.llb_fetch_map_stats_cached(C.int(t), C.uint(w.Mark), C.int(sync),
				(unsafe.Pointer(&b)), unsafe.Pointer(&p))
			if ret != 0 {
				return EbpfErrTmacAdd
			}

			packets += uint64(p)
			bytes += uint64(b)
		}

		for _, t := range polTbl {

			ret := C.llb_fetch_pol_map_stats(C.int(t), C.uint(w.Mark), (unsafe.Pointer(&p)), unsafe.Pointer(&b))
			if ret != 0 {
				return EbpfErrTmacAdd
			}

			packets += uint64(p)
			dropPackets += uint64(b)
		}

		if packets != 0 || bytes != 0 || dropPackets != 0 {
			if w.Packets != nil {
				*w.Packets = uint64(packets)
			}
			if w.Bytes != nil {
				*w.Bytes = uint64(bytes)
			}
			if w.DropPackets != nil {
				*w.DropPackets = uint64(dropPackets)
			}
		}
	} else if w.Work == DpStatsClr {
		for _, t := range tbl {
			C.llb_clear_map_stats(C.int(t), C.uint(w.Mark))
		}
	}

	return 0
}

func (ct *DpCtInfo) convDPCt2GoObjFixup(ctKey *C.struct_dp_ct_key, ctDat *C.struct_dp_ct_dat, fixup bool) *DpCtInfo {
	if ctKey.v6 == 0 {
		ct.DIP = tk.NltoIP(uint32(ctKey.daddr[0]))
		ct.SIP = tk.NltoIP(uint32(ctKey.saddr[0]))
	} else {
		ct.SIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.saddr[0]))
		ct.DIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.daddr[0]))
	}
	ct.Dport = tk.Ntohs(uint16(ctKey.dport))
	ct.Sport = tk.Ntohs(uint16(ctKey.sport))

	p := uint8(ctKey.l4proto)
	switch {
	case p == 1 || p == 58:
		if p == 1 {
			ct.Proto = "icmp"
		} else {
			ct.Proto = "icmp6"
		}
	case p == 6:
		ct.Proto = "tcp"
	case p == 17:
		ct.Proto = "udp"
	case p == 132:
		ct.Proto = "sctp"
	default:
		ct.Proto = fmt.Sprintf("%d", p)
	}

	if ctDat == nil {
		ct.CAct = "n/a"
		ct.CState = "closed"
		return ct
	}

	switch {
	case p == 1 || p == 58:
		if p == 1 {
			ct.Proto = "icmp"
		} else {
			ct.Proto = "icmp6"
		}
		i := (*C.ct_icmp_pinf_t)(unsafe.Pointer(&ctDat.pi))
		switch {
		case i.state&C.CT_ICMP_DUNR != 0:
			ct.CState = "dest-unr"
		case i.state&C.CT_ICMP_TTL != 0:
			ct.CState = "ttl-exp"
		case i.state&C.CT_ICMP_RDR != 0:
			ct.CState = "icmp-redir"
		case i.state == C.CT_ICMP_CLOSED:
			ct.CState = "closed"
		case i.state == C.CT_ICMP_REQS:
			ct.CState = "req-sent"
		case i.state == C.CT_ICMP_REPS:
			ct.CState = "bidir"
		}
	case p == 6:
		ct.Proto = "tcp"
		t := (*C.ct_tcp_pinf_t)(unsafe.Pointer(&ctDat.pi))
		switch {
		case t.state == C.CT_TCP_CLOSED:
			ct.CState = "closed"
		case t.state == C.CT_TCP_SS:
			ct.CState = "sync-sent"
		case t.state == C.CT_TCP_SA:
			ct.CState = "sync-ack"
		case t.state == C.CT_TCP_EST:
			ct.CState = "est"
		case t.state == C.CT_TCP_ERR:
			ct.CState = "h/e"
		case t.state == C.CT_TCP_CW:
			ct.CState = "closed-wait"
		default:
			ct.CState = "fini"
		}
	case p == 17:
		ct.Proto = "udp"
		u := (*C.ct_udp_pinf_t)(unsafe.Pointer(&ctDat.pi))
		switch {
		case u.state == C.CT_UDP_CNI:
			ct.CState = "closed"
		case u.state == C.CT_UDP_UEST:
			ct.CState = "udp-uni"
		case u.state == C.CT_UDP_EST:
			ct.CState = "udp-est"
		default:
			ct.CState = "unk"
		}
	case p == 132:
		ct.Proto = "sctp"
		s := (*C.ct_sctp_pinf_t)(unsafe.Pointer(&ctDat.pi))
		switch {
		case s.state == C.CT_SCTP_PRE_EST:
			ct.CState = "pre-est"
		case s.state == C.CT_SCTP_EST:
			ct.CState = "est"
		case s.state == C.CT_SCTP_CLOSED:
			ct.CState = "closed"
		case s.state == C.CT_SCTP_ERR:
			ct.CState = "err"
		case s.state == C.CT_SCTP_INIT:
			ct.CState = "init"
		case s.state == C.CT_SCTP_INITA:
			ct.CState = "init-ack"
		case s.state == C.CT_SCTP_COOKIE:
			ct.CState = "cookie-echo"
		case s.state == C.CT_SCTP_COOKIEA:
			ct.CState = "cookie-echo-resp"
		case s.state == C.CT_SCTP_SHUT:
			ct.CState = "shut"
		case s.state == C.CT_SCTP_SHUTA:
			ct.CState = "shut-ack"
		case s.state == C.CT_SCTP_SHUTC:
			ct.CState = "shut-complete"
		case s.state == C.CT_SCTP_ABRT:
			ct.CState = "abort"
		default:
			ct.CState = "unk"
		}
	default:
		ct.Proto = fmt.Sprintf("%d", p)
	}

	ct.Packets = uint64(ctDat.pb.packets)
	ct.Bytes = uint64(ctDat.pb.bytes)

	if ctDat.xi.nat_flags == C.LLB_NAT_DST ||
		ctDat.xi.nat_flags == C.LLB_NAT_SRC ||
		ctDat.xi.nat_flags == C.LLB_NAT_HDST ||
		ctDat.xi.nat_flags == C.LLB_NAT_HSRC {
		var xip net.IP

		if ctDat.xi.nv6 == 0 {
			xip = append(xip, uint8(ctDat.xi.nat_xip[0]&0xff))
			xip = append(xip, uint8(ctDat.xi.nat_xip[0]>>8&0xff))
			xip = append(xip, uint8(ctDat.xi.nat_xip[0]>>16&0xff))
			xip = append(xip, uint8(ctDat.xi.nat_xip[0]>>24&0xff))
		} else {
			xip = convDPv6Addr2NetIP(unsafe.Pointer(&ctDat.xi.nat_xip[0]))
		}

		port := tk.Ntohs(uint16(ctDat.xi.nat_xport))
		if fixup {
			if ctDat.xi.osp != 0 {
				aSport := tk.Ntohs(uint16(ctDat.xi.osp))
				aDport := tk.Ntohs(uint16(ctDat.xi.odp))
				ct.CState = fmt.Sprintf("frag:%d->%d", aSport, aDport)
			}
		}

		if ctDat.xi.nat_flags == C.LLB_NAT_DST || ctDat.xi.nat_flags == C.LLB_NAT_HDST {
			if ctDat.xi.nat_rip[0] == 0 && ctDat.xi.nat_rip[1] == 0 &&
				ctDat.xi.nat_rip[2] == 0 && ctDat.xi.nat_rip[3] == 0 {
				nmode := ""
				if ctDat.xi.dsr != 0 {
					nmode = "ddsr"
				} else {
					if ctDat.xi.nat_flags == C.LLB_NAT_HDST {
						nmode = "hdnat"
					} else {
						nmode = "dnat"
					}
				}
				ct.CAct = fmt.Sprintf("%s-%s:%d:w%d", nmode, xip.String(), port, ctDat.xi.wprio)
			} else {
				var rip net.IP

				if ctDat.xi.nv6 == 0 {
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>8&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>16&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>24&0xff))
				} else {
					rip = convDPv6Addr2NetIP(unsafe.Pointer(&ctDat.xi.nat_rip[0]))
				}
				ct.CAct = fmt.Sprintf("fdnat-%s,%s:%d:w%d", rip.String(), xip.String(), port, ctDat.xi.wprio)
			}
		} else if ctDat.xi.nat_flags == C.LLB_NAT_SRC || ctDat.xi.nat_flags == C.LLB_NAT_HSRC {
			if ctDat.xi.nat_rip[0] == 0 && ctDat.xi.nat_rip[1] == 0 &&
				ctDat.xi.nat_rip[2] == 0 && ctDat.xi.nat_rip[3] == 0 {
				nmode := ""
				if ctDat.xi.dsr != 0 {
					nmode = "sdsr"
				} else {
					if ctDat.xi.nat_flags == C.LLB_NAT_HSRC {
						nmode = "hsnat"
					} else {
						nmode = "snat"
					}
				}
				ct.CAct = fmt.Sprintf("%s-%s:%d:w%d", nmode, xip.String(), port, ctDat.xi.wprio)
			} else {
				var rip net.IP

				if ctDat.xi.nv6 == 0 {
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>8&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>16&0xff))
					rip = append(rip, uint8(ctDat.xi.nat_rip[0]>>24&0xff))
				} else {
					rip = convDPv6Addr2NetIP(unsafe.Pointer(&ctDat.xi.nat_rip[0]))
				}
				ct.CAct = fmt.Sprintf("fsnat-%s,%s:%d:w%d", xip.String(), rip.String(), port, ctDat.xi.wprio)
			}
		}
	}

	return ct
}

func (ct *DpCtInfo) convDPCt2GoObj(ctKey *C.struct_dp_ct_key, ctDat *C.struct_dp_ct_dat) *DpCtInfo {
	return ct.convDPCt2GoObjFixup(ctKey, ctDat, false)
}

func (ct *DpCtInfo) convDPCtKey2GoObj(ctKey *C.struct_dp_ct_key) *DpCtInfo {
	if ctKey.v6 == 0 {
		ct.DIP = tk.NltoIP(uint32(ctKey.daddr[0]))
		ct.SIP = tk.NltoIP(uint32(ctKey.saddr[0]))
	} else {
		ct.SIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.saddr[0]))
		ct.DIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.daddr[0]))
	}
	ct.Dport = tk.Ntohs(uint16(ctKey.dport))
	ct.Sport = tk.Ntohs(uint16(ctKey.sport))

	p := uint8(ctKey.l4proto)
	switch {
	case p == 1 || p == 58:
		if p == 1 {
			ct.Proto = "icmp"
		} else {
			ct.Proto = "icmp6"
		}
	case p == 6:
		ct.Proto = "tcp"
	case p == 17:
		ct.Proto = "udp"
	case p == 132:
		ct.Proto = "sctp"
	default:
		ct.Proto = fmt.Sprintf("%d", p)
	}
	return ct
}

func (ct *DpCtInfo) convDPCtProxy2ActString(ctKey *C.struct_dp_ct_key) {
	var DIP net.IP
	var SIP net.IP

	if ctKey.v6 == 0 {
		DIP = tk.NltoIP(uint32(ctKey.daddr[0]))
		SIP = tk.NltoIP(uint32(ctKey.saddr[0]))
	} else {
		SIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.saddr[0]))
		DIP = convDPv6Addr2NetIP(unsafe.Pointer(&ctKey.daddr[0]))
	}
	Dport := tk.Ntohs(uint16(ctKey.dport))
	Sport := tk.Ntohs(uint16(ctKey.sport))
	Proto := ""

	p := uint8(ctKey.l4proto)
	switch {
	case p == 1 || p == 58:
		if p == 1 {
			Proto = "icmp"
		} else {
			Proto = "icmp6"
		}
	case p == 6:
		Proto = "tcp"
	case p == 17:
		Proto = "udp"
	case p == 132:
		Proto = "sctp"
	default:
		Proto = fmt.Sprintf("%d", p)
	}

	ct.CAct = fmt.Sprintf("fp|%s:%d->%s:%d|%s", SIP.String(), Sport, DIP.String(), Dport, Proto)
}

//export goProxyEntCollector
func goProxyEntCollector(e *proxtCT) {

	proxyCt := new(DpCtInfo)
	proxyCt.convDPCtKey2GoObj(&e.ct_in)
	proxyCt.convDPCtProxy2ActString(&e.ct_out)
	proxyCt.Bytes = uint64(e.st_out.bytes)
	proxyCt.Bytes += uint64(e.st_in.bytes)

	proxyCt.Packets = uint64(e.st_out.packets)
	proxyCt.Packets += uint64(e.st_in.packets)
	proxyCt.RuleID = uint32(e.rid)
	proxyCt.CState = "est"

	proxyCtInfo = append(proxyCtInfo, proxyCt)
}

// DpTableGet - routine to work on a ebpf map get request
func (e *DpEbpfH) DpTableGet(w *TableDpWorkQ) (DpRetT, error) {
	var tbl int

	if w.Work != DpMapGet {
		return EbpfErrWqUnk, errors.New("unknown work type")
	}

	switch {
	case w.Name == MapNameCt4:
		tbl = C.LL_DP_CT_MAP
	default:
		return EbpfErrWqUnk, errors.New("unknown work type")
	}

	if tbl == C.LL_DP_CT_MAP {
		ctMap := make(map[string]*DpCtInfo)
		var key *C.struct_dp_ct_key
		nextKey := new(C.struct_dp_ct_key)
		var tact C.struct_dp_ct_tact
		var act *C.struct_dp_ct_dat

		n := 0
		fd := C.llb_map2fd(C.int(tbl))

		for C.bpf_map_get_next_key(C.int(fd), (unsafe.Pointer)(key), (unsafe.Pointer)(nextKey)) == 0 {
			ctKey := (*C.struct_dp_ct_key)(unsafe.Pointer(nextKey))

			if C.bpf_map_lookup_elem(C.int(fd), (unsafe.Pointer)(nextKey), (unsafe.Pointer)(&tact)) != 0 {
				continue
			}

			act = &tact.ctd

			if act.dir == C.CT_DIR_IN || act.dir == C.CT_DIR_OUT {
				var b, p uint64
				goCt4Ent := new(DpCtInfo)
				goCt4Ent.convDPCt2GoObjFixup(ctKey, act, true)
				ret := C.llb_fetch_map_stats_cached(C.int(C.LL_DP_CT_STATS_MAP), C.uint(tact.ca.cidx), C.int(1),
					(unsafe.Pointer(&b)), unsafe.Pointer(&p))
				if ret == 0 {
					goCt4Ent.Bytes += b
					goCt4Ent.Packets += p
				}
				goCt4Ent.RuleID = uint32(act.rid)
				//fmt.Println(goCt4Ent)
				ctMap[goCt4Ent.Key()] = goCt4Ent
			}
			key = nextKey
			n++
		}

		proxyCtInfo = nil
		C.llb_trigger_get_proxy_entries()
		for e, proxyCt := range proxyCtInfo {
			ePCT := ctMap[proxyCt.Key()]
			if ePCT != nil {
				if e > 0 {
					ePCT.CAct += " "
				}
				ePCT.CAct += proxyCt.CAct
				ePCT.Bytes += proxyCt.Bytes
				ePCT.Packets += proxyCt.Packets
			} else {
				ctMap[proxyCt.Key()] = proxyCt
			}
		}
		proxyCtInfo = nil

		return ctMap, nil
	}

	return EbpfErrWqUnk, errors.New("unknown work type")
}

// DpUlClMod - routine to work on a ebpf ul-cl filter change request
func (e *DpEbpfH) DpUlClMod(w *UlClDpWorkQ) int {
	key := new(sess4Key)

	key.daddr = C.uint(tk.IPtonl(w.MDip))
	key.saddr = C.uint(tk.IPtonl(w.MSip))
	key.teid = C.uint(tk.Htonl(w.mTeID))
	key.r = 0

	if w.Work == DpCreate {
		dat := new(sessAct)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_sess_tact)

		if key.teid != 0 || w.Type == DpTunIPIP {
			if w.Type == DpTunIPIP {
				dat.ca.act_type = C.DP_SET_RM_IPIP
			} else {
				dat.ca.act_type = C.DP_SET_RM_GTP
			}

			dat.ca.cidx = C.uint(w.Mark)
			dat.qfi = C.uchar(w.Qfi)
		} else {
			dat.ca.act_type = C.DP_SET_ADD_GTP
			dat.ca.cidx = C.uint(w.Mark)
			dat.qfi = C.uchar(w.Qfi)
			dat.rip = C.uint(tk.IPtonl(w.TDip))
			dat.sip = C.uint(tk.IPtonl(w.TSip))
			dat.teid = C.uint(tk.Htonl(w.TTeID))
		}

		ret := C.llb_add_map_elem(C.LL_DP_SESS4_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))

		if ret != 0 {
			return EbpfErrSess4Add
		}

		return 0
	} else if w.Work == DpRemove {
		C.llb_del_map_elem(C.LL_DP_SESS4_MAP, unsafe.Pointer(key))
		return 0
	}
	return EbpfErrWqUnk
}

// DpUlClAdd - routine to work on a ebpf ul-cl filter add request
func (e *DpEbpfH) DpUlClAdd(w *UlClDpWorkQ) int {
	return e.DpUlClMod(w)
}

// DpUlClDel - routine to work on a ebpf ul-cl filter delete request
func (e *DpEbpfH) DpUlClDel(w *UlClDpWorkQ) int {
	return e.DpUlClMod(w)
}

// DpPolMod - routine to work on a ebpf policer change request
func (e *DpEbpfH) DpPolMod(w *PolDpWorkQ) int {
	key := C.uint(w.Mark)

	if w.Work == DpCreate {
		dat := new(polTact)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_pol_tact)
		dat.ca.act_type = C.DP_SET_DO_POLICER
		// For finding pa, we need to account for padding of 4
		pa := (*polAct)(getPtrOffset(unsafe.Pointer(dat),
			C.sizeof_struct_dp_cmn_act+C.sizeof_struct_bpf_spin_lock+4))

		if w.Srt == false {
			pa.trtcm = 1
		} else {
			pa.trtcm = 0
		}

		if w.Color == false {
			pa.color_aware = 0
		} else {
			pa.color_aware = 1
		}

		pa.toksc_pus = C.ulonglong(w.Cir / (8000000))
		pa.tokse_pus = C.ulonglong(w.Pir / (8000000))
		pa.cbs = C.uint(w.Cbs)
		pa.ebs = C.uint(w.Ebs)
		pa.tok_c = pa.cbs
		pa.tok_e = pa.ebs
		pa.lastc_uts = C.get_os_usecs()
		pa.laste_uts = pa.toksc_pus
		pa.drop_prio = C.LLB_PIPE_COL_YELLOW

		ret := C.llb_add_map_elem(C.LL_DP_POL_MAP,
			unsafe.Pointer(&key),
			unsafe.Pointer(dat))

		if ret != 0 {
			*w.Status = 1
			return EbpfErrPolAdd
		}

		*w.Status = 0

	} else if w.Work == DpRemove {
		// Array map types need to be zeroed out first
		dat := new(polTact)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_pol_tact)
		C.llb_add_map_elem(C.LL_DP_POL_MAP, unsafe.Pointer(&key), unsafe.Pointer(dat))
		// This operation is unnecessary
		C.llb_del_map_elem(C.LL_DP_POL_MAP, unsafe.Pointer(&key))
		return 0
	}
	return 0
}

// DpPolAdd - routine to work on a ebpf policer add request
func (e *DpEbpfH) DpPolAdd(w *PolDpWorkQ) int {
	return e.DpPolMod(w)
}

// DpPolDel - routine to work on a ebpf policer delete request
func (e *DpEbpfH) DpPolDel(w *PolDpWorkQ) int {
	return e.DpPolMod(w)
}

// DpMirrMod - routine to work on a ebpf mirror modify request
func (e *DpEbpfH) DpMirrMod(w *MirrDpWorkQ) int {
	key := C.uint(w.Mark)

	if w.Work == DpCreate {
		dat := new(mirrTact)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_mirr_tact)

		if w.MiBD != 0 {
			dat.ca.act_type = C.DP_SET_ADD_L2VLAN
		} else {
			dat.ca.act_type = C.DP_SET_RM_L2VLAN
		}

		la := (*l2VlanAct)(getPtrOffset(unsafe.Pointer(dat), C.sizeof_struct_dp_cmn_act))

		la.oport = C.ushort(w.MiPortNum)
		la.vlan = C.ushort(w.MiBD)

		ret := C.llb_add_map_elem(C.LL_DP_MIRROR_MAP, unsafe.Pointer(&key), unsafe.Pointer(dat))

		if ret != 0 {
			*w.Status = 1
			return EbpfErrMirrAdd
		}

		*w.Status = 0

	} else if w.Work == DpRemove {
		// Array map types need to be zeroed out first
		dat := new(mirrTact)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_dp_mirr_tact)
		C.llb_add_map_elem(C.LL_DP_MIRROR_MAP, unsafe.Pointer(&key), unsafe.Pointer(dat))
		C.llb_del_map_elem(C.LL_DP_MIRROR_MAP, unsafe.Pointer(&key))
		return 0
	}
	return 0
}

// DpMirrAdd - routine to work on a ebpf mirror add request
func (e *DpEbpfH) DpMirrAdd(w *MirrDpWorkQ) int {
	return e.DpMirrMod(w)
}

// DpMirrDel - routine to work on a ebpf mirror delete request
func (e *DpEbpfH) DpMirrDel(w *MirrDpWorkQ) int {
	return e.DpMirrMod(w)
}

// DpFwRuleMod - routine to work on a ebpf fw mod request
func (e *DpEbpfH) DpFwRuleMod(w *FwDpWorkQ) int {

	fwe := new(fw4Ent)

	C.memset(unsafe.Pointer(fwe), 0, C.sizeof_struct_dp_fwv4_ent)

	if len(w.DstIP.IP) != 0 {
		fwe.k.dest.val = C.uint(tk.Ntohl(tk.IPtonl(w.DstIP.IP)))
		fwe.k.dest.valid = C.uint(tk.Ntohl(tk.IPtonl(net.IP(w.DstIP.Mask))))
	}

	if len(w.SrcIP.IP) != 0 {
		fwe.k.source.val = C.uint(tk.Ntohl(tk.IPtonl(w.SrcIP.IP)))
		fwe.k.source.valid = C.uint(tk.Ntohl(tk.IPtonl(net.IP(w.SrcIP.Mask))))
	}

	if w.L4SrcMin == w.L4SrcMax {
		if w.L4SrcMin != 0 {
			fwe.k.sport.has_range = C.uint(0)
			ptr := (*C.ushort)(unsafe.Pointer(&fwe.k.sport.u[0]))
			*ptr = C.ushort(w.L4SrcMin)
			ptr = (*C.ushort)(unsafe.Pointer(&fwe.k.sport.u[2]))
			*ptr = C.ushort(0xffff)
		}
	} else {
		fwe.k.sport.has_range = C.uint(1)
		ptr := (*C.ushort)(unsafe.Pointer(&fwe.k.sport.u[0]))
		*ptr = C.ushort(w.L4SrcMin)
		ptr = (*C.ushort)(unsafe.Pointer(&fwe.k.sport.u[2]))
		*ptr = C.ushort(w.L4SrcMax)
	}

	if w.L4DstMin == w.L4DstMax {
		if w.L4DstMin != 0 {
			fwe.k.dport.has_range = C.uint(0)
			ptr := (*C.ushort)(unsafe.Pointer(&fwe.k.dport.u[0]))
			*ptr = C.ushort(w.L4DstMin)
			ptr = (*C.ushort)(unsafe.Pointer(&fwe.k.dport.u[2]))
			*ptr = C.ushort(0xffff)
		}
	} else {
		fwe.k.dport.has_range = C.uint(1)
		ptr := (*C.ushort)(unsafe.Pointer(&fwe.k.dport.u[0]))
		*ptr = C.ushort(w.L4DstMin)
		ptr = (*C.ushort)(unsafe.Pointer(&fwe.k.dport.u[2]))
		*ptr = C.ushort(w.L4DstMax)
	}

	if w.Port != 0 {
		fwe.k.inport.val = C.ushort(w.Port)
		fwe.k.inport.valid = C.ushort(0xffff)
	}

	if w.Proto != 0 {
		fwe.k.protocol.val = C.uchar(w.Proto)
		fwe.k.protocol.valid = C.uchar(255)
	}

	if w.ZoneNum != 0 {
		fwe.k.zone.val = C.ushort(w.ZoneNum)
		fwe.k.zone.valid = C.ushort(0xffff)
	}

	fwe.fwa.ca.cidx = C.uint(w.Mark)
	fwe.fwa.ca.oaux = C.ushort(w.Pref) // Overloaded field

	if w.Work == DpCreate {
		if w.FwType == DpFwFwd {
			fwe.fwa.ca.act_type = C.DP_SET_NOP
		} else if w.FwType == DpFwDrop {
			fwe.fwa.ca.act_type = C.DP_SET_DROP
		} else if w.FwType == DpFwRdr {
			fwe.fwa.ca.act_type = C.DP_SET_RDR_PORT
			pRdr := (*portAct)(getPtrOffset(unsafe.Pointer(&fwe.fwa),
				C.sizeof_struct_dp_cmn_act))
			pRdr.oport = C.ushort(w.FwVal1)
		} else if w.FwType == DpFwTrap {
			fwe.fwa.ca.act_type = C.DP_SET_TOCP
		}
		fwe.fwa.ca.mark = C.ushort(w.FwVal2)
		if w.FwRecord {
			fwe.fwa.ca.record = C.ushort(1)
		}
		ret := C.llb_add_map_elem(C.LL_DP_FW4_MAP, unsafe.Pointer(fwe), unsafe.Pointer(nil))
		if ret != 0 {
			tk.LogIt(tk.LogError, "ebpf fw error\n")
			return EbpfErrFwAdd
		}
	} else if w.Work == DpRemove {
		C.llb_del_map_elem(C.LL_DP_FW4_MAP, unsafe.Pointer(fwe))
	}

	return 0
}

// DpFwRuleAdd - routine to work on a ebpf fw add request
func (e *DpEbpfH) DpFwRuleAdd(w *FwDpWorkQ) int {
	ec := e.DpFwRuleMod(w)
	if ec != 0 {
		*w.Status = DpCreateErr
	} else {
		*w.Status = 0
	}
	return ec
}

// DpFwRuleDel - routine to work on a ebpf fw delete request
func (e *DpEbpfH) DpFwRuleDel(w *FwDpWorkQ) int {
	return e.DpFwRuleMod(w)
}

// DpSockVIPMod - routine to work on a ebpf local VIP-port rewrite modification
func (e *DpEbpfH) DpSockVIPMod(w *SockVIPDpWorkQ) int {
	key := new(vipKey)

	if tk.IsNetIPv6(w.VIP.String()) {
		return EbpfErrSockVIPMod
	}

	C.memset(unsafe.Pointer(key), 0, C.sizeof_struct_sock_rwr_key)
	key.vip[0] = C.uint(tk.IPtonl(w.VIP))
	key.port = C.ushort(tk.Htons(w.Port))

	if w.Work == DpCreate {
		dat := new(vipAct)
		C.memset(unsafe.Pointer(dat), 0, C.sizeof_struct_sock_rwr_action)
		dat.rw_port = C.ushort(tk.Htons(w.RwPort))

		ret := C.llb_add_map_elem(C.LL_DP_SOCK_RWR_MAP,
			unsafe.Pointer(key),
			unsafe.Pointer(dat))

		if ret != 0 {
			*w.Status = 1
			tk.LogIt(tk.LogError, "sock-vip rwr add failed\n")
			return EbpfErrSockVIPAdd
		}

		tk.LogIt(tk.LogDebug, "sock-vip (%s:%v) rwr (%v) added\n",
			w.VIP.String(), w.Port, w.RwPort)

		*w.Status = 0

	} else if w.Work == DpRemove {
		C.llb_del_map_elem(C.LL_DP_SOCK_RWR_MAP, unsafe.Pointer(key))
		return 0
	}
	return 0
}

// DpSockVIPAdd - routine to work on a ebpf local VIP-port rewrite addition
func (e *DpEbpfH) DpSockVIPAdd(w *SockVIPDpWorkQ) int {
	ec := e.DpSockVIPMod(w)
	if ec != 0 {
		*w.Status = DpCreateErr
	} else {
		*w.Status = 0
	}
	return ec
}

// DpSockVIPDel - routine to work on a ebpf local VIP-port rewrite delete
func (e *DpEbpfH) DpSockVIPDel(w *SockVIPDpWorkQ) int {
	ec := e.DpSockVIPMod(w)
	if ec != 0 {
		*w.Status = DpRemoveErr
	} else {
		*w.Status = 0
	}
	return ec
}

//export goMapNotiHandler
func goMapNotiHandler(m *mapNoti) {

	ctKey := (*C.struct_dp_ct_key)(unsafe.Pointer(m.key))

	// Only connection oriented protocols
	if m.addop == 0 || mh.dpEbpf == nil || (ctKey.l4proto != 6 && ctKey.l4proto != 132) {
		return
	}

	goCtEnt := new(DpCtInfo)
	goCtEnt.PKey = C.GoBytes(unsafe.Pointer(m.key), m.key_len)
	if m.addop != 0 {
		// No value in delete op
		goCtEnt.PVal = C.GoBytes(unsafe.Pointer(m.val), m.val_len)
	}

	dpCTMapNotifierWorker(goCtEnt)
	//mh.dpEbpf.ToMapCh <- goCtEnt
}

func dpCTMapNotifierWorker(cti *DpCtInfo) {
	var tact *C.struct_dp_ct_tact
	var act *C.struct_dp_ct_dat
	var addOp bool
	var opStr string

	ctKey := (*C.struct_dp_ct_key)(unsafe.Pointer(&cti.PKey[0]))
	if len(cti.PVal) != 0 {
		tact = (*C.struct_dp_ct_tact)(unsafe.Pointer(&cti.PVal[0]))
		act = &tact.ctd
		if (uint)(act.nid) != mh.dpEbpf.nID {
			return
		}
		addOp = true
		opStr = "Add"
	} else {
		addOp = false
		tact = nil
		act = nil
		opStr = "Delete"
	}

	cti.convDPCt2GoObj(ctKey, act)
	cti.LTs = time.Now()

	if addOp {
		// Need to completely initialize the cti
		mh.mtx.Lock()
		r := mh.zr.Rules.GetNatLbRuleByID(uint32(act.rid))
		mh.mtx.Unlock()
		if r == nil {
			return
		}
		cti.ServiceIP = r.tuples.l3Dst.addr.IP
		cti.L4ServPort = r.tuples.l4Dst.val
		cti.BlockNum = r.tuples.pref
		cti.CI = r.ci
		if r.tuples.l4Prot.val == 6 {
			cti.ServProto = "tcp"
		} else if r.tuples.l4Prot.val == 132 {
			cti.ServProto = "sctp"
		} else {
			return
		}
	}

	mh.dpEbpf.mtx.Lock()
	defer mh.dpEbpf.mtx.Unlock()

	if addOp == false {
		cti = mh.dpEbpf.ctMap[cti.Key()]
		if cti == nil || cti.Deleted > 0 {
			return
		}
		cti.Deleted = 1
		cti.XSync = true
		cti.NTs = time.Now()
		// Immediately notify for delete
		//ret := mh.dp.DpXsyncRPC(DpSyncDelete, cti)
		//if ret == 0 {
		//	delete(mh.dpEbpf.ctMap, cti.Key())
		// This is a strange fix - Sometimes loxilb runs as multiple docker
		// instances in the same host. So, the map tracing infra can send notifications
		// generated by other instances here. Depending on the timing, it is possible
		// that the original deleter gets notified after it is handled in the remote
		// instance. This is to handle such special cases.
		//	C.llb_del_map_elem(C.LL_DP_CT_MAP, unsafe.Pointer(&cti.PKey[0]))
		//}
	} else {
		cte := mh.dpEbpf.ctMap[cti.Key()]
		if cte != nil {
			if cte.CState == cti.CState && cte.CAct == cti.CAct {
				return
			}
			delete(mh.dpEbpf.ctMap, cti.Key())
		}
		mh.dpEbpf.ctMap[cti.Key()] = cti
		if cti.CState == "est" {
			cti.XSync = true
			cti.NTs = time.Now()
		}
	}

	tk.LogIt(tk.LogDebug, "[CT] %s - %s\n", opStr, cti.String())
}

func dpCTMapBcast() {
	mh.dpEbpf.mtx.Lock()

	for _, cti := range mh.dpEbpf.ctMap {
		if cti.Deleted <= 0 && cti.CState == "est" {
			cti.XSync = true
		}
	}

	mh.dpEbpf.mtx.Unlock()

	cti := new(DpCtInfo)
	cti.Proto = "xsync"
	cti.Sport = uint16(mh.self)
	mh.dp.DpXsyncRPC(DpSyncBcast, cti)
	tk.LogIt(tk.LogInfo, "[CT]  CTBcast Complete \n")
}

func dpCTMapChkUpdates() {
	mh.dpEbpf.mtx.Lock()
	defer mh.dpEbpf.mtx.Unlock()
	var tact C.struct_dp_ct_tact
	var act *C.struct_dp_ct_dat
	var blkCti []DpCtInfo
	var blkDelCti []DpCtInfo

	tc := time.Now()
	fd := C.llb_map2fd(C.LL_DP_CT_MAP)

	if len(mh.dpEbpf.ctMap) > 0 {
		tk.LogIt(tk.LogInfo, "[CT] Map size %d\n", len(mh.dpEbpf.ctMap))
	}

	for _, cti := range mh.dpEbpf.ctMap {
		// tk.LogIt(tk.LogDebug, "[CT] check %s:%s:%v\n", cti.Key(), cti.CState, cti.XSync)
		if cti.CState != "est" {
			if C.bpf_map_lookup_elem(C.int(fd), unsafe.Pointer(&cti.PKey[0]), unsafe.Pointer(&tact)) != 0 {
				delete(mh.dpEbpf.ctMap, cti.Key())
				continue
			}

			act = &tact.ctd
			goCtEnt := new(DpCtInfo)
			goCtEnt.convDPCt2GoObj((*C.struct_dp_ct_key)(unsafe.Pointer(&cti.PKey[0])), act)
			goCtEnt.LTs = tc

			if goCtEnt.CState != cti.CState ||
				goCtEnt.CAct != cti.CState {
				goCtEnt.PKey = cti.PKey
				// Key will remain the same but value might change
				goCtEnt.PVal = C.GoBytes(unsafe.Pointer(&tact), C.sizeof_struct_dp_ct_tact)

				// Copy rule associations
				goCtEnt.ServiceIP = cti.ServiceIP
				goCtEnt.L4ServPort = cti.L4ServPort
				goCtEnt.BlockNum = cti.BlockNum
				goCtEnt.ServProto = cti.ServProto
				goCtEnt.CI = cti.CI
				delete(mh.dpEbpf.ctMap, cti.Key())
				mh.dpEbpf.ctMap[goCtEnt.Key()] = goCtEnt
				ctStr := goCtEnt.String()
				tk.LogIt(tk.LogDebug, "[CT] %s - %s\n", "update", ctStr)
				if goCtEnt.CState == "est" {
					goCtEnt.XSync = true
					goCtEnt.NTs = tc
				}
				continue
			}
		} else {
			var b uint64
			var p uint64

			// Make sure CT shadow entries are in sync
			if time.Duration(tc.Sub(cti.LTs).Seconds()) >= time.Duration(5*60) {
				tk.LogIt(tk.LogInfo, "[CT] out-of-sync %s:%s:%v\n", cti.Key(), cti.CState, cti.XSync)
				if C.bpf_map_lookup_elem(C.int(fd), unsafe.Pointer(&cti.PKey[0]), unsafe.Pointer(&tact)) != 0 {
					tk.LogIt(tk.LogInfo, "[CT] out-of-sync not found %s:%s:%v\n", cti.Key(), cti.CState, cti.XSync)
					delete(mh.dpEbpf.ctMap, cti.Key())
					continue
				}
				cti.PVal = C.GoBytes(unsafe.Pointer(&tact), C.sizeof_struct_dp_ct_tact)
				cti.LTs = tc
			}

			if len(cti.PVal) > 0 && cti.XSync == false {
				if time.Duration(tc.Sub(cti.NTs).Seconds()) < time.Duration(60) {
					continue
				}
				if C.bpf_map_lookup_elem(C.int(fd), unsafe.Pointer(&cti.PKey[0]), unsafe.Pointer(&tact)) != 0 {
					tk.LogIt(tk.LogDebug, "[CT] ent not found %s\n", cti.Key())
					//delete(mh.dpEbpf.ctMap, cti.Key())
					cti.Deleted++
					cti.XSync = true
				} else {
					ptact := (*C.struct_dp_ct_tact)(unsafe.Pointer(&cti.PVal[0]))
					ret := C.llb_fetch_map_stats_cached(C.int(C.LL_DP_CT_STATS_MAP), C.uint(ptact.ca.cidx), C.int(0),
						(unsafe.Pointer(&b)), unsafe.Pointer(&p))
					if ret == 0 {
						if cti.Packets != p+uint64(tact.ctd.pb.packets) {
							cti.Bytes = b + uint64(tact.ctd.pb.bytes)
							cti.Packets = p + uint64(tact.ctd.pb.packets)
							cti.XSync = true
							cti.NTs = tc
							cti.LTs = tc
						}
					}
				}
			}
		}
		if cti.XSync == true &&
			time.Duration(tc.Sub(cti.NTs).Seconds()) >= time.Duration(10) {
			tk.LogIt(tk.LogDebug, "[CT] Sync - %s\n", cti.String())

			ret := 0
			if cti.Deleted > 0 {
				//ret = mh.dp.DpXsyncRPC(DpSyncDelete, cti)
				blkDelCti = append(blkDelCti, *cti)
				cti.Deleted++
			} else {
				blkCti = append(blkCti, *cti)
				//ret = mh.dp.DpXsyncRPC(DpSyncAdd, cti)
			}
			if ret == 0 || cti.Deleted > ctiDeleteSyncRetries {
				cti.XSync = false

				if cti.Deleted > 0 {
					delete(mh.dpEbpf.ctMap, cti.Key())
					// This is a strange fix - See comment above. Do we still need it ?
					// C.llb_del_map_elem(C.LL_DP_CT_MAP, unsafe.Pointer(&cti.PKey[0]))
				}
			}
		}

		if len(blkCti) >= blkCtiMaxLen {
			tk.LogIt(tk.LogDebug, "[CT] Block Add Sync - \n")
			tc1 := time.Now()
			mh.dp.DpXsyncRPC(DpSyncAdd, blkCti)
			tc2 := time.Now()
			tk.LogIt(tk.LogInfo, "[CT] Block Add Sync %d took %v- \n", len(blkCti), time.Duration(tc2.Sub(tc1)))
			blkCti = nil
		}

		if len(blkDelCti) >= blkCtiMaxLen {
			tk.LogIt(tk.LogDebug, "[CT] Block Del Sync - \n")
			mh.dp.DpXsyncRPC(DpSyncDelete, blkDelCti)
			blkDelCti = nil
		}
	}

	if len(blkCti) > 0 {
		tc1 := time.Now()
		tk.LogIt(tk.LogDebug, "[CT] Block Add Sync - \n")
		mh.dp.DpXsyncRPC(DpSyncAdd, blkCti)
		tc2 := time.Now()
		tk.LogIt(tk.LogInfo, "[CT] Block Add Sync %d took %v- \n", len(blkCti), time.Duration(tc2.Sub(tc1)))
	}

	if len(blkDelCti) > 0 {
		tk.LogIt(tk.LogDebug, "[CT] Block Del Sync - \n")
		mh.dp.DpXsyncRPC(DpSyncDelete, blkDelCti)
	}
}

// dpMapNotifierWorker - Work on any map notifications
func dpMapNotifierWorker(f chan int, ch chan interface{}) {
	// Stack trace logger
	defer func() {
		if e := recover(); e != nil {
			tk.LogIt(tk.LogCritical, "%s: %s", e, debug.Stack())
		}
		if mh.dp != nil {
			mh.dp.DpHooks.DpEbpfUnInit()
		}
		os.Exit(1)
	}()

	for {
		select {
		case m := <-ch:
			switch mq := m.(type) {
			case *DpCtInfo:
				dpCTMapNotifierWorker(mq)
			}
		case <-f:
			return
		}
	}
}

// DpCtAdd - routine to work on a ebpf ct add request
func (e *DpEbpfH) DpCtAdd(w *DpCtInfo) int {
	var serv cmn.LbServiceArg

	serv.ServIP = w.ServiceIP.String()
	serv.Proto = w.ServProto
	serv.ServPort = w.L4ServPort
	serv.BlockNum = w.BlockNum

	mh.mtx.Lock()
	r := mh.zr.Rules.GetNatLbRuleByServArgs(serv)
	mh.mtx.Unlock()

	if r == nil || len(w.PVal) == 0 || len(w.PKey) == 0 || w.CState != "est" {
		tk.LogIt(tk.LogDebug, "Invalid CT op/No LB - %v\n", serv)
		return EbpfErrCtAdd
	}

	// Fix few things
	ptact := (*C.struct_dp_ct_tact)(unsafe.Pointer(&w.PVal[0]))
	ptact.ctd.rid = C.ushort(r.ruleNum) // Race-condition here
	ptact.ctd.nid = C.uint(mh.dpEbpf.nID)
	ptact.lts = C.get_os_nsecs()

	mh.dpEbpf.mtx.Lock()
	defer mh.dpEbpf.mtx.Unlock()

	mapKey := w.Key()
	cti := new(DpCtInfo)
	*cti = *w

	cte := mh.dpEbpf.ctMap[mapKey]
	if cte != nil {
		if cte.CState != cti.CState ||
			cte.CAct != cti.CAct {
			delete(mh.dpEbpf.ctMap, mapKey)
			mh.dpEbpf.ctMap[mapKey] = cti
			cte = cti
		}
	} else {
		mh.dpEbpf.ctMap[mapKey] = cti
		cte = cti
	}

	cte.XSync = false
	cte.NTs = time.Now()
	//cte.LTs = cti.NTs
	cte.LTs = time.Now()

	ret := C.llb_add_map_elem(C.LL_DP_CT_MAP, unsafe.Pointer(&cti.PKey[0]), unsafe.Pointer(&cti.PVal[0]))
	if ret != 0 {
		delete(mh.dpEbpf.ctMap, mapKey)
		tk.LogIt(tk.LogError, "ctInfo (%s) rpc add error\n", cti.String())
		return EbpfErrCtAdd
	}

	return 0
}

// DpCtDel - routine to work on a ebpf ct delete request
func (e *DpEbpfH) DpCtDel(w *DpCtInfo) int {
	mh.dpEbpf.mtx.Lock()
	defer mh.dpEbpf.mtx.Unlock()

	if len(w.PKey) == 0 {
		tk.LogIt(tk.LogDebug, "Invalid CT op - %v", w)
		return EbpfErrCtDel
	}

	mapKey := w.Key()
	cti := mh.dpEbpf.ctMap[mapKey]
	if cti == nil {
		tk.LogIt(tk.LogDebug, "ctInfo-key (%v) not present\n", mapKey)
		return 0
	}

	delete(mh.dpEbpf.ctMap, mapKey)
	C.llb_del_map_elem(C.LL_DP_CT_MAP, unsafe.Pointer(&w.PKey[0]))

	return 0
}

// DpCtGetAsync - routine to work on a ebpf ct get async request
func (e *DpEbpfH) DpCtGetAsync() {
	e.ctBcast <- true
}

// DpTakeLock - routine to take underlying DP lock
func (e *DpEbpfH) DpGetLock() {
	C.llb_xh_lock()
}

// DpRelLock - routine to release underlying DP lock
func (e *DpEbpfH) DpRelLock() {
	C.llb_xh_unlock()
}

// DpTableGC - Work on table garbage collection
func (e *DpEbpfH) DpTableGC() {
	e.trigGC <- true
}

//export goLinuxArpResolver
func goLinuxArpResolver(dIP C.uint) {
	goDest := uint32(dIP)
	utils.ArpResolver(goDest)
}
