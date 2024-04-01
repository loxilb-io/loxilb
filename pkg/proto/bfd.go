/*
 * Copyright (c) 2024 NetLOX Inc
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

package bfd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

type SessionState uint8

const (
	BFDAdminDown SessionState = iota
	BFDDown
	BFDInit
	BFDUp
)

var BFDStateMap = map[uint8]string{
	uint8(BFDAdminDown): "BFDAdminDown",
	uint8(BFDDown):      "BFDDown",
	uint8(BFDInit):      "BFDInit",
	uint8(BFDUp):        "BFDUp",
}

const (
	BFDMinSysTXIntervalUs = 100000
	BFDDflSysTXIntervalUs = 200000
	BFDMinSysRXIntervalUs = 200000
	BFDIncrDiscVal        = 0x00000001
)

type ConfigArgs struct {
	RemoteIP string
	SourceIP string
	Port     uint16
	Interval uint32
	Multi    uint8
	Instance string
}

type WireRaw struct {
	Version       uint8
	Length        uint8
	State         SessionState
	Multi         uint8
	Disc          uint32
	RDisc         uint32
	DesMinTxInt   uint32
	ReqMinRxInt   uint32
	ReqMinEchoInt uint32
}

type Notifer interface {
	BFDSessionNotify(instance string, remote string, state string)
}

type bfdSession struct {
	RemoteName     string
	Instance       string
	Cxn            net.Conn
	State          SessionState
	CiState        string
	MyMulti        uint8
	RemMulti       uint8
	MyIP           net.IP
	MyDisc         uint32
	RemDisc        uint32
	DesMinTxInt    uint32
	RemDesMinTxInt uint32
	ReqMinRxInt    uint32
	TimeOut        uint32
	ReqMinEchoInt  uint32
	LastRxTS       time.Time
	TxTicker       *time.Ticker
	RxTicker       *time.Ticker
	Fin            chan bool
	Mutex          sync.RWMutex
	Notify         Notifer
	PktDat         [24]byte
}

type Struct struct {
	BFDSessMap map[string]*bfdSession
	BFDMtx     sync.RWMutex
}

func StructNew(port uint16) *Struct {
	bfdStruct := new(Struct)

	bfdStruct.BFDSessMap = make(map[string]*bfdSession)
	go bfdStruct.bfdStartListener(port)
	return bfdStruct
}

func (bs *Struct) BFDAddRemote(args ConfigArgs, cbs Notifer) error {
	bs.BFDMtx.Lock()
	defer bs.BFDMtx.Unlock()

	sess := bs.BFDSessMap[args.RemoteIP]
	if sess != nil {
		var update bool
		if sess.Instance == args.Instance {
			if args.Interval != 0 && sess.DesMinTxInt != args.Interval {
				sess.DesMinTxInt = args.Interval
				sess.ReqMinRxInt = args.Interval
				sess.ReqMinEchoInt = args.Interval
				update = true
			}
			if args.Multi != 0 && sess.MyMulti != args.Multi {
				sess.MyMulti = args.Multi
				update = true
			}
			if update {
				sess.Fin <- true
				sess.TxTicker.Stop()
				sess.RxTicker.Stop()
				sess.State = BFDDown

				sess.TxTicker = time.NewTicker(time.Duration(sess.DesMinTxInt) * time.Microsecond)
				sess.RxTicker = time.NewTicker(time.Duration(BFDMinSysRXIntervalUs) * time.Microsecond)
				go sess.bfdSessionTicker()

				return nil
			}
		}
		return errors.New("bfd existing session")
	}

	if args.Interval < BFDMinSysTXIntervalUs || args.Multi == 0 {
		return errors.New("bfd malformed args")
	}

	sess = new(bfdSession)
	sess.Instance = args.Instance
	sess.Notify = cbs

	err := sess.initialize(args.RemoteIP, args.SourceIP, args.Port, args.Interval, args.Multi)
	if err != nil {
		return err
		//return errors.New("bfd failed to init session")
	}

	bs.BFDSessMap[args.RemoteIP] = sess

	return nil
}

func (bs *Struct) BFDDeleteRemote(args ConfigArgs) error {
	bs.BFDMtx.Lock()
	defer bs.BFDMtx.Unlock()

	sess := bs.BFDSessMap[args.RemoteIP]
	if sess == nil {
		return errors.New("no bfd session")
	}

	sess.destruct()
	delete(bs.BFDSessMap, args.RemoteIP)

	return nil
}

func (bs *Struct) BFDGet() ([]cmn.BFDMod, error) {
	var res []cmn.BFDMod

	bs.BFDMtx.Lock()
	defer bs.BFDMtx.Unlock()

	for _, s := range bs.BFDSessMap {
		var temp cmn.BFDMod
		pair := strings.Split(s.RemoteName, ":")
		temp.Instance = s.Instance
		temp.RemoteIP = net.ParseIP(pair[0])
		temp.SourceIP = s.MyIP
		port, _ := strconv.Atoi(pair[1])
		temp.Port = uint16(port)
		temp.Interval = uint64(s.DesMinTxInt)
		temp.RetryCount = s.MyMulti
		temp.State = BFDStateMap[uint8(s.State)]
		res = append(res, temp)
	}

	return res, nil
}

func decodeCtrlPacket(buf []byte, size int) *WireRaw {

	if size < 24 {
		return nil
	}

	var raw WireRaw

	raw.Version = buf[0] >> 5 & 0x7
	raw.State = SessionState(buf[1] >> 6 & 0x3)
	raw.Multi = buf[2]
	raw.Length = buf[3]

	raw.Disc = binary.BigEndian.Uint32(buf[4:])
	raw.RDisc = binary.BigEndian.Uint32(buf[8:])
	raw.DesMinTxInt = binary.BigEndian.Uint32(buf[12:])
	raw.ReqMinRxInt = binary.BigEndian.Uint32(buf[16:])
	raw.ReqMinEchoInt = binary.BigEndian.Uint32(buf[20:])

	return &raw
}

func (bs *Struct) processBFD(conn *net.UDPConn) {
	var buf [1024]byte

	n, addr, err := conn.ReadFromUDP(buf[:])
	if err != nil {
		return
	}

	raw := decodeCtrlPacket(buf[:], n)

	remIP := addr.IP
	if remIP != nil {
		//fmt.Printf("raw %v:%s:%v\n", raw, remIP.String(), raw.State)
		bs.BFDMtx.Lock()
		defer bs.BFDMtx.Unlock()

		sess := bs.BFDSessMap[remIP.String()]
		if sess != nil {
			sess.RunSessionSM(raw)
		} /* else {
			tk.LogIt(tk.LogDebug, "bfd session(%s) not found\n", remIP.String())
		} */
	}
}

