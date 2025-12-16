package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/testforge"
)

func TestDocumentationDiscovery(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "docbuilder-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test repository structure
	repoDir := filepath.Join(tempDir, "test-repo")
	docsDir := filepath.Join(repoDir, "docs")

	// Create directories
	if err := os.MkdirAll(filepath.Join(docsDir, "api"), 0o750); err != nil {
		t.Fatalf("mkdir api: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(docsDir, "guides"), 0o750); err != nil {
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
		err := os.WriteFile(fullPath, []byte(content), 0o600)
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

	// Verify results (now including README.md)
	expectedFiles := []string{
		"index.md",
		"README.md",
		"api/overview.md",
		"api/reference.md",
		"guides/getting-started.md",
	}

	if len(docFiles) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(docFiles))
	}

	// Check that .txt files are ignored (README.md is now kept)
	for _, file := range docFiles {
		if file.Extension == ".txt" {
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
	// Note: README.md is still in the ignored list, but discovery specifically
	// excludes it from being ignored at root level (section == "") so it can
	// be used as the repository index.
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
	defer func() { _ = os.RemoveAll(tempDir) }()

	mkRepo := func(name, forgeType string) (config.Repository, string) {
		repoDir := filepath.Join(tempDir, name)
		docsDir := filepath.Join(repoDir, "docs")
		if err := os.MkdirAll(docsDir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, "page.md"), []byte("# Page"), 0o600); err != nil {
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
		// GetHugoPath lowercases components for URL compatibility
		expectedSegment := strings.ToLower(f.Forge) + string(filepath.Separator) + strings.ToLower(f.Repository)
		if !strings.Contains(f.GetHugoPath(), expectedSegment) {
			t.Fatalf("hugo path missing forge segment: %s (expected %s)", f.GetHugoPath(), expectedSegment)
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
	defer func() { _ = os.RemoveAll(tempDir) }()

	mkRepo := func(name string) (config.Repository, string) {
		repoDir := filepath.Join(tempDir, name)
		docsDir := filepath.Join(repoDir, "docs")
		if err := os.MkdirAll(docsDir, 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(docsDir, "page.md"), []byte("# Page"), 0o600); err != nil {
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

func TestDiscoveryWithTestForgeIntegration(t *testing.T) {
	t.Run("LargeScaleRepositoryDiscovery", func(t *testing.T) {
		// Test discovery performance with organization-scale repository volumes
		forge := testforge.NewTestForge("large-scale-discovery", config.ForgeGitHub)

		// Add multiple test repositories to simulate a medium-sized organization
		for i := 0; i < 50; i++ {
			repo := testforge.TestRepository{
				ID:            fmt.Sprintf("repo-%d", i+1),
				Name:          fmt.Sprintf("docs-repo-%d", i+1),
				FullName:      fmt.Sprintf("test-org/docs-repo-%d", i+1),
				CloneURL:      fmt.Sprintf("https://test-forge.example.com/test-org/docs-repo-%d.git", i+1),
				SSHURL:        fmt.Sprintf("git@test-forge.example.com:test-org/docs-repo-%d.git", i+1),
				DefaultBranch: "main",
				Description:   fmt.Sprintf("Documentation repository %d for testing", i+1),
				Topics:        []string{"docs", "testing", fmt.Sprintf("repo-%d", i+1)},
				Language:      "Markdown",
				Private:       false,
				Archived:      false,
				Fork:          false,
				HasDocs:       true,
				HasDocIgnore:  false,
			}
			forge.AddRepository(repo)
		}

		repositories := forge.ToConfigRepositories()

		// Create temporary directories for all repositories
		tempDir, err := os.MkdirTemp("", "large-scale-discovery-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		repoPaths := make(map[string]string)
		for _, repo := range repositories {
			repoDir := filepath.Join(tempDir, repo.Name)
			docsDir := filepath.Join(repoDir, "docs")

			if err := os.MkdirAll(docsDir, 0o750); err != nil {
				t.Fatalf("Failed to create docs dir for %s: %v", repo.Name, err)
			}

			// Create realistic documentation files
			docFiles := map[string]string{
				"docs/index.md":           "# " + repo.Name + " Documentation\n\nOverview of the project.",
				"docs/api/endpoints.md":   "# API Endpoints\n\nAPI documentation for " + repo.Name,
				"docs/guides/setup.md":    "# Setup Guide\n\nHow to set up " + repo.Name,
				"docs/guides/examples.md": "# Examples\n\nUsage examples for " + repo.Name,
			}

			for path, content := range docFiles {
				fullPath := filepath.Join(repoDir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
					t.Fatalf("Failed to create dir for %s: %v", fullPath, err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
					t.Fatalf("Failed to write file %s: %v", fullPath, err)
				}
			}

			repoPaths[repo.Name] = repoDir
		}

		// Create discovery instance with realistic configuration
		discovery := NewDiscovery(repositories, &config.BuildConfig{
			NamespaceForges: config.NamespacingAuto,
		})

		// Perform discovery
		docFiles, err := discovery.DiscoverDocs(repoPaths)
		if err != nil {
			t.Fatalf("DiscoverDocs failed: %v", err)
		}

		// Validate results
		expectedFilesPerRepo := 4 // index.md, endpoints.md, setup.md, examples.md
		expectedTotalFiles := len(repositories) * expectedFilesPerRepo

		if len(docFiles) != expectedTotalFiles {
			t.Errorf("Expected %d total files, got %d (repos: %d, files per repo: %d)",
				expectedTotalFiles, len(docFiles), len(repositories), expectedFilesPerRepo)
		}

		// Verify file grouping by repository
		filesByRepo := discovery.GetDocFilesByRepository()
		if len(filesByRepo) != len(repositories) {
			t.Errorf("Expected %d repositories with docs, got %d", len(repositories), len(filesByRepo))
		}

		// Verify each repository has the expected number of files
		for _, repo := range repositories {
			repoFiles := filesByRepo[repo.Name]
			if len(repoFiles) != expectedFilesPerRepo {
				t.Errorf("Repository %s: expected %d files, got %d",
					repo.Name, expectedFilesPerRepo, len(repoFiles))
			}
		}

		t.Logf("✓ Large-scale discovery validated: %d repositories, %d total files",
			len(repositories), len(docFiles))
	})

	t.Run("MultiPlatformDiscoveryValidation", func(t *testing.T) {
		// Test discovery across different forge platforms
		factory := testforge.NewFactory()

		githubForge := factory.CreateGitHubTestForge("discovery-github")
		gitlabForge := factory.CreateGitLabTestForge("discovery-gitlab")
		forgejoForge := factory.CreateForgejoTestForge("discovery-forgejo")

		// Add additional repositories to each forge for more comprehensive testing
		githubForge.AddRepository(testforge.TestRepository{
			Name:        "github-actions-docs",
			FullName:    "github-org/github-actions-docs",
			CloneURL:    "https://github.com/github-org/github-actions-docs.git",
			Description: "GitHub Actions documentation",
			Topics:      []string{"github-actions", "ci-cd"},
			Language:    "Markdown",
			Private:     false,
			Archived:    false,
			Fork:        false,
		})

		gitlabForge.AddRepository(testforge.TestRepository{
			Name:        "gitlab-ci-docs",
			FullName:    "gitlab-group/gitlab-ci-docs",
			CloneURL:    "https://gitlab.example.com/gitlab-group/gitlab-ci-docs.git",
			Description: "GitLab CI documentation",
			Topics:      []string{"gitlab-ci", "pipelines"},
			Language:    "Markdown",
			Private:     false,
			Archived:    false,
			Fork:        false,
		})

		forgejoForge.AddRepository(testforge.TestRepository{
			Name:        "forgejo-setup-docs",
			FullName:    "forgejo-org/forgejo-setup-docs",
			CloneURL:    "https://forgejo.example.com/forgejo-org/forgejo-setup-docs.git",
			Description: "Forgejo setup documentation",
			Topics:      []string{"setup", "configuration"},
			Language:    "Markdown",
			Private:     false,
			Archived:    false,
			Fork:        false,
		})

		// Combine all repositories
		allRepositories := append(
			append(githubForge.ToConfigRepositories(), gitlabForge.ToConfigRepositories()...),
			forgejoForge.ToConfigRepositories()...,
		)

		// Add forge type tags for proper namespacing
		for i, repo := range allRepositories {
			if repo.Tags == nil {
				allRepositories[i].Tags = make(map[string]string)
			}

			if strings.Contains(repo.Name, "github") {
				allRepositories[i].Tags["forge_type"] = "github"
			} else if strings.Contains(repo.Name, "gitlab") {
				allRepositories[i].Tags["forge_type"] = "gitlab"
			} else if strings.Contains(repo.Name, "forgejo") {
				allRepositories[i].Tags["forge_type"] = "forgejo"
			}
		}

		// Create temporary directories
		tempDir, err := os.MkdirTemp("", "multi-platform-discovery-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		repoPaths := make(map[string]string)
		for _, repo := range allRepositories {
			repoDir := filepath.Join(tempDir, repo.Name)
			docsDir := filepath.Join(repoDir, "docs")

			if err := os.MkdirAll(docsDir, 0o750); err != nil {
				t.Fatalf("Failed to create docs dir for %s: %v", repo.Name, err)
			}

			// Create platform-specific documentation patterns
			var docFiles map[string]string
			forgeType := repo.Tags["forge_type"]

			switch forgeType {
			case "github":
				docFiles = map[string]string{
					"docs/README.md":          "# " + repo.Name + " (GitHub)\n\nShould be ignored",
					"docs/index.md":           "# " + repo.Name + " Documentation\n\nGitHub repository docs.",
					"docs/actions/ci.md":      "# GitHub Actions\n\nCI/CD with GitHub Actions.",
					"docs/security/policy.md": "# Security Policy\n\nSecurity guidelines.",
				}
			case "gitlab":
				docFiles = map[string]string{
					"docs/index.md":          "# " + repo.Name + " Documentation\n\nGitLab repository docs.",
					"docs/cicd/pipelines.md": "# GitLab Pipelines\n\nCI/CD with GitLab.",
					"docs/merge/requests.md": "# Merge Requests\n\nMR workflow guide.",
				}
			case "forgejo":
				docFiles = map[string]string{
					"docs/index.md":        "# " + repo.Name + " Documentation\n\nForgejo repository docs.",
					"docs/git/workflow.md": "# Git Workflow\n\nForgejo git workflow.",
					"docs/self-hosted.md":  "# Self-Hosted Setup\n\nSelf-hosting guide.",
				}
			default:
				// Fallback for repositories without forge_type tag
				docFiles = map[string]string{
					"docs/index.md": "# " + repo.Name + " Documentation\n\nGeneric documentation.",
					"docs/guide.md": "# Guide\n\nGeneral guide.",
					"docs/api.md":   "# API\n\nAPI documentation.",
				}
			}

			for path, content := range docFiles {
				fullPath := filepath.Join(repoDir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
					t.Fatalf("Failed to create dir for %s: %v", fullPath, err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
					t.Fatalf("Failed to write file %s: %v", fullPath, err)
				}
			}

			repoPaths[repo.Name] = repoDir
		}

		// Test with auto-namespacing (should namespace due to multiple forge types)
		discovery := NewDiscovery(allRepositories, &config.BuildConfig{
			NamespaceForges: config.NamespacingAuto,
		})

		docFiles, err := discovery.DiscoverDocs(repoPaths)
		if err != nil {
			t.Fatalf("DiscoverDocs failed: %v", err)
		}

		// Verify forge namespacing
		forgeTypes := make(map[string]int)
		for _, file := range docFiles {
			if file.Forge != "" {
				forgeTypes[file.Forge]++
			}
		}

		// Should have files from all forge types
		if len(forgeTypes) == 0 {
			t.Error("Expected forge namespacing with multiple forge types")
		}

		// Verify file grouping
		filesByRepo := discovery.GetDocFilesByRepository()

		// Count files by forge type
		githubRepoCount := 0
		gitlabRepoCount := 0
		forgejoRepoCount := 0

		for repoName, files := range filesByRepo {
			if strings.Contains(repoName, "github") {
				githubRepoCount++
				// GitHub repos should have 4 files (README.md now included)
				if len(files) != 4 {
					t.Errorf("GitHub repo %s: expected 4 files, got %d", repoName, len(files))
				}
			} else if strings.Contains(repoName, "gitlab") {
				gitlabRepoCount++
				// GitLab repos should have 3 files (no README.md in test data)
				if len(files) != 3 {
					t.Errorf("GitLab repo %s: expected 3 files, got %d", repoName, len(files))
				}
			} else if strings.Contains(repoName, "forgejo") {
				forgejoRepoCount++
				// Forgejo repos should have 3 files (no README.md in test data)
				if len(files) != 3 {
					t.Errorf("Forgejo repo %s: expected 3 files, got %d", repoName, len(files))
				}
			}
		}

		t.Logf("✓ Multi-platform discovery validated: GitHub(%d repos), GitLab(%d repos), Forgejo(%d repos)",
			githubRepoCount, gitlabRepoCount, forgejoRepoCount)
	})

	t.Run("DocumentationFilteringWithTestForge", func(t *testing.T) {
		// Test advanced filtering logic with realistic repositories
		forge := testforge.NewTestForge("filtering-test", config.ForgeGitHub)

		// Add multiple repositories for comprehensive filtering testing
		for i := 0; i < 5; i++ {
			repo := testforge.TestRepository{
				ID:            fmt.Sprintf("filter-repo-%d", i+1),
				Name:          fmt.Sprintf("filter-repo-%d", i+1),
				FullName:      fmt.Sprintf("test-org/filter-repo-%d", i+1),
				CloneURL:      fmt.Sprintf("https://test-forge.example.com/test-org/filter-repo-%d.git", i+1),
				SSHURL:        fmt.Sprintf("git@test-forge.example.com:test-org/filter-repo-%d.git", i+1),
				DefaultBranch: "main",
				Description:   fmt.Sprintf("Repository %d for filtering tests", i+1),
				Topics:        []string{"filtering", "testing"},
				Language:      "Markdown",
				Private:       false,
				Archived:      false,
				Fork:          false,
				HasDocs:       true,
				HasDocIgnore:  false,
			}
			forge.AddRepository(repo)
		}

		repositories := forge.ToConfigRepositories()

		// Create comprehensive test scenarios with various file types
		tempDir, err := os.MkdirTemp("", "filtering-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		repoPaths := make(map[string]string)
		for _, repo := range repositories {
			repoDir := filepath.Join(tempDir, repo.Name)
			docsDir := filepath.Join(repoDir, "docs")

			if err := os.MkdirAll(docsDir, 0o750); err != nil {
				t.Fatalf("Failed to create docs dir: %v", err)
			}

			// Create a mix of files to test filtering
			testFiles := map[string]string{
				// Valid markdown files
				"docs/index.md":             "# Index",
				"docs/guide.markdown":       "# Guide",
				"docs/api.mdown":            "# API",
				"docs/reference.mkd":        "# Reference",
				"docs/tutorial/basics.md":   "# Basics",
				"docs/tutorial/advanced.md": "# Advanced",

				// Files that should be ignored (except README.md which is used as repository index)
				"docs/README.md":       "# README - used as repository index",
				"docs/CONTRIBUTING.md": "# Contributing - should be ignored",
				"docs/CHANGELOG.md":    "# Changelog - should be ignored",
				"docs/LICENSE.md":      "# License - should be ignored",

				// Non-markdown files that should be ignored
				"docs/config.txt": "Configuration file",
				"docs/image.png":  "Binary image data",
				"docs/style.css":  "CSS styles",
				"docs/script.js":  "JavaScript code",
				"docs/data.json":  "JSON data",

				// Files without extensions
				"docs/INSTALL": "Installation instructions",
				"docs/NOTES":   "Project notes",
			}

			for path, content := range testFiles {
				fullPath := filepath.Join(repoDir, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
					t.Fatalf("Failed to create dir: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
			}

			repoPaths[repo.Name] = repoDir
		}

		// Perform discovery
		discovery := NewDiscovery(repositories, &config.BuildConfig{
			NamespaceForges: config.NamespacingNever,
		})

		docFiles, err := discovery.DiscoverDocs(repoPaths)
		if err != nil {
			t.Fatalf("DiscoverDocs failed: %v", err)
		}

		// Verify filtering results
		expectedMarkdownFiles := 7 // index.md, README.md, guide.markdown, api.mdown, reference.mkd, basics.md, advanced.md
		expectedAssetFiles := 2    // data.json, image.png
		expectedTotalFiles := len(repositories) * (expectedMarkdownFiles + expectedAssetFiles)

		if len(docFiles) != expectedTotalFiles {
			t.Errorf("Expected %d total files, got %d", expectedTotalFiles, len(docFiles))
		}

		// Count markdown and asset files
		markdownCount := 0
		assetCount := 0
		for _, file := range docFiles {
			if file.IsAsset {
				assetCount++
			} else {
				markdownCount++
			}
		}

		expectedMarkdownTotal := len(repositories) * expectedMarkdownFiles
		expectedAssetTotal := len(repositories) * expectedAssetFiles
		if markdownCount != expectedMarkdownTotal {
			t.Errorf("Expected %d markdown files, got %d", expectedMarkdownTotal, markdownCount)
		}
		if assetCount != expectedAssetTotal {
			t.Errorf("Expected %d asset files, got %d", expectedAssetTotal, assetCount)
		}

		// Verify no ignored markdown files are included
		for _, file := range docFiles {
			if file.IsAsset {
				continue // Skip validation for asset files
			}

			filename := filepath.Base(file.RelativePath)

			// Check for ignored markdown files (except README.md which is now kept at root level)
			if isIgnoredFile(filename) && !strings.EqualFold(filename, "README.md") {
				t.Errorf("Ignored file should not be included: %s", file.RelativePath)
			}

			// Verify markdown file extensions
			validExtensions := []string{".md", ".markdown", ".mdown", ".mkd"}
			hasValidExtension := false
			for _, ext := range validExtensions {
				if strings.HasSuffix(strings.ToLower(filename), ext) {
					hasValidExtension = true
					break
				}
			}
			if !hasValidExtension {
				t.Errorf("Markdown file with invalid extension: %s", file.RelativePath)
			}
		}

		// Verify directory structure preservation
		filesByRepo := discovery.GetDocFilesByRepository()
		for repoName, files := range filesByRepo {
			hasRootFiles := false
			hasTutorialFiles := false

			for _, file := range files {
				if !strings.Contains(file.RelativePath, "/") {
					hasRootFiles = true
				}
				if strings.Contains(file.RelativePath, "tutorial/") {
					hasTutorialFiles = true
				}
			}

			if !hasRootFiles {
				t.Errorf("Repository %s should have root-level files", repoName)
			}
			if !hasTutorialFiles {
				t.Errorf("Repository %s should have tutorial/ subdirectory files", repoName)
			}
		}

		t.Logf("✓ Documentation filtering validated: %d repositories, %d filtered files",
			len(repositories), len(docFiles))
	})

	t.Run("PerformanceValidationWithLargeDataset", func(t *testing.T) {
		// Test discovery performance with larger datasets
		forge := testforge.NewTestForge("performance-test", config.ForgeGitHub)

		// Add 100 test repositories for large organization simulation
		for i := 0; i < 100; i++ {
			repo := testforge.TestRepository{
				ID:            fmt.Sprintf("perf-repo-%d", i+1),
				Name:          fmt.Sprintf("perf-repo-%d", i+1),
				FullName:      fmt.Sprintf("test-org/perf-repo-%d", i+1),
				CloneURL:      fmt.Sprintf("https://test-forge.example.com/test-org/perf-repo-%d.git", i+1),
				SSHURL:        fmt.Sprintf("git@test-forge.example.com:test-org/perf-repo-%d.git", i+1),
				DefaultBranch: "main",
				Description:   fmt.Sprintf("Performance test repository %d", i+1),
				Topics:        []string{"performance", "testing"},
				Language:      "Markdown",
				Private:       false,
				Archived:      false,
				Fork:          false,
				HasDocs:       true,
				HasDocIgnore:  false,
			}
			forge.AddRepository(repo)
		}

		repositories := forge.ToConfigRepositories()

		// Create minimal file structure for performance testing
		tempDir, err := os.MkdirTemp("", "performance-test")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		repoPaths := make(map[string]string)
		for _, repo := range repositories {
			repoDir := filepath.Join(tempDir, repo.Name)
			docsDir := filepath.Join(repoDir, "docs")

			if err := os.MkdirAll(docsDir, 0o750); err != nil {
				t.Fatalf("Failed to create docs dir: %v", err)
			}

			// Create minimal documentation for performance testing
			docFiles := map[string]string{
				"docs/index.md": "# " + repo.Name,
				"docs/api.md":   "# API",
				"docs/guide.md": "# Guide",
			}

			for path, content := range docFiles {
				fullPath := filepath.Join(repoDir, path)
				if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
					t.Fatalf("Failed to write file: %v", err)
				}
			}

			repoPaths[repo.Name] = repoDir
		}

		// Perform discovery and measure basic metrics
		discovery := NewDiscovery(repositories, &config.BuildConfig{
			NamespaceForges: config.NamespacingNever,
		})

		docFiles, err := discovery.DiscoverDocs(repoPaths)
		if err != nil {
			t.Fatalf("DiscoverDocs failed: %v", err)
		}

		// Validate results
		expectedFiles := len(repositories) * 3 // 3 files per repository
		if len(docFiles) != expectedFiles {
			t.Errorf("Expected %d files for performance test, got %d", expectedFiles, len(docFiles))
		}

		// Verify repository distribution
		filesByRepo := discovery.GetDocFilesByRepository()
		if len(filesByRepo) != len(repositories) {
			t.Errorf("Expected %d repositories, got %d", len(repositories), len(filesByRepo))
		}

		t.Logf("✓ Performance validation: %d repositories, %d files processed",
			len(repositories), len(docFiles))
	})
}

// TestPathCollisionDetection verifies that case-insensitive path collisions are detected.
func TestPathCollisionDetection(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "docbuilder-collision-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create test repository with files that will collide when lowercased
	repoDir := filepath.Join(tempDir, "collision-repo")
	docsDir := filepath.Join(repoDir, "docs")

	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	// Create files with different casing that will collide
	testFiles := map[string]string{
		"docs/Minutes.md": "# Meeting Minutes\n\nMonthly meeting notes.",
		"docs/minutes.md": "# Time Units\n\nTime measurement in minutes.",
		"docs/Guide.md":   "# User Guide\n\nHow to use this system.",
		"docs/guide.md":   "# Different Guide\n\nAnother guide.",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(repoDir, path)
		err := os.WriteFile(fullPath, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Create repository configuration
	repos := []config.Repository{
		{
			Name:  "collision-repo",
			Paths: []string{"docs"},
			Tags:  map[string]string{},
		},
	}

	// Create discovery instance
	discovery := NewDiscovery(repos, &config.BuildConfig{NamespaceForges: config.NamespacingNever})

	// Test discovery - should fail with collision error
	repoPaths := map[string]string{
		"collision-repo": repoDir,
	}

	docFiles, err := discovery.DiscoverDocs(repoPaths)

	// We expect an error due to path collisions
	if err == nil {
		t.Fatal("Expected error due to path collisions, but got nil")
	}

	// Verify the error message mentions collisions
	if !strings.Contains(err.Error(), "path collision") {
		t.Errorf("Error message should mention 'path collision', got: %v", err)
	}

	// Verify we still got the files discovered (error happens after discovery)
	if len(docFiles) != 4 {
		t.Errorf("Expected 4 files to be discovered before collision check, got %d", len(docFiles))
	}

	t.Logf("✓ Path collision detection working: %v", err)
}
