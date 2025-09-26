package metrics

import "time"

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
	IncBuildOutcome(outcome string) // outcome: success|warning|failed|canceled
	ObserveCloneRepoDuration(repo string, d time.Duration, success bool)
	IncCloneRepoResult(success bool)
	SetCloneConcurrency(n int)
}

// NoopRecorder is a Recorder that does nothing (default when metrics not configured).
type NoopRecorder struct{}

func (NoopRecorder) ObserveStageDuration(string, time.Duration) {}
func (NoopRecorder) ObserveBuildDuration(time.Duration)         {}
func (NoopRecorder) IncStageResult(string, ResultLabel)         {}
func (NoopRecorder) IncBuildOutcome(string)                     {}
func (NoopRecorder) ObserveCloneRepoDuration(string, time.Duration, bool) {}
func (NoopRecorder) IncCloneRepoResult(bool)                    {}
func (NoopRecorder) SetCloneConcurrency(int)                    {}
