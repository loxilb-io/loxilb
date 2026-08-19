/*
 * Copyright (c) 2026 NetLOX Inc
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

package logrotate

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// waitFor polls until cond is true or the deadline passes (rotation
// finishing steps run asynchronously).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func listBackups(t *testing.T, orig string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(orig))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if isBackupOf(orig, e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestWriterRotatesAndCompresses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilbtest.log")
	// 1 MB limit is the smallest expressible; write 1.5 MB in 64 KiB lines.
	w, err := New(path, Config{MaxSizeMB: 1, MaxBackups: 3, Compress: true})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	line := bytes.Repeat([]byte("x"), 64*1024)
	for i := 0; i < 24; i++ { // 24 * 64 KiB = 1.5 MiB
		if _, err := w.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "compressed backup", func() bool {
		bks := listBackups(t, path)
		return len(bks) == 1 && strings.HasSuffix(bks[0], ".log.gz")
	})
	// Active file was reset and keeps receiving writes.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > int64(len(line)*10) {
		t.Fatalf("active file not reset: %d bytes", st.Size())
	}
	// Backup names keep the REST-API-visible loxilb*.log.gz shape.
	bks := listBackups(t, path)
	if !strings.HasPrefix(bks[0], "loxilb") {
		t.Fatalf("backup %q lost loxilb prefix", bks[0])
	}
	// Compressed content decompresses to the original bytes.
	f, err := os.Open(filepath.Join(dir, bks[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[0] != 'x' {
		t.Fatal("backup content mismatch")
	}
}

func TestWriterConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilbcc.log")
	w, err := New(path, Config{MaxSizeMB: 1, MaxBackups: 2, Compress: false})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	line := bytes.Repeat([]byte("y"), 8*1024)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if _, err := w.Write(line); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait() // 8*40*8 KiB = 2.5 MiB total; must have rotated without error
	if bks := listBackups(t, path); len(bks) == 0 {
		t.Fatal("no rotation under concurrent writes")
	}
}

func TestPruneMaxBackups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilbprune.log")
	cfg := Config{MaxSizeMB: 1, MaxBackups: 2, Compress: false}
	// Fabricate 4 backups with distinct timestamps.
	for i := 0; i < 4; i++ {
		bak := backupName(path, time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC))
		if err := os.WriteFile(bak, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneBackups(path, cfg); err != nil {
		t.Fatal(err)
	}
	bks := listBackups(t, path)
	if len(bks) != 2 {
		t.Fatalf("want 2 backups after prune, got %v", bks)
	}
	// The two NEWEST (Jan 3, Jan 4) survive.
	for _, b := range bks {
		if !strings.Contains(b, "20260103") && !strings.Contains(b, "20260104") {
			t.Fatalf("pruned wrong file, kept %q", b)
		}
	}
}

func TestPruneMaxAge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilbage.log")
	bak := backupName(path, time.Now().AddDate(0, 0, -40))
	if err := os.WriteFile(bak, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(bak, old, old); err != nil {
		t.Fatal(err)
	}
	if err := pruneBackups(path, Config{MaxSizeMB: 1, MaxAgeDays: 28}); err != nil {
		t.Fatal(err)
	}
	if bks := listBackups(t, path); len(bks) != 0 {
		t.Fatalf("aged backup not pruned: %v", bks)
	}
}

func TestSweepCopyTruncate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilbdp.log")
	// Simulate the C library holding an O_APPEND handle.
	foreign, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	big := bytes.Repeat([]byte("z"), 1200*1024) // > 1 MB
	if _, err := foreign.Write(big); err != nil {
		t.Fatal(err)
	}

	if err := Sweep(path, Config{MaxSizeMB: 1, MaxBackups: 2, Compress: true}); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != 0 {
		t.Fatalf("file not truncated: %d", st.Size())
	}
	waitFor(t, "sweep backup", func() bool { return len(listBackups(t, path)) == 1 })
	// The foreign O_APPEND handle keeps working after truncation.
	if _, err := foreign.Write([]byte("still alive\n")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "still alive\n" {
		t.Fatalf("foreign writer broken after sweep: %q", raw)
	}
	// Below the limit: sweep is a no-op.
	if err := Sweep(path, Config{MaxSizeMB: 1}); err != nil {
		t.Fatal(err)
	}
	if bks := listBackups(t, path); len(bks) != 1 {
		t.Fatalf("no-op sweep rotated anyway: %v", bks)
	}
}

func TestDisabledConfigPassesThrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loxilboff.log")
	w, err := New(path, Config{}) // zero value = disabled
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	big := bytes.Repeat([]byte("q"), 2*1024*1024)
	if _, err := w.Write(big); err != nil {
		t.Fatal(err)
	}
	if bks := listBackups(t, path); len(bks) != 0 {
		t.Fatalf("disabled config rotated: %v", bks)
	}
}

func TestIsBackupOfRejectsForeignNames(t *testing.T) {
	orig := "/var/log/loxilb.log"
	for _, bad := range []string{
		"loxilb.log", "loxilb-other.log", "loxilb-20260101.log",
		"loxilbdp-20260101-000000.000.log", "loxilb-20260101-000000.000.txt",
	} {
		if isBackupOf(orig, bad) {
			t.Errorf("isBackupOf accepted %q", bad)
		}
	}
	good := filepath.Base(backupName(orig, time.Now()))
	if !isBackupOf(orig, good) || !isBackupOf(orig, good+".gz") {
		t.Errorf("isBackupOf rejected own backup %q", good)
	}
}
