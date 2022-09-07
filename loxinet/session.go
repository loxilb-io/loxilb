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

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
)

const (
	SessErrBase = iota - 90000
	SessModErr
	SessNoExistErr
	SessExistsErr
	SessUlClExistErr
	SessUlClNumErr
	SessUlClNoExistErr
)

const (
	MaximumUlCls = 20000
)

type UserKey struct {
	UserID string
}

type UserTun struct {
	TeID uint32
	Addr net.IP
}

type UlClStats struct {
	UlPackets uint64
	UlBytes   uint64
	DlPackets uint64
	DlBytes   uint64
}

type UlClInf struct {
	Addr   net.IP
	Qfi    uint8
	NumUl  int
	NumDl  int
	Status DpStatusT
	Stats  UlClStats
	uSess  *UserSess
}

type UserSess struct {
	Key   UserKey
	Addr  net.IP
	Zone  int
	AnTun cmn.SessTun
	CnTun cmn.SessTun
	UlCl  map[string]*UlClInf
}

type SessH struct {
	UserMap map[UserKey]*UserSess
	Zone    *Zone
	HwMark  *tk.Counter
}

func SessInit(zone *Zone) *SessH {
	var nUh = new(SessH)
	nUh.UserMap = make(map[UserKey]*UserSess)
	nUh.Zone = zone
	nUh.HwMark = tk.NewCounter(1, MaximumUlCls)
	return nUh
}

func (s *SessH) SessGet() ([]cmn.SessionMod, error) {
	var Sessions []cmn.SessionMod
	for k, v := range s.UserMap {
		Sessions = append(Sessions, cmn.SessionMod{
			Ident: k.UserID,
			IP:    v.Addr,
			AnTun: v.AnTun,
			CnTun: v.CnTun,
		})
	}
	return Sessions, nil
}

func (s *SessH) SessUlclGet() ([]cmn.SessionUlClMod, error) {
	var Ulcls []cmn.SessionUlClMod
	for sk, sv := range s.UserMap {
		for _, v := range sv.UlCl {
			Ulcls = append(Ulcls, cmn.SessionUlClMod{
				Ident: sk.UserID,
				Args: cmn.UlClArg{
					Qfi:  v.Qfi,
					Addr: v.Addr,
				},
			})

		}

	}
	return Ulcls, nil
}

func (s *SessH) SessAdd(user string, IP net.IP, anTun cmn.SessTun, cnTun cmn.SessTun) (int, error) {

	key := UserKey{user}
	us, found := s.UserMap[key]

	if found == true {

		if us.AnTun.Equal(&anTun) == false || us.CnTun.Equal(&cnTun) {
			ret, _ := s.SessDelete(user)
			if ret != 0 {
				tk.LogIt(tk.LOG_ERROR, "session add - %s:%s mod error\n", user, IP.String())
				return SessModErr, errors.New("sess-mod error")
			}
		} else {
			tk.LogIt(tk.LOG_ERROR, "session add - %s:%s  already exists\n", user, IP.String())
			return SessExistsErr, errors.New("sess-exists error")
		}
	}

	us = new(UserSess)
	us.Key.UserID = user
	us.Addr = IP
	us.AnTun = anTun
	us.CnTun = cnTun
	us.Zone = s.Zone.ZoneNum

	us.UlCl = make(map[string]*UlClInf)

	s.UserMap[us.Key] = us

	tk.LogIt(tk.LOG_DEBUG, "session added - %s:%s\n", user, IP.String())

	return 0, nil
}

func (s *SessH) SessDelete(user string) (int, error) {

	key := UserKey{user}
	us, found := s.UserMap[key]

	if found == false {
		tk.LogIt(tk.LOG_ERROR, "session delete - %s no-user\n", user)
		return SessNoExistErr, errors.New("no-user error")
	}

	// First remove all ULCL classifiers if any
	for _, ulcl := range us.UlCl {
		s.UlClDeleteCls(user, cmn.UlClArg{Addr: ulcl.Addr, Qfi: ulcl.Qfi})
	}

	delete(s.UserMap, key)

	tk.LogIt(tk.LOG_DEBUG, "session deleted - %s\n", user)

	return 0, nil
}

func (s *SessH) UlClAddCls(user string, cls cmn.UlClArg) (int, error) {

	key := UserKey{user}
	us, found := s.UserMap[key]

	if found == false {
		return SessNoExistErr, errors.New("no-user error")
	}

	ulcl, _ := us.UlCl[cls.Addr.String()]

	if ulcl != nil {
		return SessUlClExistErr, errors.New("ulcl-exists error")
	}

	ulcl = new(UlClInf)
	ulcl.NumUl, _ = s.HwMark.GetCounter()
	if ulcl.NumUl < 0 {
		return SessUlClNumErr, errors.New("ulcl-ulhwm error")
	}
	ulcl.NumDl, _ = s.HwMark.GetCounter()
	if ulcl.NumDl < 0 {
		s.HwMark.PutCounter(ulcl.NumUl)
		ulcl.NumUl = -1
		return SessUlClNumErr, errors.New("ulcl-dlhwm error")
	}

	ulcl.Qfi = cls.Qfi
	ulcl.Addr = cls.Addr
	ulcl.uSess = us

	defer ulcl.DP(DpCreate)

	us.UlCl[cls.Addr.String()] = ulcl

	tk.LogIt(tk.LOG_DEBUG, "ulcl filter added - %s:%s\n", user, cls.Addr.String())

	return 0, nil
}

