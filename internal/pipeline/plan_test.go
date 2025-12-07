package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestBuildPlanBuilder validates that the builder resolves theme features, filters, and transforms.
func TestBuildPlanBuilder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Hugo.Theme = "hextra"
	cfg.Filtering = &config.FilteringConfig{
		RequiredPaths:   []string{"docs", "content"},
		IgnoreFiles:     []string{".docignore"},
		IncludePatterns: []string{"proj-*"},
		ExcludePatterns: []string{"temp-*"},
	}
	cfg.Hugo.Transforms = &config.HugoTransforms{
		Enable: []string{"frontmatter", "links"},
	}

	plan := NewBuildPlanBuilder(cfg).
		WithOutput("/tmp/output", "/tmp/workspace").
		WithIncremental(true).
		ResolveThemeFeatures().
		ResolveFilters().
		ResolveTransforms().
		Build()

	// Validate theme features resolved
	if plan.ThemeFeatures.Name != config.ThemeHextra {
		t.Errorf("expected theme features for hextra, got %v", plan.ThemeFeatures.Name)
	}
	if !plan.ThemeFeatures.UsesModules {
		t.Error("expected hextra to use modules")
	}

	// Validate filters resolved
	if len(plan.EnabledFilters.RequiredPaths) != 2 {
		t.Errorf("expected 2 required paths, got %d", len(plan.EnabledFilters.RequiredPaths))
	}
	if len(plan.EnabledFilters.IncludePatterns) != 1 {
		t.Errorf("expected 1 include pattern, got %d", len(plan.EnabledFilters.IncludePatterns))
	}

	// Validate transforms resolved
	if len(plan.TransformNames) != 2 {
		t.Errorf("expected 2 transforms, got %d", len(plan.TransformNames))
	}
	if plan.TransformNames[0] != "frontmatter" || plan.TransformNames[1] != "links" {
		t.Errorf("expected transforms [frontmatter, links], got %v", plan.TransformNames)
	}

	// Validate output dirs
	if plan.OutputDir != "/tmp/output" {
		t.Errorf("expected output dir /tmp/output, got %s", plan.OutputDir)
	}
	if !plan.Incremental {
		t.Error("expected incremental mode enabled")
	}
}

// TestBuildPlanBuilderWithDisabledTransforms validates transform disable logic.
func TestBuildPlanBuilderWithDisabledTransforms(t *testing.T) {
	cfg := &config.Config{}
	cfg.Hugo.Transforms = &config.HugoTransforms{
		Disable: []string{"frontmatter"},
	}

	plan := NewBuildPlanBuilder(cfg).ResolveTransforms().Build()

	// Should have default transforms except disabled ones
	for _, name := range plan.TransformNames {
		if name == "frontmatter" {
			t.Error("expected frontmatter to be disabled")
		}
	}
	if len(plan.TransformNames) == 0 {
		t.Error("expected some transforms to be enabled")
	}
}
