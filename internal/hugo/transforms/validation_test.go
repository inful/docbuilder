package transforms

import (
	"strings"
	"testing"
)

// Test helper transform types
type validTestTransform struct {
	name  string
	stage TransformStage
	deps  TransformDependencies
}

func (t validTestTransform) Name() string                        { return t.name }
func (t validTestTransform) Stage() TransformStage               { return t.stage }
func (t validTestTransform) Dependencies() TransformDependencies { return t.deps }
func (t validTestTransform) Transform(PageAdapter) error         { return nil }

func TestValidatePipeline_Valid(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and add valid transforms
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "parser",
		stage: StageParse,
		deps:  TransformDependencies{},
	})

	Register(validTestTransform{
		name:  "builder",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"parser"}},
	})

	Register(validTestTransform{
		name:  "enricher",
		stage: StageEnrich,
		deps:  TransformDependencies{MustRunAfter: []string{"builder"}},
	})

	result := ValidatePipeline()

	if !result.Valid {
		t.Errorf("ValidatePipeline() returned invalid for valid pipeline")
		for _, err := range result.Errors {
			t.Logf("  Error: %s", err)
		}
	}
}

func TestValidatePipeline_MissingDependency(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and add transform with missing dependency
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "builder",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"missing_parser"}},
	})

	result := ValidatePipeline()

	if result.Valid {
		t.Error("ValidatePipeline() should return invalid for missing dependency")
	}

	if len(result.Errors) == 0 {
		t.Error("ValidatePipeline() should have error for missing dependency")
	}

	foundMissingError := false
	for _, err := range result.Errors {
		if strings.Contains(err, "missing_parser") && strings.Contains(err, "missing transform") {
			foundMissingError = true
			break
		}
	}

	if !foundMissingError {
		t.Errorf("ValidatePipeline() errors = %v, want error about missing transform", result.Errors)
	}
}

func TestValidatePipeline_CircularDependency(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and create circular dependency
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "transform_a",
		stage: StageTransform,
		deps:  TransformDependencies{MustRunAfter: []string{"transform_b"}},
	})

	Register(validTestTransform{
		name:  "transform_b",
		stage: StageTransform,
		deps:  TransformDependencies{MustRunAfter: []string{"transform_a"}},
	})

	result := ValidatePipeline()

	if result.Valid {
		t.Error("ValidatePipeline() should return invalid for circular dependency")
	}

	if len(result.Errors) == 0 {
		t.Error("ValidatePipeline() should have error for circular dependency")
	}

	foundCircularError := false
	for _, err := range result.Errors {
		if strings.Contains(strings.ToLower(err), "circular") {
			foundCircularError = true
			break
		}
	}

	if !foundCircularError {
		t.Errorf("ValidatePipeline() errors = %v, want error about circular dependency", result.Errors)
	}
}

func TestValidatePipeline_InvalidStage(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and add transform with invalid stage
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "invalid",
		stage: TransformStage("invalid_stage"),
		deps:  TransformDependencies{},
	})

	result := ValidatePipeline()

	if result.Valid {
		t.Error("ValidatePipeline() should return invalid for invalid stage")
	}

	if len(result.Errors) == 0 {
		t.Error("ValidatePipeline() should have error for invalid stage")
	}

	foundStageError := false
	for _, err := range result.Errors {
		if strings.Contains(err, "invalid stage") {
			foundStageError = true
			break
		}
	}

	if !foundStageError {
		t.Errorf("ValidatePipeline() errors = %v, want error about invalid stage", result.Errors)
	}
}

func TestValidatePipeline_CrossStageWarning(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and create cross-stage dependency issue
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "early",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"later"}},
	})

	Register(validTestTransform{
		name:  "later",
		stage: StageTransform, // Later stage
		deps:  TransformDependencies{},
	})

	result := ValidatePipeline()

	// This should generate a warning about stage ordering
	if len(result.Warnings) == 0 {
		t.Error("ValidatePipeline() should have warning for cross-stage dependency issue")
	}

	foundWarning := false
	for _, warn := range result.Warnings {
		if strings.Contains(warn, "later stage") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Errorf("ValidatePipeline() warnings = %v, want warning about stage ordering", result.Warnings)
	}
}

func TestValidatePipeline_EmptyRegistry(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry
	registry = make(map[string]Transformer)

	result := ValidatePipeline()

	// Empty registry should still be "valid" but have a warning
	if !result.Valid {
		t.Error("ValidatePipeline() should return valid for empty registry")
	}

	if len(result.Warnings) == 0 {
		t.Error("ValidatePipeline() should have warning for empty registry")
	}
}

