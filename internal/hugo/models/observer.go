package models

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

// RecorderObserver adapts metrics.Recorder into a BuildObserver.
type RecorderObserver struct{ Recorder metrics.Recorder }

func (r RecorderObserver) OnStageStart(_ StageName) {}
func (r RecorderObserver) OnStageComplete(stage StageName, d time.Duration, _ StageResult) {
	if r.Recorder != nil {
		r.Recorder.ObserveStageDuration(string(stage), d)
	}
}

func (r RecorderObserver) OnBuildComplete(report *BuildReport) {
	if r.Recorder != nil {
		r.Recorder.ObserveBuildDuration(report.End.Sub(report.Start))
		r.Recorder.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
		// Emit structured issues
		for _, is := range report.Issues {
			r.Recorder.IncIssue(string(is.Code), string(is.Stage), string(is.Severity), is.Transient)
		}
		// Record effective render mode if present
		if report.EffectiveRenderMode != "" {
			r.Recorder.SetEffectiveRenderMode(report.EffectiveRenderMode)
		}
	}
}
