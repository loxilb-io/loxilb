/*
 * Copyright (c) 2023 NetLOX Inc
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

package prometheus

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tk "github.com/loxilb-io/loxilib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Host utilization metrics, ported from loxilb-inference-gateway.
//
// These are canonical-only families: this exporter never published them, so
// there is no legacy name to keep compatible with.
//
// They are also LAZILY REGISTERED, which is load-bearing. A gauge registered at
// package init exports a literal `0` until something sets it — and a consumer
// cannot tell that 0 apart from a measurement of an idle host. On a platform
// with no /proc, or in a container where the read fails, it would stay 0
// forever and read as "0% used". Registering on the first successful sample
// makes the family ABSENT in exactly that case, which is what lets a consumer
// render an honest "not reported".

// lazyGauge registers its metric with the default registry on first Set, so an
// unsampled metric is absent from the exposition rather than present as zero.
type lazyGauge struct {
	once sync.Once
	name string
	help string
	// reg is the registry to register with; nil means the default one. Only
	// tests set it, so the absence-until-sampled invariant can be asserted
	// without depending on what else in the process has already sampled.
	reg prometheus.Registerer
	g   prometheus.Gauge
}

func (l *lazyGauge) Set(v float64) {
	l.once.Do(func() {
		reg := l.reg
		if reg == nil {
			reg = prometheus.DefaultRegisterer
		}
		l.g = promauto.With(reg).NewGauge(prometheus.GaugeOpts{Name: l.name, Help: l.help})
	})
	l.g.Set(v)
}

var (
	systemCPUUtilization = &lazyGauge{
		name: MetricSystemCPUUtilization,
		help: "Host CPU utilization percentage [0-100], sampled from /proc/stat.",
	}
	systemMemoryUtilization = &lazyGauge{
		name: MetricSystemMemoryUtilization,
		help: "Host memory utilization percentage [0-100], from /proc/meminfo (MemTotal-MemAvailable).",
	}
	systemDiskUtilization = &lazyGauge{
		name: MetricSystemDiskUtilization,
		help: "Root filesystem utilization percentage [0-100].",
	}
	conntrackMaxEntries = &lazyGauge{
		name: MetricConntrackMaxEntries,
		help: "Capacity of the datapath conntrack table (maximum concurrently tracked sessions).",
	}

	// CPU utilization is a delta between two /proc/stat samples, so the first
	// sample can only establish a baseline.
	prevCPUIdle  uint64
	prevCPUTotal uint64
	cpuInited    bool
)

// errCPUBaseline is returned by the first readCPUUtilization call. It is a
// normal condition, not a failure: there is simply no delta to report yet, and
// publishing 0 would claim an idle CPU.
var errCPUBaseline = errors.New("cpu baseline sample")

// readCPUUtilization returns busy-time percentage since the previous sample.
func readCPUUtilization() (float64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, fmt.Errorf("empty /proc/stat")
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, "cpu ") {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}

	// Fields: user nice system idle iowait irq softirq steal guest guest_nice
	parts := strings.Fields(line)
	var vals []uint64
	for _, p := range parts[1:] {
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return 0, err
		}
		vals = append(vals, v)
	}
	if len(vals) < 4 {
		return 0, fmt.Errorf("insufficient cpu fields")
	}
	idle := vals[3]
	var total uint64
	for _, v := range vals {
		total += v
	}

	if !cpuInited {
		prevCPUIdle = idle
		prevCPUTotal = total
		cpuInited = true
		return 0, errCPUBaseline
	}

	// Counters in /proc/stat are monotonic, but a suspend/resume or a CPU
	// hotplug can make them go backwards. Re-baseline instead of reporting a
	// nonsense percentage from an underflowed subtraction.
	if idle < prevCPUIdle || total < prevCPUTotal {
		prevCPUIdle = idle
		prevCPUTotal = total
		return 0, fmt.Errorf("cpu counters went backwards, re-baselined")
	}

	idleDelta := float64(idle - prevCPUIdle)
	totalDelta := float64(total - prevCPUTotal)
	prevCPUIdle = idle
	prevCPUTotal = total

	if totalDelta <= 0 {
		return 0, fmt.Errorf("non-positive total delta")
	}
	return clampPercent((1.0 - idleDelta/totalDelta) * 100.0), nil
}

// readMemoryUtilization reads /proc/meminfo to compute used percentage.
func readMemoryUtilization() (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				total, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				available, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
		if total > 0 && available > 0 {
			break
		}
	}
	if total == 0 {
		return 0, fmt.Errorf("memtotal not found")
	}
	if available > total {
		return 0, fmt.Errorf("implausible meminfo: available %d > total %d", available, total)
	}
	return clampPercent(float64(total-available) / float64(total) * 100.0), nil
}

// readDiskUtilization uses Statfs on the root filesystem.
func readDiskUtilization() (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0, err
	}
	total := float64(st.Blocks) * float64(st.Bsize)
	avail := float64(st.Bavail) * float64(st.Bsize)
	if total <= 0 {
		return 0, fmt.Errorf("invalid disk size")
	}
	return clampPercent((1.0 - (avail / total)) * 100.0), nil
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// RunSystemUtilization periodically samples host utilization. A failed read is
// logged and skipped: the gauge keeps its previous value, or stays unregistered
// if it has never succeeded, so a /proc failure never looks like an idle host.
func RunSystemUtilization(ctx context.Context) {
	ticker := time.NewTicker(PromethusDefaultPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if cpu, err := readCPUUtilization(); err == nil {
			systemCPUUtilization.Set(cpu)
		} else if !errors.Is(err, errCPUBaseline) {
			tk.LogIt(tk.LogDebug, "[Prometheus] CPU utilization read error: %v\n", err)
		}
		if mem, err := readMemoryUtilization(); err == nil {
			systemMemoryUtilization.Set(mem)
		} else {
			tk.LogIt(tk.LogDebug, "[Prometheus] Memory utilization read error: %v\n", err)
		}
		if du, err := readDiskUtilization(); err == nil {
			systemDiskUtilization.Set(du)
		} else {
			tk.LogIt(tk.LogDebug, "[Prometheus] Disk utilization read error: %v\n", err)
		}
	}
}

// SetConntrackMaxEntries publishes the datapath conntrack table capacity, which
// is what turns a raw conntrack entry count into a utilization ratio.
//
// Absent on builds with no datapath capacity — absence means "not applicable",
// never a fake 0 (which would make a utilization ratio divide by zero).
//
// Called from the datapath init, which runs before and independently of
// Prometheus collection, so this must stay safe to call with collection off.
func SetConntrackMaxEntries(n int) {
	if n <= 0 {
		return
	}
	conntrackMaxEntries.Set(float64(n))
}
