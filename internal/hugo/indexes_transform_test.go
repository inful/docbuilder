package hugo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadmeIndexPreservesTransforms verifies that when README.md is promoted to _index.md,
// all transforms (link rewrites, front matter patches, etc.) are preserved.
func TestReadmeIndexPreservesTransforms(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()

	// Create config
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	// Create generator
	gen := NewGenerator(cfg, tmpDir)

	// Create README with relative link that should be rewritten
	readmeContent := `---
title: Test Repository
---
# Guide
See [other doc](./guide.md) for details.
Also check [section doc](section/doc.md).
`

	readme := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "README.md"),
		RelativePath: "test-repo/README.md",
		Repository:   "test-repo",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte(readmeContent),
		IsAsset:      false,
	}

	// Run transform pipeline (copyContentFiles)
	ctx := context.Background()
	files := []docs.DocFile{readme}

	err := gen.copyContentFiles(ctx, files)
	require.NoError(t, err, "Transform pipeline should succeed")

	// Verify TransformedBytes was populated
	require.NotEmpty(t, files[0].TransformedBytes, "TransformedBytes should be populated after copyContentFiles")

	// Verify that transformed content has rewritten links
	transformedStr := string(files[0].TransformedBytes)
	assert.NotContains(t, transformedStr, "./guide.md", "Relative link ./guide.md should be rewritten")
	assert.Contains(t, transformedStr, "./guide/", "Should contain directory-style link")

	// Now generate indexes (which should use TransformedBytes)
	err = gen.generateRepositoryIndexes(files)
	require.NoError(t, err, "Index generation should succeed")

	// Read generated _index.md
	indexPath := filepath.Join(tmpDir, "content", "test-repo", "_index.md")
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(t, err, "Should be able to read generated index")

	indexStr := string(indexContent)

	// CRITICAL: Verify links were rewritten to directory-style (Hugo convention for index pages)
	assert.NotContains(t, indexStr, "./guide.md", "Index should not contain relative .md link")
	assert.NotContains(t, indexStr, "section/doc.md", "Index should not contain .md extension")

	// Verify directory-style links are present (Hugo index page convention)
	assert.Contains(t, indexStr, "./guide/", "Index should contain directory-style link for guide")
	assert.Contains(t, indexStr, "section/doc/", "Index should contain directory-style link for section doc")

	// Verify front matter was preserved/augmented
	assert.Contains(t, indexStr, "title:", "Should have title in front matter")
	assert.Contains(t, indexStr, "type: docs", "Should have type: docs added by index generation")
	assert.Contains(t, indexStr, "repository: test-repo", "Should have repository field")
}

// TestReadmeIndexWithoutFrontMatter verifies index generation for README without front matter
func TestReadmeIndexWithoutFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	gen := NewGenerator(cfg, tmpDir)

	// README without front matter but with relative links
	readmeContent := `# Welcome

Check [the guide](./guide.md) for more info.
`

	readme := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "README.md"),
		RelativePath: "repo/README.md",
		Repository:   "repo",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte(readmeContent),
		IsAsset:      false,
	}

	ctx := context.Background()
	files := []docs.DocFile{readme}

	err := gen.copyContentFiles(ctx, files)
	require.NoError(t, err)

	err = gen.generateRepositoryIndexes(files)
	require.NoError(t, err)

	indexPath := filepath.Join(tmpDir, "content", "repo", "_index.md")
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	indexStr := string(indexContent)

	// Should have added front matter
	assert.Contains(t, indexStr, "---\n", "Should have front matter delimiters")
	assert.Contains(t, indexStr, "type: docs", "Should have type field")

	// Should have rewritten links (directory-style for index pages)
	assert.NotContains(t, indexStr, "./guide.md", "Should not contain relative .md link")
	assert.Contains(t, indexStr, "./guide/", "Should contain directory-style link")
}

// TestReadmeIndexInSubdirectory is skipped because section indexes are generated from templates,
// not from README files. Section indexes don't support README promotion.
func TestReadmeIndexInSubdirectory(t *testing.T) {
	t.Skip("Section indexes are template-generated, not from READMEs")
}

// TestReadmeIndexWithComplexFrontMatter verifies handling of complex front matter
func TestReadmeIndexWithComplexFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	gen := NewGenerator(cfg, tmpDir)

	readmeContent := `---
title: Complex Doc
weight: 10
tags:
  - api
  - reference
custom_field: value
---
# Documentation
Link to [guide](./guide.md).
`

	readme := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "README.md"),
		RelativePath: "repo/README.md",
		Repository:   "repo",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte(readmeContent),
		IsAsset:      false,
	}

	ctx := context.Background()
	files := []docs.DocFile{readme}

	err := gen.copyContentFiles(ctx, files)
	require.NoError(t, err)

	err = gen.generateRepositoryIndexes(files)
	require.NoError(t, err)

	indexPath := filepath.Join(tmpDir, "content", "repo", "_index.md")
	indexContent, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	indexStr := string(indexContent)

	// Verify custom front matter preserved
	assert.Contains(t, indexStr, "weight: 10", "Should preserve weight")
	assert.Contains(t, indexStr, "tags:", "Should preserve tags")
	assert.Contains(t, indexStr, "custom_field: value", "Should preserve custom fields")

	// Verify required fields added
	assert.Contains(t, indexStr, "type: docs", "Should add type field")

	// Verify link rewriting still works
	assert.NotContains(t, indexStr, "./guide.md", "Should not contain relative link")
}

