// SPDX-License-Identifier: Apache 2.0
// Copyright (c) 2024 NetLOX Inc

package utils

import (
	"errors"
)

// Marker - context container
type Marker struct {
	begin   uint64
	end     uint64
	start   uint64
	len     uint64
	cap     uint64
	markers []uint64
}

// NewMarker - Allocate a set of markers
func NewMarker(begin uint64, length uint64) *Marker {
	marker := new(Marker)
	marker.markers = make([]uint64, length)
	marker.begin = begin
	marker.start = 0
	marker.end = length - 1
	marker.len = length
	marker.cap = length
	for i := uint64(0); i < length; i++ {
		marker.markers[i] = i + 1
	}
	marker.markers[length-1] = ^uint64(0)
	return marker
}

// GetMarker - Get next available marker
func (M *Marker) GetMarker() (uint64, error) {
	if M.cap <= 0 || M.start == ^uint64(0) {
		return ^uint64(0), errors.New("Overflow")
	}

	M.cap--
	var rid = M.start
	M.start = M.markers[rid]
	M.markers[rid] = ^uint64(0)
	return rid + M.begin, nil
}

// ReleaseMarker - Return a marker to the available list
func (M *Marker) ReleaseMarker(id uint64) error {
	if id < M.begin || id >= M.begin+M.len {
		return errors.New("Range")
	}
	rid := id - M.begin
	M.markers[rid] = M.start
	M.start = rid
	M.cap++
	if M.start == ^uint64(0) {
		M.start = M.end
	}
	return nil
}
