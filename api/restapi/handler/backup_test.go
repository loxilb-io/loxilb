/*
 * Copyright (c) LoxiLB Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-openapi/runtime"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	cmn "github.com/loxilb-io/loxilb/common"
)

type exportTestHooks struct{ cmn.NetHookInterface }

func (exportTestHooks) NetLbRuleGet() ([]cmn.LbRuleMod, error)   { return nil, nil }
func (exportTestHooks) NetCIStateGet() ([]cmn.HASMod, error)     { return nil, nil }
func (exportTestHooks) NetEpHostGet() ([]cmn.EndPointMod, error) { return nil, nil }
func (exportTestHooks) NetFwRuleGet() ([]cmn.FwRuleMod, error)   { return nil, nil }
func (exportTestHooks) NetMirrorGet() ([]cmn.MirrGetMod, error)  { return nil, nil }
func (exportTestHooks) NetPolicerGet() ([]cmn.PolMod, error)     { return nil, nil }

func TestConfigGetExportDoesNotPersistSnapshot(t *testing.T) {
	oldHooks := ApiHooks
	ApiHooks = exportTestHooks{}
	t.Cleanup(func() { ApiHooks = oldHooks })

	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	responder := ConfigGetExport(operations.GetConfigExportParams{}, nil)
	recorder := httptest.NewRecorder()
	responder.WriteResponse(recorder, runtime.JSONProducer())

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment; filename=loxilb-config-") || !strings.HasSuffix(got, ".json") {
		t.Errorf("Content-Disposition = %q", got)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	for _, key := range []string{"timestamp", "version", "loadbalancer", "cluster", "endpoint", "firewall", "mirror", "policy"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response is missing %q", key)
		}
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("export left %d temporary file(s)", len(entries))
	}
}
