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
	"encoding/base64"
	"errors"
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
	"github.com/go-openapi/strfmt"
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
	// defaultLogLines is served when ?lines= is absent or unparseable;
	// maxLogLines caps it, so a client cannot ask the server to materialise an
	// unbounded page.
	defaultLogLines = 100
	maxLogLines     = 10000

	minLogReadChunk = 4096
	maxLogReadChunk = 4 << 20

	// maxFilterScanBytes bounds how far back a single filtered request scans,
	// so a keyword that matches nothing cannot pin a CPU on a multi-gigabyte
	// log. Reaching the cap is reported as has_more plus a cursor, so the
	// client resumes where the scan stopped instead of starting over.
	maxFilterScanBytes = 32 << 20

	// filterScanBatchLines is the read granularity while searching. Reading
	// more lines per pass than the page needs keeps a sparse keyword from
	// costing one read per matching line.
	filterScanBatchLines = 1000

	// maxArchiveInflateBytes bounds the in-memory expansion of a rotated
	// .log.gz. Backwards paging needs random access, which a compressed
	// stream cannot offer, so the archive is inflated first; a log that
	// expands beyond this is refused rather than silently truncated.
	maxArchiveInflateBytes = 64 << 20
)

var (
	errInvalidLogFilename = errors.New("invalid log file name")
	errLogFileNotFound    = errors.New("log file not found")
	errArchiveTooLarge    = errors.New("archive too large to read; download it from /log-archives/{filename} instead")
	errArchiveCorrupt     = errors.New("archive is not a valid gzip stream")
)

// LogCursor represents a stateless cursor for log pagination
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
// changes between any two requests, so requiring it to match invalidated every
// cursor on a busy system and silently restarted paging from the tail — which
// made "load more" return the newest page over and over.
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

// logLine is one log record together with the absolute offset at which it
// starts, so a page can be bounded by the exact line it ended on rather than
// by the read window that happened to contain it.
type logLine struct {
	offset int64
	text   string
}

// readLinesBefore reads up to numLines whole lines ending at endPos and returns
// them newest-first, along with the absolute offset at which the oldest
// returned line starts. That offset is the cursor for the next, older page; a
// zero offset means the beginning of the file has been reached.
//
// Paging runs backwards in both directions of travel. The previous code read
// the first page backwards from EOF but then followed the cursor *forwards*,
// which re-served the page just returned, in the opposite order, and could not
// reach older history at all.
func readLinesBefore(src io.ReaderAt, endPos int64, numLines int) ([]string, int64) {
	records, next := readRecordsBefore(src, endPos, numLines)
	if len(records) == 0 {
		return nil, next
	}
	lines := make([]string, 0, len(records))
	for _, rec := range records {
		lines = append(lines, rec.text)
	}
	return lines, next
}

