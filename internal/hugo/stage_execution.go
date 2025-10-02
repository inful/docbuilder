package hugo

import (
	"context"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/metrics"
)

// StageExecution represents the structured result of a stage execution
type StageExecution struct {
	Err  error // error encountered during stage execution
	Skip bool  // whether subsequent stages should be skipped
}

// ExecutionSuccess returns a successful stage execution result
func ExecutionSuccess() StageExecution {
	return StageExecution{}
}

// ExecutionSuccessWithSkip returns a successful stage execution that skips remaining stages
func ExecutionSuccessWithSkip() StageExecution {
	return StageExecution{Skip: true}
}

// ExecutionFailure returns a failed stage execution result
func ExecutionFailure(err error) StageExecution {
	return StageExecution{Err: err}
}

// ExecutionFailureWithSkip returns a failed stage execution that skips remaining stages
func ExecutionFailureWithSkip(err error) StageExecution {
	return StageExecution{Err: err, Skip: true}
}

// IsSuccess returns true if the stage completed successfully (no error)
func (r StageExecution) IsSuccess() bool {
	return r.Err == nil
}

// ShouldSkip returns true if subsequent stages should be skipped
func (r StageExecution) ShouldSkip() bool {
	return r.Skip
}

// StageExecutor defines the signature for stage functions with structured results
type StageExecutor func(ctx context.Context, bs *BuildState) StageExecution

// StageDecorator defines functions that can wrap/decorate stage functions
type StageDecorator func(StageName, StageExecutor) StageExecutor

// WithTiming decorates a stage function to measure execution duration
func WithTiming(name StageName, fn StageExecutor) StageExecutor {
	return func(ctx context.Context, bs *BuildState) StageExecution {
		if bs.Generator != nil && bs.Generator.recorder != nil {
			defer func(start time.Time) {
				bs.Generator.recorder.ObserveStageDuration(string(name), time.Since(start))
			}(time.Now())
		}
		return fn(ctx, bs)
	}
}

// WithObserver decorates a stage function to record stage results
func WithObserver(name StageName, fn StageExecutor) StageExecutor {
	return func(ctx context.Context, bs *BuildState) StageExecution {
		result := fn(ctx, bs)
		if bs.Generator != nil && bs.Generator.recorder != nil {
			var label metrics.ResultLabel
			if result.IsSuccess() {
				label = metrics.ResultSuccess
			} else {
				label = metrics.ResultFatal
			}
			bs.Generator.recorder.IncStageResult(string(name), label)
		}
		return result
	}
}

// WrapLegacyStage wraps a legacy stage function (error return) as a StageExecutor
func WrapLegacyStage(fn func(ctx context.Context, bs *BuildState) error) StageExecutor {
	return func(ctx context.Context, bs *BuildState) StageExecution {
		if err := fn(ctx, bs); err != nil {
			return ExecutionFailure(err)
		}
		return ExecutionSuccess()
	}
}
