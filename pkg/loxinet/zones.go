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

	tk "github.com/loxilb-io/loxilib"
)

// This file implements self-contained network security zones
// Currently we can have upto MAX_ZONES such zones

// error codes for zone
const (
	ZoneBaseErr = iota - 6000
	ZoneExistsErr
	ZoneNotExistErr
	ZoneNumberErr
)

// constant to define maximum number of zones
const (
	MaximumZones = 256
)

// Zone - zone info
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
	Pols    *PolH
	Mirrs   *MirrH
	Mtx     sync.RWMutex
}

// ZoneH - Zone context handler
type ZoneH struct {
	ZoneMap   map[string]*Zone
	ZoneBrs   map[string]*Zone
	ZonePorts map[string]*Zone
	ZoneMark  *tk.Counter
}

// ZoneInit - routine to initialize zone context handler
func ZoneInit() *ZoneH {
	var zn = new(ZoneH)
	zn.ZoneMap = make(map[string]*Zone)
	zn.ZoneBrs = make(map[string]*Zone)
	zn.ZonePorts = make(map[string]*Zone)
	zn.ZoneMark = tk.NewCounter(1, MaximumZones)

	return zn
}

// ZoneAdd - routine to add a zone
func (z *ZoneH) ZoneAdd(name string) (int, error) {
	var err error
	zone := z.ZoneMap[name]
	if zone != nil {
		return ZoneExistsErr, errors.New("existing zone")
	}

	zone = new(Zone)

	zNum, err := z.ZoneMark.GetCounter()
	if err != nil {
		return ZoneNumberErr, errors.New("zone number err")
	}

	zone.ZoneNum = int(zNum)
	zone.Name = name
	zone.Ports = PortInit()
	zone.Vlans = VlanInit(zone)
	zone.L2 = L2Init(zone)
	zone.Nh = NeighInit(zone)
	zone.Rt = RtInit(zone)
	zone.L3 = L3Init(zone)
	zone.Rules = RulesInit(zone)
	zone.Sess = SessInit(zone)
	zone.Pols = PolInit(zone)
	zone.Mirrs = MirrInit(zone)

	z.ZoneMap[name] = zone

	return 0, nil
}

// Zonefind - routine to find a zone
func (z *ZoneH) Zonefind(name string) (*Zone, int) {
	zone := z.ZoneMap[name]
	if zone == nil {
		return nil, -1
	}

	return zone, zone.ZoneNum
}

// ZoneDelete - routine to delete a zone
func (z *ZoneH) ZoneDelete(name string) (int, error) {

	zone := z.ZoneMap[name]
	if zone == nil {
		return ZoneNotExistErr, errors.New("no such zone")
	}

	zone.Rules.RuleDestructAll()
	zone.Mirrs.MirrDestructAll()
	zone.Pols.PolDestructAll()
	zone.Rt.RtDestructAll()
	zone.Nh.NeighDestructAll()
	zone.L2.L2DestructAll()
	zone.Vlans.VlanDestructAll()
	zone.Ports.PortDestructAll()

	delete(z.ZoneMap, name)

	return 0, nil
}

// ZoneBrAdd - Routine to add a bridge in a zone
func (z *ZoneH) ZoneBrAdd(name string, zns string) (int, error) {
	zone := z.ZoneBrs[name]
	if zone != nil {
		if zone.Name == zns {
			return 0, nil
		}
		return ZoneExistsErr, errors.New("zone exists")
	}

	zone, _ = z.Zonefind(zns)
	if zone == nil {
		return ZoneNotExistErr, errors.New("zone is not set")
	}

	z.ZoneBrs[name] = zone

	return 0, nil
}

// ZoneBrDelete - routine to delete a bridge from the zone
func (z *ZoneH) ZoneBrDelete(name string) (int, error) {
	zone := z.ZoneBrs[name]
	if zone == nil {
		return ZoneNotExistErr, errors.New("zone is not set")
	}

	delete(z.ZoneBrs, name)

	return 0, nil
}

// ZonePortIsValid - routine to check if the port belongs to a zone
func (z *ZoneH) ZonePortIsValid(name string, zns string) (int, error) {
	zone := z.ZonePorts[name]
	if zone == nil {
		return 0, nil
	}

	if zone.Name == zns {
		return 0, nil
	}

	return ZoneExistsErr, errors.New("zone exists")
}

// GetPortZone - routine to identify the zone of a port
func (z *ZoneH) GetPortZone(port string) *Zone {
	return z.ZonePorts[port]
}

// ZonePortAdd - routine to add a port to a zone
func (z *ZoneH) ZonePortAdd(name string, zns string) (int, error) {
	zone := z.ZonePorts[name]
	if zone != nil {
		if zone.Name == zns {
			return 0, nil
		}
		return ZoneExistsErr, errors.New("zone exists")
	}

	zone, _ = z.Zonefind(zns)
	if zone == nil {
		return ZoneNotExistErr, errors.New("zone is not set")
	}

	z.ZonePorts[name] = zone
	return 0, nil
}

// ZonePortDelete - routine to delete a port from a zone
func (z *ZoneH) ZonePortDelete(name string) (int, error) {
	zone := z.ZonePorts[name]
	if zone == nil {
		return ZoneNotExistErr, errors.New("zone is not set")
	}

	delete(z.ZonePorts, name)

	return 0, nil
}

// ZoneTicker - This ticker routine takes care of all house-keeping operations
// for all instances of security zones. This is called from loxiNetTicker
func (z *ZoneH) ZoneTicker() {
	for _, zone := range z.ZoneMap {

		mh.mtx.Lock()
		defer mh.mtx.Unlock()

		zone.L2.FdbsTicker()
		zone.Nh.NeighsTicker()
		zone.Rules.RulesTicker()
		//zone.Vlans.VlansTicker()
		//zone.Rt.RoutesTicker()
		zone.Sess.SessionTicker()
		zone.Pols.PolTicker()
		zone.Mirrs.MirrTicker()
		zone.L3.IfasTicker()
	}
}
