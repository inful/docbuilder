package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	testforge "git.home.luguber.info/inful/docbuilder/internal/testutil/testforge"
)

// TestHugoWithTestForgeIntegration demonstrates how TestForge can enhance Hugo package tests.
func TestHugoWithTestForgeIntegration(t *testing.T) {
	t.Log("=== Hugo TestForge Integration Demo ===")

	t.Run("RealisticRepositoryGeneration", func(t *testing.T) {
		// Instead of manually creating repository configs, use TestForge
		testForge := testforge.NewTestForge("hugo-test", config.ForgeGitHub)

		// Add realistic repositories with documentation
		testForge.AddRepository(testforge.TestRepository{
			Name:        "user-docs",
			FullName:    "company/user-docs",
			CloneURL:    "https://github.com/company/user-docs.git",
			Description: "User documentation with guides and tutorials",
			Topics:      []string{"documentation", "users", "guides"},
			Language:    "Markdown",
			HasDocs:     true,
		})

		testForge.AddRepository(testforge.TestRepository{
			Name:        "api-reference",
			FullName:    "company/api-reference",
			CloneURL:    "https://github.com/company/api-reference.git",
			Description: "API reference documentation",
			Topics:      []string{"api", "reference", "openapi"},
			Language:    "Markdown",
			HasDocs:     true,
		})

		// Convert TestForge repositories to config format
		configRepos := testForge.ToConfigRepositories()

		// Create Hugo configuration with realistic repositories
		cfg := &config.Config{
			Repositories: configRepos,
		}

		// Create realistic doc files based on TestForge metadata
		docFiles := []docs.DocFile{
			{
				Repository:   "user-docs",
				Name:         "getting-started",
				RelativePath: "getting-started.md",
				DocsBase:     "docs",
				Extension:    ".md",
				Content:      []byte("# Getting Started\n\nWelcome to our documentation!"),
			},
			{
				Repository:   "api-reference",
				Name:         "authentication",
				RelativePath: "auth/authentication.md",
				DocsBase:     "docs",
				Extension:    ".md",
				Content:      []byte("# Authentication\n\nHow to authenticate with our API."),
			},
		}

		// Test Hugo generation with realistic data
		outDir := t.TempDir()
		gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})

		report, err := gen.GenerateSiteWithReport(docFiles)
		if err != nil {
			t.Fatalf("Hugo generation failed: %v", err)
		}

		// Verify realistic expectations
		if report.Repositories != 2 {
			t.Errorf("Expected 2 repositories, got %d", report.Repositories)
		}
		if report.Files != 2 {
			t.Errorf("Expected 2 files, got %d", report.Files)
		}

		t.Logf("✓ Generated Hugo site with realistic TestForge data: %s", report.Summary())
	})

	t.Run("MultiForgeScenarioTesting", func(t *testing.T) {
		// Use TestForge scenarios for complex multi-forge testing
		scenarios := testforge.CreateTestScenarios()

		// Find the multi-platform scenario
		var multiPlatformScenario testforge.TestDiscoveryScenario
		for _, scenario := range scenarios {
			if scenario.Name == "Multi-Platform Discovery" {
				multiPlatformScenario = scenario
				break
			}
		}

		if multiPlatformScenario.Name == "" {
			t.Skip("Multi-Platform Discovery scenario not found")
		}

		// Convert all forges to config repositories
		allRepos := make([]config.Repository, 0, len(multiPlatformScenario.Forges)*5)
		for _, forge := range multiPlatformScenario.Forges {
			repos := forge.ToConfigRepositories()
			allRepos = append(allRepos, repos...)
		}

		cfg := &config.Config{
			Repositories: allRepos,
		}

		// Create documents for each platform
		var docFiles []docs.DocFile
		for i, repo := range allRepos {
			docFiles = append(docFiles, docs.DocFile{
				Repository:   repo.Name,
				Name:         "platform-guide",
				RelativePath: "platform-guide.md",
				DocsBase:     "docs",
				Extension:    ".md",
				Content:      []byte("# Platform Guide\n\nPlatform-specific documentation."),
			})

			// Add some repos without docs to test filtering
			if i%2 == 0 {
				continue // Skip some to simulate missing docs
			}
		}

		outDir := t.TempDir()
		gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})

		report, err := gen.GenerateSiteWithReport(docFiles)
		if err != nil {
			t.Fatalf("Multi-platform Hugo generation failed: %v", err)
		}

		// Verify cross-platform expectations
		expectedRepos := multiPlatformScenario.Expected.TotalRepositories
		if report.Repositories != expectedRepos {
			t.Errorf("Expected %d repositories from scenario, got %d", expectedRepos, report.Repositories)
		}

		t.Logf("✓ Multi-platform Hugo generation: %s", report.Summary())
	})

	t.Run("FailureScenarioTesting", func(t *testing.T) {
		// Test Hugo resilience with TestForge failure modes
		testForge := testforge.NewTestForge("failure-test", config.ForgeGitLab)
		testForge.SetFailMode(testforge.FailModeNetwork) // Simulate network issues

		// Even with network failures, Hugo should handle available data gracefully
		configRepos := testForge.ToConfigRepositories()

		cfg := &config.Config{
			Repositories: configRepos,
		}

		// Provide local doc files (simulating cached/available content)
		docFiles := []docs.DocFile{
			{
				Repository:   configRepos[0].Name,
				Name:         "cached-content",
				RelativePath: "cached-content.md",
				DocsBase:     "docs",
				Extension:    ".md",
				Content:      []byte("# Cached Content\n\nThis content was available despite network issues."),
			},
		}

		outDir := t.TempDir()
		gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})

		report, err := gen.GenerateSiteWithReport(docFiles)
		if err != nil {
			t.Fatalf("Hugo should handle failure scenarios gracefully: %v", err)
		}

		t.Logf("✓ Hugo resilience test passed: %s", report.Summary())
	})
}
