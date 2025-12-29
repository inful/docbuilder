package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		case ".png":
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
