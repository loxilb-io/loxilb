/* SPDX-License-Identifier: Apache-2.0
 * Copyright (c) 2026 NetLOX Inc.
 */
package loxinlp

import (
	"regexp"
	"testing"
)

func TestInterfaceFilterWithoutNetlinkInitialization(t *testing.T) {
	previous := nNl
	t.Cleanup(func() { nNl = previous })
	nNl = nil
	for _, name := range []string{"lo", "eth0", "llb-rule1"} {
		if !NlpIsBlackListedIntf(name, 0) {
			t.Errorf("uninitialized netlink must exclude %q from reconciliation", name)
		}
	}
	nNl = &NlH{WhiteList: "none", BLRgx: regexp.MustCompile("^docker")}
	if NlpIsBlackListedIntf("eth0", 0) || !NlpIsBlackListedIntf("docker0", 0) || !NlpIsBlackListedIntf("lo", 0) {
		t.Fatal("initialized blacklist behavior changed")
	}
	nNl.WhiteList = "^eth0$"
	nNl.WLRgx = regexp.MustCompile(nNl.WhiteList)
	if NlpIsBlackListedIntf("eth0", 0) || !NlpIsBlackListedIntf("eth1", 0) {
		t.Fatal("initialized whitelist behavior changed")
	}
}
