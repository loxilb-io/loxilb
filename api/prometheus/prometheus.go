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
	"context"
	"fmt"
	"github.com/go-openapi/errors"
	"github.com/loxilb-io/loxilb/options"
	"net/http"
	"strings"
	"sync"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

type Stats struct {
	Bytes   uint64
	Packets uint64
}
type ConntrackKey string

var (
	hooks                  cmn.NetHookInterface
	ConntrackInfo          []cmn.CtInfo
	EndPointInfo           []cmn.EndPointMod
	LBRuleInfo             []cmn.LbRuleMod
	err                    error
	mutex                  *sync.Mutex
	ConntrackStats         map[ConntrackKey]Stats // Key [string] : sip dip pro sport dport
	PreFlowCounts          int
	PromethusDefaultPeriod = 10 * time.Second
	PromethusPartialPeriod = (PromethusDefaultPeriod / 6)
	PromethusLongPeriod    = (PromethusDefaultPeriod * 600) // To reset Period
	prometheusCtx          context.Context
	prometheusCancel       context.CancelFunc
	activeConntrackCount   = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_conntrack_count",
			Help: "The average number of active established connections from clients to targets.",
		},
	)
	activeFlowCountTcp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_tcp",
			Help: "The average number of concurrent TCP flows (or connections) from clients to targets.",
		},
	)
	activeFlowCountUdp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_udp",
			Help: "The average number of concurrent UDP flows (or connections) from clients to targets.",
		},
	)
	activeFlowCountSctp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_flow_count_sctp",
			Help: "The average number of concurrent SCTP flows (or connections) from clients to targets.",
		},
	)
	inActiveFlowCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "inactive_flow_count",
			Help: "The average number of concurrent closed flows (or connections) from clients to targets.",
		},
	)
	healthyHostCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "healthy_host_count",
			Help: "Average number of healthy targets.",
		},
	)
	unHealthyHostCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "unhealthy_host_count",
			Help: "Average number of unhealthy targets",
		},
	)
	ruleCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "lb_rule_count",
			Help: "Average number of unhealthy targets",
		},
	)
	consumedLcus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "consumed_lcus",
			Help: "The number of LCUs used by the load balancer.",
		},
	)
	newFlowCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "new_flow_count",
			Help: "The number of new TCP connections from clients to targets.",
		},
	)
	processedBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processed_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedTCPBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processed_tcp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedUDPBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processed_udp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedSCTPBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processed_sctp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedPackets = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "processed_packets",
			Help: "The total number of packets processed by the load balancer.",
		},
	)
)

func PrometheusRegister(hook cmn.NetHookInterface) {
	hooks = hook
}

func Init() {
	prometheusCtx, prometheusCancel = context.WithCancel(context.Background())

	// Make Conntrack Statistic map
	ConntrackStats = make(map[ConntrackKey]Stats)
	mutex = &sync.Mutex{}
	go RunGetConntrack(prometheusCtx)
	go RunGetEndpoint(prometheusCtx)
	go RunActiveConntrackCount(prometheusCtx)
	go RunHostCount(prometheusCtx)
	go RunProcessedStatistic(prometheusCtx)
	go RunNewFlowCount(prometheusCtx)
	go RunResetCounts(prometheusCtx)
	go RunGetLBRule(prometheusCtx)
	go RunLcusCalculator(prometheusCtx)
}

func Off() error {
	if !options.Opts.Prometheus {
		return errors.New(http.StatusBadRequest, "already prometheus turned off")
	}
	options.Opts.Prometheus = false
	prometheusCancel()
	return nil
}

func TurnOn() error {
	if options.Opts.Prometheus {
		return errors.New(http.StatusBadRequest, "already prometheus turned on")
	}
	options.Opts.Prometheus = true
	Init()
	return nil
}

func MakeConntrackKey(c cmn.CtInfo) (key ConntrackKey) {
	return ConntrackKey(fmt.Sprintf("%s|%05d|%s|%05d|%v", c.Sip, c.Sport, c.Dip, c.Dport, c.Proto))
}

func RunResetCounts(ctx context.Context) {
	for {
		// Statistic reset
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			ConntrackStats = map[ConntrackKey]Stats{}
			mutex.Unlock()
		}
		time.Sleep(PromethusLongPeriod)
	}
}

