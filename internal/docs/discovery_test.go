package docs

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestDocumentationDiscovery(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "docbuilder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test repository structure
	repoDir := filepath.Join(tempDir, "test-repo")
	docsDir := filepath.Join(repoDir, "docs")

	// Create directories
	os.MkdirAll(filepath.Join(docsDir, "api"), 0755)
	os.MkdirAll(filepath.Join(docsDir, "guides"), 0755)

	// Create test markdown files
	testFiles := map[string]string{
		"docs/index.md":                  "# Documentation Index\n\nWelcome to the docs.",
		"docs/api/overview.md":           "# API Overview\n\nAPI documentation.",
		"docs/api/reference.md":          "# API Reference\n\nDetailed API reference.",
		"docs/guides/getting-started.md": "# Getting Started\n\nHow to get started.",
		"docs/README.md":                 "# Repository README\n\nThis should be ignored.",
		"docs/non-markdown.txt":          "This is not markdown and should be ignored.",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(repoDir, path)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create repository configuration
	repos := []config.Repository{
		{
			Name:  "test-repo",
			Paths: []string{"docs"},
			Tags:  map[string]string{"section": "test"},
		},
	}

	// Create discovery instance
	discovery := NewDiscovery(repos)

	// Test discovery
	repoPaths := map[string]string{
		"test-repo": repoDir,
	}

	docFiles, err := discovery.DiscoverDocs(repoPaths)
	if err != nil {
		t.Fatalf("DiscoverDocs failed: %v", err)
	}

	// Verify results
	expectedFiles := []string{
		"index.md",
		"api/overview.md",
		"api/reference.md",
		"guides/getting-started.md",
	}

	if len(docFiles) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(docFiles))
	}

	// Check that README.md and .txt files are ignored
	for _, file := range docFiles {
		if file.Name == "README" || file.Extension == ".txt" {
			t.Errorf("File should have been ignored: %s", file.RelativePath)
		}
	}

	// Test file grouping
	filesByRepo := discovery.GetDocFilesByRepository()
	if len(filesByRepo["test-repo"]) != len(expectedFiles) {
		t.Errorf("Expected %d files for test-repo, got %d",
			len(expectedFiles), len(filesByRepo["test-repo"]))
	}
}

func TestMarkdownFileDetection(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.md", true},
		{"test.markdown", true},
		{"test.mdown", true},
		{"test.mkd", true},
		{"test.txt", false},
		{"test.html", false},
		{"test", false},
	}

	for _, test := range tests {
		result := isMarkdownFile(test.filename)
		if result != test.expected {
			t.Errorf("isMarkdownFile(%s) = %v, expected %v",
				test.filename, result, test.expected)
		}
	}
}

func TestIgnoredFiles(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"README.md", true},
		{"CONTRIBUTING.md", true},
		{"CHANGELOG.md", true},
		{"LICENSE.md", true},
		{"index.md", false},
		{"guide.md", false},
		{"api.md", false},
	}

	for _, test := range tests {
		result := isIgnoredFile(test.filename)
		if result != test.expected {
			t.Errorf("isIgnoredFile(%s) = %v, expected %v",
				test.filename, result, test.expected)
		}
	}
}
