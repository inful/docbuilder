package frontmatter

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
	"git.home.luguber.info/inful/docbuilder/internal/plugin/transforms"
)

func TestFrontmatterTransformMetadata(t *testing.T) {
	ft := NewFrontmatterTransform()
	meta := ft.Metadata()

	if meta.Name != "frontmatter" {
		t.Errorf("Expected name 'frontmatter', got %s", meta.Name)
	}

	if meta.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", meta.Version)
	}

	if meta.Type != plugin.PluginTypeTransform {
		t.Errorf("Expected type Transform, got %v", meta.Type)
	}

	if meta.Description == "" {
		t.Error("Expected non-empty description")
	}
}

func TestFrontmatterTransformValidate(t *testing.T) {
	ft := NewFrontmatterTransform()
	cfg := map[string]interface{}{
		"source_path_field": "source_file",
	}

	err := ft.Validate(cfg)
	if err != nil {
		t.Errorf("Expected no error validating config, got %v", err)
	}
}

func TestFrontmatterTransformExecute(t *testing.T) {
	ft := NewFrontmatterTransform()
	ctx := context.Background()
	pluginCtx := &plugin.PluginContext{}

	err := ft.Execute(ctx, pluginCtx)
	if err != nil {
		t.Errorf("Expected no error executing, got %v", err)
	}
}

func TestFrontmatterTransformStage(t *testing.T) {
	ft := NewFrontmatterTransform()
	stage := ft.Stage()

	if stage != transforms.StageFrontmatter {
		t.Errorf("Expected stage Frontmatter, got %v", stage)
	}
}

func TestFrontmatterTransformShouldApply(t *testing.T) {
	ft := NewFrontmatterTransform()

	tests := []struct {
		filename string
		expected bool
	}{
		{"README.md", true},
		{"guide.markdown", true},
		{"styles.css", false},
		{"image.png", false},
		{"file.txt", false},
	}

	for _, tt := range tests {
		input := &transforms.TransformInput{
			FilePath: tt.filename,
			Content:  []byte("test content"),
		}

		result := ft.ShouldApply(input)
		if result != tt.expected {
			t.Errorf("For file %s: expected %v, got %v", tt.filename, tt.expected, result)
		}
	}
}

func TestFrontmatterTransformApply(t *testing.T) {
	ft := NewFrontmatterTransform()

	input := &transforms.TransformInput{
		FilePath: "test.md",
		Content:  []byte("# Test Content\n\nBody here"),
		Metadata: map[string]interface{}{},
	}

	result := ft.Apply(input)
	if result == nil {
		t.Fatalf("Expected non-nil result from Apply")
	}

	if result.Content != nil && string(result.Content) != string(input.Content) {
		t.Errorf("Content should be unchanged. Expected %q, got %q", input.Content, result.Content)
	}

	if result.Metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}

	if _, ok := result.Metadata["title"]; !ok {
		t.Error("Expected title field to be added to metadata")
	}

	if _, ok := result.Metadata["date"]; !ok {
		t.Error("Expected date field to be added to metadata")
	}

	if _, ok := result.Metadata["draft"]; !ok {
		t.Error("Expected draft field to be added to metadata")
	}

	if processed, ok := result.Metadata["processed"]; !ok {
		t.Error("Expected processed field to be added")
	} else if v, ok := processed.(bool); !ok || !v {
		t.Error("Expected processed field to be true")
	}

	if source, ok := result.Metadata["source_file"]; !ok {
		t.Error("Expected source_file field to be added")
	} else if source != "test.md" {
		t.Errorf("Expected source_file to be 'test.md', got %v", source)
	}

	if result.Skipped {
		t.Error("Expected transform to not be skipped")
	}
}

func TestFrontmatterTransformMissingMetadata(t *testing.T) {
	ft := NewFrontmatterTransform()

	input := &transforms.TransformInput{
		FilePath: "test.md",
		Content:  []byte("# Test"),
		Metadata: nil,
	}

	result := ft.Apply(input)
	if result == nil {
		t.Fatalf("Expected non-nil result from Apply")
	}

	if result.Metadata == nil {
		t.Error("Expected metadata to be initialized by Apply")
	}

	if _, ok := result.Metadata["title"]; !ok {
		t.Error("Expected title field to be added even with nil input metadata")
	}
}

func TestFrontmatterTransformSourcePath(t *testing.T) {
	ft := NewFrontmatterTransform()

	input := &transforms.TransformInput{
		FilePath: "docs/guide.md",
		Content:  []byte("test"),
		Metadata: map[string]interface{}{},
		Config: map[string]interface{}{
			"source_path_field": "original_path",
		},
	}

	result := ft.Apply(input)
	if result == nil {
		t.Fatalf("Expected non-nil result from Apply")
	}

	if source, ok := result.Metadata["original_path"]; !ok {
		t.Error("Expected custom source path field to be added")
	} else if source != "docs/guide.md" {
		t.Errorf("Expected source path to be 'docs/guide.md', got %v", source)
	}
}

func TestFrontmatterTransformImplementsPlugin(t *testing.T) {
	var _ plugin.Plugin = (*FrontmatterTransform)(nil)
}

func TestFrontmatterTransformImplementsTransformPlugin(t *testing.T) {
	var _ transforms.TransformPlugin = (*FrontmatterTransform)(nil)
}

func TestFrontmatterTransformContentPreservation(t *testing.T) {
	ft := NewFrontmatterTransform()

	testCases := [][]byte{
		[]byte("Simple content"),
		[]byte("# Markdown\n\n## Section\n\nContent here"),
		[]byte("Line 1\nLine 2\nLine 3"),
		[]byte(""),
	}

	for _, content := range testCases {
		input := &transforms.TransformInput{
			FilePath: "test.md",
			Content:  content,
			Metadata: map[string]interface{}{},
		}

		result := ft.Apply(input)
		if result == nil {
			t.Errorf("Expected non-nil result with content %q", content)
			continue
		}

		if result.Content != nil && string(result.Content) != string(content) {
			t.Errorf("Content was modified. Expected %q, got %q", content, result.Content)
		}
	}
}

func TestFrontmatterTransformMultipleApplications(t *testing.T) {
	ft := NewFrontmatterTransform()

	input1 := &transforms.TransformInput{
		FilePath: "test1.md",
		Content:  []byte("content1"),
		Metadata: map[string]interface{}{},
	}

	result1 := ft.Apply(input1)
	if result1 == nil {
		t.Fatalf("First apply returned nil")
	}

	input2 := &transforms.TransformInput{
		FilePath: "test2.md",
		Content:  []byte("content2"),
		Metadata: map[string]interface{}{},
	}

	result2 := ft.Apply(input2)
	if result2 == nil {
		t.Fatalf("Second apply returned nil")
	}

	if result1.Metadata == nil || result2.Metadata == nil {
		t.Error("Both results should have metadata")
	}

	if result1.Metadata["source_file"] == result2.Metadata["source_file"] {
		t.Error("Metadata should be independent for different inputs")
	}
}
