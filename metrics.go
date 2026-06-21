package main

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const metricsWindow = 512

type Metrics struct {
	mu       sync.RWMutex
	started  time.Time
	switched time.Time
	byAlgo   map[string]*AlgorithmMetrics
	events   []MetricEvent
}

type AlgorithmMetrics struct {
	Count     uint64
	TotalNS   int64
	RecentNS  []int64
	recentPos int
}

type MetricEvent struct {
	At             time.Time `json:"at"`
	Scenario       string    `json:"scenario"`
	Language       string    `json:"language"`
	Algorithm      string    `json:"algorithm"`
	DataStructure  string    `json:"dataStructure"`
	Implementation string    `json:"implementation"`
	LatencyMS      float64   `json:"latencyMs"`
}

func NewMetrics() *Metrics {
	now := time.Now()
	return &Metrics{
		started:  now,
		switched: now,
		byAlgo:   make(map[string]*AlgorithmMetrics),
		events:   make([]MetricEvent, 0, metricsWindow),
	}
}

func (m *Metrics) Observe(scenario string, language string, algorithm string, dataStructure string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	implementation := implementationKey(scenario, language, algorithm, dataStructure)
	metrics := m.byAlgo[implementation]
	if metrics == nil {
		metrics = &AlgorithmMetrics{RecentNS: make([]int64, 0, metricsWindow)}
		m.byAlgo[implementation] = metrics
	}

	ns := duration.Nanoseconds()
	metrics.Count++
	metrics.TotalNS += ns
	if len(metrics.RecentNS) < metricsWindow {
		metrics.RecentNS = append(metrics.RecentNS, ns)
	} else {
		metrics.RecentNS[metrics.recentPos] = ns
		metrics.recentPos = (metrics.recentPos + 1) % len(metrics.RecentNS)
	}

	event := MetricEvent{
		At:             time.Now().UTC(),
		Scenario:       scenario,
		Language:       language,
		Algorithm:      algorithm,
		DataStructure:  normalizedDataStructure(dataStructure),
		Implementation: implementation,
		LatencyMS:      float64(ns) / float64(time.Millisecond),
	}
	if len(m.events) < metricsWindow {
		m.events = append(m.events, event)
	} else {
		copy(m.events, m.events[1:])
		m.events[len(m.events)-1] = event
	}
}

func (m *Metrics) MarkAlgorithmSwitch(_ string) {
	m.mu.Lock()
	m.switched = time.Now()
	m.mu.Unlock()
}

type StatsSnapshot struct {
	ActiveScenario       string                  `json:"activeScenario"`
	ActiveLanguage       string                  `json:"activeLanguage"`
	ActiveAlgorithm      string                  `json:"activeAlgorithm"`
	ActiveDataStructure  string                  `json:"activeDataStructure"`
	ActiveImplementation string                  `json:"activeImplementation"`
	UptimeSeconds        int64                   `json:"uptimeSeconds"`
	SinceSwitchSec       int64                   `json:"sinceSwitchSeconds"`
	Algorithms           map[string]AlgoSnapshot `json:"algorithms"`
	RecentEvents         []MetricEvent           `json:"recentEvents"`
	Load                 LoadGeneratorState      `json:"load"`
	Runtime              RuntimeStats            `json:"runtime"`
}

type AlgoSnapshot struct {
	Scenario       string  `json:"scenario"`
	Language       string  `json:"language"`
	Algorithm      string  `json:"algorithm"`
	DataStructure  string  `json:"dataStructure"`
	Implementation string  `json:"implementation"`
	Requests       uint64  `json:"requests"`
	AverageMS      float64 `json:"averageMs"`
	P50MS          float64 `json:"p50Ms"`
	P95MS          float64 `json:"p95Ms"`
	P99MS          float64 `json:"p99Ms"`
	RecentCount    int     `json:"recentCount"`
	RecentRPS      float64 `json:"recentRps"`
}

func (m *Metrics) Snapshot() StatsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	recentRPS := m.recentRPSByAlgorithmLocked(now, 5*time.Second)
	algorithms := make(map[string]AlgoSnapshot, len(m.byAlgo))
	for name, metrics := range m.byAlgo {
		algorithms[name] = snapshotAlgorithm(name, metrics, recentRPS[name])
	}

	events := append([]MetricEvent(nil), m.events...)
	return StatsSnapshot{
		UptimeSeconds:  int64(now.Sub(m.started).Seconds()),
		SinceSwitchSec: int64(now.Sub(m.switched).Seconds()),
		Algorithms:     algorithms,
		RecentEvents:   events,
	}
}