// readRecordsBefore is readLinesBefore with the per-line offsets retained. It
// takes an io.ReaderAt rather than an *os.File so an inflated .log.gz archive
// can be paged from memory by the same code that pages a live file.
func readRecordsBefore(src io.ReaderAt, endPos int64, numLines int) ([]logLine, int64) {
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
		if _, err := src.ReadAt(buf, start); err != nil {
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

	// Hand back newest-first, which is the order the page is served in.
	records := make([]logLine, 0, len(found))
	for i := len(found) - 1; i >= 0; i-- {
		records = append(records, found[i])
	}

	// The oldest line handed out this page bounds the next one. Deriving it by
	// byte offset rather than by searching the buffer for the line's text also
	// removes a mis-seek when an identical line appears earlier in the window.
	return records, found[0].offset
}

// lineMatches reports whether a line satisfies the level and keyword filters.
// Both are plain substring tests, which is what the level filter has always
// been: levels are not parsed out of the line.
func lineMatches(line, level, keyword string) bool {
	return (level == "" || strings.Contains(line, level)) &&
		(keyword == "" || strings.Contains(line, keyword))
}

// Filters logs based on level and keyword
func filterLogs(lines []string, level, keyword string) []string {
	// Never nil: a page that filters down to nothing must still serialize as
	// "logs": [] so clients can read the pagination fields beside it.
	filtered := []string{}
	for _, line := range lines {
		if lineMatches(line, level, keyword) {
			filtered = append(filtered, line) // No additional quotes
		}
	}
	return filtered
}

// collectPage gathers up to numLines lines matching level and keyword, reading
// backwards from endPos. It returns the page newest-first, the offset the next
// (older) page should read back from — zero once the start of the file has been
// reached — and the number of bytes examined.
//
// Filtering used to be applied to a single page-sized window, which made
// has_more mean "more bytes exist" rather than "more matches exist": a keyword
// absent from the newest page came back as zero lines beside has_more: true,
// and a client had to walk the entire file one page at a time to discover where
// the matches were. The search now runs server-side and stops on one of three
// conditions — the page is full, the start of the file is reached, or the scan
// cap is hit — so a filtered request answers in one round trip instead of N.
//
// scanCap bounds the backwards search in bytes; it is a parameter rather than a
// constant so the cap behaviour is testable without writing a 32 MiB fixture.
func collectPage(src io.ReaderAt, endPos int64, numLines int, level, keyword string, scanCap int64) ([]string, int64, int64) {
	page := []string{}
	if numLines <= 0 || endPos <= 0 {
		return page, 0, 0
	}

	filtering := level != "" || keyword != ""
	batch := numLines
	if filtering && batch < filterScanBatchLines {
		batch = filterScanBatchLines
	}

	pos := endPos
	for pos > 0 {
		records, stop := readRecordsBefore(src, pos, batch)
		for _, rec := range records { // newest-first
			if !lineMatches(rec.text, level, keyword) {
				continue
			}
			page = append(page, rec.text)
			if len(page) == numLines {
				// Resume immediately older than the last line handed out, so
				// the non-matching lines between it and the batch boundary are
				// re-examined by the next page rather than skipped.
				return page, rec.offset, endPos - rec.offset
			}
		}

		if stop >= pos {
			// No backwards progress: treat as the start of the file rather
			// than spin.
			pos = 0
			break
		}
		pos = stop

		// Unfiltered, one batch is the whole page by construction.
		if !filtering || endPos-pos >= scanCap {
			break
		}
	}

	return page, pos, endPos - pos
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getAvailableLogFiles returns a list of available log files with their info
// Prioritizes loxilb{hostname}.log over loxilbdp.log
func getAvailableLogFiles() ([]map[string]interface{}, error) {
	files, err := os.ReadDir(logFilePath)
	if err != nil {
		return nil, err
	}

	var hostnameLogFiles []map[string]interface{}
	var dpLogFiles []map[string]interface{}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), logFileKey) && strings.HasSuffix(file.Name(), ".log") {
			fullPath := filepath.Join(logFilePath, file.Name())
			fileInfo, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			logFileInfo := map[string]interface{}{
				"filename":     file.Name(),
				"size":         fileInfo.Size(),
				"modified":     fileInfo.ModTime().Unix(),
				"modified_str": fileInfo.ModTime().Format("2006-01-02 15:04:05"),
			}

			// Separate loxilb{hostname}.log from loxilbdp.log
			if file.Name() == "loxilbdp.log" {
				dpLogFiles = append(dpLogFiles, logFileInfo)
			} else {
				hostnameLogFiles = append(hostnameLogFiles, logFileInfo)
			}
		}
	}

	// Sort hostname log files by modification time (newest first)
	for i := 0; i < len(hostnameLogFiles)-1; i++ {
		for j := i + 1; j < len(hostnameLogFiles); j++ {
			if hostnameLogFiles[i]["modified"].(int64) < hostnameLogFiles[j]["modified"].(int64) {
				hostnameLogFiles[i], hostnameLogFiles[j] = hostnameLogFiles[j], hostnameLogFiles[i]
			}
		}
	}

	// Sort dp log files by modification time (newest first)
	for i := 0; i < len(dpLogFiles)-1; i++ {
		for j := i + 1; j < len(dpLogFiles); j++ {
			if dpLogFiles[i]["modified"].(int64) < dpLogFiles[j]["modified"].(int64) {
				dpLogFiles[i], dpLogFiles[j] = dpLogFiles[j], dpLogFiles[i]
			}
		}
	}

	// Return hostname logs first, then dp logs (priority order)
	allLogFiles := append(hostnameLogFiles, dpLogFiles...)
	return allLogFiles, nil
}

