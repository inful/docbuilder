package metrics

import "time"

// BuildOutcomeLabel is used for build outcome metrics dimensions.
type BuildOutcomeLabel string

const (
	BuildOutcomeSuccess  BuildOutcomeLabel = "success"
	BuildOutcomeWarning  BuildOutcomeLabel = "warning"
	BuildOutcomeFailed   BuildOutcomeLabel = "failed"
	BuildOutcomeCanceled BuildOutcomeLabel = "canceled"
	BuildOutcomeSkipped  BuildOutcomeLabel = "skipped"
)

// ResultLabel enumerates stage result categories for counters.
type ResultLabel string

const (
	ResultSuccess  ResultLabel = "success"
	ResultWarning  ResultLabel = "warning"
	ResultFatal    ResultLabel = "fatal"
	ResultCanceled ResultLabel = "canceled"
)

// Recorder defines observability hooks for build and stage metrics. Implementations
// may forward to Prometheus, OpenTelemetry, etc. All methods must be safe for nil receivers
// when using the NoopRecorder (allowing optional injection).
type Recorder interface {
	ObserveStageDuration(stage string, d time.Duration)
	ObserveBuildDuration(d time.Duration)
	IncStageResult(stage string, result ResultLabel)
	IncBuildOutcome(outcome BuildOutcomeLabel) // outcome: success|warning|failed|canceled
	ObserveCloneRepoDuration(repo string, d time.Duration, success bool)
	IncCloneRepoResult(success bool)
	SetCloneConcurrency(n int)
	IncBuildRetry(stage string)
	IncBuildRetryExhausted(stage string)
	// New extensibility points (additive; safe for older noop implementations)
	IncIssue(code string, stage string, severity string, transient bool)
	SetEffectiveRenderMode(mode string)
	IncContentTransformFailure(name string)
	ObserveContentTransformDuration(name string, d time.Duration, success bool)
}

// NoopRecorder is a Recorder that does nothing (default when metrics not configured).
type NoopRecorder struct{}

func (NoopRecorder) ObserveStageDuration(string, time.Duration)                  {}
func (NoopRecorder) ObserveBuildDuration(time.Duration)                          {}
func (NoopRecorder) IncStageResult(string, ResultLabel)                          {}
func (NoopRecorder) IncBuildOutcome(BuildOutcomeLabel)                           {}
func (NoopRecorder) ObserveCloneRepoDuration(string, time.Duration, bool)        {}
func (NoopRecorder) IncCloneRepoResult(bool)                                     {}
func (NoopRecorder) SetCloneConcurrency(int)                                     {}
func (NoopRecorder) IncBuildRetry(string)                                        {}
func (NoopRecorder) IncBuildRetryExhausted(string)                               {}
func (NoopRecorder) IncIssue(string, string, string, bool)                       {}
func (NoopRecorder) SetEffectiveRenderMode(string)                               {}
func (NoopRecorder) IncContentTransformFailure(string)                           {}
func (NoopRecorder) ObserveContentTransformDuration(string, time.Duration, bool) {}
