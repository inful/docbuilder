package daemon

import (
	"net/http"
	"sync/atomic"

	prom "github.com/prometheus/client_golang/prometheus"
	promcollect "github.com/prometheus/client_golang/prometheus/collectors"

	m "git.home.luguber.info/inful/docbuilder/internal/metrics"
)

var (
	// Export daemon build counters as Prometheus metrics (bridge pattern).
	daemonBuildsTotal       = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_total", Help: "Total builds processed by daemon"})
	daemonBuildsFailedTotal = prom.NewCounter(prom.CounterOpts{Namespace: "docbuilder", Name: "daemon_builds_failed_total", Help: "Failed builds processed by daemon"})
	// Last build snapshot gauges.
	daemonLastBuildRenderedPages = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_rendered_pages", Help: "Pages rendered in most recent completed build"}, func() float64 {
		return float64(lastRenderedPages.Load())
	})
	daemonLastBuildRepositories = prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_last_build_repositories", Help: "Repositories processed in most recent completed build"}, func() float64 {
		return float64(lastRepositories.Load())
	})
)

// updateDaemonPromMetrics copies selected counters from in-memory collector to Prometheus counters.
func updateDaemonPromMetrics(d *Daemon) {
	if d == nil || d.serve.metrics == nil {
		return
	}
	snap := d.serve.metrics.GetSnapshot()
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
	if d.state.buildProjection != nil {
		if last := d.state.buildProjection.GetLastCompletedBuild(); last != nil && last.ReportData != nil {
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

// prometheusOptionalHandler returns the Prometheus handler for a daemon.
// The queue gauges are bound to that daemon, so two daemons in one process
// each report their own numbers.
func prometheusOptionalHandler(d *Daemon) http.Handler {
	registry := prom.NewRegistry()
	registry.MustRegister(daemonBuildsTotal, daemonBuildsFailedTotal)
	registry.MustRegister(
		prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_active_jobs", Help: "Number of build jobs currently running"}, func() float64 {
			return float64(d.GetActiveJobs())
		}),
		prom.NewGaugeFunc(prom.GaugeOpts{Namespace: "docbuilder", Name: "daemon_queue_length", Help: "Current queued build jobs waiting for workers"}, func() float64 {
			return float64(d.GetQueueLength())
		}),
		daemonLastBuildRenderedPages,
		daemonLastBuildRepositories,
	)
	registry.MustRegister(promcollect.NewGoCollector(), promcollect.NewProcessCollector(promcollect.ProcessCollectorOpts{}))
	return m.HTTPHandler(registry)
}
