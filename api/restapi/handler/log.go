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
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/loxilb-io/loxilb/api/models"
	"github.com/loxilb-io/loxilb/api/restapi/operations"
	tk "github.com/loxilb-io/loxilib"
)

var (
	logFilePath = "/var/log/"
	logFileKey  = "loxilb"
	archivePath = "/var/log/" // Path where rotated logs are stored
)

const (
	// dpLogFile is written by the eBPF data-plane C library, which keeps its
	// own FILE*. It is only served when no control-plane log exists.
	dpLogFile = "loxilbdp.log"

	defaultLogLines = 100
	maxLogLines     = 10000

	minLogReadChunk = 4096
	maxLogReadChunk = 4 << 20
)

// LogCursor addresses a position in a specific log file. It is handed to the
// client base64-encoded and handed back verbatim, so the server keeps no
// per-client paging state: two operators (or two browser tabs) paging the same
// log cannot disturb each other's position.
type LogCursor struct {
	Filename string    `json:"filename"`
	Offset   int64     `json:"offset"`
	ModTime  time.Time `json:"mod_time"`
	FileSize int64     `json:"file_size"`
}

// encodeCursor creates a base64 encoded cursor string
func encodeCursor(cursor LogCursor) string {
	cursorStr := fmt.Sprintf("%s:%d:%d:%d",
		cursor.Filename,
		cursor.Offset,
		cursor.ModTime.Unix(),
		cursor.FileSize)
	return base64.StdEncoding.EncodeToString([]byte(cursorStr))
}

// decodeCursor parses a base64 encoded cursor string
func decodeCursor(cursorStr string) (*LogCursor, error) {
	if cursorStr == "" {
		return nil, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cursorStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor format")
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid cursor format")
	}

	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid offset in cursor")
	}

	modTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp in cursor")
	}

	fileSize, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid file size in cursor")
	}

	return &LogCursor{
		Filename: parts[0],
		Offset:   offset,
		ModTime:  time.Unix(modTime, 0),
		FileSize: fileSize,
	}, nil
}

// cursorValid reports whether a cursor still addresses the file it was minted
// against. Growth is expected and harmless: the log is appended to while the
// operator reads it, and paging runs backwards, so bytes added at the end never
// move an older offset. A file that has shrunk past the cursor was rotated or
// truncated and the offset now points into unrelated content.
//
// The recorded mtime is deliberately not compared. An active log file's mtime
// changes between any two requests, so requiring it to match would invalidate
// every cursor on a busy system and silently restart paging from the tail.
func cursorValid(cursor *LogCursor, filename string, size int64) bool {
	if cursor == nil {
		return true // No cursor means start from the newest lines
	}
	return cursor.Filename == filename && size >= cursor.Offset
}

// readChunkFor sizes the backwards read so a typical page is satisfied by one
// or two syscalls instead of hundreds of 4 KiB steps.
func readChunkFor(numLines int) int64 {
	chunk := int64(minLogReadChunk)
	if want := int64(numLines) * 256; want > chunk {
		chunk = want
	}
	if chunk > maxLogReadChunk {
		chunk = maxLogReadChunk
	}
	return chunk
}

// readLinesBefore reads up to numLines whole lines ending at endPos and returns
// them newest-first, along with the absolute offset at which the oldest
// returned line starts. That offset is the cursor for the next, older page; a
// zero offset means the beginning of the file has been reached.
//
// Reading backwards is what lets a page be re-requested without consuming it:
// the offset is derived from the file, never from what a client read before.
func readLinesBefore(file *os.File, endPos int64, numLines int) ([]string, int64) {
	if endPos <= 0 || numLines <= 0 {
		return nil, 0
	}

	chunk := readChunkFor(numLines)

	var content []byte
	start := endPos // absolute offset of content[0]

	for start > 0 {
		n := chunk
		if start < n {
			n = start
		}
		start -= n

		buf := make([]byte, n)
		if _, err := file.ReadAt(buf, start); err != nil {
			return nil, 0
		}
		content = append(buf, content...)

		// One newline more than the number of lines wanted proves the oldest
		// candidate line is whole rather than clipped by the chunk boundary.
		if bytes.Count(content, []byte{'\n'}) > numLines {
			break
		}
	}

	// When the scan stopped short of the file start, the leading fragment is
	// the tail of a line belonging to an older page — drop it.
	scanFrom := 0
	if start > 0 {
		nl := bytes.IndexByte(content, '\n')
		if nl < 0 {
			return nil, start
		}
		scanFrom = nl + 1
	}

	type logLine struct {
		offset int64
		text   string
	}

	var found []logLine
	for i := scanFrom; i < len(content); {
		end := len(content)
		nl := bytes.IndexByte(content[i:], '\n')
		if nl >= 0 {
			end = i + nl
		}
		if text := strings.TrimSpace(string(content[i:end])); text != "" {
			found = append(found, logLine{offset: start + int64(i), text: text})
		}
		if nl < 0 {
			break
		}
		i = end + 1
	}

	if len(found) == 0 {
		return nil, start
	}
	if len(found) > numLines {
		found = found[len(found)-numLines:]
	}

	lines := make([]string, 0, len(found))
	for i := len(found) - 1; i >= 0; i-- {
		lines = append(lines, found[i].text)
	}

	// The oldest line handed out this page bounds the next one.
	return lines, found[0].offset
}