func (bs *Struct) bfdStartListener(port uint16) error {
	localName := fmt.Sprintf("%s:%d", "0.0.0.0", port)
	addr, err := net.ResolveUDPAddr("udp4", localName)
	if err != nil {
		return errors.New("failed to resolve to BFD addr")
	}

	lc, err1 := net.ListenUDP("udp4", addr)
	if err1 != nil {
		return errors.New("failed to listen to BFD")
	}

	defer lc.Close()

	for {
		bs.processBFD(lc)
	}

}

func (b *bfdSession) RunSessionSM(raw *WireRaw) {
	inst := b.Instance
	rem := b.RemoteName
	oldState := b.State

	b.Mutex.Lock()

	b.RemMulti = raw.Multi
	b.RemDisc = raw.Disc
	b.RemDesMinTxInt = raw.DesMinTxInt
	if b.RemDesMinTxInt > b.ReqMinRxInt {
		b.TimeOut = uint32(b.RemMulti) * b.RemDesMinTxInt
	} else {
		b.TimeOut = uint32(b.RemMulti) * b.ReqMinRxInt
	}
	b.LastRxTS = time.Now()

	if raw.State == BFDDown {
		if b.State == BFDDown {
			b.State = BFDInit
			tk.LogIt(tk.LogInfo, "%s: BFD State -> INIT\n", b.RemoteName)
		}
	} else if raw.State == BFDInit {
		if b.State != BFDUp {
			b.State = BFDUp
			tk.LogIt(tk.LogInfo, "%s: BFD State -> UP\n", b.RemoteName)
		}
	} else if raw.State == BFDAdminDown {
		if b.State != BFDAdminDown {
			tk.LogIt(tk.LogInfo, "%s: BFD State -> AdminDown\n", b.RemoteName)
		}
		b.State = BFDInit
	} else if raw.State == BFDUp {
		if b.State != BFDUp {
			tk.LogIt(tk.LogInfo, "%s: BFD State -> UP\n", b.RemoteName)
		}
		b.State = BFDUp
		if b.CiState == "MASTER" {
			// Force reelection
			if b.MyDisc <= b.RemDisc {
				oldState = BFDDown
			}
		}
	}
	newState := b.State
	b.Mutex.Unlock()

	b.sendStateNotification(newState, oldState, inst, rem)
}

func (b *bfdSession) checkSessTimeout() {
	inst := b.Instance
	rem := b.RemoteName
	oldState := b.State

	b.Mutex.Lock()
	if b.State == BFDUp {
		if time.Duration(time.Since(b.LastRxTS).Microseconds()) > time.Duration(b.TimeOut) {
			b.State = BFDDown
			b.MyDisc = b.RemDisc + BFDIncrDiscVal
			tk.LogIt(tk.LogInfo, "%s: BFD State -> Down (%v:%v)\n", b.RemoteName, b.MyDisc, b.RemDisc)
		}
	}
	newState := b.State
	b.Mutex.Unlock()

	b.sendStateNotification(newState, oldState, inst, rem)
}

