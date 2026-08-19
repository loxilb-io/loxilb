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

// Package logrotate provides size-based log rotation with gzip compression
// and count/age retention. It has no dependencies outside the standard
// library and covers two cases:
//
//   - Writer: an io.WriteCloser for logs written by this process
//     (the tk.LogIt file).
//   - Sweep/StartSweeper: copy-truncate rotation for a file whose FILE*
//     is held by foreign code (the eBPF data-plane C library appends to
//     /var/log/loxilbdp.log and keeps the handle for the process lifetime).
//
// Rotated files are named <base>-<UTC timestamp>.<ext>[.gz] in the same
// directory, so backups of /var/log/loxilbXX.log stay visible to the
// GET /log-archives REST API (prefix "loxilb", suffix ".log"/".log.gz").
package logrotate

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Config controls rotation and retention. The zero value disables rotation
// (writes pass through unrotated), so callers must use Defaults() or set
// MaxSizeMB explicitly.
type Config struct {
	// MaxSizeMB rotates the file when it would exceed this size. <= 0
	// disables rotation entirely.
	MaxSizeMB int
	// MaxBackups is the maximum number of rotated files kept per base
	// (oldest deleted first). <= 0 keeps all until MaxAgeDays applies.
	MaxBackups int
	// MaxAgeDays deletes rotated files older than this. <= 0 keeps forever.
	MaxAgeDays int
	// Compress gzips rotated files.
	Compress bool
}

// Defaults returns the production defaults: 50 MB, 4 backups, 28 days, gzip.
func Defaults() Config {
	return Config{MaxSizeMB: 50, MaxBackups: 4, MaxAgeDays: 28, Compress: true}
}

func (c Config) enabled() bool { return c.MaxSizeMB > 0 }

func (c Config) maxBytes() int64 { return int64(c.MaxSizeMB) * 1024 * 1024 }

// backupTimeFormat sorts lexically in chronological order.
const backupTimeFormat = "20060102-150405.000"

// splitBase returns the path without extension and the extension
// (".log" for "/var/log/loxilb.log"; ".json.log" is treated as ".log"
// with the ".json" staying in the base, which keeps the REST-API-visible
// ".log"/".log.gz" suffix on backups).
func splitBase(path string) (base, ext string) {
	ext = filepath.Ext(path)
	return strings.TrimSuffix(path, ext), ext
}

// backupName builds "<base>-<ts><ext>" next to the original file.
func backupName(path string, t time.Time) string {
	base, ext := splitBase(path)
	return fmt.Sprintf("%s-%s%s", base, t.UTC().Format(backupTimeFormat), ext)
}

// isBackupOf reports whether name (no directory) is a rotated form of the
// original file name.
func isBackupOf(orig, name string) bool {
	base, ext := splitBase(filepath.Base(orig))
	if !strings.HasPrefix(name, base+"-") {
		return false
	}
	rest := strings.TrimPrefix(name, base+"-")
	rest = strings.TrimSuffix(rest, ".gz")
	if !strings.HasSuffix(rest, ext) {
		return false
	}
	ts := strings.TrimSuffix(rest, ext)
	_, err := time.Parse(backupTimeFormat, ts)
	return err == nil
}

// Writer is a size-rotating io.WriteCloser. Safe for concurrent use.
type Writer struct {
	path string
	cfg  Config

	mu   sync.Mutex
	file *os.File
	size int64
}

// New opens (appending) the log file at path and returns a rotating writer.
// Parent directories are not created; the caller owns directory layout.
func New(path string, cfg Config) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Writer{path: path, cfg: cfg, file: f, size: st.Size()}, nil
}

// Write implements io.Writer, rotating before the write that would cross
// the size limit.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cfg.enabled() && w.size+int64(len(p)) > w.cfg.maxBytes() && w.size > 0 {
		if err := w.rotateLocked(); err != nil {
			// Rotation failure must not lose log lines: keep writing to
			// the current file and retry on a later write.
			fmt.Fprintf(os.Stderr, "logrotate: rotate %s: %v\n", w.path, err)
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// Close closes the underlying file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *Writer) rotateLocked() error {
	bak := backupName(w.path, time.Now())
	if err := w.file.Close(); err != nil {
		return err
	}
	if err := os.Rename(w.path, bak); err != nil {
		// Reopen the original path either way so logging continues.
		f, oerr := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if oerr != nil {
			return fmt.Errorf("rename: %v; reopen: %w", err, oerr)
		}
		w.file = f
		st, _ := f.Stat()
		if st != nil {
			w.size = st.Size()
		}
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	go finishRotation(w.path, bak, w.cfg)
	return nil
}

// Sweep applies copy-truncate rotation to a file appended to by another
// writer that holds its own open handle (O_APPEND semantics keep the
// foreign writer valid after truncation; lines written between copy and
// truncate are lost, which is the standard copytruncate trade-off).
// It is a no-op when the file is below the limit or absent.
func Sweep(path string, cfg Config) error {
	if !cfg.enabled() {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil || st.Size() <= cfg.maxBytes() {
		return err
	}
	bak := backupName(path, time.Now())
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(bak, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(bak)
		return err
	}
	if err = dst.Close(); err != nil {
		return err
	}
	if err = os.Truncate(path, 0); err != nil {
		os.Remove(bak)
		return err
	}
	finishRotation(path, bak, cfg)
	return nil
}

// StartSweeper runs Sweep(path) every interval for the process lifetime.
func StartSweeper(path string, cfg Config, interval time.Duration) {
	if !cfg.enabled() || interval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			if err := Sweep(path, cfg); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "logrotate: sweep %s: %v\n", path, err)
			}
		}
	}()
}

// finishRotation compresses the backup (if configured) and prunes old
// backups of the same base file.
func finishRotation(orig, bak string, cfg Config) {
	if cfg.Compress {
		if err := gzipFile(bak); err != nil {
			fmt.Fprintf(os.Stderr, "logrotate: compress %s: %v\n", bak, err)
		}
	}
	if err := pruneBackups(orig, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "logrotate: prune for %s: %v\n", orig, err)
	}
}

// gzipFile replaces path with path.gz (written via a temp file so a crash
// never leaves a truncated archive in place).
func gzipFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := path + ".gz.tmp"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(dst)
	if _, err = io.Copy(zw, src); err == nil {
		err = zw.Close()
	} else {
		zw.Close()
	}
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err = os.Rename(tmp, path+".gz"); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(path)
}

// pruneBackups enforces MaxBackups and MaxAgeDays for backups of orig.
func pruneBackups(orig string, cfg Config) error {
	dir := filepath.Dir(orig)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type bk struct {
		name string
		mod  time.Time
	}
	var backups []bk
	for _, e := range entries {
		if e.IsDir() || !isBackupOf(orig, e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, bk{e.Name(), info.ModTime()})
	}
	// Newest first (timestamp in name sorts lexically).
	sort.Slice(backups, func(i, j int) bool { return backups[i].name > backups[j].name })

	cutoff := time.Time{}
	if cfg.MaxAgeDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -cfg.MaxAgeDays)
	}
	for i, b := range backups {
		tooMany := cfg.MaxBackups > 0 && i >= cfg.MaxBackups
		tooOld := !cutoff.IsZero() && b.mod.Before(cutoff)
		if tooMany || tooOld {
			os.Remove(filepath.Join(dir, b.name))
		}
	}
	return nil
}
