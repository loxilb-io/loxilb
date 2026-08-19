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
package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
)

// writeLogFile lays down a log file whose lines are individually identifiable
// so ordering, duplication and gaps are all detectable.
func writeLogFile(t *testing.T, lineCount int, trailingNewline bool) (*os.File, []string) {
	t.Helper()

	lines := make([]string, 0, lineCount)
	var sb strings.Builder
	for i := 0; i < lineCount; i++ {
		line := fmt.Sprintf("2026-08-13 05:00:00 DBG line-%04d payload", i)
		lines = append(lines, line)
		sb.WriteString(line)
		if i < lineCount-1 || trailingNewline {
			sb.WriteString("\n")
		}
	}

	path := filepath.Join(t.TempDir(), "loxilbtest.log")
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}
	t.Cleanup(func() { file.Close() })

	return file, lines
}

func fileSize(t *testing.T, file *os.File) int64 {
	t.Helper()
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	return info.Size()
}

func TestReadLinesBeforeReturnsNewestFirst(t *testing.T) {
	file, lines := writeLogFile(t, 50, true)

	got, next := readLinesBefore(file, fileSize(t, file), 5)

	if len(got) != 5 {
		t.Fatalf("got %d lines, want 5", len(got))
	}
	for i, want := range []string{lines[49], lines[48], lines[47], lines[46], lines[45]} {
		if got[i] != want {
			t.Errorf("line %d = %q, want %q", i, got[i], want)
		}
	}
	if next <= 0 {
		t.Errorf("next offset = %d, want a positive cursor into the file", next)
	}
}

// Re-reading without advancing the cursor must return the same page. This is
// the behaviour the old per-client cursor map broke: a second read drained.
func TestReadLinesBeforeIsRepeatable(t *testing.T) {
	file, _ := writeLogFile(t, 200, true)
	size := fileSize(t, file)

	first, firstNext := readLinesBefore(file, size, 20)
	for i := 0; i < 4; i++ {
		got, next := readLinesBefore(file, size, 20)
		if len(got) != len(first) {
			t.Fatalf("read %d returned %d lines, want %d", i+2, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("read %d line %d = %q, want %q", i+2, j, got[j], first[j])
			}
		}
		if next != firstNext {
			t.Fatalf("read %d cursor = %d, want %d", i+2, next, firstNext)
		}
	}
}

// Paging from the tail to the head must yield every line exactly once, in
// order, and then stop.
func TestReadLinesBeforePagesWholeFileWithoutGapsOrDuplicates(t *testing.T) {
	for _, pageSize := range []int{1, 7, 64, 500} {
		for _, trailingNewline := range []bool{true, false} {
			t.Run(fmt.Sprintf("page=%d/trailing=%v", pageSize, trailingNewline), func(t *testing.T) {
				file, lines := writeLogFile(t, 300, trailingNewline)

				var collected []string
				offset := fileSize(t, file)
				for i := 0; ; i++ {
					if i > len(lines)+10 {
						t.Fatal("paging did not terminate")
					}
					page, next := readLinesBefore(file, offset, pageSize)
					collected = append(collected, page...)
					if next <= 0 {
						break
					}
					if next >= offset {
						t.Fatalf("cursor did not advance backwards: %d -> %d", offset, next)
					}
					offset = next
				}

				if len(collected) != len(lines) {
					t.Fatalf("collected %d lines, want %d", len(collected), len(lines))
				}
				for i, want := range lines {
					got := collected[len(collected)-1-i] // collected is newest-first
					if got != want {
						t.Fatalf("line %d = %q, want %q", i, got, want)
					}
				}
			})
		}
	}
}

// A page larger than the file returns everything and reports no more pages.
func TestReadLinesBeforeShortFile(t *testing.T) {
	file, lines := writeLogFile(t, 3, true)

	got, next := readLinesBefore(file, fileSize(t, file), 1000)

	if len(got) != len(lines) {
		t.Fatalf("got %d lines, want %d", len(got), len(lines))
	}
	if next != 0 {
		t.Errorf("next offset = %d, want 0 (start of file reached)", next)
	}
}

