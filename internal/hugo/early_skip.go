package hugo

import (
	"log/slog"
)

// EarlySkipDecision represents the result of evaluating early skip conditions.
type EarlySkipDecision struct {
	ShouldSkip bool
	Reason     string
	Stage      StageName // stage after which to skip
}

// NoSkip returns a decision to continue with all stages.
func NoSkip() EarlySkipDecision {
	return EarlySkipDecision{ShouldSkip: false}
}

// SkipAfter returns a decision to skip stages after the specified stage.
func SkipAfter(stage StageName, reason string) EarlySkipDecision {
	return EarlySkipDecision{
		ShouldSkip: true,
		Reason:     reason,
		Stage:      stage,
	}
}

// EvaluateEarlySkip performs early skip logic evaluation based on repository and site state.
func EvaluateEarlySkip(bs *BuildState) EarlySkipDecision {
	// Skip after clone if no repository changes and existing site is valid
	if bs.Git.AllReposUnchanged {
		if bs.Generator != nil && bs.Generator.existingSiteValidForSkip() {
			slog.Info("Early skip condition met: no repository HEAD changes and existing site valid")
			return SkipAfter(StageCloneRepos, "no_changes")
		}
	}

	// Future: Add other skip conditions (config unchanged, etc.)

	return NoSkip()
}
