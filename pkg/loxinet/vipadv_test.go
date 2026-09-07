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
	"testing"

	"github.com/loxilb-io/loxilb/pkg/utils"
)

// portsAre - an isPort test that accepts the listed link indexes
func portsAre(indexes ...int) func(int) bool {
	set := make(map[int]bool, len(indexes))
	for _, i := range indexes {
		set[i] = true
	}
	return func(ifIndex int) bool { return set[ifIndex] }
}

func TestPickEgressCand(t *testing.T) {
	tests := []struct {
		name   string
		cands  []utils.EgressCand
		isPort func(int) bool
		want   int
	}{
		{
			name:   "no candidates",
			cands:  nil,
			isPort: portsAre(2, 3),
			want:   -1,
		},
		{
			name: "longest prefix wins",
			cands: []utils.EgressCand{
				{PrefixLen: 0, Priority: 0, IfIndex: 2},
				{PrefixLen: 24, Priority: 100, IfIndex: 3},
				{PrefixLen: 8, Priority: 0, IfIndex: 3},
			},
			isPort: portsAre(2, 3),
			want:   1,
		},
		{
			name: "lowest priority breaks a tie",
			cands: []utils.EgressCand{
				{PrefixLen: 24, Priority: 200, IfIndex: 2},
				{PrefixLen: 24, Priority: 50, IfIndex: 3},
				{PrefixLen: 24, Priority: 100, IfIndex: 2},
			},
			isPort: portsAre(2, 3),
			want:   1,
		},
		{
			name: "first of an exact tie wins",
			cands: []utils.EgressCand{
				{PrefixLen: 16, Priority: 10, IfIndex: 2},
				{PrefixLen: 16, Priority: 10, IfIndex: 3},
			},
			isPort: portsAre(2, 3),
			want:   0,
		},
		{
			name: "a non port loses to a shorter prefix",
			cands: []utils.EgressCand{
				{PrefixLen: 8, Priority: 0, IfIndex: 2},
				{PrefixLen: 24, Priority: 0, IfIndex: 9},
			},
			isPort: portsAre(2, 3),
			want:   0,
		},
		{
			name: "every candidate is a non port",
			cands: []utils.EgressCand{
				{PrefixLen: 24, Priority: 0, IfIndex: 9},
				{PrefixLen: 0, Priority: 0, IfIndex: 8},
			},
			isPort: portsAre(2, 3),
			want:   -1,
		},
		{
			name: "default route is a valid last resort",
			cands: []utils.EgressCand{
				{PrefixLen: 0, Priority: 100, IfIndex: 2},
			},
			isPort: portsAre(2),
			want:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickEgressCand(tc.cands, tc.isPort); got != tc.want {
				t.Fatalf("pickEgressCand = %d, want %d", got, tc.want)
			}
		})
	}
}