// resolveLogFile turns a client-supplied ?file= into a path on disk. Rotated
// logs are moved into the archive directories, so a name that is valid but
// absent from the live log directory is looked for there too — otherwise every
// rotated file the /log-archives listing advertises would 404 when opened.
func resolveLogFile(name string) (string, error) {
	// Traversal first: a name containing a separator must never be joined to
	// a directory, whatever else is wrong with it.
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", errInvalidLogFilename
	}
	if !strings.HasPrefix(name, logFileKey) ||
		(!strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".log.gz")) {
		return "", errInvalidLogFilename
	}

	dirs := append([]string{logFilePath}, archiveDirs()...)
	for _, dir := range dirs {
		path := filepath.Join(dir, filepath.Base(name))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", errLogFileNotFound
}

// inflateArchive decompresses a rotated .log.gz into memory so it can be paged
// backwards. A compressed stream cannot be seeked, and the alternative — the
// handler opening it raw — returned compressed bytes that parsed to zero lines
// and rendered as an empty table with no error.
//
// The read is bounded: an archive that expands past the cap is refused with a
// message pointing at the download endpoint, rather than truncated into a
// half-line or allowed to exhaust memory.
func inflateArchive(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, errArchiveCorrupt
	}
	defer zr.Close()

	// One byte past the cap distinguishes "exactly at the cap" from "over it".
	data, err := io.ReadAll(io.LimitReader(zr, maxArchiveInflateBytes+1))
	if err != nil {
		return nil, errArchiveCorrupt
	}
	if int64(len(data)) > maxArchiveInflateBytes {
		return nil, errArchiveTooLarge
	}
	return data, nil
}

