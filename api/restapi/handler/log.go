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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

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
	mu          sync.Mutex
	cursorMap   sync.Map // Tracks file cursor per client/session
)

// Function to extract client IP address from the request
func getClientIP(r *http.Request) string {
	// Try to get the IP address from the X-Forwarded-For header
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// The X-Forwarded-For header can contain multiple IP addresses, the first one is the client's IP
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}

	// If the X-Forwarded-For header is not set, use the remote address
	ip = r.RemoteAddr
	if ip != "" {
		// The remote address can contain the port, so we need to remove it
		if strings.Contains(ip, ":") {
			ip = strings.Split(ip, ":")[0]
		}
		return ip
	}

	return ""
}

// Reads the next N lines starting from a given cursor position
func readNextLines(file *os.File, startPos int64, numLines int) ([]string, int64) {
	bufferSize := 4096
	buffer := make([]byte, bufferSize)

	var lines []string
	var line string
	currentPos := startPos

	file.Seek(startPos, os.SEEK_SET) // Start reading from the stored cursor position

	for len(lines) < numLines {
		n, err := file.Read(buffer)
		if err != nil {
			break
		}

		for i := 0; i < n; i++ {
			if buffer[i] == '\n' {
				lines = append(lines, strings.TrimSpace(line))
				line = ""

				if len(lines) >= numLines {
					currentPos += int64(i + 1)
					break
				}
			} else {
				line += string(buffer[i])
			}
		}
		currentPos += int64(n)
	}

	if line != "" && len(lines) < numLines {
		lines = append(lines, strings.TrimSpace(line))
	}

	return lines, currentPos
}

// Filters logs based on level and keyword
func filterLogs(lines []string, level, keyword string) []string {
	var filtered []string
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

// Fetch logs using cursor
func ConfigGetLogs(params operations.GetLogsParams, principal interface{}) middleware.Responder {
	var result models.Logs
	clientID := getClientIP(params.HTTPRequest)

	lines := 100 // Default to 100 lines
	if params.Lines != nil {
		lines, _ = strconv.Atoi(*params.Lines)
	}

	// Find the log file with the random UUID in the name
	files, err := os.ReadDir(logFilePath)
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to read log directory"})
	}

	var logFile string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), logFileKey) && strings.HasSuffix(file.Name(), ".log") {
			logFile = filepath.Join(logFilePath, file.Name())
			break
		}
	}

	if logFile == "" {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Log file not found"})
	}

	file, err := os.Open(logFile)
	if err != nil {
		return operations.NewGetLogsInternalServerError().WithPayload(&models.Error{Message: "Failed to open log file"})
	}
	defer file.Close()

	// Get or initialize the cursor for this client
	startPos := int64(0)
	if val, ok := cursorMap.Load(clientID); ok {
		startPos = val.(int64)
	}

	// Read the next batch of lines
	nextLines, nextCursor := readNextLines(file, startPos, lines)

	// Update the cursor for this client
	cursorMap.Store(clientID, nextCursor)

	// Apply filtering if required
	level := derefString(params.Level)
	keyword := derefString(params.Keyword)
	filteredLines := filterLogs(nextLines, level, keyword)

	result.Logs = filteredLines
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
		if !file.IsDir() && (strings.HasPrefix(file.Name(), "loxilb") && (strings.HasSuffix(file.Name(), ".log") || strings.HasSuffix(file.Name(), ".log.gz"))) {
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
		w.WriteHeader(http.StatusOK)
		bytesCopied, err := io.Copy(w, file)
		if err != nil {
			tk.LogIt(tk.LogError, "Failed to copy file content: %s, error: %v\n", filePath, err)
		} else {
			tk.LogIt(tk.LogDebug, "Successfully copied %d bytes from file: %s\n", bytesCopied, filePath)
		}
	})
}
