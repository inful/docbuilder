package hugo

import (
    "context"
    "fmt"
    "time"

    "git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Stage is a discrete unit of work in the site build.
type Stage func(ctx context.Context, bs *BuildState) error

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
func runStages(ctx context.Context, bs *BuildState, stages []struct{ name string; fn Stage }) error {
    for _, st := range stages {
        select {
        case <-ctx.Done():
            return fmt.Errorf("build canceled before stage %s: %w", st.name, ctx.Err())
        default:
        }
        t0 := time.Now()
        if err := st.fn(ctx, bs); err != nil {
            return fmt.Errorf("stage %s: %w", st.name, err)
        }
        bs.Timings[st.name] = time.Since(t0)
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
    if bs.Generator.config.Hugo.Theme == "" { // fallback layouts only when no theme set
        return bs.Generator.generateBasicLayouts()
    }
    return nil
}

func stageCopyContent(ctx context.Context, bs *BuildState) error {
    return bs.Generator.copyContentFiles(bs.Docs)
}

func stageIndexes(ctx context.Context, bs *BuildState) error {
    return bs.Generator.generateIndexPages(bs.Docs)
}

func stageRunHugo(ctx context.Context, bs *BuildState) error {
    if shouldRunHugo() {
        if err := bs.Generator.runHugoBuild(); err != nil {
            // Non-fatal now: record error in report and continue (mirrors prior behavior) by returning nil
            bs.Report.Errors = append(bs.Report.Errors, fmt.Errorf("hugo build failed: %w", err))
        }
    }
    return nil
}

func stagePostProcess(ctx context.Context, bs *BuildState) error { // placeholder for future extensions
    return nil
}