func TestReadLinesBeforeEmptyFile(t *testing.T) {
	file, _ := writeLogFile(t, 0, false)

	got, next := readLinesBefore(file, fileSize(t, file), 10)

	if len(got) != 0 {
		t.Errorf("got %d lines, want 0", len(got))
	}
	if next != 0 {
		t.Errorf("next offset = %d, want 0", next)
	}
}

// Lines longer than one read chunk must not be split or dropped.
func TestReadLinesBeforeHandlesLinesLongerThanChunk(t *testing.T) {
	long := "DBG " + strings.Repeat("x", minLogReadChunk*3)
	path := filepath.Join(t.TempDir(), "loxilbtest.log")
	if err := os.WriteFile(path, []byte("DBG first\n"+long+"\nDBG last\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()

	got, _ := readLinesBefore(file, fileSize(t, file), 3)

	want := []string{"DBG last", long, "DBG first"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d mismatch (len %d, want len %d)", i, len(got[i]), len(want[i]))
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	want := LogCursor{
		Filename: "loxilbhost1.log",
		Offset:   123456,
		ModTime:  time.Unix(1755000000, 0),
		FileSize: 999999,
	}

	got, err := decodeCursor(encodeCursor(want))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Filename != want.Filename || got.Offset != want.Offset ||
		!got.ModTime.Equal(want.ModTime) || got.FileSize != want.FileSize {
		t.Errorf("round trip = %+v, want %+v", *got, want)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, in := range []string{"not-base64!!", "YWJj", "YTpiOmM="} { // "abc", "a:b:c"
		if _, err := decodeCursor(in); err == nil {
			t.Errorf("decodeCursor(%q) succeeded, want error", in)
		}
	}
	if c, err := decodeCursor(""); err != nil || c != nil {
		t.Errorf("empty cursor = (%v, %v), want (nil, nil)", c, err)
	}
}

func TestCursorValid(t *testing.T) {
	cursor := &LogCursor{Filename: "loxilbhost1.log", Offset: 500, FileSize: 1000}

	tests := []struct {
		name     string
		filename string
		size     int64
		want     bool
	}{
		{"same file, grown", "loxilbhost1.log", 5000, true},
		{"same file, unchanged", "loxilbhost1.log", 1000, true},
		{"same file, truncated past cursor", "loxilbhost1.log", 100, false},
		{"different file", "loxilbdp.log", 5000, false},
	}
	for _, tc := range tests {
		if got := cursorValid(cursor, tc.filename, tc.size); got != tc.want {
			t.Errorf("%s: cursorValid = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !cursorValid(nil, "loxilbhost1.log", 10) {
		t.Error("nil cursor should be valid (start from newest)")
	}
}

// An empty result must still serialize as [] so clients can read the
// pagination fields sitting beside it.
func TestFilterLogsNeverReturnsNil(t *testing.T) {
	if got := filterLogs([]string{"INFO a"}, "ERR", ""); got == nil {
		t.Error("filterLogs returned nil for a fully filtered page")
	}
	if got := filterLogs(nil, "", ""); got == nil {
		t.Error("filterLogs returned nil for an empty page")
	}

	got := filterLogs([]string{"DBG a", "ERR b", "DBG c"}, "DBG", "c")
	if len(got) != 1 || got[0] != "DBG c" {
		t.Errorf("filterLogs = %v, want [DBG c]", got)
	}
}

//----------------------------------------------------------------------
// Endpoint-level behaviour
//----------------------------------------------------------------------

// getLogs drives the real handler and returns the 200 payload.
func getLogs(t *testing.T, params operations.GetLogsParams) *models.Logs {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/netlox/v1/logs", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	params.HTTPRequest = req

	switch r := ConfigGetLogs(params, nil).(type) {
	case *operations.GetLogsOK:
		return r.Payload
	default:
		t.Fatalf("expected 200, got %T", r)
		return nil
	}
}

// stageLogDirIn points the handler at an empty temp directory and returns it,
// for tests that lay down their own fixtures.
func stageLogDirIn(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	origPath, origArchive := logFilePath, archivePath
	logFilePath, archivePath = dir+"/", dir+"/"
	t.Cleanup(func() { logFilePath, archivePath = origPath, origArchive })
	return dir
}

// stageLogDir points the handler at a temp directory holding one log file.
func stageLogDir(t *testing.T, name string, lineCount int) {
	t.Helper()

	dir := stageLogDirIn(t)

	var sb strings.Builder
	for i := 0; i < lineCount; i++ {
		level := "INFO"
		if i%3 == 0 {
			level = "DBG"
		}
		sb.WriteString(fmt.Sprintf("2026-08-13 05:00:00 %s line-%04d payload\n", level, i))
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sb.String()), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
}

func strptr(s string) *string { return &s }

// The regression this whole change exists for: five identical reads used to
// return 296 lines and then null four times, because the server advanced a
// per-client-IP cursor on every read. Every read must now return the same page.
func TestConfigGetLogsDoesNotDrainOnRead(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 1000)

	var first *models.Logs
	for i := 0; i < 5; i++ {
		got := getLogs(t, operations.GetLogsParams{Lines: strptr("1000")})

		if got.Logs == nil {
			t.Fatalf("read %d returned a null log list", i+1)
		}
		if i == 0 {
			first = got
			if len(got.Logs) != 1000 {
				t.Fatalf("read 1 returned %d lines, want 1000", len(got.Logs))
			}
			continue
		}
		if len(got.Logs) != len(first.Logs) {
			t.Fatalf("read %d returned %d lines, want %d", i+1, len(got.Logs), len(first.Logs))
		}
		if got.Logs[0] != first.Logs[0] {
			t.Fatalf("read %d newest line = %q, want %q", i+1, got.Logs[0], first.Logs[0])
		}
	}
}

// Server-side keyword filtering used to match only the freshly drained slice,
// so keyword=DBG came back empty. It must now match across the whole window.
func TestConfigGetLogsKeywordMatchesWholeWindow(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 1000)

	got := getLogs(t, operations.GetLogsParams{Lines: strptr("1000"), Keyword: strptr("DBG")})

	want := 334 // lines 0,3,..,999
	if len(got.Logs) != want {
		t.Fatalf("keyword=DBG returned %d lines, want %d", len(got.Logs), want)
	}
	if got.LogCount == nil || *got.LogCount != int64(want) {
		t.Errorf("log_count = %v, want %d", got.LogCount, want)
	}
	for _, line := range got.Logs {
		if !strings.Contains(line, "DBG") {
			t.Fatalf("unfiltered line leaked through: %q", line)
		}
	}
}

// The response must carry the pagination metadata the UI reads.
func TestConfigGetLogsReportsPaginationMetadata(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 1000)

	page := getLogs(t, operations.GetLogsParams{Lines: strptr("100")})

	if page.LogFile != "loxilbhost1.log" {
		t.Errorf("log_file = %q, want loxilbhost1.log", page.LogFile)
	}
	if page.LogCount == nil || *page.LogCount != 100 {
		t.Errorf("log_count = %v, want 100", page.LogCount)
	}
	if page.TotalSize == nil || *page.TotalSize <= 0 {
		t.Errorf("total_size = %v, want > 0", page.TotalSize)
	}
	if page.HasMore == nil || !*page.HasMore {
		t.Fatal("has_more = false with 900 older lines remaining")
	}
	if page.NextCursor == "" {
		t.Fatal("next_cursor empty while has_more is true")
	}

	// Following the cursor must hand back the next older page, not a repeat.
	next := getLogs(t, operations.GetLogsParams{Lines: strptr("100"), Cursor: strptr(page.NextCursor)})
	if len(next.Logs) != 100 {
		t.Fatalf("second page returned %d lines, want 100", len(next.Logs))
	}
	if next.Logs[0] == page.Logs[0] {
		t.Fatal("second page repeats the first page")
	}

	// Walking to the end must terminate and cover every line exactly once.
	seen := map[string]bool{}
	for _, l := range append(append([]string{}, page.Logs...), next.Logs...) {
		seen[l] = true
	}
	cursor := next.NextCursor
	for i := 0; cursor != ""; i++ {
		if i > 20 {
			t.Fatal("paging did not terminate")
		}
		p := getLogs(t, operations.GetLogsParams{Lines: strptr("100"), Cursor: strptr(cursor)})
		for _, l := range p.Logs {
			if seen[l] {
				t.Fatalf("line served twice: %q", l)
			}
			seen[l] = true
		}
		cursor = p.NextCursor
	}
	if len(seen) != 1000 {
		t.Errorf("paged over %d distinct lines, want 1000", len(seen))
	}
}

// A cursor minted against a file that has since been rotated away must restart
// from the newest lines rather than read from a meaningless offset.
func TestConfigGetLogsStaleCursorRestartsFromTail(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 500)

	stale := encodeCursor(LogCursor{Filename: "loxilbold.log", Offset: 999999})
	got := getLogs(t, operations.GetLogsParams{Lines: strptr("10"), Cursor: strptr(stale)})

	if len(got.Logs) != 10 {
		t.Fatalf("got %d lines, want 10", len(got.Logs))
	}
	if !strings.Contains(got.Logs[0], "line-0499") {
		t.Errorf("newest line = %q, want the tail of the current file", got.Logs[0])
	}
}

func TestConfigGetLogsRejectsBadInput(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 10)

	req, _ := http.NewRequest(http.MethodGet, "/netlox/v1/logs", nil)

	bad := ConfigGetLogs(operations.GetLogsParams{HTTPRequest: req, Cursor: strptr("!!!not-base64")}, nil)
	if _, ok := bad.(*operations.GetLogsBadRequest); !ok {
		t.Errorf("malformed cursor gave %T, want 400", bad)
	}

	bad = ConfigGetLogs(operations.GetLogsParams{HTTPRequest: req, File: strptr("../../etc/shadow")}, nil)
	if _, ok := bad.(*operations.GetLogsBadRequest); !ok {
		t.Errorf("traversal in ?file gave %T, want 400", bad)
	}
}

// bindLogsRequest runs the generated request binder over a real URL, so query
// parameters reach the handler exactly as they do in the running server.
func bindLogsRequest(t *testing.T, rawQuery string) operations.GetLogsParams {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "/netlox/v1/logs?"+rawQuery, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	params := operations.NewGetLogsParams()
	route := &middleware.MatchedRoute{}
	route.Formats = strfmt.Default
	if err := params.BindRequest(req, route); err != nil {
		t.Fatalf("bind %q: %v", rawQuery, err)
	}
	return params
}

// The cursor and file parameters have to survive the generated binder, not just
// the handler signature.
func TestGetLogsParamsBindQueryString(t *testing.T) {
	params := bindLogsRequest(t, "lines=250&level=ERR&keyword=bgp&cursor=Zm9v&file=loxilbdp.log")

	for _, tc := range []struct {
		name string
		got  *string
		want string
	}{
		{"lines", params.Lines, "250"},
		{"level", params.Level, "ERR"},
		{"keyword", params.Keyword, "bgp"},
		{"cursor", params.Cursor, "Zm9v"},
		{"file", params.File, "loxilbdp.log"},
	} {
		if tc.got == nil {
			t.Errorf("%s not bound", tc.name)
			continue
		}
		if *tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, *tc.got, tc.want)
		}
	}
}

