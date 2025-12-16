package pipeline

import (
	"errors"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessContent_EmptyInput(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	docs, err := processor.ProcessContent([]*Document{}, map[string]RepositoryInfo{})
	require.NoError(t, err)
	// Should generate main index even with no input
	assert.NotEmpty(t, docs, "should generate at least main index")
}

func TestProcessContent_WithDocuments(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test Site"},
	}
	processor := NewProcessor(cfg)

	discovered := []*Document{
		{
			Content:     "# Test Doc\n\nContent here.",
			FrontMatter: make(map[string]any),
			Path:        "repo/test.md",
			Repository:  "repo",
			Name:        "test",
			Extension:   ".md",
			IsIndex:     false,
			Generated:   false,
		},
	}

	repoMetadata := map[string]RepositoryInfo{
		"repo": {
			Name:   "repo",
			URL:    "https://example.com/repo.git",
			Commit: "abc123",
			Branch: "main",
		},
	}

	docs, err := processor.ProcessContent(discovered, repoMetadata)
	require.NoError(t, err)
	assert.Greater(t, len(docs), len(discovered), "should have generated additional documents")

	// Verify repository metadata was injected
	var foundOriginal bool
	for _, doc := range docs {
		if doc.Path == "repo/test.md" {
			foundOriginal = true
			assert.Equal(t, "https://example.com/repo.git", doc.SourceURL)
			assert.Equal(t, "abc123", doc.SourceCommit)
		}
	}
	assert.True(t, foundOriginal, "original document should be in output")
}

func TestProcessContent_GeneratorError(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Add failing generator
	failingGenerator := func(ctx *GenerationContext) ([]*Document, error) {
		return nil, errors.New("generator failed")
	}
	processor.WithGenerators([]FileGenerator{failingGenerator})

	_, err := processor.ProcessContent([]*Document{}, map[string]RepositoryInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generator 0 failed")
}

func TestProcessContent_TransformError(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Add failing transform
	failingTransform := func(doc *Document) ([]*Document, error) {
		return nil, errors.New("transform failed")
	}
	processor.WithGenerators([]FileGenerator{}) // No generators
	processor.WithTransforms([]FileTransform{failingTransform})

	discovered := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		},
	}

	_, err := processor.ProcessContent(discovered, map[string]RepositoryInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transformation phase failed")
	assert.Contains(t, err.Error(), "transform failed")
}

func TestProcessTransforms_BasicFlow(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Simple transform that adds metadata
	addMetadata := func(doc *Document) ([]*Document, error) {
		if doc.FrontMatter == nil {
			doc.FrontMatter = make(map[string]any)
		}
		doc.FrontMatter["processed"] = true
		return nil, nil
	}

	processor.WithTransforms([]FileTransform{addMetadata})

	docs := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		},
	}

	processed, err := processor.processTransforms(docs)
	require.NoError(t, err)
	require.Len(t, processed, 1)
	assert.Equal(t, true, processed[0].FrontMatter["processed"])
}

func TestProcessTransforms_DynamicGeneration(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Transform that creates a new document
	generatingTransform := func(doc *Document) ([]*Document, error) {
		// Only generate from the original doc, not from generated ones
		if doc.Path == "original.md" {
			newDoc := &Document{
				Content:     "# Generated\n\nGenerated content.",
				FrontMatter: make(map[string]any),
				Path:        "generated.md",
				Generated:   true,
			}
			return []*Document{newDoc}, nil
		}
		return nil, nil
	}

	processor.WithTransforms([]FileTransform{generatingTransform})

	docs := []*Document{
		{
			Content:     "# Original\n\nOriginal content.",
			FrontMatter: make(map[string]any),
			Path:        "original.md",
			Generated:   false,
		},
	}

	processed, err := processor.processTransforms(docs)
	require.NoError(t, err)
	assert.Len(t, processed, 2, "should have original + generated document")

	// Verify both documents are present
	paths := make([]string, len(processed))
	for i, doc := range processed {
		paths[i] = doc.Path
	}
	assert.Contains(t, paths, "original.md")
	assert.Contains(t, paths, "generated.md")
}

func TestProcessTransforms_PreventGeneratedFromGenerating(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Transform that tries to generate from any document
	badTransform := func(doc *Document) ([]*Document, error) {
		newDoc := &Document{
			Content:     "new",
			FrontMatter: make(map[string]any),
			Path:        "new-" + doc.Path,
			Generated:   true,
		}
		return []*Document{newDoc}, nil
	}

	processor.WithTransforms([]FileTransform{badTransform})

	docs := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
			Generated:   true, // This is a generated document
		},
	}

	_, err := processor.processTransforms(docs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generated document")
	assert.Contains(t, err.Error(), "attempted to create new documents")
}

func TestProcessTransforms_TransformError(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	failingTransform := func(doc *Document) ([]*Document, error) {
		return nil, errors.New("transform error")
	}

	processor.WithTransforms([]FileTransform{failingTransform})

	docs := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		},
	}

	_, err := processor.processTransforms(docs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transform 0 failed")
	assert.Contains(t, err.Error(), "test.md")
}

