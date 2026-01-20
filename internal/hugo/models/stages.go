package models

import (
	"context"
	stdErrors "errors"
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// Stage is a discrete unit of work in the site build.
type Stage func(ctx context.Context, bs *BuildState) error

// StageName is a strongly-typed identifier for a build stage.
type StageName string

// Canonical stage names.
const (
	StagePrepareOutput  StageName = "prepare_output"
	StageCloneRepos     StageName = "clone_repos"
	StageDiscoverDocs   StageName = "discover_docs"
	StageGenerateConfig StageName = "generate_config"
	StageLayouts        StageName = "layouts"
	StageCopyContent    StageName = "copy_content"
	StageIndexes        StageName = "indexes"
	StageRunHugo        StageName = "run_hugo"
	StagePostProcess    StageName = "post_process"
)

// StageErrorKind classifies the outcome of a stage.
type StageErrorKind string

const (
	StageErrorFatal    StageErrorKind = "fatal"    // Build must abort.
	StageErrorWarning  StageErrorKind = "warning"  // Non-fatal; record and continue.
	StageErrorCanceled StageErrorKind = "canceled" // Context cancellation.
)

// StageError is a structured error carrying category and underlying cause.
type StageError struct {
	Kind  StageErrorKind
	Stage StageName
	Err   error
}

func (e *StageError) Error() string { return fmt.Sprintf("%s stage %s: %v", e.Kind, e.Stage, e.Err) }
func (e *StageError) Unwrap() error { return e.Err }

// Transient reports whether the underlying error condition is likely transient.
func (e *StageError) Transient() bool {
	if e == nil {
		return false
	}
	if e.Kind == StageErrorCanceled {
		return false
	}
	cause := e.Err
	isSentinel := func(target error) bool { return stdErrors.Is(cause, target) }
	switch e.Stage {
	case StageCloneRepos:
		if isSentinel(ErrClone) {
			return true
		}
		// Use structured classification for transient errors
		if ce, ok := errors.AsClassified(cause); ok && ce.RetryStrategy() != errors.RetryNever {
			return true
		}
	case StageRunHugo:
		if isSentinel(ErrHugo) {
			return true
		}
	case StageDiscoverDocs:
		if isSentinel(ErrDiscovery) {
			return e.Kind == StageErrorWarning
		}
	case StagePrepareOutput, StageGenerateConfig, StageLayouts, StageCopyContent, StageIndexes, StagePostProcess:
		return false
	}
	return false
}

// StageResult captures the high-level outcome of a stage.
type StageResult string

const (
	StageResultSuccess  StageResult = "success"
	StageResultWarning  StageResult = "warning"
	StageResultFatal    StageResult = "fatal"
	StageResultCanceled StageResult = "canceled"
	StageResultSkipped  StageResult = "skipped"
)

// NewFatalStageError creates a new fatal stage error.
func NewFatalStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorFatal, Stage: stage, Err: err}
}

func NewWarnStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorWarning, Stage: stage, Err: err}
}

func NewCanceledStageError(stage StageName, err error) *StageError {
	return &StageError{Kind: StageErrorCanceled, Stage: stage, Err: err}
}

// StageDef pairs a stage name with its executing function.
type StageDef struct {
	Name StageName
	Fn   Stage
}

// Pipeline is a fluent builder for ordered stage definitions.
type Pipeline struct{ Defs []StageDef }

// NewPipeline creates an empty pipeline.
func NewPipeline() *Pipeline { return &Pipeline{Defs: make([]StageDef, 0, 8)} }

// Add appends a stage unconditionally.
func (p *Pipeline) Add(name StageName, fn Stage) *Pipeline {
	p.Defs = append(p.Defs, StageDef{Name: name, Fn: fn})
	return p
}

// AddIf appends a stage only if cond is true.
func (p *Pipeline) AddIf(cond bool, name StageName, fn Stage) *Pipeline {
	if cond {
		p.Add(name, fn)
	}
	return p
}

// Build returns a defensive copy of the stage definitions slice.
func (p *Pipeline) Build() []StageDef {
	out := make([]StageDef, len(p.Defs))
	copy(out, p.Defs)
	return out
}
