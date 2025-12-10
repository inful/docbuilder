package models

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformContext(t *testing.T) {
	provider := &mockGeneratorProvider{}

	context := NewTransformContext(provider).
		WithSource("test_transformer").
		WithPriority(10).
		WithProperty("test_key", "test_value")

	assert.Equal(t, "test_transformer", context.Source)
	assert.Equal(t, 10, context.Priority)

	value, exists := context.GetProperty("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", value)

	str, exists := context.GetPropertyString("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", str)

	_, exists = context.GetProperty("missing_key")
	assert.False(t, exists)
}

func TestTransformationResult(t *testing.T) {
	result := NewTransformationResult()

	// Test success
	result.SetSuccess()
	assert.True(t, result.Success)
	assert.Nil(t, result.Error)

	// Test adding changes
	result.AddChange(
		ChangeTypeContentModified,
		"content",
		"old content",
		"new content",
		"test change",
		"test_transformer",
	)

	assert.True(t, result.HasChanges())
	assert.True(t, result.ContentModified)
	assert.False(t, result.FrontMatterModified)
	assert.Len(t, result.Changes, 1)

	change := result.Changes[0]
	assert.Equal(t, ChangeTypeContentModified, change.Type)
	assert.Equal(t, "content", change.Field)
	assert.Equal(t, "old content", change.OldValue)
	assert.Equal(t, "new content", change.NewValue)

	// Test filtering changes
	contentChanges := result.GetChangesByType(ChangeTypeContentModified)
	assert.Len(t, contentChanges, 1)

	fieldChanges := result.GetChangesByField("content")
	assert.Len(t, fieldChanges, 1)
}

func TestContentPage(t *testing.T) {
	file := docs.DocFile{
		Path:       "test.md",
		Name:       "test.md",
		Repository: "test-repo",
		Section:    "docs",
	}

	page := NewContentPage(file)
	assert.Equal(t, file, page.File)
	assert.NotNil(t, page.FrontMatter)
	assert.False(t, page.IsModified())

	// Test content modification
	page.SetContent("# Test Content")
	assert.Equal(t, "# Test Content", page.GetContent())
	assert.True(t, page.ContentModified)
	assert.True(t, page.IsModified())

	// Test front matter modification
	fm := NewFrontMatter()
	fm.Title = "Test Title"
	page.SetFrontMatter(fm)
	assert.Equal(t, "Test Title", page.GetFrontMatter().Title)
	assert.True(t, page.FrontMatterModified)

	// Test patch application
	patch := NewFrontMatterPatch().SetDescription("Test Description")
	page.AddFrontMatterPatch(patch)
	assert.Len(t, page.FrontMatterPatches, 1)

	err := page.ApplyFrontMatterPatches()
	require.NoError(t, err)
	assert.Equal(t, "Test Description", page.GetFrontMatter().Description)
}

func TestContentPageClone(t *testing.T) {
	file := docs.DocFile{
		Path:       "test.md",
		Name:       "test.md",
		Repository: "test-repo",
	}

	original := NewContentPage(file)
	original.SetContent("Original content")

	fm := NewFrontMatter()
	fm.Title = "Original Title"
	original.SetFrontMatter(fm)

	clone := original.Clone()
	assert.Equal(t, original.GetContent(), clone.GetContent())
	assert.Equal(t, original.GetFrontMatter().Title, clone.GetFrontMatter().Title)

	// Modify clone and verify original is unchanged
	clone.SetContent("Modified content")
	clone.GetFrontMatter().Title = "Modified Title"

	assert.Equal(t, "Original content", original.GetContent())
	assert.Equal(t, "Original Title", original.GetFrontMatter().Title)
	assert.Equal(t, "Modified content", clone.GetContent())
	assert.Equal(t, "Modified Title", clone.GetFrontMatter().Title)
}

