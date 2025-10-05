package commands

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

func TestCommandRegistry(t *testing.T) {
	registry := NewCommandRegistry()

	// Test registration - manually register to ensure they exist
	cloneCmd := NewCloneReposCommand()
	discoverCmd := NewDiscoverDocsCommand()
	prepareCmd := NewPrepareOutputCommand()

	registry.Register(prepareCmd)
	registry.Register(cloneCmd)
	registry.Register(discoverCmd)

	// Test retrieval
	if cmd, exists := registry.Get(hugo.StageCloneRepos); !exists {
		t.Errorf("CloneRepos command not found")
	} else if cmd.Name() != hugo.StageCloneRepos {
		t.Errorf("CloneRepos command name mismatch")
	}

	if cmd, exists := registry.Get(hugo.StageDiscoverDocs); !exists {
		t.Errorf("DiscoverDocs command not found")
	} else if cmd.Name() != hugo.StageDiscoverDocs {
		t.Errorf("DiscoverDocs command name mismatch")
	}

	if cmd, exists := registry.Get(hugo.StagePrepareOutput); !exists {
		t.Errorf("PrepareOutput command not found")
	} else if cmd.Name() != hugo.StagePrepareOutput {
		t.Errorf("PrepareOutput command name mismatch")
	}

	// Test listing
	commands := registry.List()
	if len(commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(commands))
	}

	// Test dependency validation
	if err := registry.ValidateDependencies(); err != nil {
		t.Errorf("Dependency validation failed: %v", err)
	}
}

func TestCloneReposCommand(t *testing.T) {
	cmd := NewCloneReposCommand()

	// Test metadata
	if cmd.Name() != hugo.StageCloneRepos {
		t.Errorf("Expected name %s, got %s", hugo.StageCloneRepos, cmd.Name())
	}

	if cmd.Description() == "" {
		t.Errorf("Description should not be empty")
	}

	deps := cmd.Dependencies()
	if len(deps) != 1 || deps[0] != hugo.StagePrepareOutput {
		t.Errorf("Expected dependency on %s, got %v", hugo.StagePrepareOutput, deps)
	}

	// Test skip condition
	buildState := &hugo.BuildState{}
	if !cmd.ShouldSkip(buildState) {
		t.Errorf("Should skip when no repositories configured")
	}

	buildState.Git.Repositories = []config.Repository{{Name: "test", URL: "https://example.com/repo.git"}}
	if cmd.ShouldSkip(buildState) {
		t.Errorf("Should not skip when repositories are configured")
	}
}

func TestDiscoverDocsCommand(t *testing.T) {
	cmd := NewDiscoverDocsCommand()

	// Test metadata
	if cmd.Name() != hugo.StageDiscoverDocs {
		t.Errorf("Expected name %s, got %s", hugo.StageDiscoverDocs, cmd.Name())
	}

	if cmd.Description() == "" {
		t.Errorf("Description should not be empty")
	}

	deps := cmd.Dependencies()
	if len(deps) != 1 || deps[0] != hugo.StageCloneRepos {
		t.Errorf("Expected dependency on %s, got %v", hugo.StageCloneRepos, deps)
	}

	// Test skip condition
	buildState := &hugo.BuildState{}
	if !cmd.ShouldSkip(buildState) {
		t.Errorf("Should skip when no repository paths available")
	}

	buildState.Git.RepoPaths = map[string]string{"test": "/path/to/test"}
	if cmd.ShouldSkip(buildState) {
		t.Errorf("Should not skip when repository paths are available")
	}
}

func TestCommandExecution(t *testing.T) {
	// This is a simplified test - full execution would require setting up
	// complete build state with generators, configs, etc.

	cmd := NewDiscoverDocsCommand()
	ctx := context.Background()

	// Test with minimal build state (should skip due to no repo paths)
	buildState := &hugo.BuildState{}

	// Check if skip condition is working
	if cmd.ShouldSkip(buildState) {
		t.Log("Command correctly skips when no repo paths available")
		return // Skip the actual execution test since it would skip
	}

	// If it doesn't skip, we need to provide a minimal config to prevent panic
	cfg := &config.Config{
		Build: config.BuildConfig{},
	}
	gen := hugo.NewGenerator(cfg, "/tmp/test")
	buildState.Generator = gen

	result := cmd.Execute(ctx, buildState)

	// Should either skip or error with empty build state
	if !result.ShouldSkip() && result.Err == nil {
		t.Errorf("Expected skip or error with empty build state")
	}
}

func TestBaseCommand(t *testing.T) {
	metadata := CommandMetadata{
		Name:         hugo.StageCloneRepos,
		Description:  "Test command",
		Dependencies: []hugo.StageName{hugo.StagePrepareOutput},
		Optional:     true,
		SkipIf: func(bs *hugo.BuildState) bool {
			return bs == nil
		},
	}

	base := NewBaseCommand(metadata)

	if base.Name() != hugo.StageCloneRepos {
		t.Errorf("Name mismatch")
	}

	if base.Description() != "Test command" {
		t.Errorf("Description mismatch")
	}

	if !base.IsOptional() {
		t.Errorf("Should be optional")
	}

	if !base.ShouldSkip(nil) {
		t.Errorf("Should skip with nil build state")
	}

	if base.ShouldSkip(&hugo.BuildState{}) {
		t.Errorf("Should not skip with valid build state")
	}
}