func TestIsValidStage(t *testing.T) {
	tests := []struct {
		stage TransformStage
		valid bool
	}{
		{StageParse, true},
		{StageBuild, true},
		{StageEnrich, true},
		{StageMerge, true},
		{StageTransform, true},
		{StageFinalize, true},
		{StageSerialize, true},
		{TransformStage("invalid"), false},
		{TransformStage(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			got := isValidStage(tt.stage)
			if got != tt.valid {
				t.Errorf("isValidStage(%q) = %v, want %v", tt.stage, got, tt.valid)
			}
		})
	}
}

func TestIsValidDependencyStage(t *testing.T) {
	tests := []struct {
		name         string
		currentStage TransformStage
		depStage     TransformStage
		valid        bool
	}{
		{"same stage", StageBuild, StageBuild, true},
		{"earlier dep", StageTransform, StageBuild, true},
		{"later dep", StageBuild, StageTransform, false},
		{"parse before build", StageBuild, StageParse, true},
		{"serialize after finalize", StageSerialize, StageFinalize, true},
		{"finalize after serialize", StageFinalize, StageSerialize, false},
		{"invalid current", TransformStage("invalid"), StageBuild, false},
		{"invalid dep", StageBuild, TransformStage("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDependencyStage(tt.currentStage, tt.depStage)
			if got != tt.valid {
				t.Errorf("isValidDependencyStage(%q, %q) = %v, want %v",
					tt.currentStage, tt.depStage, got, tt.valid)
			}
		})
	}
}

func TestGetPipelineInfo(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and add some transforms
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "parser",
		stage: StageParse,
		deps:  TransformDependencies{},
	})

	Register(validTestTransform{
		name:  "builder",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"parser"}},
	})

	info, err := GetPipelineInfo()
	if err != nil {
		t.Fatalf("GetPipelineInfo() error = %v", err)
	}

	if !strings.Contains(info, "parser") {
		t.Error("GetPipelineInfo() should include parser")
	}

	if !strings.Contains(info, "builder") {
		t.Error("GetPipelineInfo() should include builder")
	}

	if !strings.Contains(info, "MustRunAfter") {
		t.Error("GetPipelineInfo() should include dependency information")
	}

	if !strings.Contains(info, "Total transforms: 2") {
		t.Error("GetPipelineInfo() should include transform count")
	}
}

func TestPrintValidationResult(t *testing.T) {
	tests := []struct {
		name   string
		result *ValidationResult
		want   []string // strings that should appear in output
	}{
		{
			name:   "valid with no warnings",
			result: &ValidationResult{Valid: true},
			want:   []string{"✓", "valid", "no warnings"},
		},
		{
			name: "with errors",
			result: &ValidationResult{
				Valid:  false,
				Errors: []string{"error 1", "error 2"},
			},
			want: []string{"✗", "Errors", "error 1", "error 2"},
		},
		{
			name: "with warnings",
			result: &ValidationResult{
				Valid:    true,
				Warnings: []string{"warning 1"},
			},
			want: []string{"⚠", "Warnings", "warning 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := PrintValidationResult(tt.result)

			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("PrintValidationResult() output missing %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestListTransformNames(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and add some transforms
	registry = make(map[string]Transformer)

	Register(validTestTransform{name: "zebra", stage: StageParse})
	Register(validTestTransform{name: "alpha", stage: StageBuild})
	Register(validTestTransform{name: "beta", stage: StageTransform})

	names := ListTransformNames()

	if len(names) != 3 {
		t.Fatalf("ListTransformNames() returned %d names, want 3", len(names))
	}

	// Should be sorted
	expected := []string{"alpha", "beta", "zebra"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("ListTransformNames()[%d] = %q, want %q", i, names[i], name)
		}
	}
}

func TestValidatePipelineWithSuggestions(t *testing.T) {
	// Save and restore registry
	saved := snapshotRegistry()
	defer restoreRegistry(saved)

	// Clear registry and create an invalid pipeline
	registry = make(map[string]Transformer)

	Register(validTestTransform{
		name:  "broken",
		stage: StageBuild,
		deps:  TransformDependencies{MustRunAfter: []string{"missing"}},
	})

	result, suggestions := ValidatePipelineWithSuggestions()

	if result.Valid {
		t.Error("ValidatePipelineWithSuggestions() should return invalid result")
	}

	if len(suggestions) == 0 {
		t.Error("ValidatePipelineWithSuggestions() should provide suggestions")
	}

	// Check that suggestions contain helpful text
	suggestText := strings.Join(suggestions, " ")
	if !strings.Contains(suggestText, "fix") && !strings.Contains(suggestText, "Common") {
		t.Errorf("Suggestions should contain helpful guidance, got: %v", suggestions)
	}
}