// Fetch logs using stateless cursor
func ConfigGetLogs(params operations.GetLogsParams, principal interface{}) middleware.Responder {
	var result models.Logs

	// A non-numeric, zero or negative ?lines= falls back to the default rather
	// than to zero, which would otherwise answer every such request with an
	// empty page and has_more: false.
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

	// Check if a specific log file is requested
	requestedFile := derefString(params.File)

	var logFile string
	var currentLogFilename string

	if requestedFile != "" {
		path, err := resolveLogFile(requestedFile)
		switch {
		case errors.Is(err, errInvalidLogFilename):
			return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid log file name"})
		case err != nil:
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Requested log file not found"})
		}
		logFile = path
		currentLogFilename = requestedFile
	} else {
		// Find the current log file with specific priority:
		// 1st Priority: loxilb{hostname}.log (most recent)
		// 2nd Priority: loxilbdp.log (fallback)
		files, err := os.ReadDir(logFilePath)
		if err != nil {
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to read log directory"})
		}

		var latestModTime time.Time
		var fallbackFile string
		var fallbackFilename string
		var fallbackModTime time.Time

		// Find log files with preference: loxilb{hostname}.log > loxilbdp.log
		for _, file := range files {
			if strings.HasPrefix(file.Name(), logFileKey) && strings.HasSuffix(file.Name(), ".log") {
				fullPath := filepath.Join(logFilePath, file.Name())
				fileInfo, err := os.Stat(fullPath)
				if err != nil {
					continue // Skip files we can't stat
				}

				fileName := file.Name()

				// Priority 1: loxilb{hostname}.log (not loxilbdp.log)
				if fileName != "loxilbdp.log" && strings.HasPrefix(fileName, "loxilb") && strings.HasSuffix(fileName, ".log") {
					// This is a loxilb{hostname}.log file - preferred
					if logFile == "" || fileInfo.ModTime().After(latestModTime) {
						logFile = fullPath
						currentLogFilename = fileName
						latestModTime = fileInfo.ModTime()
					}
				} else if fileName == "loxilbdp.log" {
					// Priority 2: loxilbdp.log as fallback
					if fallbackFile == "" || fileInfo.ModTime().After(fallbackModTime) {
						fallbackFile = fullPath
						fallbackFilename = fileName
						fallbackModTime = fileInfo.ModTime()
					}
				}
			}
		}

		// If no loxilb{hostname}.log found, use loxilbdp.log as fallback
		if logFile == "" && fallbackFile != "" {
			logFile = fallbackFile
			currentLogFilename = fallbackFilename
		}
	}

	if logFile == "" {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Log file not found"})
	}

	// Rotated archives are gzipped. Backwards paging needs random access, so a
	// .gz is inflated up front and paged from memory; everything downstream is
	// offset arithmetic over an io.ReaderAt and does not care which it has.
	var (
		src     io.ReaderAt
		srcSize int64
		modTime time.Time
	)

	if strings.HasSuffix(currentLogFilename, ".gz") {
		info, err := os.Stat(logFile)
		if err != nil {
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to get file info"})
		}
		data, err := inflateArchive(logFile)
		switch {
		case errors.Is(err, errArchiveTooLarge), errors.Is(err, errArchiveCorrupt):
			return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: err.Error()})
		case err != nil:
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to read log archive"})
		}
		src = bytes.NewReader(data)
		srcSize = int64(len(data))
		modTime = info.ModTime()
	} else {
		file, err := os.Open(logFile)
		if err != nil {
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to open log file"})
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to get file info"})
		}
		src = file
		srcSize = fileInfo.Size()
		modTime = fileInfo.ModTime()
	}

	// Without a usable cursor, serve the newest lines. A cursor for another
	// file, or for a file that has since rotated, restarts from the tail
	// rather than reading from a meaningless offset.
	endPos := srcSize
	if cursor != nil && cursorValid(cursor, currentLogFilename, srcSize) {
		endPos = cursor.Offset
	}

	// Collect a page of matching lines. Filtering happens inside the backwards
	// scan, not on the page it returns, so has_more reflects the search rather
	// than the read window.
	level := derefString(params.Level)
	keyword := derefString(params.Keyword)
	pageLines, nextOffset, scanned := collectPage(src, endPos, lines, level, keyword, maxFilterScanBytes)

	// Older lines remain whenever this page did not reach the file start.
	hasMore := nextOffset > 0
	logCount := int64(len(pageLines))
	totalSize := srcSize
	scannedBytes := scanned

	result.Logs = pageLines
	result.LogFile = currentLogFilename
	result.LogCount = &logCount
	result.TotalSize = &totalSize
	result.HasMore = &hasMore
	result.ScannedBytes = &scannedBytes
	if hasMore {
		result.NextCursor = encodeCursor(LogCursor{
			Filename: currentLogFilename,
			Offset:   nextOffset,
			ModTime:  modTime,
			FileSize: srcSize,
		})
	}

	return operations.NewGetLogsOK().WithPayload(&result)
}

//----------------------------------------------
// Log File Management
//----------------------------------------------

