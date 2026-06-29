package models

import (
	"time"
)

// BuildObserver receives callbacks around stage execution and build lifecycle.
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

// Compile-time assertion that the canonical observer implementations satisfy
// the BuildObserver contract.
var _ BuildObserver = NoopObserver{}
