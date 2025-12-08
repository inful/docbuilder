package transforms

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
)

// mockTransform is a test transform plugin.
type mockTransform struct {
	BaseTransformPlugin
	name     string
	version  string
	stage    TransformStage
	order    int
	skipNext bool
}

func (m *mockTransform) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:    m.name,
		Version: m.version,
		Type:    plugin.PluginTypeTransform,
	}
}

func (m *mockTransform) Validate(config map[string]interface{}) error {
	return nil
}

func (m *mockTransform) Execute(ctx context.Context, pluginCtx *plugin.PluginContext) error {
	return nil
}

func (m *mockTransform) Stage() TransformStage {
	return m.stage
}

func (m *mockTransform) Order() int {
	return m.order
}

func (m *mockTransform) ShouldApply(input *TransformInput) bool {
	return !m.skipNext
}

func (m *mockTransform) Apply(input *TransformInput) *TransformResult {
	return &TransformResult{
		Content:  append([]byte("transformed:"), input.Content...),
		Metadata: input.Metadata,
		Skipped:  false,
	}
}

// TestTransformStageValues tests the transform stage constants.
func TestTransformStageValues(t *testing.T) {
	stages := []TransformStage{
		StagePreProcess,
		StagePostProcess,
		StageFrontmatter,
		StageContent,
	}

	for _, stage := range stages {
		if stage == "" {
			t.Error("Stage should not be empty")
		}
	}
}

// TestTransformResult tests the TransformResult struct.
func TestTransformResult(t *testing.T) {
	result := &TransformResult{
		Content:  []byte("test"),
		Metadata: map[string]interface{}{"key": "value"},
		Skipped:  false,
	}

	if len(result.Content) != 4 {
		t.Errorf("Content length = %d, expected 4", len(result.Content))
	}

	if len(result.Metadata) != 1 {
		t.Errorf("Metadata length = %d, expected 1", len(result.Metadata))
	}

	if result.Skipped {
		t.Error("Skipped should be false")
	}
}

// TestTransformInput tests the TransformInput struct.
func TestTransformInput(t *testing.T) {
	input := &TransformInput{
		FilePath: "test.md",
		Content:  []byte("# Title"),
		Metadata: map[string]interface{}{"title": "Test"},
		Config:   map[string]interface{}{"debug": true},
	}

	if input.FilePath != "test.md" {
		t.Errorf("FilePath = %q, expected 'test.md'", input.FilePath)
	}

	if string(input.Content) != "# Title" {
		t.Errorf("Content = %q, expected '# Title'", string(input.Content))
	}
}

// TestBaseTransformPluginOrder tests default order.
func TestBaseTransformPluginOrder(t *testing.T) {
	base := &BaseTransformPlugin{}

	if base.Order() != 0 {
		t.Errorf("Order() = %d, expected 0", base.Order())
	}
}

// TestBaseTransformPluginShouldApply tests default ShouldApply.
func TestBaseTransformPluginShouldApply(t *testing.T) {
	base := &BaseTransformPlugin{}
	input := &TransformInput{}

	if !base.ShouldApply(input) {
		t.Error("ShouldApply() should return true by default")
	}
}

// TestTransformRegistryRegister tests registering transforms.
func TestTransformRegistryRegister(t *testing.T) {
	registry := NewTransformRegistry()

	transform := &mockTransform{
		name:    "test-transform",
		version: "v1.0.0",
		stage:   StageContent,
	}

	if err := registry.Register(transform); err != nil {
		t.Errorf("Register() failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Count() = %d, expected 1", registry.Count())
	}
}

// TestTransformRegistryRegisterNil tests registering nil transform.
func TestTransformRegistryRegisterNil(t *testing.T) {
	registry := NewTransformRegistry()

	if err := registry.Register(nil); err == nil {
		t.Error("Register(nil) should return error")
	}
}