// API to list available active log files (not archives)
func ConfigGetLogFiles(params operations.GetLogArchivesParams, principal interface{}) middleware.Responder {
	logFiles, err := getAvailableLogFiles()
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to list log files"})
	}

	// Use the same response structure as archives for now
	var result models.LogArchives
	var fileNames []string
	var info []*models.LogArchiveInfo

	for _, logFile := range logFiles {
		name := logFile["filename"].(string)
		fileNames = append(fileNames, name)
		info = append(info, &models.LogArchiveInfo{
			Name:      name,
			SizeBytes: logFile["size"].(int64),
			Modified:  strfmt.DateTime(time.Unix(logFile["modified"].(int64), 0).UTC()),
		})
	}

	result.Archives = fileNames
	result.ArchiveInfo = info
	return operations.NewGetLogArchivesOK().WithPayload(&result)
}

//----------------------------------------------
// Log Archives
//----------------------------------------------

// archiveDirs are scanned for rotated logs: the classic tk.LogIt location
// and the structured-log directory (pkg/loxilog), whose files rotate there.
func archiveDirs() []string {
	return []string{archivePath, filepath.Join(archivePath, "loxilb")}
}

// API to list available log archives
func ConfigGetLogArchives(params operations.GetLogArchivesParams, principal interface{}) middleware.Responder {
	var result models.LogArchives

	seen := map[string]bool{}
	// Never nil, for the same reason "logs" is never nil: neither field carries
	// omitempty, so a nil slice serializes as null and a client that iterates
	// the list has to special-case it. An empty archive directory is an empty
	// list, not an absent one.
	archives := []string{}
	info := []*models.LogArchiveInfo{}
	listed := false
	for _, dir := range archiveDirs() {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		listed = true
		for _, file := range files {
			name := file.Name()
			if file.IsDir() || seen[name] {
				continue
			}
			if strings.HasPrefix(name, "loxilb") && (strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".log.gz")) {
				seen[name] = true
				archives = append(archives, name)
				info = append(info, archiveInfoFor(name, file))
			}
		}
	}
	if !listed {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to list log archives"})
	}

	result.Archives = archives
	result.ArchiveInfo = info
	return operations.NewGetLogArchivesOK().WithPayload(&result)
}

// archiveInfoFor describes one archive. Size and mtime are what an operator
// actually chooses on: a name like loxilb958f33103408.log carries no timestamp,
// so a bare list of names offers nothing to pick between. Metadata that cannot
// be stat'd is left zero rather than dropping the entry — the name still has to
// line up positionally with the archives array.
func archiveInfoFor(name string, entry os.DirEntry) *models.LogArchiveInfo {
	out := &models.LogArchiveInfo{Name: name}
	stat, err := entry.Info()
	if err != nil {
		return out
	}
	out.SizeBytes = stat.Size()
	out.Modified = strfmt.DateTime(stat.ModTime().UTC())
	return out
}

// API to download a specific log archive
func ConfigGetLogArchivesFilename(params operations.GetLogArchivesFilenameParams, principal interface{}) middleware.Responder {
	filename := params.Filename

	if filename == "" {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Filename is required"})
	}

	// Security: Prevent path traversal attacks
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid filename"})
	}

	// Validate filename pattern
	if !strings.HasPrefix(filename, "loxilb") || (!strings.HasSuffix(filename, ".log") && !strings.HasSuffix(filename, ".log.gz")) {
		return operations.NewGetLogsBadRequest().WithPayload(&models.Error{Message: "Invalid log file format"})
	}

	// Resolve across the archive directories (filename is already
	// traversal-checked above; Base is belt-and-braces).
	var file *os.File
	var err error
	for _, dir := range archiveDirs() {
		file, err = os.Open(filepath.Join(dir, filepath.Base(filename)))
		if err == nil {
			break
		}
	}
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
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		w.WriteHeader(http.StatusOK)
		bytesCopied, err := io.Copy(w, file)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to copy file content: %s, error: %v\n", file.Name(), err)
		} else {
			tk.LogIt(tk.LogDebug, "Successfully copied %d bytes from file: %s\n", bytesCopied, file.Name())
		}
	})
}
