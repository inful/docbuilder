package hugo

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

// StageExecutor and decorators removed. Timing and observation are handled centrally
// in the runner for error-returning stages. Structured StageExecution is used by
// command-style stages and they can record as needed within Execute.
