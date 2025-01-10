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
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/errors"
	"github.com/loxilb-io/loxilb/options"

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
	newFlowCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "new_flow_count",
			Help: "The number of new TCP connections from clients to targets.",
		},
	)
	processedBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedTCPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_tcp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedUDPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_udp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedSCTPBytes = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_sctp_bytes",
			Help: "The total number of bytes processed by the load balancer, including TCP/IP headers.",
		},
	)
	processedPackets = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "processed_packets",
			Help: "The total number of packets processed by the load balancer.",
		},
	)
	lbRuleProcessedBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lb_rule_processed_bytes",
			Help: "The total number of bytes processed by the load balancer for each rule, including TCP/IP headers.",
		},
		[]string{"rule"},
	)
	lbRuleProcessedPackets = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lb_rule_processed_packets",
			Help: "The total number of packets processed by the load balancer for each rule.",
		},
		[]string{"rule"},
	)
	hostProcessedBytes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "host_processed_bytes",
			Help: "The total number of bytes processed by the load balancer for each host, including TCP/IP headers.",
		},
		[]string{"host"},
	)
	hostProcessedPackets = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "host_processed_packets",
			Help: "The total number of packets processed by the load balancer for each host.",
		},
		[]string{"host"},
	)
	prevConntrackStats = make(map[ConntrackKey]Stats)
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
	return ConntrackKey(fmt.Sprintf("%s|%05d|%s|%05d|%v|%s",
		c.Sip, c.Sport, c.Dip, c.Dport, c.Proto, c.ServiceName))
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
		}

		ConntrackInfo, err = hooks.NetCtInfoGet()
		if err != nil {
			tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			continue
		}
		localStats := make(map[ConntrackKey]Stats, len(ConntrackInfo))
		for _, ct := range ConntrackInfo {
			key := MakeConntrackKey(ct)
			localStats[key] = Stats{
				Bytes:   ct.Bytes,
				Packets: ct.Pkts,
			}
		}

		mutex.Lock()
		ConntrackStats = localStats
		mutex.Unlock()

		time.Sleep(PromethusDefaultPeriod)
	}
}

func RunGetEndpoint(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			info, err := hooks.NetEpHostGet()
			if err != nil {
				tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
				continue
			}

			mutex.Lock()
			EndPointInfo = info
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
		}

		info, err := hooks.NetLbRuleGet()
		if err != nil {
			tk.LogIt(tk.LogDebug, "[Prometheus] Error occur : %v\n", err)
			continue
		}

		mutex.Lock()
		LBRuleInfo = info
		mutex.Unlock()

		ruleCount.Set(float64(len(info)))

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
			info := make([]cmn.CtInfo, len(ConntrackInfo))
			copy(info, ConntrackInfo)
			mutex.Unlock()

			tcpCount, udpCount, sctpCount, closedCount := 0, 0, 0, 0
			for _, ct := range info {
				switch ct.Proto {
				case "tcp":
					tcpCount++
				case "udp":
					udpCount++
				case "sctp":
					sctpCount++
				}
				if ct.CState == "closed" {
					closedCount++
				}
			}
			activeConntrackCount.Set(float64(len(info)))
			activeFlowCountTcp.Set(float64(tcpCount))
			activeFlowCountUdp.Set(float64(udpCount))
			activeFlowCountSctp.Set(float64(sctpCount))
			inActiveFlowCount.Set(float64(closedCount))
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
		}

		mutex.Lock()
		localEndPointInfo := EndPointInfo
		mutex.Unlock()

		healthyHostCount.Set(0)
		unHealthyHostCount.Set(0)

		for _, ep := range localEndPointInfo {
			if ep.CurrState == "ok" {
				healthyHostCount.Inc()
			} else if ep.CurrState == "nok" {
				unHealthyHostCount.Inc()
			}
		}

		time.Sleep(PromethusDefaultPeriod)
	}
}

func parseConntrackKey(key ConntrackKey) (sip, sport, dip, dport, proto, serviceName string) {
	parts := strings.Split(string(key), "|")
	if len(parts) == 6 {
		return parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
	}
	return "", "", "", "", "", ""
}

func RunProcessedStatistic(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mutex.Lock()
			localPrevConntrackStats := make(map[ConntrackKey]Stats, len(ConntrackStats))
			for k, ct := range ConntrackStats {
				localPrevConntrackStats[k] = ct
			}
			mutex.Unlock()

			for k, ct := range localPrevConntrackStats {
				prevStats, exists := prevConntrackStats[k]
				if !exists {
					prevStats = Stats{Bytes: 0, Packets: 0}
				}
				diffBytes := ct.Bytes - prevStats.Bytes
				diffPackets := ct.Packets - prevStats.Packets

				if diffBytes < 0 {
					diffBytes = ct.Bytes
				}
				if diffPackets < 0 {
					diffPackets = ct.Packets
				}

				if diffBytes > 0 || diffPackets > 0 {
					if strings.Contains(string(k), "tcp") {
						processedTCPBytes.Add(float64(diffBytes))
					} else if strings.Contains(string(k), "udp") {
						processedUDPBytes.Add(float64(diffBytes))
					} else if strings.Contains(string(k), "sctp") {
						processedSCTPBytes.Add(float64(diffBytes))
					}
					processedPackets.Add(float64(diffPackets))
					processedBytes.Add(float64(diffBytes))

					// Update per-rule and per-endpoint metrics
					_, _, dip, _, _, serviceName := parseConntrackKey(k)
					lbRuleProcessedBytes.WithLabelValues(serviceName).Add(float64(diffBytes))
					lbRuleProcessedPackets.WithLabelValues(serviceName).Add(float64(diffPackets))

					hostProcessedBytes.WithLabelValues(dip).Add(float64(diffBytes))
					hostProcessedPackets.WithLabelValues(dip).Add(float64(diffPackets))

				}
			}

			mutex.Lock()
			prevConntrackStats = localPrevConntrackStats
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
			CurrentFlowCounts := len(ConntrackInfo)
			mutex.Unlock()

			diff := CurrentFlowCounts - PreFlowCounts
			if diff > 0 {
				newFlowCount.Add(float64(diff))
			}
			PreFlowCounts = CurrentFlowCounts
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
			var LCUNewFlowCount = &dto.Metric{}
			var LCUActiveFlowCount = &dto.Metric{}
			var LCURuleCount = &dto.Metric{}
			var LCUProcessedBytes = &dto.Metric{}

			mutex.Lock()
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
			localConntrackStatsLen := len(ConntrackStats)
			mutex.Unlock()

			// LCU of accumulated Flow count = Flowcount / 2160000
			// LCU of Rule = ruleCount/1000
			// LCU of Byte = processedBytes(Gb)/1h
			if LCURuleCount.Gauge != nil && LCURuleCount.Gauge.Value != nil && LCUProcessedBytes.Gauge != nil && LCUProcessedBytes.Gauge.Value != nil {
				consumedLcus.Set(float64(localConntrackStatsLen)/2160000 +
					*LCURuleCount.Gauge.Value/1000 +
					(*LCUProcessedBytes.Gauge.Value*8)/360000000000) // (byte * 8)/ (60*60*1G)/10
			}
		}
		time.Sleep(PromethusDefaultPeriod)
	}
}
