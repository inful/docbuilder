package metrics

import "time"

// Collector is the sink of the metrics.
//
// The daemon has one, and the recorder of the build writes into it.
type Collector interface {
	IncrementCounter(name string)
	AddCounter(name string, delta int64)
	SetGauge(name string, value int64)
	RecordHistogram(name string, value float64)
	SetCustomMetric(name string, value any)
}

// CollectorRecorder is a Recorder that writes into a collector.
//
// The recorder of the build was never given to anybody: every call went
// to the no-op recorder, and the metrics of the build stayed at zero.
type CollectorRecorder struct {
	Collector Collector
}

// ObserveStageDuration records the duration of a stage.
func (r CollectorRecorder) ObserveStageDuration(stage string, d time.Duration) {
	r.Collector.RecordHistogram("stage_duration_seconds_"+stage, d.Seconds())
}

// ObserveBuildDuration records the duration of a build.
func (r CollectorRecorder) ObserveBuildDuration(d time.Duration) {
	r.Collector.RecordHistogram("build_duration_seconds", d.Seconds())
}

// IncStageResult records the result of a stage.
func (r CollectorRecorder) IncStageResult(stage string, result ResultLabel) {
	r.Collector.IncrementCounter("stage_result_" + stage + "_" + string(result) + "_total")
}

// IncBuildOutcome records the outcome of a build.
func (r CollectorRecorder) IncBuildOutcome(outcome BuildOutcomeLabel) {
	r.Collector.IncrementCounter("build_outcome_" + string(outcome) + "_total")
}

// ObserveCloneRepoDuration records the duration of a clone.
//
// The name of the repository is not part of the name of the histogram:
// a portal with a hundred repositories would have a hundred histograms.
func (r CollectorRecorder) ObserveCloneRepoDuration(_ string, d time.Duration, _ bool) {
	r.Collector.RecordHistogram("clone_repo_duration_seconds", d.Seconds())
}

// IncCloneRepoResult records the result of a clone.
func (r CollectorRecorder) IncCloneRepoResult(success bool) {
	if success {
		r.Collector.IncrementCounter("clone_repo_success_total")
		return
	}
	r.Collector.IncrementCounter("clone_repo_failure_total")
}

// SetCloneConcurrency records how many repositories are cloned at once.
func (r CollectorRecorder) SetCloneConcurrency(n int) {
	r.Collector.SetGauge("clone_concurrency", int64(n))
}

// IncBuildRetry records a retry of a stage.
func (r CollectorRecorder) IncBuildRetry(stage string) {
	r.Collector.IncrementCounter("build_retry_" + stage + "_total")
}

// IncBuildRetryExhausted records a stage whose retries are used up.
func (r CollectorRecorder) IncBuildRetryExhausted(stage string) {
	r.Collector.IncrementCounter("build_retry_exhausted_" + stage + "_total")
}

// IncIssue records an issue of a build.
func (r CollectorRecorder) IncIssue(code string, stage string, _ string, _ bool) {
	r.Collector.IncrementCounter("issue_" + code + "_" + stage + "_total")
}

// SetEffectiveRenderMode records the mode the build renders with.
func (r CollectorRecorder) SetEffectiveRenderMode(mode string) {
	r.Collector.SetCustomMetric("effective_render_mode", mode)
}

// IncContentTransformFailure records a transform of the content that failed.
func (r CollectorRecorder) IncContentTransformFailure(name string) {
	r.Collector.IncrementCounter("content_transform_failure_" + name + "_total")
}

// ObserveContentTransformDuration records the duration of a transform of
// the content.
func (r CollectorRecorder) ObserveContentTransformDuration(name string, d time.Duration, _ bool) {
	r.Collector.RecordHistogram("content_transform_duration_seconds_"+name, d.Seconds())
}