// TestReadmeTransformedBytesNotPopulated verifies error handling when TransformedBytes is empty
func TestReadmeTransformedBytesNotPopulated(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	gen := NewGenerator(cfg, tmpDir)

	// Create README but DON'T run copyContentFiles
	readme := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "README.md"),
		RelativePath: "repo/README.md",
		Repository:   "repo",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte("# Test"),
		IsAsset:      false,
		// TransformedBytes deliberately NOT populated
	}

	files := []docs.DocFile{readme}

	// Try to generate index without running transforms
	err := gen.generateRepositoryIndexes(files)

	// Should get an error about missing TransformedBytes
	require.Error(t, err, "Should fail when TransformedBytes not populated")
	assert.Contains(t, err.Error(), "not yet transformed", "Error should mention transform requirement")
}

// TestMultipleRepositoriesWithREADMEs verifies handling of multiple repos each with README
func TestMultipleRepositoriesWithREADMEs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	gen := NewGenerator(cfg, tmpDir)

	// Create two READMEs with cross-references
	readme1 := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "repo1", "README.md"),
		RelativePath: "repo1/README.md",
		Repository:   "repo1",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte("# Repo 1\nSee [local](./local.md) and [repo2 doc](../repo2/doc.md)."),
		IsAsset:      false,
	}

	readme2 := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "repo2", "README.md"),
		RelativePath: "repo2/README.md",
		Repository:   "repo2",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte("# Repo 2\nSee [guide](./guide.md)."),
		IsAsset:      false,
	}

	ctx := context.Background()
	files := []docs.DocFile{readme1, readme2}

	err := gen.copyContentFiles(ctx, files)
	require.NoError(t, err)

	err = gen.generateRepositoryIndexes(files)
	require.NoError(t, err)

	// Check repo1 index
	index1Path := filepath.Join(tmpDir, "content", "repo1", "_index.md")
	index1Content, err := os.ReadFile(index1Path)
	require.NoError(t, err)

	index1Str := string(index1Content)
	assert.NotContains(t, index1Str, "./local.md", "Should not contain relative .md link")
	assert.Contains(t, index1Str, "./local/", "Should contain directory-style link for repo1")

	// Check repo2 index
	index2Path := filepath.Join(tmpDir, "content", "repo2", "_index.md")
	index2Content, err := os.ReadFile(index2Path)
	require.NoError(t, err)

	index2Str := string(index2Content)
	assert.NotContains(t, index2Str, "./guide.md", "Should not contain relative .md link")
	assert.Contains(t, index2Str, "./guide/", "Should contain directory-style link for repo2")
}

// TestTransformPipelineOrderInvariance verifies behavior is consistent regardless of file order
func TestTransformPipelineOrderInvariance(t *testing.T) {
	// Run the same test twice with files in different order
	for testRun := 0; testRun < 2; testRun++ {
		tmpDir := t.TempDir()

		cfg := &config.Config{
			Hugo: config.HugoConfig{
				Title: "Test Site",
			},
		}

		gen := NewGenerator(cfg, tmpDir)

		readme := docs.DocFile{
			Path:         filepath.Join(tmpDir, "src", "README.md"),
			RelativePath: "repo/README.md",
			Repository:   "repo",
			Name:         "README",
			Extension:    ".md",
			Content:      []byte("# Test\n[link](./doc.md)"),
			IsAsset:      false,
		}

		other := docs.DocFile{
			Path:         filepath.Join(tmpDir, "src", "other.md"),
			RelativePath: "repo/other.md",
			Repository:   "repo",
			Name:         "other",
			Extension:    ".md",
			Content:      []byte("# Other\n[link](./doc.md)"),
			IsAsset:      false,
		}

		var files []docs.DocFile
		if testRun == 0 {
			files = []docs.DocFile{readme, other}
		} else {
			files = []docs.DocFile{other, readme}
		}

		ctx := context.Background()
		err := gen.copyContentFiles(ctx, files)
		require.NoError(t, err)

		err = gen.generateRepositoryIndexes(files)
		require.NoError(t, err)

		indexPath := filepath.Join(tmpDir, "content", "repo", "_index.md")
		indexContent, err := os.ReadFile(indexPath)
		require.NoError(t, err)

		indexStr := string(indexContent)
		assert.NotContains(t, indexStr, "./doc.md", "Should not contain relative .md link regardless of order")
		assert.Contains(t, indexStr, "./doc/", "Should contain directory-style link regardless of order")
	}
}

// TestReadmeWithForgeNamespace verifies README handling with forge namespacing
func TestReadmeWithForgeNamespace(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Test Site",
		},
	}

	gen := NewGenerator(cfg, tmpDir)

	readme := docs.DocFile{
		Path:         filepath.Join(tmpDir, "src", "README.md"),
		RelativePath: "github/myorg/repo/README.md",
		Repository:   "repo",
		Forge:        "github/myorg",
		Name:         "README",
		Extension:    ".md",
		Content:      []byte("# Test\n[guide](./guide.md)"),
		IsAsset:      false,
	}

	ctx := context.Background()
	files := []docs.DocFile{readme}

	err := gen.copyContentFiles(ctx, files)
	require.NoError(t, err)

	// Verify the file has TransformedBytes
	require.NotEmpty(t, files[0].TransformedBytes)

	// The transformed content should have forge-aware links
	transformedStr := string(files[0].TransformedBytes)
	// Link rewriter should handle forge namespace in path
	assert.NotContains(t, transformedStr, "./guide.md", "Should not contain relative link")
}
