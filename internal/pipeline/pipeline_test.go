package pipeline

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/commands"
)

func TestPipelineDependencyResolution(t *testing.T) {
	registry := commands.NewCommandRegistry()

	// Register commands in the registry
	prepareCmd := commands.NewPrepareOutputCommand()
	cloneCmd := commands.NewCloneReposCommand()
	discoverCmd := commands.NewDiscoverDocsCommand()

	registry.Register(prepareCmd)
	registry.Register(cloneCmd)
	registry.Register(discoverCmd)

	pipeline := NewPipeline(registry)

	// Test execution plan generation
	plan, err := pipeline.BuildExecutionPlan([]hugo.StageName{
		hugo.StageDiscoverDocs, // Should include dependencies automatically
	})

	if err != nil {
		t.Fatalf("Failed to build execution plan: %v", err)
	}

	if len(plan.Order) != 3 {
		t.Errorf("Expected 3 stages in execution order, got %d: %v", len(plan.Order), plan.Order)
	}

	// Verify correct dependency order
	expectedOrder := []hugo.StageName{
		hugo.StagePrepareOutput,
		hugo.StageCloneRepos,
		hugo.StageDiscoverDocs,
	}

	for i, expected := range expectedOrder {
		if i >= len(plan.Order) || plan.Order[i] != expected {
			t.Errorf("Expected stage %d to be %s, got %s", i, expected, plan.Order[i])
		}
	}
}

func TestPipelineExecution(t *testing.T) {
	registry := commands.NewCommandRegistry()

	// Register a simple command for testing
	prepareCmd := commands.NewPrepareOutputCommand()
	registry.Register(prepareCmd)

	pipeline := NewPipeline(registry)

	// Create minimal build state
	cfg := &config.Config{
		Build: config.BuildConfig{},
	}
	gen := hugo.NewGenerator(cfg, "/tmp/test")
	buildState := &hugo.BuildState{
		Generator: gen,
	}
	buildState.Git.WorkspaceDir = "/tmp/test-workspace"

	ctx := context.Background()

	// Execute just the prepare stage
	result, err := pipeline.Execute(ctx, buildState, hugo.StagePrepareOutput)

	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Pipeline execution was not successful")
	}

	if len(result.ExecutedStages) != 1 {
		t.Errorf("Expected 1 executed stage, got %d", len(result.ExecutedStages))
	}

	if _, exists := result.ExecutedStages[hugo.StagePrepareOutput]; !exists {
		t.Errorf("PrepareOutput stage was not executed")
	}
}

func TestPipelineSkipping(t *testing.T) {
	registry := commands.NewCommandRegistry()

	// Register commands (including dependencies)
	prepareCmd := commands.NewPrepareOutputCommand()
	cloneCmd := commands.NewCloneReposCommand()
	registry.Register(prepareCmd)
	registry.Register(cloneCmd)

	pipeline := NewPipeline(registry)

	// Create build state with no repositories (should cause skip)
	cfg := &config.Config{
		Build: config.BuildConfig{},
	}
	gen := hugo.NewGenerator(cfg, "/tmp/test")
	buildState := &hugo.BuildState{
		Generator: gen,
		Report:    &hugo.BuildReport{}, // Initialize report to prevent nil access
	}
	buildState.Git.WorkspaceDir = "/tmp/test-workspace"
	buildState.Git.Repositories = []config.Repository{} // Empty repositories - should skip clone

	ctx := context.Background()

	// This should execute prepare_output and clone_repos (clone_repos will skip due to no repositories)
	result, err := pipeline.Execute(ctx, buildState, hugo.StageCloneRepos)

	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Should execute both prepare_output and clone_repos (with clone_repos skipping)
	if len(result.ExecutedStages) != 2 {
		t.Errorf("Expected 2 executed stages, got %d", len(result.ExecutedStages))
	}
}

func TestPipelineDependencyError(t *testing.T) {
	registry := commands.NewCommandRegistry()

	// Register only clone command, but not its dependency
	cloneCmd := commands.NewCloneReposCommand()
	registry.Register(cloneCmd)

	pipeline := NewPipeline(registry)

	// Try to build execution plan - should fail due to missing dependency
	_, err := pipeline.BuildExecutionPlan([]hugo.StageName{hugo.StageCloneRepos})

	if err == nil {
		t.Errorf("Expected error due to missing dependency, but got none")
	}
}
