package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/commands"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/middleware"
)

// Pipeline orchestrates the execution of stage commands in dependency order.
// This implements the Command pattern for build stage management.
type Pipeline struct {
	registry     *commands.CommandRegistry
	middleware   []middleware.Middleware
	stopOnError  bool
	parallelized bool
}

// PipelineOption configures pipeline behavior.
type PipelineOption func(*Pipeline)

// WithMiddleware adds middleware to the pipeline.
func WithMiddleware(mw ...middleware.Middleware) PipelineOption {
	return func(p *Pipeline) {
		p.middleware = append(p.middleware, mw...)
	}
}

// WithStopOnError configures whether the pipeline stops on first error.
func WithStopOnError(stop bool) PipelineOption {
	return func(p *Pipeline) {
		p.stopOnError = stop
	}
}

// WithParallel enables parallel execution of independent stages.
func WithParallel(parallel bool) PipelineOption {
	return func(p *Pipeline) {
		p.parallelized = parallel
	}
}

// NewPipeline creates a new stage pipeline.
func NewPipeline(registry *commands.CommandRegistry, options ...PipelineOption) *Pipeline {
	p := &Pipeline{
		registry:    registry,
		stopOnError: true,
		middleware:  middleware.DefaultMiddleware(),
	}

	for _, opt := range options {
		opt(p)
	}

	return p
}

// ExecutionPlan represents the planned execution order of commands.
type ExecutionPlan struct {
	Order []hugo.StageName
	Graph map[hugo.StageName][]hugo.StageName // adjacency list of dependencies
}

// BuildExecutionPlan creates an execution plan based on command dependencies.
func (p *Pipeline) BuildExecutionPlan(stages []hugo.StageName) (*ExecutionPlan, error) {
	if len(stages) == 0 {
		return &ExecutionPlan{Order: []hugo.StageName{}, Graph: make(map[hugo.StageName][]hugo.StageName)}, nil
	}

	// Validate all stages exist
	for _, stage := range stages {
		if _, exists := p.registry.Get(stage); !exists {
			return nil, fmt.Errorf("stage %s not found in registry", stage)
		}
	}

	// Build dependency graph
	graph := make(map[hugo.StageName][]hugo.StageName)
	inDegree := make(map[hugo.StageName]int)

	// Initialize with requested stages
	stageSet := make(map[hugo.StageName]bool)
	for _, stage := range stages {
		stageSet[stage] = true
	}

	// Add dependencies transitively
	var addDependencies func(hugo.StageName) error
	addDependencies = func(stage hugo.StageName) error {
		cmd, exists := p.registry.Get(stage)
		if !exists {
			return fmt.Errorf("dependency %s not found", stage)
		}

		for _, dep := range cmd.Dependencies() {
			if !stageSet[dep] {
				stageSet[dep] = true
				if err := addDependencies(dep); err != nil {
					return err
				}
			}
			graph[dep] = append(graph[dep], stage)
		}
		return nil
	}

	for _, stage := range stages {
		if err := addDependencies(stage); err != nil {
			return nil, fmt.Errorf("resolving dependencies for %s: %w", stage, err)
		}
	}

	// Calculate in-degrees
	for stage := range stageSet {
		inDegree[stage] = 0
	}
	for _, dependents := range graph {
		for _, dependent := range dependents {
			inDegree[dependent]++
		}
	}

	// Topological sort
	var order []hugo.StageName
	queue := make([]hugo.StageName, 0)

	// Start with stages that have no dependencies
	for stage, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, stage)
		}
	}

	// Sort queue for deterministic order
	sort.Slice(queue, func(i, j int) bool {
		return string(queue[i]) < string(queue[j])
	})

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		// Process dependents
		dependents := graph[current]
		sort.Slice(dependents, func(i, j int) bool {
			return string(dependents[i]) < string(dependents[j])
		})

		for _, dependent := range dependents {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for cycles
	if len(order) != len(stageSet) {
		return nil, fmt.Errorf("circular dependency detected among stages")
	}

	return &ExecutionPlan{Order: order, Graph: graph}, nil
}

