package hugo

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/metrics"
)

// BuildObserver receives callbacks around stage execution and build lifecycle.
// It intentionally abstracts away the metrics.Recorder so future observers
// (logging, tracing, notifications) can hook in without changing stage code.
type BuildObserver interface {
	OnStageStart(stage StageName)
	OnStageComplete(stage StageName, duration time.Duration, result StageResult)
	OnBuildComplete(report *BuildReport)
}

// NoopObserver is a no-op implementation.
type NoopObserver struct{}

func (NoopObserver) OnStageStart(_ StageName)                                    {}
func (NoopObserver) OnStageComplete(_ StageName, _ time.Duration, _ StageResult) {}
func (NoopObserver) OnBuildComplete(_ *BuildReport)                              {}

// recorderObserver adapts metrics.Recorder into a BuildObserver.
type recorderObserver struct{ recorder metrics.Recorder }

func (r recorderObserver) OnStageStart(_ StageName) {}
func (r recorderObserver) OnStageComplete(stage StageName, d time.Duration, _ StageResult) {
	if r.recorder != nil {
		r.recorder.ObserveStageDuration(string(stage), d)
	}
}
func (r recorderObserver) OnBuildComplete(report *BuildReport) {
	if r.recorder != nil {
		r.recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		r.recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
		// Emit structured issues
		for _, is := range report.Issues {
			r.recorder.IncIssue(string(is.Code), string(is.Stage), string(is.Severity), is.Transient)
		}
		// Record effective render mode if present
		if report.EffectiveRenderMode != "" {
			r.recorder.SetEffectiveRenderMode(report.EffectiveRenderMode)
		}
	}
}