func (m *Metrics) recentRPSByAlgorithmLocked(now time.Time, window time.Duration) map[string]float64 {
	counts := make(map[string]int)
	cutoff := now.Add(-window)
	for _, event := range m.events {
		if event.At.After(cutoff) || event.At.Equal(cutoff) {
			counts[event.Implementation]++
		}
	}

	rates := make(map[string]float64, len(counts))
	seconds := window.Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	for algorithm, count := range counts {
		rates[algorithm] = float64(count) / seconds
	}
	return rates
}

func snapshotAlgorithm(implementation string, metrics *AlgorithmMetrics, recentRPS float64) AlgoSnapshot {
	recent := append([]int64(nil), metrics.RecentNS...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i] < recent[j]
	})

	avg := 0.0
	if metrics.Count > 0 {
		avg = float64(metrics.TotalNS) / float64(metrics.Count) / float64(time.Millisecond)
	}

	scenario, language, algorithm, dataStructure := implementationParts(implementation)
	return AlgoSnapshot{
		Scenario:       scenario,
		Language:       language,
		Algorithm:      algorithm,
		DataStructure:  dataStructure,
		Implementation: implementation,
		Requests:       metrics.Count,
		AverageMS:      avg,
		P50MS:          percentileNS(recent, 0.50),
		P95MS:          percentileNS(recent, 0.95),
		P99MS:          percentileNS(recent, 0.99),
		RecentCount:    len(recent),
		RecentRPS:      recentRPS,
	}
}

func percentileNS(sorted []int64, pct float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(pct*float64(len(sorted)))) - 1
	idx = max(0, min(idx, len(sorted)-1))
	return float64(sorted[idx]) / float64(time.Millisecond)
}

func (m *Metrics) PrometheusText(activeAlgorithm string, runtimeStats RuntimeStats, load LoadGeneratorState) string {
	var b strings.Builder

	b.WriteString("# HELP lab_requests_total Total profile lookup requests.\n")
	b.WriteString("# TYPE lab_requests_total counter\n")
	b.WriteString("# HELP lab_request_duration_seconds_sum Total profile lookup time in seconds.\n")
	b.WriteString("# TYPE lab_request_duration_seconds_sum counter\n")
	b.WriteString("# HELP lab_request_duration_seconds_count Total profile lookup observations.\n")
	b.WriteString("# TYPE lab_request_duration_seconds_count counter\n")

	m.mu.RLock()
	names := make([]string, 0, len(m.byAlgo))
	for name := range m.byAlgo {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		metrics := m.byAlgo[name]
		scenario, language, algorithm, dataStructure := implementationParts(name)
		labels := prometheusImplementationLabels(scenario, language, algorithm, dataStructure, name)
		fmt.Fprintf(&b, "lab_requests_total{%s} %d\n", labels, metrics.Count)
		fmt.Fprintf(&b, "lab_request_duration_seconds_sum{%s} %.9f\n", labels, float64(metrics.TotalNS)/float64(time.Second))
		fmt.Fprintf(&b, "lab_request_duration_seconds_count{%s} %d\n", labels, metrics.Count)
	}
	m.mu.RUnlock()

	scenario, language, algorithm, dataStructure := implementationParts(activeAlgorithm)
	fmt.Fprintf(&b, "lab_active_algorithm_info{%s} 1\n", prometheusImplementationLabels(scenario, language, algorithm, dataStructure, activeAlgorithm))
	fmt.Fprintf(&b, "lab_load_running %d\n", boolFloat(load.Running))
	fmt.Fprintf(&b, "lab_load_in_flight %d\n", load.InFlight)
	fmt.Fprintf(&b, "lab_runtime_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "lab_runtime_heap_bytes %d\n", runtimeStats.HeapBytes)
	if runtimeStats.CPUPercent >= 0 {
		fmt.Fprintf(&b, "lab_process_cpu_percent %.2f\n", runtimeStats.CPUPercent)
	}

	return b.String()
}

func implementationParts(implementation string) (string, string, string, string) {
	parts := strings.Split(implementation, ":")
	if len(parts) >= 4 {
		return parts[0], parts[1], parts[2], parts[3]
	}
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], defaultDataStructure
	}
	return "", "", implementation, defaultDataStructure
}

func prometheusImplementationLabels(scenario string, language string, algorithm string, dataStructure string, implementation string) string {
	return fmt.Sprintf(
		"scenario=%q,language=%q,algorithm=%q,data_structure=%q,implementation=%q",
		prometheusLabel(scenario),
		prometheusLabel(language),
		prometheusLabel(algorithm),
		prometheusLabel(dataStructure),
		prometheusLabel(implementation),
	)
}

func boolFloat(value bool) int {
	if value {
		return 1
	}
	return 0
}

func prometheusLabel(value string) string {
	return strings.ReplaceAll(value, "\\", "\\\\")
}