func TestProcessTransforms_MultipleTransforms(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// First transform adds "step1" field
	transform1 := func(doc *Document) ([]*Document, error) {
		if doc.FrontMatter == nil {
			doc.FrontMatter = make(map[string]any)
		}
		doc.FrontMatter["step1"] = true
		return nil, nil
	}

	// Second transform adds "step2" field
	transform2 := func(doc *Document) ([]*Document, error) {
		if doc.FrontMatter == nil {
			doc.FrontMatter = make(map[string]any)
		}
		doc.FrontMatter["step2"] = true
		return nil, nil
	}

	processor.WithTransforms([]FileTransform{transform1, transform2})

	docs := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		},
	}

	processed, err := processor.processTransforms(docs)
	require.NoError(t, err)
	require.Len(t, processed, 1)

	// Both transforms should have run
	assert.Equal(t, true, processed[0].FrontMatter["step1"])
	assert.Equal(t, true, processed[0].FrontMatter["step2"])
}

func TestWithGenerators(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	customGenerator := func(ctx *GenerationContext) ([]*Document, error) {
		return []*Document{
			{
				Content:     "# Custom\n\nCustom generated content.",
				FrontMatter: make(map[string]any),
				Path:        "custom.md",
				Generated:   true,
			},
		}, nil
	}

	processor.WithGenerators([]FileGenerator{customGenerator})

	docs, err := processor.ProcessContent([]*Document{}, map[string]RepositoryInfo{})
	require.NoError(t, err)

	// Should only have the custom generated document
	require.Len(t, docs, 1)
	assert.Equal(t, "custom.md", docs[0].Path)
}

func TestWithTransforms(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	customTransform := func(doc *Document) ([]*Document, error) {
		if doc.FrontMatter == nil {
			doc.FrontMatter = make(map[string]any)
		}
		doc.FrontMatter["custom"] = "transformed"
		return nil, nil
	}

	processor.WithGenerators([]FileGenerator{}) // Disable default generators
	processor.WithTransforms([]FileTransform{customTransform})

	discovered := []*Document{
		{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		},
	}

	docs, err := processor.ProcessContent(discovered, map[string]RepositoryInfo{})
	require.NoError(t, err)
	require.Len(t, docs, 1)
	assert.Equal(t, "transformed", docs[0].FrontMatter["custom"])
}

func TestProcessContent_RepositoryMetadataInjection(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)
	processor.WithGenerators([]FileGenerator{}) // No generators
	processor.WithTransforms([]FileTransform{})  // No transforms

	discovered := []*Document{
		{
			Content:     "test1",
			FrontMatter: make(map[string]any),
			Path:        "repo1/test.md",
			Repository:  "repo1",
		},
		{
			Content:     "test2",
			FrontMatter: make(map[string]any),
			Path:        "repo2/test.md",
			Repository:  "repo2",
		},
		{
			Content:     "test3",
			FrontMatter: make(map[string]any),
			Path:        "unknown/test.md",
			Repository:  "unknown", // No metadata for this repo
		},
	}

	repoMetadata := map[string]RepositoryInfo{
		"repo1": {
			URL:    "https://example.com/repo1.git",
			Commit: "commit1",
		},
		"repo2": {
			URL:    "https://example.com/repo2.git",
			Commit: "commit2",
		},
	}

	docs, err := processor.ProcessContent(discovered, repoMetadata)
	require.NoError(t, err)

	// Find documents and verify metadata
	for _, doc := range docs {
		switch doc.Repository {
		case "repo1":
			assert.Equal(t, "https://example.com/repo1.git", doc.SourceURL)
			assert.Equal(t, "commit1", doc.SourceCommit)
		case "repo2":
			assert.Equal(t, "https://example.com/repo2.git", doc.SourceURL)
			assert.Equal(t, "commit2", doc.SourceCommit)
		case "unknown":
			assert.Empty(t, doc.SourceURL, "should not have URL for unknown repo")
			assert.Empty(t, doc.SourceCommit, "should not have commit for unknown repo")
		}
	}
}

func TestProcessTransforms_LargeDocumentSet(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}
	processor := NewProcessor(cfg)

	// Simple no-op transform
	noopTransform := func(doc *Document) ([]*Document, error) {
		return nil, nil
	}

	processor.WithTransforms([]FileTransform{noopTransform})

	// Create 150 documents to test progress logging (triggers every 100)
	docs := make([]*Document, 150)
	for i := 0; i < 150; i++ {
		docs[i] = &Document{
			Content:     "test",
			FrontMatter: make(map[string]any),
			Path:        "test.md",
		}
	}

	processed, err := processor.processTransforms(docs)
	require.NoError(t, err)
	assert.Len(t, processed, 150)
}

func TestDefaultTransforms_Order(t *testing.T) {
	cfg := &config.Config{
		Hugo: config.HugoConfig{Title: "Test"},
	}

	transforms := defaultTransforms(cfg)

	// Verify we have all expected transforms
	assert.Len(t, transforms, 11, "should have 11 transforms in pipeline")

	// Verify order by testing a document through the pipeline
	doc := &Document{
		Content: `---
title: Original Title
---
# Extracted Title

Content with [link](./other.md) and ![image](./img.png).
`,
		FrontMatter: make(map[string]any),
		Path:        "repo/_index.md",
		Repository:  "repo",
		Name:        "_index",
		IsIndex:     true,
		DocsBase:    "docs",
		Extension:   ".md",
	}

	// Run all transforms
	for _, transform := range transforms {
		_, err := transform(doc)
		require.NoError(t, err)
	}

	// After full pipeline, document should be serialized
	assert.NotNil(t, doc.Raw, "should have serialized output")
	assert.NotEmpty(t, doc.FrontMatter, "should have front matter")
}
