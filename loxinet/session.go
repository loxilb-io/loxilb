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
    "net"
    "fmt"
    tk "loxilb/loxilib"
    cmn "loxilb/common"
)

const (
    SESS_ERR_BASE = iota - 90000
    SESS_MOD_ERR
    SESS_NOEXIST_ERR
    SESS_EXISTS_ERR
    SESS_ULCLEXIST_ERR
    SESS_ULCLNUM_ERR
    SESS_ULCLNOEXIST_ERR
)

const (
    MAX_ULCLS = 20000
)

type UserKey struct {
    UserID string
}

type UserTun struct {
    TeID  uint32
    Addr  net.IP
}

type UlClStats struct {
    UlPackets uint64
    UlBytes  uint64
    DlPackets uint64
    DlBytes uint64
}

type UlClInf struct {
    Addr    net.IP
    Qfi	    uint8
    NumUl   int
    NumDl   int
    Status  DpStatusT
    Stats   UlClStats
    uSess   *UserSess
}

type UserSess struct {
    Key       UserKey
    Addr      net.IP
    Zone      int
    AnTun	  cmn.SessTun
    CnTun     cmn.SessTun
    UlCl	  map[string]*UlClInf
}

type SessH struct {
    UserMap  map[UserKey]*UserSess
    Zone   *Zone
    HwMark *tk.Counter
}

func SessInit(zone *Zone) *SessH {
    var nUh = new(SessH)
    nUh.UserMap = make(map[UserKey]*UserSess)
    nUh.Zone = zone
    nUh.HwMark = tk.NewCounter(1, MAX_ULCLS)
    return nUh
}

func (s *SessH) SessAdd(user string, IP net.IP, anTun cmn.SessTun, cnTun cmn.SessTun) (int, error) {

    key := UserKey{user}
    us, found := s.UserMap[key]

    if found == true {

        if us.AnTun.Equal(&anTun) == false ||  us.CnTun.Equal(&cnTun) {
            ret, _ := s.SessDelete(user)
            if ret != 0 {
                return SESS_MOD_ERR, errors.New("sess mod error")
            }  
        } else {
            return SESS_EXISTS_ERR, errors.New("sess exists")
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

    return 0, nil
}

func (s *SessH) SessDelete(user string) (int, error) {

    key := UserKey{user}
    us, found := s.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    // First remove all ULCL classifiers if any
    for _,ulcl := range(us.UlCl) {
        s.UlClDeleteCls(user, cmn.UlClArg{Addr:ulcl.Addr, Qfi:ulcl.Qfi})
    }

    delete(s.UserMap, key)

    return 0, nil
}

func (s *SessH) UlClAddCls(user string, cls cmn.UlClArg) (int, error) {

    key := UserKey{user}
    us, found := s.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    ulcl, _ := us.UlCl[cls.Addr.String()]

    if ulcl != nil {
        return SESS_ULCLEXIST_ERR, errors.New("ulcl exists")
    }

    ulcl = new(UlClInf)
    ulcl.NumUl, _ = s.HwMark.GetCounter()
    if ulcl.NumUl < 0 {
        return SESS_ULCLNUM_ERR, errors.New("ulcl - ul num err")
    }
    ulcl.NumDl, _ = s.HwMark.GetCounter()
    if ulcl.NumDl < 0 {
        s.HwMark.PutCounter(ulcl.NumUl)
        ulcl.NumUl = -1
        return SESS_ULCLNUM_ERR, errors.New("ulcl - dl num err")
    }

    ulcl.Qfi = cls.Qfi
    ulcl.Addr = cls.Addr
    ulcl.uSess = us

    defer ulcl.DP(DP_CREATE)

    us.UlCl[cls.Addr.String()] = ulcl

    return 0, nil
}

func (s *SessH) UlClDeleteCls(user string, cls cmn.UlClArg) (int, error) {

    key := UserKey{user}
    us, found := s.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    ulcl, _ := us.UlCl[cls.Addr.String()]

    if ulcl == nil {
        return SESS_ULCLNOEXIST_ERR, errors.New("ulcl doesnt exists")
    }

    ulcl.DP(DP_REMOVE)

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
    for _, ulcl := range(us.UlCl) {
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

func (s *SessH) SessionsSync()  {
    for _, us := range s.UserMap {
        for _, ulcl := range(us.UlCl) {
            ulcl.DP(DP_STATS_GET)
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

func (ulcl *UlClInf) DP(work DpWorkT) int {

    if ulcl.uSess == nil {
        return -1
    }

    if work == DP_STATS_GET {
        uStat := new(StatDpWorkQ)
        uStat.Work = work
        uStat.HwMark = uint32(ulcl.NumUl)
        uStat.Name = MAP_NAME_ULCL
        uStat.Bytes = &ulcl.Stats.UlBytes
        uStat.Packets =  &ulcl.Stats.UlBytes

        mh.dp.ToDpCh <- uStat

        dStat := new(StatDpWorkQ)
        dStat.Work = work
        dStat.HwMark = uint32(ulcl.NumDl)
        dStat.Name = MAP_NAME_ULCL
        dStat.Bytes = &ulcl.Stats.DlBytes
        dStat.Packets =  &ulcl.Stats.DlBytes

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
