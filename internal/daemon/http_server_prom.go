package daemon

import (
	"net/http"
	"sync"
	"sync/atomic"

	prom "github.com/prometheus/client_golang/prometheus"
	promcollect "github.com/prometheus/client_golang/prometheus/collectors"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
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
		return float64(defaultDaemonInstance.queueLength.Load())
	})
	// Last build snapshot gauges.
	daemonLastBuildRenderedPages = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_rendered_pages", Help: "Pages rendered in most recent completed build"}, func() float64 {
		return float64(lastRenderedPages.Load())
	})
	daemonLastBuildRepositories = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_repositories", Help: "Repositories processed in most recent completed build"}, func() float64 {
		return float64(lastRepositories.Load())
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
			lastRenderedPages.Store(int64(last.ReportData.RenderedPages))
			lastRepositories.Store(int64(last.RepoCount))
		}
	}
}

var (
	lastCompleted     int64
	lastFailed        int64
	lastRenderedPages atomic.Int64
	lastRepositories  atomic.Int64
)

func atomicLoadInt64(p *int64) int64     { return atomic.LoadInt64(p) }
func atomicStoreInt64(p *int64, v int64) { atomic.StoreInt64(p, v) }

// prometheusOptionalHandler returns handler and periodically syncs daemon metrics.
func prometheusOptionalHandler() http.Handler {
	registerBaseCollectors()
	return promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{Registry: promRegistry})
}
