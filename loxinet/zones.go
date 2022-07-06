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
    "sync"
    tk "loxilb/loxilib"
)

const (
    ZONE_BASE_ERR = iota - RT_ERR_BASE - 1000
    ZONE_EXISTS_ERR
    ZONE_NOTEXIST_ERR
    ZONE_NUMBER_ERR
)

const (
    MAX_ZONES = 256
)

type Zone struct {
    Name    string
    ZoneNum int
    Ports   *PortsH
    Vlans   *VlansH
    L2      *L2H
    Nh      *NeighH
    Rt      *RtH
    L3      *L3H
    Rules   *RuleH
    Sess    *SessH
    Mtx     sync.RWMutex
}

type ZoneH struct {
    ZoneMap   map[string]*Zone
    ZoneBrs   map[string]*Zone
    ZonePorts map[string]*Zone
    ZoneMark  *tk.Counter
}

func ZoneInit() *ZoneH {
    var zn = new(ZoneH)
    zn.ZoneMap = make(map[string]*Zone)
    zn.ZoneBrs = make(map[string]*Zone)
    zn.ZonePorts = make(map[string]*Zone)
    zn.ZoneMark = tk.NewCounter(0, MAX_ZONES)

    return zn
}

func (z *ZoneH) ZoneAdd(name string) (int, error) {
    var err error
    zone := z.ZoneMap[name]
    if zone != nil {
        return ZONE_EXISTS_ERR, errors.New("existing zone")
    }

    zone = new(Zone)

    zone.ZoneNum, err = z.ZoneMark.GetCounter()
    if err != nil {
        return ZONE_NUMBER_ERR, errors.New("zone number err")
    }

    zone.Name = name
    zone.Ports = PortInit()
    zone.Vlans = VlanInit(zone)
    zone.L2 = L2Init(zone)
    zone.Nh = NeighInit(zone)
    zone.Rt = RtInit(zone)
    zone.L3 = L3Init(zone)
    zone.Rules = RulesInit(zone)
    zone.Sess = SessInit(zone)

    z.ZoneMap[name] = zone

    return 0, nil
}

func (z *ZoneH) Zonefind(name string) (*Zone, int) {
    zone := z.ZoneMap[name]
    if zone == nil {
        return nil, -1
    }

    return zone, zone.ZoneNum
}

func (z *ZoneH) ZoneDelete(name string) (int, error) {

    zone := z.ZoneMap[name]
    if zone == nil {
        return ZONE_NOTEXIST_ERR, errors.New("no such zone")
    }

    zone.Name = name
    zone.Rt.RtDestructAll()
    zone.Nh.NeighhDestructAll()
    zone.L2.L2DestructAll()
    zone.Vlans.VlanDestructAll()
    zone.Ports.PortDestructAll()

    delete(z.ZoneMap, name)

    return 0, nil
}

func (z *ZoneH) ZoneBrAdd(name string, zns string) (int, error) {
    zone := z.ZoneBrs[name]
    if zone != nil {
        if zone.Name == name {
            return 0, nil
        }
        return ZONE_EXISTS_ERR, errors.New("zone exists")
    }

    zone, _ = z.Zonefind(zns)
    if zone == nil {
        return ZONE_NOTEXIST_ERR, errors.New("zone is not set")
    }

    z.ZoneBrs[name] = zone

    return 0, nil
}

func (z *ZoneH) ZoneBrDelete(name string) (int, error) {
    zone := z.ZoneBrs[name]
    if zone == nil {
        return ZONE_NOTEXIST_ERR, errors.New("zone is not set")
    }

    delete(z.ZoneBrs, name)

    return 0, nil
}

func (z *ZoneH) ZonePortIsValid(name string, zns string) (int, error) {
    zone := z.ZonePorts[name]
    if zone == nil {
        return 0, nil
    }

    if zone.Name == zns {
        return 0, nil
    }

    return ZONE_EXISTS_ERR, errors.New("zone exists")
}

func (z *ZoneH) GetPortZone(port string) (*Zone) {
    zone := z.ZonePorts[port]
    if zone == nil {
        return nil
    }

    return zone
}

func (z *ZoneH) ZonePortAdd(name string, zns string) (int, error) {
    zone := z.ZonePorts[name]
    if zone != nil {
        if zone.Name == name {
            return 0, nil
        }
        return ZONE_EXISTS_ERR, errors.New("zone exists")
    }

    zone, _ = z.Zonefind(zns)
    if zone == nil {
        return ZONE_NOTEXIST_ERR, errors.New("zone is not set")
    }

    z.ZonePorts[name] = zone
    return 0, nil
}

func (z *ZoneH) ZonePortDelete(name string) (int, error) {
    zone := z.ZonePorts[name]
    if zone == nil {
        return ZONE_NOTEXIST_ERR, errors.New("zone is not set")
    }

    delete(z.ZonePorts, name)

    return 0, nil
}

func (z *ZoneH) ZoneTicker() {
    for _, zone := range(z.ZoneMap) {

        mh.mtx.Lock()
        zone.L2.FdbsTicker()
        zone.Nh.NeighsTicker()
        mh.mtx.Unlock()

        /* NOTE - No need to hold lock for an extended period */

        mh.mtx.RLock()
        zone.Rules.RulesTicker()
        mh.mtx.RUnlock()

        //mh.mtx.RLock()
        //zone.Vlans.VlansTicker()
        //mh.mtx.RUnlock()

        //mh.mtx.RLock()
        //zone.Rt.RoutesTicker()
        //mh.mtx.RUnlock()
    }
}