package daemon

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// CustomMetric represents user-defined metrics with constrained JSON-friendly types.
type CustomMetric any

// MetricsCollector aggregates and exposes daemon metrics.
type MetricsCollector struct {
	startTime     time.Time
	mu            sync.RWMutex
	counters      map[string]*int64
	gauges        map[string]*int64
	histograms    map[string]*Histogram
	customMetrics map[string]CustomMetric
}

// Histogram tracks value distributions over time.
type Histogram struct {
	mu     sync.RWMutex
	values []float64
	sum    float64
	count  int64
}

// MetricSnapshot represents a point-in-time view of metrics.
type MetricSnapshot struct {
	Timestamp     time.Time                 `json:"timestamp"`
	Uptime        string                    `json:"uptime"`
	Counters      map[string]int64          `json:"counters"`
	Gauges        map[string]int64          `json:"gauges"`
	Histograms    map[string]HistogramStats `json:"histograms"`
	SystemMetrics SystemMetricsSnapshot     `json:"system_metrics"`
	Custom        map[string]CustomMetric   `json:"custom_metrics"`
}

// HistogramStats provides statistical summary of histogram data.
type HistogramStats struct {
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	Mean  float64 `json:"mean"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
}

// SystemMetricsSnapshot captures system-level metrics.
type SystemMetricsSnapshot struct {
	GoVersion   string  `json:"go_version"`
	Goroutines  int     `json:"goroutines"`
	MemAllocMB  float64 `json:"mem_alloc_mb"`
	MemSysMB    float64 `json:"mem_sys_mb"`
	MemUsedMB   float64 `json:"mem_used_mb"`
	GCRuns      uint32  `json:"gc_runs"`
	NextGCMB    float64 `json:"next_gc_mb"`
	LastGCPause float64 `json:"last_gc_pause_ms"`
	CPUUsage    float64 `json:"cpu_usage_percent"`
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:     time.Now(),
		counters:      make(map[string]*int64),
		gauges:        make(map[string]*int64),
		histograms:    make(map[string]*Histogram),
		customMetrics: make(map[string]CustomMetric),
	}
}

// IncrementCounter increments a counter metric.
func (mc *MetricsCollector) IncrementCounter(name string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if counter, exists := mc.counters[name]; exists {
		atomic.AddInt64(counter, 1)
	} else {
		var val int64 = 1
		mc.counters[name] = &val
	}
}

// SetGauge sets a gauge metric value.
func (mc *MetricsCollector) SetGauge(name string, value int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if gauge, exists := mc.gauges[name]; exists {
		atomic.StoreInt64(gauge, value)
	} else {
		val := value
		mc.gauges[name] = &val
	}
}

// RecordHistogram records a value in a histogram.
func (mc *MetricsCollector) RecordHistogram(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if hist, exists := mc.histograms[name]; exists {
		hist.Record(value)
	} else {
		hist := &Histogram{}
		hist.Record(value)
		mc.histograms[name] = hist
	}
}

// SetCustomMetric sets a custom metric value.
func (mc *MetricsCollector) SetCustomMetric(name string, value CustomMetric) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.customMetrics[name] = value
}

// GetSnapshot returns a complete metrics snapshot.
func (mc *MetricsCollector) GetSnapshot() *MetricSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := &MetricSnapshot{
		Timestamp:  time.Now(),
		Uptime:     time.Since(mc.startTime).String(),
		Counters:   make(map[string]int64),
		Gauges:     make(map[string]int64),
		Histograms: make(map[string]HistogramStats),
		Custom:     make(map[string]CustomMetric),
	}

	// Copy counters
	for name, counter := range mc.counters {
		snapshot.Counters[name] = atomic.LoadInt64(counter)
	}

	// Copy gauges
	for name, gauge := range mc.gauges {
		snapshot.Gauges[name] = atomic.LoadInt64(gauge)
	}

	// Copy histogram stats
	for name, hist := range mc.histograms {
		snapshot.Histograms[name] = hist.GetStats()
	}

	// Copy custom metrics
	maps.Copy(snapshot.Custom, mc.customMetrics)

	// Collect system metrics
	snapshot.SystemMetrics = mc.getSystemMetrics()

	return snapshot
}

// getSystemMetrics collects Go runtime and system metrics.
func (mc *MetricsCollector) getSystemMetrics() SystemMetricsSnapshot {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemMetricsSnapshot{
		GoVersion:   runtime.Version(),
		Goroutines:  runtime.NumGoroutine(),
		MemAllocMB:  float64(memStats.Alloc) / 1024 / 1024,
		MemSysMB:    float64(memStats.Sys) / 1024 / 1024,
		MemUsedMB:   float64(memStats.HeapInuse) / 1024 / 1024,
		GCRuns:      memStats.NumGC,
		NextGCMB:    float64(memStats.NextGC) / 1024 / 1024,
		LastGCPause: float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6, // Convert to milliseconds
		CPUUsage:    0.0,                                                       // TODO: Implement CPU usage tracking
	}
}

// Record adds a value to the histogram.
func (h *Histogram) Record(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.values = append(h.values, value)
	h.sum += value
	h.count++

	// Keep only the last 1000 values to prevent unbounded growth
	if len(h.values) > 1000 {
		copy(h.values, h.values[len(h.values)-1000:])
		h.values = h.values[:1000]
	}
}

// GetStats calculates statistical summary of the histogram.
func (h *Histogram) GetStats() HistogramStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return HistogramStats{}
	}

	stats := HistogramStats{
		Count: h.count,
		Sum:   h.sum,
		Mean:  h.sum / float64(h.count),
	}

	if len(h.values) > 0 {
		// Create a sorted copy for percentile calculations
		sorted := make([]float64, len(h.values))
		copy(sorted, h.values)

		// Simple bubble sort (adequate for small datasets)
		n := len(sorted)
		for i := range n - 1 {
			for j := range n - i - 1 {
				if sorted[j] > sorted[j+1] {
					sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
				}
			}
		}

		stats.Min = sorted[0]
		stats.Max = sorted[n-1]
		stats.P50 = percentile(sorted, 0.50)
		stats.P95 = percentile(sorted, 0.95)
		stats.P99 = percentile(sorted, 0.99)
	}

	return stats
}

// percentile calculates the value at a given percentile.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	index := p * float64(n-1)
	lower := int(index)
	upper := lower + 1

	if upper >= n {
		return sorted[n-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// MetricsHandler serves metrics in JSON format.
func (mc *MetricsCollector) MetricsHandler(w http.ResponseWriter, _ *http.Request) {
	snapshot := mc.GetSnapshot()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		adapter := errors.NewHTTPErrorAdapter(nil)
		e := errors.WrapError(err, errors.CategoryInternal, "failed to encode metrics").Build()
		adapter.WriteErrorResponse(w, nil, e)
		return
	}
}

// PrometheusHandler serves metrics in Prometheus format (basic implementation).
func (mc *MetricsCollector) PrometheusHandler(w http.ResponseWriter, _ *http.Request) {
	snapshot := mc.GetSnapshot()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Write basic Prometheus metrics
	_, _ = fmt.Fprintf(w, "# HELP docbuilder_uptime_seconds Total uptime in seconds\n")
	_, _ = fmt.Fprintf(w, "# TYPE docbuilder_uptime_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "docbuilder_uptime_seconds %f\n", time.Since(mc.startTime).Seconds())

	// Counters
	for name, value := range snapshot.Counters {
		_, _ = fmt.Fprintf(w, "# HELP docbuilder_%s_total Total count of %s\n", name, name)
		_, _ = fmt.Fprintf(w, "# TYPE docbuilder_%s_total counter\n", name)
		_, _ = fmt.Fprintf(w, "docbuilder_%s_total %d\n", name, value)
	}

	// Gauges
	for name, value := range snapshot.Gauges {
		_, _ = fmt.Fprintf(w, "# HELP docbuilder_%s Current value of %s\n", name, name)
		_, _ = fmt.Fprintf(w, "# TYPE docbuilder_%s gauge\n", name)
		_, _ = fmt.Fprintf(w, "docbuilder_%s %d\n", name, value)
	}

	// System metrics
	sys := snapshot.SystemMetrics
	_, _ = fmt.Fprintf(w, "# HELP docbuilder_goroutines Number of goroutines\n")
	_, _ = fmt.Fprintf(w, "# TYPE docbuilder_goroutines gauge\n")
	_, _ = fmt.Fprintf(w, "docbuilder_goroutines %d\n", sys.Goroutines)

	_, _ = fmt.Fprintf(w, "# HELP docbuilder_memory_alloc_bytes Allocated memory in bytes\n")
	_, _ = fmt.Fprintf(w, "# TYPE docbuilder_memory_alloc_bytes gauge\n")
	_, _ = fmt.Fprintf(w, "docbuilder_memory_alloc_bytes %f\n", sys.MemAllocMB*1024*1024)
}
