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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

// Dual-emit wrappers.
//
// Each wrapper owns two registered metrics — the legacy name and the canonical
// `loxilb_*` name from metric_names.go — and writes every update to both. The
// collector loops keep calling Set/Add exactly as before, so the mirroring adds
// no branching to the collection paths and cannot drift between the two names.
//
// A canonical name of "" means "legacy only" (metrics the gateway deliberately
// dropped); the canonical half is then nil and every write skips it. Reads
// (Write, for the LCU calculator) always come from the legacy half.

// dualGauge mirrors a gauge value onto the legacy and canonical families.
type dualGauge struct {
	legacy    prometheus.Gauge
	canonical prometheus.Gauge
}

func newDualGauge(legacyName, canonicalName, help string) *dualGauge {
	d := &dualGauge{}
	if canonicalName == "" {
		d.legacy = promauto.NewGauge(prometheus.GaugeOpts{Name: legacyName, Help: help})
		return d
	}
	d.legacy = promauto.NewGauge(prometheus.GaugeOpts{
		Name: legacyName,
		Help: deprecatedHelp(help, canonicalName),
	})
	d.canonical = promauto.NewGauge(prometheus.GaugeOpts{Name: canonicalName, Help: help})
	return d
}

func (d *dualGauge) Set(v float64) {
	d.legacy.Set(v)
	if d.canonical != nil {
		d.canonical.Set(v)
	}
}

// Write reads the legacy half. Both halves always hold the same value, so the
// choice is arbitrary — but the legacy half is the one guaranteed to exist.
func (d *dualGauge) Write(out *dto.Metric) error {
	return d.legacy.Write(out)
}

// dualCounter mirrors a monotonic counter onto both families.
type dualCounter struct {
	legacy    prometheus.Counter
	canonical prometheus.Counter
}

func newDualCounter(legacyName, canonicalName, help string) *dualCounter {
	d := &dualCounter{}
	if canonicalName == "" {
		d.legacy = promauto.NewCounter(prometheus.CounterOpts{Name: legacyName, Help: help})
		return d
	}
	d.legacy = promauto.NewCounter(prometheus.CounterOpts{
		Name: legacyName,
		Help: deprecatedHelp(help, canonicalName),
	})
	d.canonical = promauto.NewCounter(prometheus.CounterOpts{Name: canonicalName, Help: help})
	return d
}

func (d *dualCounter) Add(v float64) {
	d.legacy.Add(v)
	if d.canonical != nil {
		d.canonical.Add(v)
	}
}

func (d *dualCounter) Write(out *dto.Metric) error {
	return d.legacy.Write(out)
}

// dualCounterVec mirrors a labeled counter family. Both halves carry identical
// label names, so one label-value slice addresses both children.
type dualCounterVec struct {
	legacy    *prometheus.CounterVec
	canonical *prometheus.CounterVec
}

// dualCounterChild is the pair of children addressed by one label-value set.
// Returned by value: it holds two interface pointers and nothing else.
type dualCounterChild struct {
	legacy    prometheus.Counter
	canonical prometheus.Counter
}

func newDualCounterVec(legacyName, canonicalName, help string, labels []string) *dualCounterVec {
	d := &dualCounterVec{}
	if canonicalName == "" {
		d.legacy = promauto.NewCounterVec(prometheus.CounterOpts{Name: legacyName, Help: help}, labels)
		return d
	}
	d.legacy = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: legacyName,
		Help: deprecatedHelp(help, canonicalName),
	}, labels)
	d.canonical = promauto.NewCounterVec(prometheus.CounterOpts{Name: canonicalName, Help: help}, labels)
	return d
}

func (d *dualCounterVec) WithLabelValues(lvs ...string) dualCounterChild {
	c := dualCounterChild{legacy: d.legacy.WithLabelValues(lvs...)}
	if d.canonical != nil {
		c.canonical = d.canonical.WithLabelValues(lvs...)
	}
	return c
}

func (c dualCounterChild) Add(v float64) {
	c.legacy.Add(v)
	if c.canonical != nil {
		c.canonical.Add(v)
	}
}

// dualGaugeVec mirrors a labeled gauge family.
type dualGaugeVec struct {
	legacy    *prometheus.GaugeVec
	canonical *prometheus.GaugeVec
}

type dualGaugeChild struct {
	legacy    prometheus.Gauge
	canonical prometheus.Gauge
}

func newDualGaugeVec(legacyName, canonicalName, help string, labels []string) *dualGaugeVec {
	d := &dualGaugeVec{}
	if canonicalName == "" {
		d.legacy = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: legacyName, Help: help}, labels)
		return d
	}
	d.legacy = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: legacyName,
		Help: deprecatedHelp(help, canonicalName),
	}, labels)
	d.canonical = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: canonicalName, Help: help}, labels)
	return d
}

func (d *dualGaugeVec) WithLabelValues(lvs ...string) dualGaugeChild {
	g := dualGaugeChild{legacy: d.legacy.WithLabelValues(lvs...)}
	if d.canonical != nil {
		g.canonical = d.canonical.WithLabelValues(lvs...)
	}
	return g
}

func (g dualGaugeChild) Set(v float64) {
	g.legacy.Set(v)
	if g.canonical != nil {
		g.canonical.Set(v)
	}
}

// DeleteLabelValues drops the series from both halves so a removed rule or
// endpoint stops being exported rather than freezing at its last value.
func (d *dualGaugeVec) DeleteLabelValues(lvs ...string) {
	d.legacy.DeleteLabelValues(lvs...)
	if d.canonical != nil {
		d.canonical.DeleteLabelValues(lvs...)
	}
}