// The measured contract gap: loxilb returned only "logs", where clients expect
// the pagination fields beside it.
func TestConfigGetLogsJSONShape(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 500)

	params := bindLogsRequest(t, "lines=100")
	payload := getLogs(t, params)

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"logs", "log_file", "log_count", "total_size", "has_more", "next_cursor", "scanned_bytes"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("response is missing %q; got keys %v", key, sortedKeys(decoded))
		}
	}

	// An empty page must serialize as [] rather than null: clients that guard
	// on a falsy log list would otherwise discard the pagination fields.
	// lines=1000 over a 500-line file reaches the start of the file, so
	// has_more is false here for the file, not merely for the filter.
	empty := getLogs(t, bindLogsRequest(t, "lines=1000&keyword=no-such-line-anywhere"))
	raw, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	if !strings.Contains(string(raw), `"logs":[]`) {
		t.Errorf("empty page serialized as %s, want \"logs\":[]", raw)
	}

	// The zero values must survive serialization too, so a client never has to
	// treat "absent" as a third state alongside true and false.
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	for _, key := range []string{"logs", "log_count", "total_size", "has_more", "scanned_bytes"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("empty page is missing %q; got keys %v", key, sortedKeys(decoded))
		}
	}
	if decoded["has_more"] != false {
		t.Errorf("has_more = %v, want false", decoded["has_more"])
	}
	if decoded["log_count"] != float64(0) {
		t.Errorf("log_count = %v, want 0", decoded["log_count"])
	}
}