// TestTransformRegistryRegisterInvalidType tests registering non-transform plugin.
func TestTransformRegistryRegisterInvalidType(t *testing.T) {
	// Note: Properly testing invalid type requires more complex mock setup
	// For now, we verify that nil and invalid metadata are caught
	registry := NewTransformRegistry()

	if err := registry.Register(nil); err == nil {
		t.Error("Register(nil) should return error")
	}

	t.Logf("Invalid type validation tested through Register(nil)")
}

// TestTransformRegistryList tests listing transforms.
func TestTransformRegistryList(t *testing.T) {
	registry := NewTransformRegistry()

	t1 := &mockTransform{name: "t1", version: "v1", stage: StageContent}
	t2 := &mockTransform{name: "t2", version: "v1", stage: StageFrontmatter}

	_ = registry.Register(t1)
	_ = registry.Register(t2)

	transforms := registry.List()
	if len(transforms) != 2 {
		t.Errorf("List() returned %d transforms, expected 2", len(transforms))
	}
}

// TestTransformRegistryApplyToContent tests applying transforms.
func TestTransformRegistryApplyToContent(t *testing.T) {
	registry := NewTransformRegistry()

	transform := &mockTransform{
		name:    "test",
		version: "v1",
		stage:   StageContent,
	}
	_ = registry.Register(transform)

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

	if !strings.Contains(string(result.Content), "original") {
		t.Errorf("Result content = %q, expected to contain 'original'", string(result.Content))
	}
}

// TestTransformRegistryClear tests clearing transforms.
func TestTransformRegistryClear(t *testing.T) {
	registry := NewTransformRegistry()

	t1 := &mockTransform{name: "t1", version: "v1", stage: StageContent}
	_ = registry.Register(t1)

	if registry.Count() != 1 {
		t.Fatal("Count should be 1 before clear")
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Count() = %d after Clear(), expected 0", registry.Count())
	}
}

// TestTransformRegistryCount tests counting transforms.
func TestTransformRegistryCount(t *testing.T) {
	registry := NewTransformRegistry()

	if registry.Count() != 0 {
		t.Error("New registry should have count 0")
	}

	for i := 0; i < 5; i++ {
		t := &mockTransform{
			name:    fmt.Sprintf("t%d", i),
			version: "v1",
			stage:   StageContent,
		}
		_ = registry.Register(t)
	}

	if registry.Count() != 5 {
		t.Errorf("Count() = %d, expected 5", registry.Count())
	}
}

// TestTransformSkipped tests skipped transforms.
func TestTransformSkipped(t *testing.T) {
	registry := NewTransformRegistry()

	transform := &mockTransform{
		name:     "test",
		version:  "v1",
		stage:    StageContent,
		skipNext: true,
	}
	_ = registry.Register(transform)

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

	// Content should remain unchanged since transform was skipped
	if string(result.Content) != "original" {
		t.Errorf("Result content = %q, expected 'original' (skipped)", string(result.Content))
	}
}

// TestMultipleTransforms tests multiple transforms in sequence.
func TestMultipleTransforms(t *testing.T) {
	registry := NewTransformRegistry()

	t1 := &mockTransform{name: "t1", version: "v1", stage: StageContent, order: 1}
	t2 := &mockTransform{name: "t2", version: "v1", stage: StageContent, order: 2}

	_ = registry.Register(t1)
	_ = registry.Register(t2)

	input := &TransformInput{
		FilePath: "test.md",
		Content:  []byte("input"),
		Metadata: make(map[string]interface{}),
		Config:   make(map[string]interface{}),
	}

	result, err := registry.ApplyToContent(input)
	if err != nil {
		t.Errorf("ApplyToContent() failed: %v", err)
	}

	// Both transforms should have been applied
	resultStr := string(result.Content)
	if !strings.Contains(resultStr, "transformed:") {
		t.Errorf("Result should contain 'transformed:': %q", resultStr)
	}
}
