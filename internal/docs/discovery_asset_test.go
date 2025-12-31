package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

const testImageExtension = ".png"

func TestAssetPathInSubdirectories(t *testing.T) {
	// Create a temporary test repository
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	docsPath := filepath.Join(repoPath, "docs")
	guidesPath := filepath.Join(docsPath, "guides")
	imagesPath := filepath.Join(guidesPath, "images")

	// Create directory structure
	require.NoError(t, os.MkdirAll(imagesPath, 0o755))

	// Create markdown file in subdirectory
	mdContent := []byte("# Guide\n\n![Image](images/test.png)")
	require.NoError(t, os.WriteFile(filepath.Join(guidesPath, "tutorial.md"), mdContent, 0o644))

	// Create image in subdirectory's images folder
	imageContent := []byte("fake png content")
	require.NoError(t, os.WriteFile(filepath.Join(imagesPath, "test.png"), imageContent, 0o644))

	// Setup config
	cfg := &config.Config{
		Repositories: []config.Repository{
			{
				Name:  "test-repo",
				Paths: []string{"docs"},
				Tags:  map[string]string{},
			},
		},
	}

	disc := NewDiscovery(cfg.Repositories, &config.BuildConfig{})
	files, err := disc.DiscoverDocs(map[string]string{
		"test-repo": repoPath,
	})
	require.NoError(t, err)
	require.Len(t, files, 2, "Should discover markdown and image file")

	// Find the markdown and image files
	var mdFile, imageFile *DocFile
	for i := range files {
		switch files[i].Extension {
		case ".md":
			mdFile = &files[i]
		case testImageExtension:
			imageFile = &files[i]
		}
	}

	require.NotNil(t, mdFile, "Should find markdown file")
	require.NotNil(t, imageFile, "Should find image file")

	// Check relative paths
	assert.Equal(t, "guides/tutorial.md", mdFile.RelativePath)
	assert.Equal(t, "guides/images/test.png", imageFile.RelativePath)

	// Check sections
	assert.Equal(t, "guides", mdFile.Section)
	assert.Equal(t, "guides/images", imageFile.Section, "Image should preserve full path as section")

	// Check Hugo paths
	mdHugoPath := mdFile.GetHugoPath()
	imageHugoPath := imageFile.GetHugoPath()

	t.Logf("Markdown HugoPath: %s", mdHugoPath)
	t.Logf("Image HugoPath: %s", imageHugoPath)

	// The markdown file should be at: content/test-repo/guides/tutorial.md
	assert.Equal(t, filepath.Join("content", "test-repo", "guides", "tutorial.md"), mdHugoPath)

	// The image should be at: content/test-repo/guides/images/test.png
	// This is the key test - the image must preserve its relative path from the markdown file
	assert.Equal(t, filepath.Join("content", "test-repo", "guides", "images", "test.png"), imageHugoPath,
		"Image should be placed relative to markdown file to preserve references")
}

func TestAssetPathInRootWithImageSubdirectory(t *testing.T) {
	// Test case: docs/tutorial.md references docs/images/logo.png
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	docsPath := filepath.Join(repoPath, "docs")
	imagesPath := filepath.Join(docsPath, "images")

	require.NoError(t, os.MkdirAll(imagesPath, 0o755))

	// Create markdown in docs root
	mdContent := []byte("# Guide\n\n![Logo](images/logo.png)")
	require.NoError(t, os.WriteFile(filepath.Join(docsPath, "index.md"), mdContent, 0o644))

	// Create image in docs/images
	imageContent := []byte("fake png content")
	require.NoError(t, os.WriteFile(filepath.Join(imagesPath, "logo.png"), imageContent, 0o644))

	cfg := &config.Config{
		Repositories: []config.Repository{
			{
				Name:  "test-repo",
				Paths: []string{"docs"},
				Tags:  map[string]string{},
			},
		},
	}

	disc := NewDiscovery(cfg.Repositories, &config.BuildConfig{})
	files, err := disc.DiscoverDocs(map[string]string{
		"test-repo": repoPath,
	})
	require.NoError(t, err)

	// Find files
	var mdFile, imageFile *DocFile
	for i := range files {
		switch files[i].Extension {
		case ".md":
			mdFile = &files[i]
		case ".png":
			imageFile = &files[i]
		}
	}

	require.NotNil(t, mdFile)
	require.NotNil(t, imageFile)

	// Markdown at root has empty section
	assert.Equal(t, "", mdFile.Section)
	assert.Equal(t, filepath.Join("content", "test-repo", "_index.md"), mdFile.GetHugoPath())

	// Image should be in images subdirectory
	assert.Equal(t, "images", imageFile.Section)
	assert.Equal(t, filepath.Join("content", "test-repo", "images", "logo.png"), imageFile.GetHugoPath(),
		"Image should preserve images/ subdirectory")
}

func TestAssetMixedCaseFilename(t *testing.T) {
	// Test case: Image files with mixed case like 6_3_approve_MR.png
	// Should be normalized to lowercase for URL compatibility
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")
	docsPath := filepath.Join(repoPath, "docs")
	imagesPath := filepath.Join(docsPath, "images")

	require.NoError(t, os.MkdirAll(imagesPath, 0o755))

	// Create markdown referencing mixed-case image
	mdContent := []byte("# Guide\n\n![Approve button](./images/6_3_approve_MR.png)")
	require.NoError(t, os.WriteFile(filepath.Join(docsPath, "tutorial.md"), mdContent, 0o644))

	// Create image with mixed-case filename
	imageContent := []byte("fake png content")
	require.NoError(t, os.WriteFile(filepath.Join(imagesPath, "6_3_approve_MR.png"), imageContent, 0o644))

	cfg := &config.Config{
		Repositories: []config.Repository{
			{
				Name:  "test-repo",
				Paths: []string{"docs"},
				Tags:  map[string]string{},
			},
		},
	}

	disc := NewDiscovery(cfg.Repositories, &config.BuildConfig{})
	files, err := disc.DiscoverDocs(map[string]string{
		"test-repo": repoPath,
	})
	require.NoError(t, err)

	// Find the image file
	var imageFile *DocFile
	for i := range files {
		if files[i].Extension == ".png" {
			imageFile = &files[i]
			break
		}
	}

	require.NotNil(t, imageFile)

	// Image filename should be normalized to lowercase
	hugoPath := imageFile.GetHugoPath()
	expectedPath := filepath.Join("content", "test-repo", "images", "6_3_approve_mr.png")

	t.Logf("Image Hugo Path: %s", hugoPath)
	t.Logf("Expected Path: %s", expectedPath)
	t.Logf("Image Name: %s", imageFile.Name)
	t.Logf("Image Extension: %s", imageFile.Extension)

	assert.Equal(t, expectedPath, hugoPath,
		"Image filename should be fully lowercase for URL compatibility, including extension")
	assert.Equal(t, "6_3_approve_MR", imageFile.Name, "Original name preserved in struct")
	assert.Equal(t, ".png", imageFile.Extension, "Original extension preserved in struct")
}
