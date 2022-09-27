/*
Copyright © 2022 Netlox Inc. <backguyn@netlox.io>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package ippool

import (
	"net"
	"sync"
)

type IPPool struct {
	IPv4Generator *IPGenerater
	IPv4Pool      *IPSet
	mutex         sync.Mutex
}

// Initailize IP Pool
func NewIPPool(netCIDR string) (*IPPool, error) {
	genIPv4, err := InitIPGenerater(netCIDR)
	if err != nil {
		return nil, err
	}

	poolIPv4 := NewSet()
	poolIPv4.Add(genIPv4.GetNetwork().String())
	poolIPv4.Add(genIPv4.GetBroadcastIP().String())

	return &IPPool{
		IPv4Generator: genIPv4,
		IPv4Pool:      poolIPv4,
		mutex:         sync.Mutex{},
	}, nil
}

// AssignNewIPv4 generate new IP and add key(IP) in IP Pool.
// If IP is already in pool, try to generate next IP.
// Returns nil If all IPs in the subnet are already in the pool.
func (i *IPPool) AssignNewIPv4() net.IP {
	startNewIP := i.IPv4Generator.NextIP()

	i.mutex.Lock()
	defer i.mutex.Unlock()

	id := startNewIP.String()

	if ok := i.IPv4Pool.Contains(id); !ok {
		i.IPv4Pool.Add(id)
		return startNewIP
	}

	for {
		newIP := i.IPv4Generator.NextIP()
		id := newIP.String()
		if ok := i.IPv4Pool.Contains(id); !ok {
			i.IPv4Pool.Add(id)
			return newIP
		}

		if startNewIP.Equal(newIP) {
			return nil
		}
	}
}

// RetrieveIPv4 remove key(IP) in IP Pool
func (i *IPPool) RetrieveIPv4(retrieveIP string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if ok := i.IPv4Pool.Contains(retrieveIP); ok {
		i.IPv4Pool.Remove(retrieveIP)
	}
}

func (i *IPPool) UpdateAllocateddIPv4(allocatedIP string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	if ok := i.IPv4Pool.Contains(allocatedIP); !ok {
		i.IPv4Pool.Add(allocatedIP)
	}
}

func (i *IPPool) CheckSubnetAndUpdateIPPool(ip string) bool {
	if i.IPv4Generator.CheckIPAddressInSubnet(ip) {
		i.UpdateAllocateddIPv4(ip)
		return true
	}

	return false
}
