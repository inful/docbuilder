package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	testforge "git.home.luguber.info/inful/docbuilder/internal/testutil/testforge"
)

// TestGenerateSite_Smoke validates that a minimal set of doc files are processed
// and written with front matter and link rewriting applied. It does not invoke the external hugo binary.
func TestGenerateSite_Smoke(t *testing.T) {
	// Arrange temporary output dir
	outDir := t.TempDir()

	// Use TestForge to generate realistic repository configuration
	forge := testforge.NewTestForge("smoke-test", config.ForgeGitHub)
	repositories := forge.ToConfigRepositories()

	if len(repositories) == 0 {
		t.Fatal("TestForge should generate at least one repository")
	}

	testRepo := repositories[0]
	cfg := &config.Config{
		Repositories: []config.Repository{testRepo},
	}

	// Fake discovered docs (normally discovered by Discovery)
	files := []docs.DocFile{
		{Repository: testRepo.Name, Name: "intro", RelativePath: "intro.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("# Intro\n\nSee [Guide](guide.md).")},
		{Repository: testRepo.Name, Name: "guide", RelativePath: "guide.md", DocsBase: "docs", Section: "", Extension: ".md", Content: []byte("# Guide\n")},
	}

	gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})

	// Act
	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("GenerateSite failed: %v", err)
	}

	// Assert expected content files exist
	// Single-repo build: paths should not include repository namespace (ADR-006)
	introPath := filepath.Join(outDir, "content", "intro.md")
	guidePath := filepath.Join(outDir, "content", "guide.md")
	if _, err := os.Stat(introPath); err != nil {
		t.Fatalf("intro file missing: %v", err)
	}
	if _, err := os.Stat(guidePath); err != nil {
		t.Fatalf("guide file missing: %v", err)
	}

	// #nosec G304 -- test utility reading from test output directory
	b, err := os.ReadFile(introPath)
	if err != nil {
		t.Fatalf("read intro: %v", err)
	}
	content := string(b)

	// Front matter delimiter
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("front matter missing at start: %s", content[:30])
	}
	if !strings.Contains(content, "repository: "+testRepo.Name) {
		t.Fatalf("repository not in front matter: %s", content)
	}
	if !strings.Contains(content, "editURL:") {
		t.Fatalf("expected editURL in front matter for relearn theme")
	}
	if strings.Contains(content, "guide.md") {
		t.Fatalf("link rewriting failed to strip .md: %s", content)
	}

	// Ensure date field present (format not strictly validated here for simplicity)
	if !strings.Contains(content, "date:") {
		t.Fatalf("date missing in front matter")
	}

	// Sanity: guide file also has front matter
	// #nosec G304 -- test utility reading from test output directory
	gb, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read guide: %v", err)
	}
	if !strings.HasPrefix(string(gb), "---\n") {
		t.Fatalf("guide front matter missing")
	}

	// TestForge integration validation
	t.Logf("✓ TestForge integration: Successfully generated site for repository %s", testRepo.Name)
	t.Logf("✓ Repository URL: %s", testRepo.URL)
	t.Logf("✓ Content files generated in: content/ (single-repo build, no namespace)")
}

// TestGenerateSite_TestForgeRealisticWorkflow demonstrates end-to-end Hugo generation with TestForge.
func TestGenerateSite_TestForgeRealisticWorkflow(t *testing.T) {
	outDir := t.TempDir()

	// Create a multi-repository TestForge scenario
	forge := testforge.NewTestForge("realistic-workflow", config.ForgeGitHub)
	repositories := forge.ToConfigRepositories()

	if len(repositories) < 2 {
		t.Fatal("TestForge should generate multiple repositories for realistic workflow testing")
	}

	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:   "TestForge Multi-Repository Documentation",
			BaseURL: "https://testforge-docs.example.com",
		},
		Repositories: repositories,
	}

	// Create realistic documentation files for multiple repositories
	files := make([]docs.DocFile, 0, 3*len(repositories))
	for i, repo := range repositories {
		// Create different types of documentation for each repository
		files = append(files,
			docs.DocFile{
				Repository:   repo.Name,
				Name:         "README",
				RelativePath: "README.md",
				DocsBase:     "docs",
				Section:      "",
				Extension:    ".md",
				Content:      []byte("# " + repo.Name + "\n\nThis is the main documentation for " + repo.Name + "."),
			},
			docs.DocFile{
				Repository:   repo.Name,
				Name:         "getting-started",
				RelativePath: "guides/getting-started.md",
				DocsBase:     "docs",
				Section:      "guides",
				Extension:    ".md",
				Content:      []byte("# Getting Started with " + repo.Name + "\n\nQuick start guide."),
			},
			docs.DocFile{
				Repository:   repo.Name,
				Name:         "api-reference",
				RelativePath: "api/reference.md",
				DocsBase:     "docs",
				Section:      "api",
				Extension:    ".md",
				Content:      []byte("# API Reference\n\nAPI documentation for " + repo.Name + "."),
			},
		)

		// Limit to first 2 repositories to keep test manageable
		if i >= 1 {
			break
		}
	}

	gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})

	// Act - Generate the complete site
	if err := gen.GenerateSite(files); err != nil {
		t.Fatalf("GenerateSite failed with TestForge data: %v", err)
	}

	// Assert - Validate the generated structure
	for i, repo := range repositories {
		if i >= 2 { // Only check first 2 repos
			break
		}

		// Check repository directory exists
		repoDir := filepath.Join(outDir, "content", repo.Name)
		if _, err := os.Stat(repoDir); err != nil {
			t.Fatalf("Repository directory missing for %s: %v", repo.Name, err)
		}

		// Check section directories and files
		sections := []string{"guides", "api"}
		for _, section := range sections {
			sectionDir := filepath.Join(repoDir, section)
			if _, err := os.Stat(sectionDir); err != nil {
				t.Fatalf("Section directory missing for %s/%s: %v", repo.Name, section, err)
			}
		}

		// Validate a sample file has proper front matter
		// When README.md is the only index file, it becomes _index.md
		indexPath := filepath.Join(repoDir, "_index.md")
		// #nosec G304 -- test utility reading from test output directory
		if content, err := os.ReadFile(indexPath); err != nil {
			t.Fatalf("Failed to read index for %s: %v", repo.Name, err)
		} else {
			contentStr := string(content)
			if !strings.Contains(contentStr, "repository: "+repo.Name) {
				t.Errorf("Front matter missing repository field for %s", repo.Name)
			}
			if !strings.HasPrefix(contentStr, "---\n") {
				t.Errorf("Front matter delimiter missing for %s", repo.Name)
			}
		}
	}

	// Check Hugo configuration was generated
	hugoConfigPath := filepath.Join(outDir, "hugo.yaml")
	if _, err := os.Stat(hugoConfigPath); err != nil {
		t.Fatalf("Hugo configuration file missing: %v", err)
	}

	// Validate main index was created
	indexPath := filepath.Join(outDir, "content", "_index.md")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("Main index file missing: %v", err)
	}

	t.Logf("✓ TestForge realistic workflow: Generated site with %d repositories", len(repositories))
	t.Logf("✓ Processed %d documentation files across multiple sections", len(files))
	t.Logf("✓ Site structure validated for repositories: %v", []string{repositories[0].Name, repositories[1].Name})

	// Log repository metadata from TestForge
	for i, repo := range repositories {
		if i >= 2 {
			break
		}
		t.Logf("✓ Repository %s: URL=%s, Tags=%v", repo.Name, repo.URL, repo.Tags)
	}
}

// Removed BuildFrontMatter legacy path; date injection now verified indirectly via V2 builder tests.
