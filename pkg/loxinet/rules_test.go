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
// like the connect probes, need a probe port. probesni only makes sense for
// https probes.
func TestLbRuleProbeArgs(t *testing.T) {
	ensureLoxinet(t)

	eps := []cmn.LbEndPointArg{{EpIP: "192.0.2.20", EpPort: 8080, Weight: 1}}

	tests := []struct {
		name  string
		ptype string
		pport uint16
		psni  string
		ok    bool
	}{
		{"http", HostProbeHTTP, 8080, "", true},
		{"https", HostProbeHTTPS, 8443, "", true},
		{"https with sni", HostProbeHTTPS, 8443, "example.com", true},
		{"http without port", HostProbeHTTP, 0, "", false},
		{"https without port", HostProbeHTTPS, 0, "", false},
		{"sni with http", HostProbeHTTP, 8080, "example.com", false},
		{"sni with tcp", HostProbeConnectTCP, 8080, "example.com", false},
		{"sni without probetype", "", 0, "example.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serv := cmn.LbServiceArg{ServIP: "10.10.20.1", ServPort: 8080, Proto: "tcp",
				Sel: cmn.LbSelRr, ProbeType: tc.ptype, ProbePort: tc.pport, ProbeSNI: tc.psni}
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

// TestLbRuleHTTPSProbeSNI - a rule-level probesni must reach the ep-host that
// runs the https probe, be reported back by GetLBRule and be updatable.
func TestLbRuleHTTPSProbeSNI(t *testing.T) {
	ensureLoxinet(t)

	serv := cmn.LbServiceArg{ServIP: "10.10.20.2", ServPort: 443, Proto: "tcp", Sel: cmn.LbSelRr,
		ProbeType: HostProbeHTTPS, ProbePort: 8443, ProbeReq: "health", ProbeResp: "OK",
		ProbeSNI: "example.com"}
	eps := []cmn.LbEndPointArg{{EpIP: "192.0.2.21", EpPort: 443, Weight: 1}}

	if _, err := mh.zr.Rules.AddLbRule(serv, nil, nil, eps); err != nil {
		t.Fatalf("failed to add lb rule: %v", err)
	}
	defer mh.zr.Rules.DeleteLbRule(serv)

	srch := serv
	srch.ServPortMax = srch.ServPort
	rule := mh.zr.Rules.GetLBRuleByServArgs(srch)
	if rule == nil {
		t.Fatalf("lb rule not found")
	}
	if rule.hChk.prbSNI != "example.com" || !rule.hChk.actChk {
		t.Errorf("rule probe sni %q monitor %v, want example.com and monitor on",
			rule.hChk.prbSNI, rule.hChk.actChk)
	}

	// The ep-host is keyed by the service protocol and target port, and
	// carries the rule's probe settings
	epKey := makeEPKey("192.0.2.21", HostProbeConnectTCP, 443)
	mh.zr.Rules.epMx.Lock()
	ep := mh.zr.Rules.epMap[epKey]
	if ep == nil {
		t.Errorf("ep-host %s not found", epKey)
	} else if ep.opts.probeType != HostProbeHTTPS || ep.opts.probePort != 8443 ||
		ep.opts.probeSNI != "example.com" {
		t.Errorf("ep-host %s probe %s:%d sni %q, want https:8443 sni example.com",
			epKey, ep.opts.probeType, ep.opts.probePort, ep.opts.probeSNI)
	}
	mh.zr.Rules.epMx.Unlock()

	rules, err := mh.zr.Rules.GetLBRule()
	if err != nil {
		t.Fatalf("failed to get lb rules: %v", err)
	}
	found := false
	for _, r := range rules {
		if r.Serv.ServIP == serv.ServIP && r.Serv.ServPort == serv.ServPort {
			found = true
			if r.Serv.ProbeSNI != "example.com" {
				t.Errorf("GetLBRule probe sni %q, want example.com", r.Serv.ProbeSNI)
			}
		}
	}
	if !found {
		t.Errorf("lb rule missing from GetLBRule")
	}

	// Changing only the sni is a rule update, not an lbrule-exists error
	serv.ProbeSNI = "www.example.com"
	if _, err := mh.zr.Rules.AddLbRule(serv, nil, nil, eps); err != nil {
		t.Fatalf("failed to update probe sni: %v", err)
	}
	if rule.hChk.prbSNI != "www.example.com" {
		t.Errorf("rule probe sni %q after update, want www.example.com", rule.hChk.prbSNI)
	}
}
