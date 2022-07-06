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

func (ut *UserTun) Equal(ut1 *UserTun) bool {
    if ut.TeID == ut1.TeID && ut.Addr.Equal(ut1.Addr) {
        return true
    }
    return false
}

type UlClInf struct {
    Addr    net.IP
    Qfi	    uint8
    Num     int
    Status  DpStatusT
    uSess   *UserSess
}

type UserSess struct {
    Key       UserKey
    Addr      net.IP
    Zone      int
    AnTun	  UserTun
    CnTun     UserTun
    UlCl	  map[string]*UlClInf
}

type UserH struct {
    UserMap  map[UserKey]*UserSess
    Zone   *Zone
    HwMark *tk.Counter
}

func SessInit(zone *Zone) *UserH {
    var nUh = new(UserH)
    nUh.UserMap = make(map[UserKey]*UserSess)
    nUh.Zone = zone
    nUh.HwMark = tk.NewCounter(1, MAX_ULCLS)
    return nUh
}

func (u *UserH) SessAdd(user string, IP net.IP, anTun UserTun, cnTun UserTun) (int, error) {

    key := UserKey{user}
    us, found := u.UserMap[key]

    if found == true {

        if us.AnTun.Equal(&anTun) == false ||  us.CnTun.Equal(&cnTun) {
            ret, _ := u.SessDelete(user)
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
    us.Zone = u.Zone.ZoneNum

    us.UlCl = make(map[string]*UlClInf)

    u.UserMap[us.Key] = us

    return 0, nil
}

func (u *UserH) SessDelete(user string) (int, error) {

    key := UserKey{user}
    us, found := u.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    // First remove all ULCL classifiers if any
    for _,ulcl := range(us.UlCl) {
        ulcl.DP(DP_REMOVE)
        delete(us.UlCl, ulcl.Addr.String())
    }

    delete(u.UserMap, key)

    return 0, nil
}

func (u *UserH) UlClAddCls(user string, cls cmn.UlClArg) (int, error) {

    key := UserKey{user}
    us, found := u.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    ulcl, _ := us.UlCl[cls.Addr.String()]

    if ulcl != nil {
        return SESS_ULCLEXIST_ERR, errors.New("ulcl exists")
    }

    ulcl = new(UlClInf)
    ulcl.Num, _ = u.HwMark.GetCounter()
    if ulcl.Num < 0 {
        return SESS_ULCLNUM_ERR, errors.New("ulcl num err")
    }
    ulcl.Qfi = cls.Qfi
    ulcl.Addr = cls.Addr
    ulcl.uSess = us

    defer ulcl.DP(DP_CREATE)

    us.UlCl[cls.Addr.String()] = ulcl

    return 0, nil
}

func (u *UserH) UlClDeleteCls(user string, cls cmn.UlClArg) (int, error) {

    key := UserKey{user}
    us, found := u.UserMap[key]

    if found == false {
        return SESS_NOEXIST_ERR, errors.New("user doesnt exists")
    }

    ulcl, _ := us.UlCl[cls.Addr.String()]

    if ulcl == nil {
        return SESS_ULCLNOEXIST_ERR, errors.New("ulcl doesnt exists")
    }

    ulcl.DP(DP_REMOVE)

    delete(us.UlCl, cls.Addr.String())

    return 0, nil
}

func Us2String(us *UserSess) string {
    var tStr string

    tStr += fmt.Sprintf("%s:%s AN(%s:0x%x) CN(%s:0x%x)", 
                        us.Key.UserID, us.Addr.String(),
                        us.AnTun.Addr.String(), us.AnTun.TeID,
                        us.CnTun.Addr.String(), us.CnTun.TeID)
    for _, ulcl := range(us.UlCl) {
        tStr += fmt.Sprintf("\t\t%s,qfi-%d, num-%d\n", ulcl.Addr.String(), ulcl.Qfi, ulcl.Num)
    }

    return tStr
}

func (u *UserH) USess2String(it IterIntf) error {
	for _, us := range u.UserMap {
		uBuf := Us2String(us)
		it.NodeWalker(uBuf)
	}
	return nil
}

func (ulcl *UlClInf) DP(work DpWorkT) int {

    if ulcl.uSess == nil {
        return -1
    }

	// For UL dir
    ucn := new(UlClDpWorkQ)
    ucn.Work = work
    ucn.mDip = ulcl.Addr
    ucn.mSip = ulcl.uSess.Addr
    ucn.mTeID = ulcl.uSess.AnTun.TeID
    ucn.Zone = ulcl.uSess.Zone
    ucn.HwMark = ulcl.Num
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
    ucn.HwMark = ulcl.Num
    ucn.Qfi = ulcl.Qfi
	ucn.tDip = ulcl.uSess.AnTun.Addr
	ucn.tSip = ulcl.uSess.CnTun.Addr
	ucn.tTeID = ulcl.uSess.AnTun.TeID

    mh.dp.ToDpCh <- ucn

    return 0
}