func (s *SessH) UlClDeleteCls(user string, cls cmn.UlClArg) (int, error) {

	key := UserKey{user}
	us, found := s.UserMap[key]

	if found == false {
		return SessNoExistErr, errors.New("no-user error")
	}

	ulcl, _ := us.UlCl[cls.Addr.String()]

	if ulcl == nil {
		return SessUlClNoExistErr, errors.New("no-ulcl error")
	}

	tk.LogIt(tk.LOG_DEBUG, "ulcl filter deleted - %s:%s\n", user, cls.Addr.String())

	ulcl.DP(DpRemove)

	s.HwMark.PutCounter(ulcl.NumUl)
	delete(us.UlCl, cls.Addr.String())

	return 0, nil
}

func Us2String(us *UserSess) string {
	var tStr string

	tStr += fmt.Sprintf("%s:%s AN(%s:0x%x) CN(%s:0x%x) ULCLs ##",
		us.Key.UserID, us.Addr.String(),
		us.AnTun.Addr.String(), us.AnTun.TeID,
		us.CnTun.Addr.String(), us.CnTun.TeID)
	for _, ulcl := range us.UlCl {
		tStr += fmt.Sprintf("\n\t%s,qfi-%d,n-%d", ulcl.Addr.String(), ulcl.Qfi, ulcl.NumUl)
	}

	return tStr
}

func (s *SessH) USess2String(it IterIntf) error {
	for _, us := range s.UserMap {
		uBuf := Us2String(us)
		it.NodeWalker(uBuf)
	}
	return nil
}

func (s *SessH) SessionsSync() {
	for _, us := range s.UserMap {
		for _, ulcl := range us.UlCl {
			ulcl.DP(DpStatsGet)
			if ulcl.Stats.DlPackets != 0 || ulcl.Stats.UlPackets != 0 {
				fmt.Printf("%s,qfi-%d,n-%d Dl %v:%v Ul %v:%v\n",
					ulcl.Addr.String(), ulcl.Qfi, ulcl.NumUl,
					ulcl.Stats.DlPackets, ulcl.Stats.DlBytes,
					ulcl.Stats.UlPackets, ulcl.Stats.UlBytes)
			}
		}
	}
	return
}

func (s *SessH) SessionTicker() {
	s.SessionsSync()
}

// Sync state of session and ulcl filter entities to data-path
func (ulcl *UlClInf) DP(work DpWorkT) int {

	if ulcl.uSess == nil {
		return -1
	}

	if work == DpStatsGet {
		uStat := new(StatDpWorkQ)
		uStat.Work = work
		uStat.HwMark = uint32(ulcl.NumUl)
		uStat.Name = MapNameULCL
		uStat.Bytes = &ulcl.Stats.UlBytes
		uStat.Packets = &ulcl.Stats.UlBytes

		mh.dp.ToDpCh <- uStat

		dStat := new(StatDpWorkQ)
		dStat.Work = work
		dStat.HwMark = uint32(ulcl.NumDl)
		dStat.Name = MapNameULCL
		dStat.Bytes = &ulcl.Stats.DlBytes
		dStat.Packets = &ulcl.Stats.DlBytes

		mh.dp.ToDpCh <- dStat

		return 0
	}

	// For UL dir
	ucn := new(UlClDpWorkQ)
	ucn.Work = work
	ucn.mDip = ulcl.Addr
	ucn.mSip = ulcl.uSess.Addr
	ucn.mTeID = ulcl.uSess.AnTun.TeID
	ucn.Zone = ulcl.uSess.Zone
	ucn.HwMark = ulcl.NumUl
	ucn.Qfi = ulcl.Qfi
	ucn.tTeID = 0

	mh.dp.ToDpCh <- ucn

	// For DL dir
	ucn = new(UlClDpWorkQ)
	ucn.Work = work
	ucn.mSip = ulcl.Addr
	ucn.mDip = ulcl.uSess.Addr
	ucn.mTeID = 0
	ucn.Zone = ulcl.uSess.Zone
	ucn.HwMark = ulcl.NumDl
	ucn.Qfi = ulcl.Qfi
	ucn.tDip = ulcl.uSess.AnTun.Addr
	ucn.tSip = ulcl.uSess.CnTun.Addr
	ucn.tTeID = ulcl.uSess.AnTun.TeID

	mh.dp.ToDpCh <- ucn

	return 0
}
