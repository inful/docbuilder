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
func (NoopObserver) OnStageStart(stage StageName)                               {}
func (NoopObserver) OnStageComplete(stage StageName, d time.Duration, r StageResult) {}
func (NoopObserver) OnBuildComplete(report *BuildReport)                        {}

// recorderObserver adapts metrics.Recorder into a BuildObserver.
type recorderObserver struct { rec metrics.Recorder }
func (r recorderObserver) OnStageStart(stage StageName) {}
func (r recorderObserver) OnStageComplete(stage StageName, d time.Duration, _ StageResult) {
  if r.rec != nil { r.rec.ObserveStageDuration(string(stage), d) }
}
func (r recorderObserver) OnBuildComplete(report *BuildReport) {
  if r.rec != nil {
    r.rec.ObserveBuildDuration(report.End.Sub(report.Start))
    r.rec.IncBuildOutcome(metrics.BuildOutcomeLabel(report.Outcome))
  }
}