func (b *bfdSession) sendStateNotification(newState, oldState SessionState, inst string, remote string) {
	if newState == oldState || b.RemDisc == 0 {
		return
	}

	if newState == BFDUp {
		ciState := "BACKUP"
		if b.MyDisc > b.RemDisc {
			ciState = "MASTER"
		}
		tk.LogIt(tk.LogInfo, "%s: State change (%v:%v)\n", b.RemoteName, b.MyDisc, b.RemDisc)
		b.CiState = ciState
		b.Notify.BFDSessionNotify(inst, remote, ciState)
	} else if newState == BFDDown && oldState == BFDUp {
		ciState := "MASTER"
		b.CiState = ciState
		b.Notify.BFDSessionNotify(inst, remote, ciState)
	} else if b.RemDisc == b.MyDisc {
		ciState := "NOT_DEFINED"
		b.CiState = ciState
		b.Notify.BFDSessionNotify(inst, remote, ciState)
	}
}

func (b *bfdSession) bfdSessionTicker() {
	for {
		select {
		case <-b.Fin:
			return
		case t := <-b.RxTicker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			b.checkSessTimeout()
		case t := <-b.TxTicker.C:
			tk.LogIt(-1, "Tick at %v\n", t)
			b.encodeCtrlPacket()
			b.sendBFDPacket()
		}
	}
}

// getMyDisc - Get My Discriminator based on remote
func getMyDisc(ip net.IP) net.IP {
	// get list of available addresses
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	var first net.IP

	for _, addr := range addr {
		if ipnet, ok := addr.(*net.IPNet); ok {
			// check if IPv4 or IPv6 is not nil
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				if ipnet.Contains(ip) {
					tk.LogIt(tk.LogDebug, "bfd mydisc : %s\n", ipnet.IP.String())
					return ipnet.IP
				}
				if first == nil {
					first = ipnet.IP
				}
			}
		}
	}

	tk.LogIt(tk.LogDebug, "bfd mydisc : %s\n", first.String())

	return first
}

func (b *bfdSession) initialize(remoteIP string, sourceIP string, port uint16, interval uint32, multi uint8) error {
	var err error
	b.RemoteName = fmt.Sprintf("%s:%d", remoteIP, port)

	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return errors.New("address malformed")
	}

	myIP := net.ParseIP(sourceIP)
	if myIP == nil {
		return errors.New("source address malformed")
	}

	if myIP.IsUnspecified() {
		myIP = getMyDisc(ip)
		if myIP == nil {
			return errors.New("my discriminator not found")
		}
	} else {
		tk.LogIt(tk.LogDebug, "using bfd bind mydisc  : %s\n", myIP.String())
	}
	b.MyIP = myIP
	b.MyDisc = tk.IPtonl(myIP)
	b.RemDisc = 0 //tk.IPtonl(ip)
	b.MyMulti = multi
	b.DesMinTxInt = interval
	b.ReqMinRxInt = interval
	b.ReqMinEchoInt = interval
	b.State = BFDDown

	b.Cxn, err = net.DialTimeout("udp4", b.RemoteName, 1*time.Second)
	if err != nil || b.Cxn == nil {
		return errors.New("failed to dial BFD")
	}

	b.Fin = make(chan bool)
	b.TxTicker = time.NewTicker(time.Duration(b.DesMinTxInt) * time.Microsecond)
	b.RxTicker = time.NewTicker(time.Duration(BFDMinSysRXIntervalUs) * time.Microsecond)

	go b.bfdSessionTicker()
	return nil
}

func (b *bfdSession) destruct() {
	b.State = BFDAdminDown
	b.Fin <- true
	b.TxTicker.Stop()
	b.RxTicker.Stop()
	// Signal ADMIN Down to peer
	b.encodeCtrlPacket()
	b.sendBFDPacket()
}

func (b *bfdSession) encodeCtrlPacket() error {

	b.PktDat[0] = byte(byte(0x1<<5) | byte(0))
	b.PktDat[1] = (uint8(b.State) << 6)
	b.PktDat[2] = b.MyMulti
	b.PktDat[3] = 24

	binary.BigEndian.PutUint32(b.PktDat[4:], uint32(b.MyDisc))
	binary.BigEndian.PutUint32(b.PktDat[8:], uint32(b.RemDisc))
	binary.BigEndian.PutUint32(b.PktDat[12:], uint32(b.DesMinTxInt))
	binary.BigEndian.PutUint32(b.PktDat[16:], uint32(b.ReqMinRxInt))
	binary.BigEndian.PutUint32(b.PktDat[20:], uint32(b.ReqMinEchoInt))

	return nil
}

func (b *bfdSession) sendBFDPacket() error {
	b.Cxn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	_, err := b.Cxn.Write(b.PktDat[:])
	if err != nil {
		tk.LogIt(-1, "Error in sending %s\n", err)
	}
	return err
}