func RunGetConntrack(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			ConntrackInfo, err = hooks.NetCtInfoGet()
			if err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}

			for _, ct := range ConntrackInfo {
				k := MakeConntrackKey(ct)
				var tmpStats Stats
				_, ok := ConntrackStats[k]
				if ok {
					tmpStats = Stats{
						Bytes:   ConntrackStats[k].Bytes + ct.Bytes,
						Packets: ConntrackStats[k].Packets + ct.Pkts,
					}
				} else {
					tmpStats = Stats{
						Bytes:   ct.Bytes,
						Packets: ct.Pkts,
					}
				}

				ConntrackStats[k] = tmpStats
			}
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetEndpoint(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			EndPointInfo, err = hooks.NetEpHostGet()
			if err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetLBRule(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			LBRuleInfo, err = hooks.NetLbRuleGet()
			if err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			ruleCount.Set(float64(len(LBRuleInfo)))
			mutex.Unlock()
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunActiveConntrackCount(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			// init Counts
			activeFlowCountTcp.Set(0)
			activeFlowCountUdp.Set(0)
			activeFlowCountSctp.Set(0)
			inActiveFlowCount.Set(0)

			// Total flow count
			activeConntrackCount.Set(float64(len(ConntrackInfo)))

			for _, ct := range ConntrackInfo {
				// TCP flow count
				if ct.Proto == "tcp" {
					activeFlowCountTcp.Inc()
				}
				// UDP flow count
				if ct.Proto == "udp" {
					activeFlowCountUdp.Inc()
				}
				// SCTP flow count
				if ct.Proto == "sctp" {
					activeFlowCountSctp.Inc()
				}
				// Closed flow count
				if ct.CState == "closed" {
					inActiveFlowCount.Inc()
				}
			}
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunHostCount(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			healthyHostCount.Set(0)
			unHealthyHostCount.Set(0)
			for _, ep := range EndPointInfo {
				if ep.CurrState == "ok" {
					healthyHostCount.Inc()
				}
				if ep.CurrState == "nok" {
					unHealthyHostCount.Inc()
				}
			}
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunProcessedStatistic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			// Init Stats
			processedPackets.Set(0)
			processedBytes.Set(0)
			processedTCPBytes.Set(0)
			processedUDPBytes.Set(0)
			processedSCTPBytes.Set(0)
			for k, ct := range ConntrackStats {
				if strings.Contains(string(k), "tcp") {
					processedTCPBytes.Add(float64(ct.Bytes))
				}
				if strings.Contains(string(k), "udp") {
					processedUDPBytes.Add(float64(ct.Bytes))
				}
				if strings.Contains(string(k), "sctp") {
					processedSCTPBytes.Add(float64(ct.Bytes))
				}
				processedPackets.Add(float64(ct.Packets))
				processedBytes.Add(float64(ct.Bytes))
			}
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunNewFlowCount(ctx context.Context) {
	PreFlowCounts = 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			// Total new flow count
			CurrentFlowCounts := len(ConntrackInfo)
			diff := CurrentFlowCounts - PreFlowCounts
			if diff > 0 {
				newFlowCount.Set(float64(diff))
			} else {
				newFlowCount.Set(0)
			}
			PreFlowCounts = CurrentFlowCounts
			mutex.Unlock()
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunLcusCalculator(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			var LCUNewFlowCount = &dto.Metric{}
			var LCUActiveFlowCount = &dto.Metric{}
			var LCURuleCount = &dto.Metric{}
			var LCUProcessedBytes = &dto.Metric{}
			if err := newFlowCount.Write(LCUNewFlowCount); err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			if err := activeConntrackCount.Write(LCUActiveFlowCount); err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			if err := ruleCount.Write(LCURuleCount); err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			if err := processedBytes.Write(LCUProcessedBytes); err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			}
			// LCU of accumulated Flow count = Flowcount / 2160000
			// LCU of Rule = ruleCount/1000
			// LCU of Byte = processedBytes(Gb)/1h
			if LCURuleCount.Gauge.Value != nil && LCUProcessedBytes.Gauge.Value != nil {
				consumedLcus.Set(float64(len(ConntrackStats))/2160000 +
					*LCURuleCount.Gauge.Value/1000 +
					(*LCUProcessedBytes.Gauge.Value*8)/360000000000) // (byte * 8)/ (60*60*1G)/10
			}

			mutex.Unlock()
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}
