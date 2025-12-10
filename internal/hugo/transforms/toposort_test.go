package transforms

import (
	"testing"
)

// Mock transformer for testing
type mockTransformer struct {
	name         string
	stage        TransformStage
	dependencies TransformDependencies
}

func (m mockTransformer) Name() string                        { return m.name }
func (m mockTransformer) Stage() TransformStage               { return m.stage }
func (m mockTransformer) Dependencies() TransformDependencies { return m.dependencies }
func (m mockTransformer) Transform(p PageAdapter) error       { return nil }

// TestTopologicalSort_Simple tests basic linear dependencies
func TestTopologicalSort_Simple(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "third",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"second"},
			},
		},
		mockTransformer{
			name:  "first",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{},
			},
		},
		mockTransformer{
			name:  "second",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"first"},
			},
		},
	}

	result, err := topologicalSort(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 transforms, got %d", len(result))
	}

	// Verify order: first -> second -> third
	if result[0].Name() != "first" {
		t.Errorf("Expected first transform to be 'first', got %q", result[0].Name())
	}
	if result[1].Name() != "second" {
		t.Errorf("Expected second transform to be 'second', got %q", result[1].Name())
	}
	if result[2].Name() != "third" {
		t.Errorf("Expected third transform to be 'third', got %q", result[2].Name())
	}
}

// TestTopologicalSort_ComplexGraph tests a more complex dependency graph
func TestTopologicalSort_ComplexGraph(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{name: "a", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "b", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"a"}}},
		mockTransformer{name: "c", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"a"}}},
		mockTransformer{name: "d", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"b", "c"}}},
	}

	result, err := topologicalSort(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("Expected 4 transforms, got %d", len(result))
	}

	// Build position map
	pos := make(map[string]int)
	for i, tr := range result {
		pos[tr.Name()] = i
	}

	// Verify constraints
	if pos["a"] >= pos["b"] {
		t.Error("'a' must come before 'b'")
	}
	if pos["a"] >= pos["c"] {
		t.Error("'a' must come before 'c'")
	}
	if pos["b"] >= pos["d"] {
		t.Error("'b' must come before 'd'")
	}
	if pos["c"] >= pos["d"] {
		t.Error("'c' must come before 'd'")
	}
}

// TestTopologicalSort_CircularDependency detects cycles
func TestTopologicalSort_CircularDependency(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "a",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"b"},
			},
		},
		mockTransformer{
			name:  "b",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"a"},
			},
		},
	}

	_, err := topologicalSort(transforms)
	if err == nil {
		t.Fatal("Expected error for circular dependency, got nil")
	}

	expectedMsg := "circular dependency"
	if !containsSubstring(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

// TestTopologicalSort_MissingDependency tests that topologicalSort tolerates
// missing dependencies (for cross-stage deps). Global validation is done by ValidateDependencies.
func TestTopologicalSort_MissingDependency(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "a",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"nonexistent"},
			},
		},
	}

	// topologicalSort should succeed - it assumes missing deps are in other stages
	ordered, err := topologicalSort(transforms)
	if err != nil {
		t.Fatalf("topologicalSort should tolerate missing deps, got error: %v", err)
	}

	if len(ordered) != 1 || ordered[0].Name() != "a" {
		t.Errorf("Expected ['a'], got %d transforms", len(ordered))
	}

	// ValidateDependencies should catch truly missing dependencies
	if err := ValidateDependencies(transforms); err == nil {
		t.Fatal("ValidateDependencies should detect missing dependency")
	} else if !containsSubstring(err.Error(), "missing transform") {
		t.Errorf("Expected error about missing transform, got: %v", err)
	}
}

// TestTopologicalSort_MustRunBefore tests reverse dependencies
func TestTopologicalSort_MustRunBefore(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "first",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunBefore: []string{"second"},
			},
		},
		mockTransformer{
			name:         "second",
			stage:        StageParse,
			dependencies: TransformDependencies{},
		},
	}

	result, err := topologicalSort(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 transforms, got %d", len(result))
	}

	if result[0].Name() != "first" {
		t.Errorf("Expected 'first' to run first, got %q", result[0].Name())
	}
	if result[1].Name() != "second" {
		t.Errorf("Expected 'second' to run second, got %q", result[1].Name())
	}
}

