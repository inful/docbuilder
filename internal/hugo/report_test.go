package hugo

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	testforge "git.home.luguber.info/inful/docbuilder/internal/testutil/testforge"
)

func TestGenerateSiteWithReport(t *testing.T) {
	outDir := t.TempDir()

	// Use TestForge to generate realistic repository data
	forge := testforge.NewTestForge("report-test", config.ForgeGitHub)
	repositories := forge.ToConfigRepositories()

	// Use the first repository for testing
	if len(repositories) == 0 {
		t.Fatal("TestForge should generate at least one repository")
	}
	testRepo := repositories[0]

	cfg := &config.Config{
		Hugo:         config.HugoConfig{Title: "TestForge Report"},
		Repositories: []config.Repository{testRepo},
	}

	files := []docs.DocFile{{
		Repository:   testRepo.Name,
		Name:         "test-page",
		RelativePath: "test-page.md",
		DocsBase:     "docs",
		Extension:    ".md",
		Content:      []byte("# TestForge Generated Content\n\nThis is realistic test documentation."),
	}}

	gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})
	rep, err := gen.GenerateSiteWithReport(files)
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	if rep.Repositories != 1 || rep.Files != 1 {
		t.Fatalf("unexpected counts: %+v", rep)
	}
	if rep.End.IsZero() {
		t.Fatalf("report end time not set")
	}
	if !strings.Contains(rep.Summary(), "repos=1") || rep.Outcome == "" {
		t.Fatalf("summary unexpected: %s", rep.Summary())
	}
	if rep.RenderedPages == 0 {
		t.Fatalf("expected rendered pages > 0, got %d", rep.RenderedPages)
	}

	// TestForge integration validation
	t.Logf("✓ TestForge integration: Generated report for repository %s (%s)", testRepo.Name, testRepo.URL)
	t.Logf("✓ Repository metadata: %v", testRepo.Tags)
}

// TestMultiPlatformHugoGeneration demonstrates TestForge integration across multiple forge platforms.
func TestMultiPlatformHugoGeneration(t *testing.T) {
	platforms := []struct {
		name      string
		forgeType config.ForgeType
		theme     string
	}{
		{"github", config.ForgeGitHub, "relearn"},
		{"gitlab", config.ForgeGitLab, "relearn"},
		{"forgejo", config.ForgeForgejo, "relearn"},
	}

	for _, platform := range platforms {
		t.Run(platform.name, func(t *testing.T) {
			outDir := t.TempDir()

			// Create platform-specific TestForge
			forge := testforge.NewTestForge(platform.name+"-test", platform.forgeType)
			repositories := forge.ToConfigRepositories()

			if len(repositories) == 0 {
				t.Fatalf("TestForge should generate repositories for %s", platform.name)
			}

			cfg := &config.Config{
				Hugo: config.HugoConfig{
					Title: platform.name + " Documentation",
				},
				Repositories: repositories,
			}

			// Create test files for all repositories
			var files []docs.DocFile
			for _, repo := range repositories {
				files = append(files, docs.DocFile{
					Repository:   repo.Name,
					Name:         "platform-test",
					RelativePath: "platform-test.md",
					DocsBase:     "docs",
					Extension:    ".md",
					Content:      []byte("# " + platform.name + " Platform Test\n\nGenerated with TestForge for " + repo.Name),
				})
			}

			gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})
			rep, err := gen.GenerateSiteWithReport(files)
			if err != nil {
				t.Fatalf("generation failed for %s: %v", platform.name, err)
			}

			// Validate report
			if rep.Repositories != len(repositories) {
				t.Errorf("Expected %d repositories, got %d", len(repositories), rep.Repositories)
			}
			if rep.Files != len(files) {
				t.Errorf("Expected %d files, got %d", len(files), rep.Files)
			}
			if rep.RenderedPages == 0 {
				t.Errorf("Expected rendered pages > 0 for %s", platform.name)
			}

			t.Logf("✓ %s integration: %d repos, %d files, %d pages rendered",
				platform.name, rep.Repositories, rep.Files, rep.RenderedPages)
		})
	}
}

// TestTestForgeFailureScenarios demonstrates failure scenario testing with TestForge.
func TestTestForgeFailureScenarios(t *testing.T) {
	outDir := t.TempDir()

	// Create TestForge with failure mode
	forge := testforge.NewTestForge("failure-test", config.ForgeGitHub)
	forge.SetFailMode(testforge.FailModeAuth) // Set authentication failure mode
	repositories := forge.ToConfigRepositories()

	if len(repositories) == 0 {
		t.Fatal("TestForge should generate repositories even in failure mode")
	}

	cfg := &config.Config{
		Hugo:         config.HugoConfig{Title: "Failure Test"},
		Repositories: repositories,
	}

	// Hugo should still generate successfully even with problematic repository data
	files := []docs.DocFile{{
		Repository:   repositories[0].Name,
		Name:         "failure-test",
		RelativePath: "failure-test.md",
		DocsBase:     "docs",
		Extension:    ".md",
		Content:      []byte("# Failure Scenario Test\n\nTesting resilience with TestForge failures."),
	}}

	gen := NewGenerator(cfg, outDir).WithRenderer(&NoopRenderer{})
	rep, err := gen.GenerateSiteWithReport(files)
	// Hugo generation should succeed despite forge failures
	if err != nil {
		t.Fatalf("Hugo generation should succeed despite forge failures: %v", err)
	}

	if rep.Repositories != 1 || rep.Files != 1 {
		t.Fatalf("Unexpected counts in failure scenario: %+v", rep)
	}

	t.Logf("✓ Failure scenario: Hugo successfully generated despite TestForge failure mode")
	t.Logf("✓ Report outcome: %s", rep.Outcome)
}
