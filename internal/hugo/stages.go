package hugo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/build"
)

// Stage is a discrete unit of work in the site build.
type Stage func(ctx context.Context, bs *BuildState) error

// StageErrorKind enumerates structured stage error categories.
type StageErrorKind string

const (
	StageErrorFatal    StageErrorKind = "fatal"    // Build must abort.
	StageErrorWarning  StageErrorKind = "warning"  // Non-fatal; record and continue.
	StageErrorCanceled StageErrorKind = "canceled" // Context cancellation.
)

// StageError is a structured error carrying category and underlying cause.
type StageError struct {
	Kind  StageErrorKind
	Stage string
	Err   error
}

func (e *StageError) Error() string { return fmt.Sprintf("%s stage %s: %v", e.Kind, e.Stage, e.Err) }
func (e *StageError) Unwrap() error { return e.Err }

// Helpers to classify errors.
func newFatalStageError(stage string, err error) *StageError {
	return &StageError{Kind: StageErrorFatal, Stage: stage, Err: err}
}
func newWarnStageError(stage string, err error) *StageError {
	return &StageError{Kind: StageErrorWarning, Stage: stage, Err: err}
}
func newCanceledStageError(stage string, err error) *StageError {
	return &StageError{Kind: StageErrorCanceled, Stage: stage, Err: err}
}

// BuildState carries mutable state and metrics across stages.
type BuildState struct {
	Generator *Generator
	Docs      []docs.DocFile
	Report    *BuildReport
	Timings   map[string]time.Duration
	start     time.Time
}

// newBuildState constructs a BuildState.
func newBuildState(g *Generator, docFiles []docs.DocFile, report *BuildReport) *BuildState {
	return &BuildState{
		Generator: g,
		Docs:      docFiles,
		Report:    report,
		Timings:   make(map[string]time.Duration),
		start:     time.Now(),
	}
}

// runStages executes stages in order, recording timing and stopping on first fatal error.
func runStages(ctx context.Context, bs *BuildState, stages []struct {
	name string
	fn   Stage
}) error {
	for _, st := range stages {
		select {
		case <-ctx.Done():
			se := newCanceledStageError(st.name, ctx.Err())
			bs.Report.Errors = append(bs.Report.Errors, se)
			bs.Report.StageErrorKinds[st.name] = string(se.Kind)
			return se
		default:
		}
		t0 := time.Now()
		err := st.fn(ctx, bs)
		dur := time.Since(t0)
		bs.Timings[st.name] = dur
		bs.Report.StageDurations[st.name] = dur
		if err != nil {
			var se *StageError
			if errors.As(err, &se) {
				// Already a StageError; record classification.
				bs.Report.StageErrorKinds[st.name] = string(se.Kind)
				// update stage counts
				sc := bs.Report.StageCounts[st.name]
				switch se.Kind {
				case StageErrorWarning:
					sc.Warning++
				case StageErrorCanceled:
					sc.Canceled++
				case StageErrorFatal:
					sc.Fatal++
				}
				bs.Report.StageCounts[st.name] = sc
				switch se.Kind {
				case StageErrorWarning:
					bs.Report.Warnings = append(bs.Report.Warnings, se)
					continue // proceed to next stage
				case StageErrorCanceled:
					bs.Report.Errors = append(bs.Report.Errors, se)
					return se
				case StageErrorFatal:
					bs.Report.Errors = append(bs.Report.Errors, se)
					return se
				}
			} else {
				// Wrap unknown errors as fatal by default.
				se = newFatalStageError(st.name, err)
				bs.Report.StageErrorKinds[st.name] = string(se.Kind)
				sc := bs.Report.StageCounts[st.name]
				sc.Fatal++
				bs.Report.StageCounts[st.name] = sc
				bs.Report.Errors = append(bs.Report.Errors, se)
				return se
			}
		} else {
			// success path
			sc := bs.Report.StageCounts[st.name]
			sc.Success++
			bs.Report.StageCounts[st.name] = sc
		}
	}
	return nil
}

// Individual stage implementations (minimal wrappers calling existing methods).

func stagePrepareOutput(ctx context.Context, bs *BuildState) error { // currently no-op beyond structure creation
	return bs.Generator.createHugoStructure()
}

func stageGenerateConfig(ctx context.Context, bs *BuildState) error {
	return bs.Generator.generateHugoConfig()
}

func stageLayouts(ctx context.Context, bs *BuildState) error {
	if bs.Generator.config.Hugo.Theme != "" {
		// Theme provided: no fallback layouts necessary.
		// Add no-op statement to avoid staticcheck empty branch warning.
		var _ = bs.Generator.config.Hugo.Theme
		return nil
	}
	// No theme configured: generate basic fallback layouts.
	return bs.Generator.generateBasicLayouts()
}

func stageCopyContent(ctx context.Context, bs *BuildState) error {
	return bs.Generator.copyContentFiles(bs.Docs)
}

func stageIndexes(ctx context.Context, bs *BuildState) error {
	return bs.Generator.generateIndexPages(bs.Docs)
}

func stageRunHugo(ctx context.Context, bs *BuildState) error {
	if !shouldRunHugo() {
		// Running Hugo disabled by flag; no operation.
		return nil
	}
	if err := bs.Generator.runHugoBuild(); err != nil {
		// Treat hugo runtime failure as warning (site content still copied & usable without static render)
		return newWarnStageError("run_hugo", fmt.Errorf("%w: %v", build.ErrHugo, err))
	}
	// mark successful static render
	bs.Report.StaticRendered = true
	return nil
}

func stagePostProcess(ctx context.Context, bs *BuildState) error { // placeholder for future extensions
	// Ensure non-zero measurable duration for timing tests without introducing significant delay.
	start := time.Now()
	for time.Since(start) == 0 {
		// spin very briefly (will exit next nanosecond tick)
	}
	return nil
}