// Execute runs the pipeline with the given stages.
func (p *Pipeline) Execute(ctx context.Context, bs *hugo.BuildState, stages ...hugo.StageName) (*ExecutionResult, error) {
	// Build execution plan
	plan, err := p.BuildExecutionPlan(stages)
	if err != nil {
		return nil, fmt.Errorf("building execution plan: %w", err)
	}

	slog.Info("Executing pipeline",
		slog.Int("stages", len(plan.Order)),
		slog.Any("order", plan.Order))

	result := &ExecutionResult{
		ExecutedStages: make(map[hugo.StageName]hugo.StageExecution),
		Plan:           plan,
	}

	// Execute stages in dependency order
	for _, stageName := range plan.Order {
		select {
		case <-ctx.Done():
			result.ExecutedStages[stageName] = hugo.ExecutionFailure(ctx.Err())
			result.Canceled = true
			return result, ctx.Err()
		default:
		}

		cmd, exists := p.registry.Get(stageName)
		if !exists {
			err := fmt.Errorf("stage %s not found during execution", stageName)
			result.ExecutedStages[stageName] = hugo.ExecutionFailure(err)
			if p.stopOnError {
				return result, err
			}
			continue
		}

		// Apply middleware
		wrappedCmd := cmd
		for _, mw := range p.middleware {
			wrappedCmd = mw(wrappedCmd)
		}

		// Execute stage
		stageResult := wrappedCmd.Execute(ctx, bs)
		result.ExecutedStages[stageName] = stageResult

		// Handle result
		if !stageResult.IsSuccess() {
			slog.Error("Stage failed",
				slog.String("stage", string(stageName)),
				slog.Any("error", stageResult.Err))

			if p.stopOnError {
				return result, stageResult.Err
			}
		} else {
			slog.Debug("Stage completed", slog.String("stage", string(stageName)))
		}

		// Handle skip requests
		if stageResult.ShouldSkip() {
			slog.Info("Pipeline skip requested", slog.String("stage", string(stageName)))
			result.Skipped = true
			break
		}
	}

	return result, nil
}

// ExecuteAll runs all registered stages in dependency order.
func (p *Pipeline) ExecuteAll(ctx context.Context, bs *hugo.BuildState) (*ExecutionResult, error) {
	allStages := p.registry.List()
	return p.Execute(ctx, bs, allStages...)
}

// ExecutionResult contains the results of pipeline execution.
type ExecutionResult struct {
	ExecutedStages map[hugo.StageName]hugo.StageExecution
	Plan           *ExecutionPlan
	Canceled       bool
	Skipped        bool
}

// IsSuccess returns true if all executed stages completed successfully.
func (r *ExecutionResult) IsSuccess() bool {
	if r.Canceled {
		return false
	}

	for _, result := range r.ExecutedStages {
		if !result.IsSuccess() {
			return false
		}
	}
	return true
}

// GetFailedStages returns the names of stages that failed.
func (r *ExecutionResult) GetFailedStages() []hugo.StageName {
	var failed []hugo.StageName
	for stage, result := range r.ExecutedStages {
		if !result.IsSuccess() {
			failed = append(failed, stage)
		}
	}
	return failed
}

// GetSuccessfulStages returns the names of stages that succeeded.
func (r *ExecutionResult) GetSuccessfulStages() []hugo.StageName {
	var successful []hugo.StageName
	for stage, result := range r.ExecutedStages {
		if result.IsSuccess() {
			successful = append(successful, stage)
		}
	}
	return successful
}

// DefaultPipeline creates a pipeline with the default registry and middleware.
func DefaultPipeline() *Pipeline {
	return NewPipeline(commands.DefaultRegistry)
}
