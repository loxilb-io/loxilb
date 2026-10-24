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

	cmn "github.com/loxilb-io/loxilb/common"
)

// TestLbRuleProbeArgs - http and https are valid rule-level probe types and,
// like the connect probes, need a probe port.
func TestLbRuleProbeArgs(t *testing.T) {
	ensureLoxinet(t)

	eps := []cmn.LbEndPointArg{{EpIP: "192.0.2.20", EpPort: 8080, Weight: 1}}

	tests := []struct {
		name  string
		ptype string
		pport uint16
		ok    bool
	}{
		{"http", HostProbeHTTP, 8080, true},
		{"https", HostProbeHTTPS, 8443, true},
		{"http without port", HostProbeHTTP, 0, false},
		{"https without port", HostProbeHTTPS, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serv := cmn.LbServiceArg{ServIP: "10.10.20.1", ServPort: 8080, Proto: "tcp",
				Sel: cmn.LbSelRr, ProbeType: tc.ptype, ProbePort: tc.pport}
			_, err := mh.zr.Rules.AddLbRule(serv, nil, nil, eps)
			if tc.ok {
				if err != nil {
					t.Fatalf("lb rule rejected: %v", err)
				}
				if _, err := mh.zr.Rules.DeleteLbRule(serv); err != nil {
					t.Fatalf("failed to delete lb rule: %v", err)
				}
			} else if err == nil {
				mh.zr.Rules.DeleteLbRule(serv)
				t.Fatalf("lb rule accepted")
			}
		})
	}
}