func TestTypedTransformerRegistry(t *testing.T) {
	registry := NewTypedTransformerRegistry()

	// Create test transformers
	transformer1 := &mockTransformer{
		name:     "transformer1",
		priority: 20,
	}
	transformer2 := &mockTransformer{
		name:     "transformer2",
		priority: 10,
	}

	// Register transformers
	err := registry.Register(transformer1)
	require.NoError(t, err)

	err = registry.Register(transformer2)
	require.NoError(t, err)

	// Test duplicate registration
	err = registry.Register(transformer1)
	assert.Error(t, err)

	// Test retrieval
	retrieved, exists := registry.Get("transformer1")
	assert.True(t, exists)
	assert.Equal(t, transformer1, retrieved)

	_, exists = registry.Get("nonexistent")
	assert.False(t, exists)

	// Test listing
	all := registry.List()
	assert.Len(t, all, 2)

	// Test priority sorting
	sorted := registry.ListByPriority()
	assert.Len(t, sorted, 2)
	assert.Equal(t, "transformer2", sorted[0].Name()) // Lower priority first
	assert.Equal(t, "transformer1", sorted[1].Name())
}

func TestFrontMatterParserV2(t *testing.T) {
	parser := NewFrontMatterParserV2()

	assert.Equal(t, "front_matter_parser_v2", parser.Name())

	// Test with front matter
	content := `---
title: Test Title
description: Test Description
---

# Content Here`

	file := docs.DocFile{Path: "test.md", Name: "test.md"}
	page := NewContentPage(file)
	page.SetContent(content)

	context := NewTransformContext(&mockGeneratorProvider{})

	canTransform := parser.CanTransform(page, context)
	assert.True(t, canTransform)

	result, err := parser.Transform(page, context)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify front matter was parsed
	assert.True(t, page.HadOriginalFrontMatter)
	assert.NotNil(t, page.GetOriginalFrontMatter())
	assert.Equal(t, "Test Title", page.GetOriginalFrontMatter().Title)
	assert.Equal(t, "Test Description", page.GetOriginalFrontMatter().Description)

	// Verify content was updated
	assert.Equal(t, "\n# Content Here", page.GetContent())

	// Verify changes were recorded
	assert.True(t, result.HasChanges())
	assert.True(t, result.FrontMatterModified)
	assert.True(t, result.ContentModified)
}

func TestFrontMatterParserV2_NoFrontMatter(t *testing.T) {
	parser := NewFrontMatterParserV2()

	content := "# Just Content"

	file := docs.DocFile{Path: "test.md", Name: "test.md"}
	page := NewContentPage(file)
	page.SetContent(content)

	context := NewTransformContext(&mockGeneratorProvider{})

	canTransform := parser.CanTransform(page, context)
	assert.False(t, canTransform)

	result, err := parser.Transform(page, context)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify nothing changed
	assert.False(t, page.HadOriginalFrontMatter)
	assert.Equal(t, content, page.GetContent())
	assert.False(t, result.HasChanges())
}

func TestFrontMatterBuilderV3(t *testing.T) {
	builder := NewFrontMatterBuilderV3()

	assert.Equal(t, "front_matter_builder_v3", builder.Name())

	file := docs.DocFile{
		Path:       "docs/api/example.md",
		Name:       "example.md",
		Repository: "test-repo",
		Section:    "api",
		Forge:      "github",
		Metadata: map[string]string{
			"category": "api",
			"version":  "1.0",
		},
	}

	page := NewContentPage(file)
	context := NewTransformContext(&mockGeneratorProvider{})

	canTransform := builder.CanTransform(page, context)
	assert.True(t, canTransform)

	result, err := builder.Transform(page, context)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// Verify patch was added
	assert.Len(t, page.FrontMatterPatches, 1)

	// Apply patches and verify content
	err = page.ApplyFrontMatterPatches()
	require.NoError(t, err)

	fm := page.GetFrontMatter()
	assert.Equal(t, "example", fm.Title)
	assert.Equal(t, "test-repo", fm.Repository)
	assert.Equal(t, "api", fm.Section)
	assert.Equal(t, "github", fm.Forge)

	// Verify custom metadata
	category, exists := fm.GetCustomString("category")
	assert.True(t, exists)
	assert.Equal(t, "api", category)

	version, exists := fm.GetCustomString("version")
	assert.True(t, exists)
	assert.Equal(t, "1.0", version)
}

