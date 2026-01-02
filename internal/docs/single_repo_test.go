package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSingleRepoPathGeneration tests that single-repository builds skip the repository namespace.
// This implements ADR-006: Drop "local" namespace for single-repository builds.
func TestSingleRepoPathGeneration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test repository structure with API docs
	repoDir := filepath.Join(tempDir, "my-docs")
	docsDir := filepath.Join(repoDir, "docs")

	require.NoError(t, os.MkdirAll(filepath.Join(docsDir, "api"), 0o750))

	testFiles := map[string]string{
		"docs/index.md":       "# Documentation\n",
		"docs/api/guide.md":   "# API Guide\n",
		"docs/api/methods.md": "# API Methods\n",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(repoDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o600))
	}

	// Create repository configuration (single repo)
	repos := []config.Repository{
		{
			Name:  "my-docs",
			URL:   "https://github.com/example/my-docs.git",
			Paths: []string{"docs"},
		},
	}

	repoPaths := map[string]string{
		"my-docs": repoDir,
	}

	// Run discovery
	d := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})
	files, err := d.DiscoverDocs(repoPaths)
	require.NoError(t, err)
	require.Len(t, files, 3, "expected 3 markdown files")

	// Verify isSingleRepo flag was set
	assert.True(t, d.isSingleRepo, "isSingleRepo should be true for single repository")

	// Verify Hugo paths skip repository namespace
	expectedPaths := map[string]bool{
		filepath.Join("content", "_index.md"):         false,
		filepath.Join("content", "api", "guide.md"):   false,
		filepath.Join("content", "api", "methods.md"): false,
	}

	for _, file := range files {
		hugoPath := file.GetHugoPath(d.isSingleRepo)
		t.Logf("File: %s → Hugo Path: %s", file.RelativePath, hugoPath)

		if _, exists := expectedPaths[hugoPath]; !exists {
			t.Errorf("Unexpected Hugo path: %s (file: %s)", hugoPath, file.RelativePath)
		}
		expectedPaths[hugoPath] = true

		// Verify paths do NOT contain repository name
		assert.NotContains(t, hugoPath, "my-docs", "Single-repo path should not contain repository name")
	}

	// Verify all expected paths were found
	for path, found := range expectedPaths {
		assert.True(t, found, "Expected path not found: %s", path)
	}
}

// TestMultiRepoPathGeneration tests that multi-repository builds include the repository namespace.
func TestMultiRepoPathGeneration(t *testing.T) {
	tempDir := t.TempDir()

	// Create two test repositories
	repo1Dir := filepath.Join(tempDir, "repo-a")
	repo2Dir := filepath.Join(tempDir, "repo-b")

	require.NoError(t, os.MkdirAll(filepath.Join(repo1Dir, "docs"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(repo2Dir, "docs"), 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(repo1Dir, "docs", "index.md"), []byte("# Repo A\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repo2Dir, "docs", "index.md"), []byte("# Repo B\n"), 0o600))

	// Create repository configurations (multi repo)
	repos := []config.Repository{
		{Name: "repo-a", URL: "https://github.com/example/repo-a.git", Paths: []string{"docs"}},
		{Name: "repo-b", URL: "https://github.com/example/repo-b.git", Paths: []string{"docs"}},
	}

	repoPaths := map[string]string{
		"repo-a": repo1Dir,
		"repo-b": repo2Dir,
	}

	// Run discovery
	d := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})
	files, err := d.DiscoverDocs(repoPaths)
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Verify isSingleRepo flag was NOT set
	assert.False(t, d.isSingleRepo, "isSingleRepo should be false for multiple repositories")

	// Verify Hugo paths INCLUDE repository namespace
	for _, file := range files {
		hugoPath := file.GetHugoPath(d.isSingleRepo)
		t.Logf("File: %s (repo: %s) → Hugo Path: %s", file.RelativePath, file.Repository, hugoPath)

		// Paths MUST contain repository name
		assert.Contains(t, hugoPath, file.Repository,
			"Multi-repo path must contain repository name: %s", file.Repository)

		// Expected format: content/{repository}/_index.md
		expectedPath := filepath.Join("content", file.Repository, "_index.md")
		assert.Equal(t, expectedPath, hugoPath)
	}
}

// TestGetHugoPath_ParameterOverride tests that the isSingleRepo parameter overrides struct behavior.
func TestGetHugoPath_ParameterOverride(t *testing.T) {
	// Create a DocFile instance
	df := &DocFile{
		Repository:   "my-docs",
		Section:      "api",
		Name:         "guide",
		Extension:    ".md",
		RelativePath: "api/guide.md",
	}

	// Test with isSingleRepo=true (skip namespace)
	singleRepoPath := df.GetHugoPath(true)
	expectedSingle := filepath.Join("content", "api", "guide.md")
	assert.Equal(t, expectedSingle, singleRepoPath,
		"Single-repo mode should skip repository namespace")

	// Test with isSingleRepo=false (include namespace)
	multiRepoPath := df.GetHugoPath(false)
	expectedMulti := filepath.Join("content", "my-docs", "api", "guide.md")
	assert.Equal(t, expectedMulti, multiRepoPath,
		"Multi-repo mode should include repository namespace")

	// Verify they are different
	assert.NotEqual(t, singleRepoPath, multiRepoPath,
		"Single-repo and multi-repo paths should differ")
}
