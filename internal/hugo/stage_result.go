package hugo

import "git.home.luguber.info/inful/docbuilder/internal/metrics"

// StageResult enumerates per-stage classification outcomes.
// Mirrors metrics.ResultLabel values to simplify emission.
type StageResult string

const (
	StageResultSuccess  StageResult = "success"
	StageResultWarning  StageResult = "warning"
	StageResultFatal    StageResult = "fatal"
	StageResultCanceled StageResult = "canceled"
)

// recordStageResult updates BuildReport counters and emits metrics (if recorder non-nil).
func (r *BuildReport) recordStageResult(stage StageName, res StageResult, recorder metrics.Recorder) {
	sc := r.StageCounts[stage]
	switch res {
	case StageResultSuccess:
		sc.Success++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultSuccess)
		}
	case StageResultWarning:
		sc.Warning++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultWarning)
		}
	case StageResultFatal:
		sc.Fatal++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultFatal)
		}
	case StageResultCanceled:
		sc.Canceled++
		if recorder != nil {
			recorder.IncStageResult(string(stage), metrics.ResultCanceled)
		}
	}
	r.StageCounts[stage] = sc
}