// Filters logs based on level and keyword
func filterLogs(lines []string, level, keyword string) []string {
	// Never nil: a page that filters down to nothing must still serialize as
	// "logs": [] so clients can read the pagination fields beside it.
	filtered := []string{}
	for _, line := range lines {
		if (level == "" || strings.Contains(line, level)) &&
			(keyword == "" || strings.Contains(line, keyword)) {
			filtered = append(filtered, line) // No additional quotes
		}
	}
	return filtered
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// isLogFileName reports whether name is a log file this API may serve. The
// prefix and suffix checks already exclude a path, but separators and parent
// references are rejected explicitly so the intent survives future edits.
func isLogFileName(name string, allowGzip bool) bool {
	if name == "" || name != filepath.Base(name) {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	if !strings.HasPrefix(name, logFileKey) {
		return false
	}
	return strings.HasSuffix(name, ".log") || (allowGzip && strings.HasSuffix(name, ".log.gz"))
}

// currentLogFile picks the file the endpoint reads when the caller names none:
// the most recently written loxilb<hostname>.log, falling back to the
// data-plane log only when no control-plane log exists. Directory order alone
// is not enough — os.ReadDir sorts by name, so whether loxilbdp.log or
// loxilb<hostname>.log came first depended on the hostname string.
func currentLogFile() (string, error) {
	files, err := os.ReadDir(logFilePath)
	if err != nil {
		return "", err
	}

	var (
		name        string
		latest      time.Time
		fallback    string
		fallbackMod time.Time
	)

	for _, file := range files {
		if file.IsDir() || !isLogFileName(file.Name(), false) {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue // Skip entries we cannot stat
		}

		if file.Name() == dpLogFile {
			if fallback == "" || info.ModTime().After(fallbackMod) {
				fallback, fallbackMod = file.Name(), info.ModTime()
			}
			continue
		}
		if name == "" || info.ModTime().After(latest) {
			name, latest = file.Name(), info.ModTime()
		}
	}

	if name == "" {
		name = fallback
	}
	return name, nil
}

// Fetch logs using a stateless cursor
func ConfigGetLogs(params operations.GetLogsParams, principal interface{}) middleware.Responder {
	lines := defaultLogLines
	if params.Lines != nil {
		if n, err := strconv.Atoi(*params.Lines); err == nil && n > 0 {
			lines = n
		}
	}
	if lines > maxLogLines {
		lines = maxLogLines
	}

	cursor, err := decodeCursor(derefString(params.Cursor))
	if err != nil {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid cursor format"})
	}

	logFileName := derefString(params.File)
	if logFileName != "" {
		if !isLogFileName(logFileName, false) {
			return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid log file name"})
		}
	} else if logFileName, err = currentLogFile(); err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to read log directory"})
	}

	if logFileName == "" {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Log file not found"})
	}

	file, err := os.Open(filepath.Join(logFilePath, logFileName))
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to open log file"})
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to get file info"})
	}

	// Without a usable cursor, serve the newest lines. A cursor for another
	// file, or for a file that has since rotated, restarts from the tail
	// rather than reading from a meaningless offset.
	endPos := fileInfo.Size()
	if cursorValid(cursor, logFileName, fileInfo.Size()) && cursor != nil {
		endPos = cursor.Offset
	}

	pageLines, nextOffset := readLinesBefore(file, endPos, lines)

	// Apply filtering if required
	level := derefString(params.Level)
	keyword := derefString(params.Keyword)
	filteredLines := filterLogs(pageLines, level, keyword)

	// Older lines remain whenever this page did not reach the file start.
	hasMore := nextOffset > 0
	logCount := int64(len(filteredLines))
	totalSize := fileInfo.Size()

	result := models.Logs{
		Logs:      filteredLines,
		LogFile:   logFileName,
		LogCount:  &logCount,
		TotalSize: &totalSize,
		HasMore:   &hasMore,
	}
	if hasMore {
		result.NextCursor = encodeCursor(LogCursor{
			Filename: logFileName,
			Offset:   nextOffset,
			ModTime:  fileInfo.ModTime(),
			FileSize: fileInfo.Size(),
		})
	}

	return operations.NewGetLogsOK().WithPayload(&result)
}

// API to list available log archives
func ConfigGetLogArchives(params operations.GetLogArchivesParams, principal interface{}) middleware.Responder {
	var result models.LogArchives

	files, err := os.ReadDir(archivePath)
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to list log archives"})
	}

	var archives []string
	for _, file := range files {
		if !file.IsDir() && isLogFileName(file.Name(), true) {
			archives = append(archives, file.Name())
		}
	}

	result.Archives = archives
	return operations.NewGetLogArchivesOK().WithPayload(&result)
}

// API to download a specific log archive
func ConfigGetLogArchivesFilename(params operations.GetLogArchivesFilenameParams, principal interface{}) middleware.Responder {
	filename := params.Filename

	if filename == "" {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Filename is required"})
	}

	// The name is joined onto a server-side directory, so it has to be a bare
	// log file name. Without this check "../../etc/shadow" reads any file the
	// process can — and loxilb runs as root with the REST API unauthenticated
	// unless a user service is enabled.
	if !isLogFileName(filename, true) {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid log file name"})
	}

	filePath := filepath.Join(archivePath, filename)
	file, err := os.Open(filePath)
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "File not found"})
	}

	// Check if the file is empty
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to get file info"})
	}
	if fileInfo.Size() == 0 {
		file.Close()
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "File is empty"})
	}

	// Set headers and send the file
	return middleware.ResponderFunc(func(w http.ResponseWriter, _ runtime.Producer) {
		defer file.Close()
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		w.WriteHeader(http.StatusOK)
		bytesCopied, err := io.Copy(w, file)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to copy file content: %s, error: %v\n", filePath, err)
		} else {
			tk.LogIt(tk.LogDebug, "Successfully copied %d bytes from file: %s\n", bytesCopied, filePath)
		}
	})
}
