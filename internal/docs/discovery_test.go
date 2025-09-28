package docs

import (
	"os"
	"path/filepath"
	"strings"
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
	if err := os.MkdirAll(filepath.Join(docsDir, "api"), 0755); err != nil {
		t.Fatalf("mkdir api: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(docsDir, "guides"), 0755); err != nil {
		t.Fatalf("mkdir guides: %v", err)
	}

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
	discovery := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})

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

func TestForgeNamespacingModes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docbuilder-ns-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mkRepo := func(name, forgeType string) (config.Repository, string) {
		repoDir := filepath.Join(tempDir, name)
		docsDir := filepath.Join(repoDir, "docs")
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, "page.md"), []byte("# Page"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return config.Repository{Name: name, Paths: []string{"docs"}, Tags: map[string]string{"forge_type": forgeType}}, repoDir
	}

	// Two repos on different forges
	r1, p1 := mkRepo("repoA", "github")
	r2, p2 := mkRepo("repoB", "gitlab")

	repoPaths := map[string]string{r1.Name: p1, r2.Name: p2}
	repos := []config.Repository{r1, r2}

	run := func(mode config.NamespacingMode) []DocFile {
		bc := &config.BuildConfig{NamespaceForges: mode}
		d := NewDiscovery(repos, bc)
		files, err := d.DiscoverDocs(repoPaths)
		if err != nil {
			t.Fatalf("DiscoverDocs: %v", err)
		}
		return files
	}

	// auto -> since two distinct forges, expect forge prefix
	autoFiles := run(config.NamespacingAuto)
	for _, f := range autoFiles {
		if f.Forge == "" {
			t.Fatalf("expected forge set in auto mode with multi-forge")
		}
		if !strings.Contains(f.GetHugoPath(), f.Forge+string(filepath.Separator)+f.Repository) {
			t.Fatalf("hugo path missing forge segment: %s", f.GetHugoPath())
		}
	}

	// always -> same expectation
	alwaysFiles := run(config.NamespacingAlways)
	for _, f := range alwaysFiles {
		if f.Forge == "" {
			t.Fatalf("expected forge set in always mode")
		}
	}

	// never -> Forge field empty, path lacks forge segment
	neverFiles := run(config.NamespacingNever)
	for _, f := range neverFiles {
		if f.Forge != "" {
			t.Fatalf("expected no forge in never mode")
		}
		if strings.Contains(f.GetHugoPath(), "github") || strings.Contains(f.GetHugoPath(), "gitlab") {
			t.Fatalf("path should not contain forge in never mode: %s", f.GetHugoPath())
		}
	}
}

func TestForgeNamespacingAutoSingleForge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docbuilder-ns-single")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	mkRepo := func(name string) (config.Repository, string) {
		repoDir := filepath.Join(tempDir, name)
		docsDir := filepath.Join(repoDir, "docs")
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, "page.md"), []byte("# Page"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return config.Repository{Name: name, Paths: []string{"docs"}, Tags: map[string]string{"forge_type": "github"}}, repoDir
	}

	r1, p1 := mkRepo("service-a")
	r2, p2 := mkRepo("service-b")
	repos := []config.Repository{r1, r2}
	repoPaths := map[string]string{r1.Name: p1, r2.Name: p2}

	d := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingAuto})
	files, err := d.DiscoverDocs(repoPaths)
	if err != nil {
		t.Fatalf("DiscoverDocs: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected files discovered")
	}
	for _, f := range files {
		if f.Forge != "" {
			t.Fatalf("expected empty forge for single-forge auto mode, got %q", f.Forge)
		}
		if strings.Contains(f.GetHugoPath(), "github") {
			t.Fatalf("path should not contain forge segment: %s", f.GetHugoPath())
		}
	}
}