func TestTransformationPipeline(t *testing.T) {
	pipeline := NewTransformationPipeline(
		"test_pipeline",
		"Test transformation pipeline",
		"1.0.0",
	)

	assert.Equal(t, "test_pipeline", pipeline.Name)
	assert.False(t, pipeline.IsRunning())
	assert.False(t, pipeline.IsComplete())
	assert.False(t, pipeline.IsFailed())

	// Start pipeline
	pipeline.Start()
	assert.True(t, pipeline.Started)
	assert.True(t, pipeline.IsRunning())

	// Add results
	successResult := NewTransformationResult().SetSuccess().SetDuration(time.Millisecond)
	failureResult := NewTransformationResult().SetError(assert.AnError).SetDuration(time.Millisecond)

	pipeline.AddResult(*successResult).AddResult(*failureResult)
	assert.Len(t, pipeline.Results, 2)

	// Complete pipeline
	pipeline.Complete()
	assert.True(t, pipeline.Completed)
	assert.False(t, pipeline.IsRunning())
	assert.True(t, pipeline.IsComplete())

	// Test result filtering
	successful := pipeline.GetSuccessfulResults()
	failed := pipeline.GetFailedResults()

	assert.Len(t, successful, 1)
	assert.Len(t, failed, 1)
	assert.True(t, successful[0].Success)
	assert.False(t, failed[0].Success)
}

// Mock implementations for testing

type mockGeneratorProvider struct{}

func (m *mockGeneratorProvider) GetConfig() ConfigProvider {
	return &mockConfigProvider{}
}

func (m *mockGeneratorProvider) GetEditLinkResolver() EditLinkResolver {
	return &mockEditLinkResolver{}
}

func (m *mockGeneratorProvider) GetThemeCapabilities() ThemeCapabilities {
	return ThemeCapabilities{
		WantsPerPageEditLinks: true,
	}
}

func (m *mockGeneratorProvider) GetForgeCapabilities(_ string) ForgeCapabilities {
	return ForgeCapabilities{
		SupportsEditLinks: true,
	}
}

type mockConfigProvider struct{}

func (m *mockConfigProvider) GetHugoConfig() HugoConfig {
	return HugoConfig{ThemeType: "hextra"}
}

func (m *mockConfigProvider) GetForgeConfig() ForgeConfig {
	return ForgeConfig{EditLinks: true}
}

func (m *mockConfigProvider) GetTransformConfig() TransformConfig {
	return TransformConfig{}
}

type mockEditLinkResolver struct{}

func (m *mockEditLinkResolver) Resolve(file docs.DocFile) string {
	return "https://github.com/test/edit/" + file.Path
}

func (m *mockEditLinkResolver) SupportsFile(_ docs.DocFile) bool {
	return true
}

type mockTransformer struct {
	name     string
	priority int
	stage    TransformStage
	enabled  bool
}

func (m *mockTransformer) Name() string        { return m.name }
func (m *mockTransformer) Description() string { return "Mock transformer" }
func (m *mockTransformer) Version() string     { return "1.0.0" }
func (m *mockTransformer) Priority() int       { return m.priority }
func (m *mockTransformer) Stage() TransformStage {
	if m.stage == "" {
		return StageParse // Default stage
	}
	return m.stage
}
func (m *mockTransformer) Dependencies() TransformerDependencies { return TransformerDependencies{} }
func (m *mockTransformer) Configuration() TransformerConfiguration {
	return TransformerConfiguration{Enabled: m.enabled}
}
func (m *mockTransformer) CanTransform(_ *ContentPage, _ *TransformContext) bool {
	return m.enabled
}
func (m *mockTransformer) RequiredContext() []string { return []string{} }
func (m *mockTransformer) Transform(_ *ContentPage, _ *TransformContext) (*TransformationResult, error) {
	return NewTransformationResult().SetSuccess(), nil
}
