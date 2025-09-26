//go:build prometheus

package daemon

import (
	"net/http"
	"sync/atomic"
	"time"

	m "git.home.luguber.info/inful/docbuilder/internal/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
	promcollect "github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	promRegistry = prom.NewRegistry()
	// Export daemon build counters as Prometheus metrics (bridge pattern)
	daemonBuildsTotal       = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_total", Help: "Total builds processed by daemon"})
	daemonBuildsFailedTotal = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_failed_total", Help: "Failed builds processed by daemon"})
	// Gauges (scrape-time via GaugeFunc)
	daemonActiveJobsGauge = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_active_jobs", Help: "Number of build jobs currently running"}, func() float64 {
		if defaultDaemonInstance == nil { return 0 }
		return float64(atomic.LoadInt32(&defaultDaemonInstance.activeJobs))
	})
	daemonQueueLengthGauge = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_queue_length", Help: "Current queued build jobs waiting for workers"}, func() float64 {
		if defaultDaemonInstance == nil { return 0 }
		return float64(atomic.LoadInt32(&defaultDaemonInstance.queueLength))
	})
)

// register base collectors once
func init() {
	promRegistry.MustRegister(daemonBuildsTotal, daemonBuildsFailedTotal)
	promRegistry.MustRegister(daemonActiveJobsGauge, daemonQueueLengthGauge)
	promRegistry.MustRegister(promcollect.NewGoCollector(), promcollect.NewProcessCollector(promcollect.ProcessCollectorOpts{}))
}

// updateDaemonPromMetrics copies selected counters from in-memory collector to Prometheus counters.
func updateDaemonPromMetrics(d *Daemon) {
	if d == nil || d.metrics == nil { return }
	snap := d.metrics.GetSnapshot()
	if v, ok := snap.Counters["build_completed_total"]; ok {
		// Prometheus counters are monotonically increasing; compute delta naive (approx) by adding difference.
		// For simplicity (first pass) just set via Add difference from a stored last value in custom metric map.
		prev := atomicLoadInt64(&lastCompleted)
		if v > prev { daemonBuildsTotal.Add(float64(v - prev)); atomicStoreInt64(&lastCompleted, v) }
	}
	if v, ok := snap.Counters["build_failed_total"]; ok {
		prev := atomicLoadInt64(&lastFailed)
		if v > prev { daemonBuildsFailedTotal.Add(float64(v - prev)); atomicStoreInt64(&lastFailed, v) }
	}
}

var lastCompleted int64
var lastFailed int64

func atomicLoadInt64(p *int64) int64 { return atomic.LoadInt64(p) }
func atomicStoreInt64(p *int64, v int64) { atomic.StoreInt64(p, v) }

// prometheusOptionalHandler returns handler and periodically syncs daemon metrics.
func prometheusOptionalHandler() http.Handler {
	go func() {
		for {
			if defaultDaemonInstance != nil { // global pointer we establish in daemon init
				updateDaemonPromMetrics(defaultDaemonInstance)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return m.HTTPHandler(promRegistry)
}