//----------------------------------------------------------------------
// Filter-aware paging (patch request 1)
//----------------------------------------------------------------------

// writeSparseLogFile lays down lineCount lines of which only every nth carries
// the marker, so a filtered read has to scan past long stretches of noise.
func writeSparseLogFile(t *testing.T, dir, name string, lineCount, nth int, marker string) int {
	t.Helper()

	matches := 0
	var sb strings.Builder
	for i := 0; i < lineCount; i++ {
		if i%nth == 0 {
			matches++
			sb.WriteString(fmt.Sprintf("2026-08-13 05:00:00 %s line-%05d payload\n", marker, i))
			continue
		}
		sb.WriteString(fmt.Sprintf("2026-08-13 05:00:00 INFO line-%05d payload\n", i))
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sb.String()), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return matches
}

// The core of the request: a keyword that appears nowhere near the tail must
// still come back on the first request. The old code filtered one page-sized
// window, so this returned zero lines beside has_more: true and the client had
// to page the whole file to find them.
func TestCollectPageSearchesPastNonMatchingLines(t *testing.T) {
	dir := t.TempDir()
	writeSparseLogFile(t, dir, "loxilbhost1.log", 5000, 1000, "ERROR") // 5 matches, oldest first

	file, err := os.Open(filepath.Join(dir, "loxilbhost1.log"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()

	page, next, scanned := collectPage(file, fileSize(t, file), 10, "", "ERROR", maxFilterScanBytes)

	if len(page) != 5 {
		t.Fatalf("got %d matches, want 5", len(page))
	}
	for _, line := range page {
		if !strings.Contains(line, "ERROR") {
			t.Fatalf("non-matching line leaked through: %q", line)
		}
	}
	if !strings.Contains(page[0], "line-04000") {
		t.Errorf("newest match = %q, want line-04000", page[0])
	}
	if next != 0 {
		t.Errorf("next offset = %d, want 0 (search reached the start of the file)", next)
	}
	if scanned != fileSize(t, file) {
		t.Errorf("scanned = %d, want the whole file (%d)", scanned, fileSize(t, file))
	}
}

// Filling the page mid-scan must bound the next page by the last line handed
// out, so nothing between it and the read boundary is skipped.
func TestCollectPageFilteredPagingIsGapFree(t *testing.T) {
	dir := t.TempDir()
	want := writeSparseLogFile(t, dir, "loxilbhost1.log", 3000, 7, "ERROR")

	file, err := os.Open(filepath.Join(dir, "loxilbhost1.log"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()

	seen := map[string]bool{}
	pos := fileSize(t, file)
	for i := 0; ; i++ {
		if i > want+10 {
			t.Fatal("filtered paging did not terminate")
		}
		page, next, _ := collectPage(file, pos, 13, "", "ERROR", maxFilterScanBytes)
		for _, line := range page {
			if seen[line] {
				t.Fatalf("line served twice: %q", line)
			}
			seen[line] = true
		}
		if next <= 0 {
			break
		}
		if next >= pos {
			t.Fatalf("cursor did not advance backwards: %d -> %d", pos, next)
		}
		pos = next
	}

	if len(seen) != want {
		t.Errorf("paged over %d matches, want %d", len(seen), want)
	}
}

// A query that matches nothing must not run forever: the scan stops at the cap
// and hands back a cursor so the client can resume rather than restart.
func TestCollectPageStopsAtScanCap(t *testing.T) {
	dir := t.TempDir()
	writeSparseLogFile(t, dir, "loxilbhost1.log", 20000, 1000000, "ERROR") // no marker lines

	file, err := os.Open(filepath.Join(dir, "loxilbhost1.log"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer file.Close()

	size := fileSize(t, file)
	const cap = 50000
	if size <= cap {
		t.Fatalf("fixture too small to exercise the cap: %d bytes", size)
	}

	page, next, scanned := collectPage(file, size, 10, "", "nothing-matches-this", cap)

	if len(page) != 0 {
		t.Fatalf("got %d lines, want 0", len(page))
	}
	if next <= 0 {
		t.Fatal("hitting the scan cap must hand back a cursor to resume from")
	}
	if scanned < cap {
		t.Errorf("scanned = %d, want at least the cap (%d)", scanned, cap)
	}
	if scanned > size {
		t.Errorf("scanned = %d, more than the file holds (%d)", scanned, size)
	}
}

// End to end: one request, not fifty, and the reported metadata matches.
func TestConfigGetLogsFilteredSearchIsOneRoundTrip(t *testing.T) {
	dir := stageLogDirIn(t)
	matches := writeSparseLogFile(t, dir, "loxilbhost1.log", 5000, 500, "ERROR")

	got := getLogs(t, operations.GetLogsParams{Lines: strptr("100"), Keyword: strptr("ERROR")})

	if len(got.Logs) != matches {
		t.Fatalf("got %d lines, want all %d matches on the first request", len(got.Logs), matches)
	}
	if got.HasMore == nil || *got.HasMore {
		t.Error("has_more = true after the search reached the start of the file")
	}
	if got.LogCount == nil || *got.LogCount != int64(matches) {
		t.Errorf("log_count = %v, want %d", got.LogCount, matches)
	}
	if got.ScannedBytes == nil || got.TotalSize == nil {
		t.Fatal("scanned_bytes/total_size missing")
	}
	if *got.ScannedBytes != *got.TotalSize {
		t.Errorf("scanned_bytes = %d, want the whole file (%d)", *got.ScannedBytes, *got.TotalSize)
	}
}

// An unfiltered page reads only as far as it needs to, so scanned_bytes stays
// far below the file size — that is what makes it a usable progress signal.
func TestConfigGetLogsUnfilteredScanIsBounded(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 5000)

	got := getLogs(t, operations.GetLogsParams{Lines: strptr("10")})

	if got.ScannedBytes == nil || got.TotalSize == nil {
		t.Fatal("scanned_bytes/total_size missing")
	}
	if *got.ScannedBytes <= 0 || *got.ScannedBytes >= *got.TotalSize {
		t.Errorf("scanned_bytes = %d, want a small fraction of %d", *got.ScannedBytes, *got.TotalSize)
	}
}

//----------------------------------------------------------------------
// Compressed archives (patch request 2)
//----------------------------------------------------------------------

// writeGzipLog lays down a rotated, gzipped archive alongside the live log.
func writeGzipLog(t *testing.T, dir, name string, lineCount int) []string {
	t.Helper()

	lines := make([]string, 0, lineCount)
	var raw bytes.Buffer
	for i := 0; i < lineCount; i++ {
		line := fmt.Sprintf("2026-08-13 05:00:00 INFO archived-%04d payload", i)
		lines = append(lines, line)
		raw.WriteString(line + "\n")
	}

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), compressed.Bytes(), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	return lines
}

// A .gz archive used to be opened raw, so it came back as compressed bytes that
// parsed to zero lines and rendered as an empty table with no error.
func TestConfigGetLogsReadsGzipArchive(t *testing.T) {
	dir := stageLogDirIn(t)
	writeSparseLogFile(t, dir, "loxilbhost1.log", 10, 100, "ERROR")
	want := writeGzipLog(t, dir, "loxilbhost1-20260815-175401.656.log.gz", 500)

	got := getLogs(t, operations.GetLogsParams{
		Lines: strptr("50"),
		File:  strptr("loxilbhost1-20260815-175401.656.log.gz"),
	})

	if len(got.Logs) != 50 {
		t.Fatalf("got %d lines from the archive, want 50", len(got.Logs))
	}
	if got.Logs[0] != want[len(want)-1] {
		t.Errorf("newest line = %q, want %q", got.Logs[0], want[len(want)-1])
	}
	if got.TotalSize == nil || *got.TotalSize <= 0 {
		t.Error("total_size must report the uncompressed size the offsets address")
	}
	if got.HasMore == nil || !*got.HasMore {
		t.Fatal("has_more = false with 450 older lines in the archive")
	}

	// Paging within the archive must behave exactly as it does for a live file.
	next := getLogs(t, operations.GetLogsParams{
		Lines:  strptr("50"),
		File:   strptr("loxilbhost1-20260815-175401.656.log.gz"),
		Cursor: strptr(got.NextCursor),
	})
	if len(next.Logs) != 50 {
		t.Fatalf("second archive page returned %d lines, want 50", len(next.Logs))
	}
	if next.Logs[0] == got.Logs[0] {
		t.Fatal("second archive page repeats the first")
	}
}

// A truncated or non-gzip .gz must say so rather than serve bytes that parse to
// nothing.
func TestConfigGetLogsRejectsCorruptArchive(t *testing.T) {
	dir := stageLogDirIn(t)
	writeSparseLogFile(t, dir, "loxilbhost1.log", 10, 100, "ERROR")
	if err := os.WriteFile(filepath.Join(dir, "loxilbbroken.log.gz"), []byte("not gzip at all"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/netlox/v1/logs", nil)
	got := ConfigGetLogs(operations.GetLogsParams{HTTPRequest: req, File: strptr("loxilbbroken.log.gz")}, nil)

	bad, ok := got.(*operations.GetLogsBadRequest)
	if !ok {
		t.Fatalf("corrupt archive gave %T, want 400", got)
	}
	if bad.Payload == nil || bad.Payload.Message == "" {
		t.Error("400 carried no explanation")
	}
}

//----------------------------------------------------------------------
// Archive metadata (patch request 3)
//----------------------------------------------------------------------

// A bare list of names gives an operator nothing to choose on: loxilb958f3310.log
// carries no timestamp. Size and mtime must ride alongside.
func TestConfigGetLogArchivesCarriesMetadata(t *testing.T) {
	dir := stageLogDirIn(t)
	writeSparseLogFile(t, dir, "loxilbhost1.log", 100, 50, "ERROR")
	writeGzipLog(t, dir, "loxilbhost1-20260815-175401.656.log.gz", 200)

	req, _ := http.NewRequest(http.MethodGet, "/netlox/v1/log-archives", nil)
	res := ConfigGetLogArchives(operations.GetLogArchivesParams{HTTPRequest: req}, nil)

	ok, isOK := res.(*operations.GetLogArchivesOK)
	if !isOK {
		t.Fatalf("expected 200, got %T", res)
	}
	payload := ok.Payload

	if len(payload.Archives) == 0 {
		t.Fatal("archives list is empty; the existing field must stay populated")
	}
	if len(payload.ArchiveInfo) != len(payload.Archives) {
		t.Fatalf("archive_info has %d entries, archives has %d — they must line up positionally",
			len(payload.ArchiveInfo), len(payload.Archives))
	}
	for i, info := range payload.ArchiveInfo {
		if info.Name != payload.Archives[i] {
			t.Errorf("archive_info[%d].name = %q, want %q", i, info.Name, payload.Archives[i])
		}
		if info.SizeBytes <= 0 {
			t.Errorf("%s: size_bytes = %d, want > 0", info.Name, info.SizeBytes)
		}
		if time.Time(info.Modified).IsZero() {
			t.Errorf("%s: modified is zero", info.Name)
		}
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

//----------------------------------------------------------------------
// Filename handling
//
// These cover the loxilb side of the API, where the REST server may be running
// unauthenticated and as root. They are kept alongside the shared log tests so
// a future sync between the two trees cannot quietly drop them.
//----------------------------------------------------------------------

// resolveLogFile is the only thing standing between a client-supplied ?file=
// and an arbitrary read: the name is joined onto a server-side directory, so a
// traversal that slips through reads whatever the process can.
func TestResolveLogFileRejectsTraversal(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 5)

	bad := []string{
		"", "..", "../../etc/shadow", "loxilb/../../etc/shadow",
		"/etc/shadow", `..\..\windows\system32`, "sub/loxilb.log",
		"passwd", "loxilb.txt", "loxilb.log.gz.exe",
	}
	for _, name := range bad {
		if _, err := resolveLogFile(name); !errors.Is(err, errInvalidLogFilename) {
			t.Errorf("resolveLogFile(%q) err = %v, want errInvalidLogFilename", name, err)
		}
	}

	if _, err := resolveLogFile("loxilbhost1.log"); err != nil {
		t.Errorf("resolveLogFile(loxilbhost1.log) = %v, want nil", err)
	}
	// A well-formed name that is not on disk is a lookup miss, not a rejection.
	if _, err := resolveLogFile("loxilbabsent.log"); !errors.Is(err, errLogFileNotFound) {
		t.Errorf("resolveLogFile(absent) err = %v, want errLogFileNotFound", err)
	}
}

// The same check on the download endpoint, driven through the handler: this is
// the path that was demonstrated serving /etc/shadow.
func TestConfigGetLogArchivesFilenameRejectsTraversal(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 10)

	for _, name := range []string{"../../etc/shadow", "/etc/shadow", "..", "passwd", ""} {
		got := ConfigGetLogArchivesFilename(
			operations.GetLogArchivesFilenameParams{Filename: name}, nil)
		if _, ok := got.(*operations.GetLogsBadRequest); !ok {
			t.Errorf("filename %q gave %T, want 400", name, got)
		}
	}
}

// ?file= must be rejected before it reaches the filesystem, with 400 rather
// than a 500 that would distinguish "exists but unreadable" from "absent".
func TestConfigGetLogsRejectsTraversalFile(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 10)

	req, err := http.NewRequest(http.MethodGet, "/netlox/v1/logs", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for _, name := range []string{"../../etc/shadow", "/etc/shadow", "passwd"} {
		params := operations.GetLogsParams{HTTPRequest: req, File: strptr(name)}
		if got := ConfigGetLogs(params, nil); !isBadRequest(got) {
			t.Errorf("file %q gave %T, want 400", name, got)
		}
	}
}

func isBadRequest(r middleware.Responder) bool {
	_, ok := r.(*operations.GetLogsBadRequest)
	return ok
}

// The control-plane log wins over loxilbdp.log even when the data-plane log is
// newer and sorts first by name, so an operator opening /logs sees the log the
// rest of the UI talks about.
func TestConfigGetLogsPrefersControlPlaneLog(t *testing.T) {
	dir := stageLogDirIn(t)

	write := func(name string, age time.Duration) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("2026-08-13 05:00:00 INFO x\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}

	write("loxilbhost1.log", time.Hour)
	write("loxilbdp.log", time.Minute)
	write("unrelated.log", time.Minute)

	if got := getLogs(t, operations.GetLogsParams{}).LogFile; got != "loxilbhost1.log" {
		t.Errorf("log_file = %q, want loxilbhost1.log", got)
	}

	// With no control-plane log present, the data-plane log is the fallback.
	if err := os.Remove(filepath.Join(dir, "loxilbhost1.log")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := getLogs(t, operations.GetLogsParams{}).LogFile; got != "loxilbdp.log" {
		t.Errorf("log_file = %q, want loxilbdp.log", got)
	}
}

// ?lines= is a string in the spec, so anything can arrive in it. A value that
// does not parse must fall back to the default rather than to zero — which
// would answer with an empty page — and a huge value must be capped.
func TestConfigGetLogsBoundsLines(t *testing.T) {
	stageLogDir(t, "loxilbhost1.log", 300)

	for _, lines := range []string{"abc", "0", "-5", ""} {
		got := getLogs(t, operations.GetLogsParams{Lines: strptr(lines)})
		if *got.LogCount != defaultLogLines {
			t.Errorf("lines=%q gave log_count %d, want %d", lines, *got.LogCount, defaultLogLines)
		}
	}

	got := getLogs(t, operations.GetLogsParams{Lines: strptr("999999")})
	if *got.LogCount != 300 {
		t.Errorf("lines=999999 gave log_count %d, want 300 (whole file)", *got.LogCount)
	}
	if *got.HasMore {
		t.Error("has_more set after reading the whole file")
	}
}