// TestTopologicalSort_Deterministic ensures consistent ordering
func TestTopologicalSort_Deterministic(t *testing.T) {
	// Multiple independent transforms should sort alphabetically
	transforms := []Transformer{
		mockTransformer{name: "zebra", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "alpha", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "beta", stage: StageParse, dependencies: TransformDependencies{}},
	}

	result, err := topologicalSort(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result[0].Name() != "alpha" || result[1].Name() != "beta" || result[2].Name() != "zebra" {
		t.Error("Expected alphabetical ordering for independent transforms")
	}
}

// TestTopologicalSort_Empty handles empty input
func TestTopologicalSort_Empty(t *testing.T) {
	result, err := topologicalSort([]Transformer{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d transforms", len(result))
	}
}

// TestBuildPipeline_Stages tests multi-stage pipeline construction
func TestBuildPipeline_Stages(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{name: "serialize", stage: StageSerialize, dependencies: TransformDependencies{}},
		mockTransformer{name: "parse", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "transform", stage: StageTransform, dependencies: TransformDependencies{}},
		mockTransformer{name: "build", stage: StageBuild, dependencies: TransformDependencies{}},
	}

	result, err := BuildPipeline(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 4 {
		t.Fatalf("Expected 4 transforms, got %d", len(result))
	}

	// Verify stage order
	expectedOrder := []string{"parse", "build", "transform", "serialize"}
	for i, expected := range expectedOrder {
		if result[i].Name() != expected {
			t.Errorf("Position %d: expected %q, got %q", i, expected, result[i].Name())
		}
	}
}

// TestBuildPipeline_CrossStageOrder tests dependencies across stages
func TestBuildPipeline_CrossStageOrder(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "build_b",
			stage: StageBuild,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"build_a"},
			},
		},
		mockTransformer{
			name:         "build_a",
			stage:        StageBuild,
			dependencies: TransformDependencies{},
		},
		mockTransformer{
			name:         "parse_x",
			stage:        StageParse,
			dependencies: TransformDependencies{},
		},
	}

	result, err := BuildPipeline(transforms)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Parse stage should come first
	if result[0].Name() != "parse_x" {
		t.Errorf("Expected parse stage first, got %q", result[0].Name())
	}

	// Within build stage, a should come before b
	if result[1].Name() != "build_a" {
		t.Errorf("Expected build_a at position 1, got %q", result[1].Name())
	}
	if result[2].Name() != "build_b" {
		t.Errorf("Expected build_b at position 2, got %q", result[2].Name())
	}
}

// TestBuildPipeline_InvalidStage detects invalid stages
func TestBuildPipeline_InvalidStage(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:         "invalid",
			stage:        TransformStage("invalid_stage"),
			dependencies: TransformDependencies{},
		},
	}

	_, err := BuildPipeline(transforms)
	if err == nil {
		t.Fatal("Expected error for invalid stage, got nil")
	}

	expectedMsg := "invalid stage"
	if !containsSubstring(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

// TestBuildPipeline_Empty handles empty pipeline
func TestBuildPipeline_Empty(t *testing.T) {
	result, err := BuildPipeline([]Transformer{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d transforms", len(result))
	}
}

// TestValidateDependencies_Valid checks validation of valid dependencies
func TestValidateDependencies_Valid(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{name: "a", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "b", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"a"}}},
	}

	err := ValidateDependencies(transforms)
	if err != nil {
		t.Errorf("Expected valid dependencies, got error: %v", err)
	}
}

// TestValidateDependencies_Missing detects missing dependencies
func TestValidateDependencies_Missing(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{
			name:  "a",
			stage: StageParse,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"missing"},
			},
		},
	}

	err := ValidateDependencies(transforms)
	if err == nil {
		t.Fatal("Expected error for missing dependency, got nil")
	}
}

// TestValidateDependencies_Circular detects circular dependencies
func TestValidateDependencies_Circular(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{name: "a", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"b"}}},
		mockTransformer{name: "b", stage: StageParse, dependencies: TransformDependencies{MustRunAfter: []string{"a"}}},
	}

	err := ValidateDependencies(transforms)
	if err == nil {
		t.Fatal("Expected error for circular dependency, got nil")
	}
}

// TestTopologicalSort_DuplicateNames detects duplicate transformer names
func TestTopologicalSort_DuplicateNames(t *testing.T) {
	transforms := []Transformer{
		mockTransformer{name: "duplicate", stage: StageParse, dependencies: TransformDependencies{}},
		mockTransformer{name: "duplicate", stage: StageParse, dependencies: TransformDependencies{}},
	}

	_, err := topologicalSort(transforms)
	if err == nil {
		t.Fatal("Expected error for duplicate names, got nil")
	}

	expectedMsg := "duplicate transformer name"
	if !containsSubstring(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedMsg, err)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
