package metrics

import (
	"sync"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
)

// PrometheusRecorder implements Recorder using Prometheus metrics.
type PrometheusRecorder struct {
	once             sync.Once
	stageDuration    *prom.HistogramVec
	buildDuration    prom.Histogram
	stageResults     *prom.CounterVec
	buildOutcome     *prom.CounterVec
	cloneDuration    *prom.HistogramVec
	cloneResults     *prom.CounterVec
	cloneConcurrency prom.Gauge
	retries          *prom.CounterVec
	retriesExhausted *prom.CounterVec
}

// NewPrometheusRecorder constructs and registers Prometheus metrics (idempotent).
func NewPrometheusRecorder(reg *prom.Registry) *PrometheusRecorder {
	if reg == nil {
		reg = prom.NewRegistry()
	}
	pr := &PrometheusRecorder{}
	pr.once.Do(func() {
		pr.stageDuration = prom.NewHistogramVec(prom.HistogramOpts{
			Namespace: "docbuilder",
			Name:      "stage_duration_seconds",
			Help:      "Duration of individual build stages",
			Buckets:   prom.DefBuckets,
		}, []string{"stage"})
		pr.buildDuration = prom.NewHistogram(prom.HistogramOpts{
			Namespace: "docbuilder",
			Name:      "build_duration_seconds",
			Help:      "Total build duration",
			Buckets:   prom.DefBuckets,
		})
		pr.stageResults = prom.NewCounterVec(prom.CounterOpts{
			Namespace: "docbuilder",
			Name:      "stage_results_total",
			Help:      "Stage result counts by outcome",
		}, []string{"stage", "result"})
		pr.buildOutcome = prom.NewCounterVec(prom.CounterOpts{
			Namespace: "docbuilder",
			Name:      "build_outcomes_total",
			Help:      "Build outcomes by final status",
		}, []string{"outcome"})
		pr.cloneDuration = prom.NewHistogramVec(prom.HistogramOpts{
			Namespace: "docbuilder",
			Name:      "clone_repo_duration_seconds",
			Help:      "Duration of individual repository clone operations",
			Buckets:   prom.DefBuckets,
		}, []string{"repo", "result"})
		pr.cloneResults = prom.NewCounterVec(prom.CounterOpts{
			Namespace: "docbuilder",
			Name:      "clone_repo_results_total",
			Help:      "Clone results by success/failure",
		}, []string{"result"})
		pr.cloneConcurrency = prom.NewGauge(prom.GaugeOpts{
			Namespace: "docbuilder",
			Name:      "clone_concurrency",
			Help:      "Observed clone concurrency for the last build stage",
		})
		pr.retries = prom.NewCounterVec(prom.CounterOpts{
			Namespace: "docbuilder",
			Name:      "build_retries_total",
			Help:      "Total build stage retries (transient failures)",
		}, []string{"stage"})
		pr.retriesExhausted = prom.NewCounterVec(prom.CounterOpts{
			Namespace: "docbuilder",
			Name:      "build_retry_exhausted_total",
			Help:      "Count of stages where retries were exhausted", 
		}, []string{"stage"})
		reg.MustRegister(pr.stageDuration, pr.buildDuration, pr.stageResults, pr.buildOutcome, pr.cloneDuration, pr.cloneResults, pr.cloneConcurrency, pr.retries, pr.retriesExhausted)
	})
	return pr
}

func (p *PrometheusRecorder) ObserveStageDuration(stage string, d time.Duration) {
	if p == nil || p.stageDuration == nil {
		return
	}
	p.stageDuration.WithLabelValues(stage).Observe(d.Seconds())
}
func (p *PrometheusRecorder) ObserveBuildDuration(d time.Duration) {
	if p == nil || p.buildDuration == nil {
		return
	}
	p.buildDuration.Observe(d.Seconds())
}
func (p *PrometheusRecorder) IncStageResult(stage string, result ResultLabel) {
	if p == nil || p.stageResults == nil {
		return
	}
	p.stageResults.WithLabelValues(stage, string(result)).Inc()
}
func (p *PrometheusRecorder) IncBuildOutcome(outcome BuildOutcomeLabel) {
	if p == nil || p.buildOutcome == nil {
		return
	}
	p.buildOutcome.WithLabelValues(string(outcome)).Inc()
}

func (p *PrometheusRecorder) ObserveCloneRepoDuration(repo string, d time.Duration, success bool) {
	if p == nil || p.cloneDuration == nil {
		return
	}
	res := "failed"
	if success {
		res = "success"
	}
	p.cloneDuration.WithLabelValues(repo, res).Observe(d.Seconds())
}

func (p *PrometheusRecorder) IncCloneRepoResult(success bool) {
	if p == nil || p.cloneResults == nil {
		return
	}
	res := "failed"
	if success {
		res = "success"
	}
	p.cloneResults.WithLabelValues(res).Inc()
}

func (p *PrometheusRecorder) SetCloneConcurrency(n int) {
	if p == nil || p.cloneConcurrency == nil {
		return
	}
	p.cloneConcurrency.Set(float64(n))
}

func (p *PrometheusRecorder) IncBuildRetry(stage string) {
	if p == nil || p.retries == nil {
		return
	}
	p.retries.WithLabelValues(stage).Inc()
}

func (p *PrometheusRecorder) IncBuildRetryExhausted(stage string) {
	if p == nil || p.retriesExhausted == nil {
		return
	}
	p.retriesExhausted.WithLabelValues(stage).Inc()
}
