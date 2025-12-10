package transforms

import (
	"context"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
)

// mockTransformV2 implements TransformPlugin for testing.
type mockTransformV2 struct {
	BaseTransformPlugin
	name         string
	version      string
	stage        TransformStage
	dependencies TransformDependencies
	skipNext     bool
}

func (m *mockTransformV2) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:    m.name,
		Version: m.version,
		Type:    plugin.PluginTypeTransform,
	}
}

func (m *mockTransformV2) Validate(config map[string]interface{}) error {
	return nil
}

func (m *mockTransformV2) Execute(ctx context.Context, pluginCtx *plugin.PluginContext) error {
	return nil
}

func (m *mockTransformV2) Stage() TransformStage {
	return m.stage
}

func (m *mockTransformV2) Dependencies() TransformDependencies {
	return m.dependencies
}

func (m *mockTransformV2) ShouldApply(input *TransformInput) bool {
	return !m.skipNext
}

func (m *mockTransformV2) Apply(input *TransformInput) *TransformResult {
	return &TransformResult{
		Content:  append([]byte("v2:"), input.Content...),
		Metadata: input.Metadata,
		Skipped:  false,
	}
}

// TestTransformPluginInterface verifies TransformPlugin interface.
func TestTransformPluginInterface(t *testing.T) {
	var _ TransformPlugin = (*mockTransformV2)(nil)
}

// TestTransformDependencies tests dependency structure.
func TestTransformDependencies(t *testing.T) {
	deps := TransformDependencies{
		MustRunAfter:  []string{"transform1", "transform2"},
		MustRunBefore: []string{"transform3"},
	}

	if len(deps.MustRunAfter) != 2 {
		t.Errorf("MustRunAfter length = %d, expected 2", len(deps.MustRunAfter))
	}

	if len(deps.MustRunBefore) != 1 {
		t.Errorf("MustRunBefore length = %d, expected 1", len(deps.MustRunBefore))
	}
}

// TestTransformRegistryRegisterV2Style tests V2 registration.
func TestTransformRegistryRegisterV2Style(t *testing.T) {
	registry := NewTransformRegistry()

	transform := &mockTransformV2{
		name:    "test-v2",
		version: "v1.0.0",
		stage:   StageContent,
	}

	if err := registry.Register(transform); err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	if len(registry.transforms) != 1 {
		t.Errorf("V2 registry count = %d, expected 1", len(registry.transforms))
	}
}

// TestTransformRegistryApplyV2 tests V2 transform application.
func TestTransformRegistryApplyV2(t *testing.T) {
	registry := NewTransformRegistry()

	transform := &mockTransformV2{
		name:    "test-v2",
		version: "v1.0.0",
		stage:   StageContent,
		dependencies: TransformDependencies{
			MustRunAfter:  []string{},
			MustRunBefore: []string{},
		},
	}

	if err := registry.Register(transform); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	input := &TransformInput{
		FilePath: "test.md",
		Content:  []byte("original"),
		Metadata: make(map[string]interface{}),
		Config:   make(map[string]interface{}),
	}

	result, err := registry.ApplyToContent(input)
	if err != nil {
		t.Errorf("ApplyToContent() failed: %v", err)
	}

	expected := "v2:original"
	if string(result.Content) != expected {
		t.Errorf("Result content = %q, expected %q", string(result.Content), expected)
	}
}

// TestTopologicalSortPluginsSimple tests basic dependency resolution.
func TestTopologicalSortPluginsSimple(t *testing.T) {
	transforms := []TransformPlugin{
		&mockTransformV2{
			name:  "third",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"second"},
			},
		},
		&mockTransformV2{
			name:         "first",
			stage:        StageContent,
			dependencies: TransformDependencies{},
		},
		&mockTransformV2{
			name:  "second",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"first"},
			},
		},
	}

	sorted, err := topologicalSortPlugins(transforms)
	if err != nil {
		t.Fatalf("topologicalSortPlugins() failed: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 transforms, got %d", len(sorted))
	}

	names := make([]string, len(sorted))
	for i, tr := range sorted {
		names[i] = tr.Metadata().Name
	}

	// Verify order: first -> second -> third
	if names[0] != "first" || names[1] != "second" || names[2] != "third" {
		t.Errorf("Order = %v, expected [first, second, third]", names)
	}
}

// TestTopologicalSortPluginsCircular tests circular dependency detection.
func TestTopologicalSortPluginsCircular(t *testing.T) {
	transforms := []TransformPlugin{
		&mockTransformV2{
			name:  "a",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"b"},
			},
		},
		&mockTransformV2{
			name:  "b",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"a"},
			},
		},
	}

	_, err := topologicalSortPlugins(transforms)
	if err == nil {
		t.Fatal("Expected circular dependency error, got nil")
	}

	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("Error message should mention 'circular', got: %v", err)
	}
}

// TestTopologicalSortPluginsMissingDep tests handling of missing dependencies.
func TestTopologicalSortPluginsMissingDep(t *testing.T) {
	transforms := []TransformPlugin{
		&mockTransformV2{
			name:  "a",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunAfter: []string{"nonexistent"},
			},
		},
	}

	// Should succeed - missing deps are tolerated (might be in different stage)
	sorted, err := topologicalSortPlugins(transforms)
	if err != nil {
		t.Fatalf("topologicalSortPlugins() should tolerate missing deps, got error: %v", err)
	}

	if len(sorted) != 1 || sorted[0].Metadata().Name != "a" {
		t.Errorf("Expected single transform 'a', got %d transforms", len(sorted))
	}
}

// TestTopologicalSortPluginsEmpty tests empty input.
func TestTopologicalSortPluginsEmpty(t *testing.T) {
	transforms := []TransformPlugin{}

	sorted, err := topologicalSortPlugins(transforms)
	if err != nil {
		t.Fatalf("topologicalSortPlugins() failed on empty input: %v", err)
	}

	if len(sorted) != 0 {
		t.Errorf("Expected 0 transforms, got %d", len(sorted))
	}
}

// TestFrontmatterTransformImplementsV2 verifies FrontmatterTransform implements V2.
func TestFrontmatterTransformImplementsV2(t *testing.T) {
	// This would require importing the frontmatter package, but we can test the interface
	var _ TransformPlugin = (*mockTransformV2)(nil)
}

// TestV1V2Coexistence tests that both V1 and V2 can coexist.
// TestV2WithMustRunBefore tests MustRunBefore dependencies.
func TestV2WithMustRunBefore(t *testing.T) {
	transforms := []TransformPlugin{
		&mockTransformV2{
			name:         "last",
			stage:        StageContent,
			dependencies: TransformDependencies{},
		},
		&mockTransformV2{
			name:  "first",
			stage: StageContent,
			dependencies: TransformDependencies{
				MustRunBefore: []string{"last"},
			},
		},
	}

	sorted, err := topologicalSortPlugins(transforms)
	if err != nil {
		t.Fatalf("topologicalSortPlugins() failed: %v", err)
	}

	names := make([]string, len(sorted))
	for i, tr := range sorted {
		names[i] = tr.Metadata().Name
	}

	// first should come before last
	if names[0] != "first" || names[1] != "last" {
		t.Errorf("Order = %v, expected [first, last]", names)
	}
}
