package daemon

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	promcollect "github.com/prometheus/client_golang/prometheus/collectors"

	m "git.home.luguber.info/inful/docbuilder/internal/metrics"
)

var (
	promRegistry = prom.NewRegistry()
	// Export daemon build counters as Prometheus metrics (bridge pattern).
	daemonBuildsTotal       = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_total", Help: "Total builds processed by daemon"})
	daemonBuildsFailedTotal = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_failed_total", Help: "Failed builds processed by daemon"})
	// Gauges (scrape-time via GaugeFunc).
	daemonActiveJobsGauge = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_active_jobs", Help: "Number of build jobs currently running"}, func() float64 {
		if defaultDaemonInstance == nil {
			return 0
		}
		return float64(atomic.LoadInt32(&defaultDaemonInstance.activeJobs))
	})
	daemonQueueLengthGauge = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_queue_length", Help: "Current queued build jobs waiting for workers"}, func() float64 {
		if defaultDaemonInstance == nil {
			return 0
		}
		return float64(atomic.LoadInt32(&defaultDaemonInstance.queueLength))
	})
	// Last build snapshot gauges.
	daemonLastBuildRenderedPages = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_rendered_pages", Help: "Pages rendered in most recent completed build"}, func() float64 {
		return float64(atomic.LoadInt64(&lastRenderedPages))
	})
	daemonLastBuildRepositories = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_repositories", Help: "Repositories processed in most recent completed build"}, func() float64 {
		return float64(atomic.LoadInt64(&lastRepositories))
	})
)

var registerMetricsOnce sync.Once

// registerBaseCollectors registers base collectors once.
func registerBaseCollectors() {
	registerMetricsOnce.Do(func() {
		promRegistry.MustRegister(daemonBuildsTotal, daemonBuildsFailedTotal)
		promRegistry.MustRegister(daemonActiveJobsGauge, daemonQueueLengthGauge, daemonLastBuildRenderedPages, daemonLastBuildRepositories)
		promRegistry.MustRegister(promcollect.NewGoCollector(), promcollect.NewProcessCollector(promcollect.ProcessCollectorOpts{}))
	})
}

// updateDaemonPromMetrics copies selected counters from in-memory collector to Prometheus counters.
func updateDaemonPromMetrics(d *Daemon) {
	if d == nil || d.metrics == nil {
		return
	}
	snap := d.metrics.GetSnapshot()
	if v, ok := snap.Counters["build_completed_total"]; ok {
		prev := atomicLoadInt64(&lastCompleted)
		if v > prev {
			daemonBuildsTotal.Add(float64(v - prev))
			atomicStoreInt64(&lastCompleted, v)
		}
	}
	if v, ok := snap.Counters["build_failed_total"]; ok {
		prev := atomicLoadInt64(&lastFailed)
		if v > prev {
			daemonBuildsFailedTotal.Add(float64(v - prev))
			atomicStoreInt64(&lastFailed, v)
		}
	}
	// Update snapshot gauges from last build report via event-sourced projection (Phase B)
	if d.buildProjection != nil {
		if last := d.buildProjection.GetLastCompletedBuild(); last != nil && last.ReportData != nil {
			atomic.StoreInt64(&lastRenderedPages, int64(last.ReportData.RenderedPages))
			atomic.StoreInt64(&lastRepositories, int64(last.RepoCount))
		}
	}
}

var (
	lastCompleted     int64
	lastFailed        int64
	lastRenderedPages int64
	lastRepositories  int64
)

func atomicLoadInt64(p *int64) int64     { return atomic.LoadInt64(p) }
func atomicStoreInt64(p *int64, v int64) { atomic.StoreInt64(p, v) }

// prometheusOptionalHandler returns handler and periodically syncs daemon metrics.
func prometheusOptionalHandler() http.Handler {
	registerBaseCollectors()
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
