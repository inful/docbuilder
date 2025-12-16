package hugo

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/testforge"
)

func fixedTime() time.Time { return time.Date(2025, 9, 26, 12, 34, 56, 0, time.UTC) }

func TestBuildFrontMatter_TitleAndBasicFields(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/project.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "repo1", Name: "getting-started", Section: "guide"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Existing: nil, Config: cfg, Now: fixedTime()})

	if fm["title"] != "Getting Started" {
		t.Fatalf("expected title 'Getting Started', got %v", fm["title"])
	}
	if fm["repository"] != "repo1" {
		t.Fatalf("repository not set correctly: %v", fm["repository"])
	}
	if fm["section"] != "guide" {
		t.Fatalf("section not set: %v", fm["section"])
	}
	if fm["date"] == nil {
		t.Fatalf("date should be set")
	}
}

func TestBuildFrontMatter_IndexNoTitle(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1"}}}
	file := docs.DocFile{Repository: "repo1", Name: "index"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if _, exists := fm["title"]; exists {
		t.Fatalf("index file should not auto-generate title, got %v", fm["title"])
	}
}

func TestBuildFrontMatter_MetadataPassthrough(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1"}}}
	file := docs.DocFile{Repository: "repo1", Name: "ref", Metadata: map[string]string{"product": "alpha"}}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if fm["product"] != "alpha" {
		t.Fatalf("metadata not passed through: %v", fm["product"])
	}
}

func TestBuildFrontMatter_EditURL_GitHub(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/project.git", Branch: "develop"}}}
	file := docs.DocFile{Repository: "repo1", Name: "intro", RelativePath: "intro.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://github.com/org/project/edit/develop/docs/intro.md"
	if fm["editURL"] != want {
		t.Fatalf("expected editURL %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_GitLabSSH(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "r", URL: "git@gitlab.com:group/proj.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "r", Name: "guide", RelativePath: "dir/guide.md", DocsBase: "documentation"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://gitlab.com/group/proj/-/edit/main/documentation/dir/guide.md"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_Bitbucket(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "bb", URL: "https://bitbucket.org/team/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "bb", Name: "page", RelativePath: "page.md", DocsBase: "."}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://bitbucket.org/team/repo/src/main/page.md?mode=edit"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_Gitea(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "gt", URL: "https://git.home.luguber.info/org/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "gt", Name: "usage", RelativePath: "nested/usage.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://git.home.luguber.info/org/repo/_edit/main/docs/nested/usage.md"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_SiteBaseSuppressesPerPage(t *testing.T) {
	params := map[string]any{"editURL": map[string]any{"base": "https://example.com/edit"}}
	cfg := &config.Config{Hugo: config.HugoConfig{Params: params}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "repo1", Name: "conf", RelativePath: "conf.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if _, exists := fm["editURL"]; exists {
		t.Fatalf("per-page editURL should be suppressed when site base provided, got %v", fm["editURL"])
	}
}

func TestBuildFrontMatter_ExistingPreserved(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo1"}}}
	existing := map[string]any{"title": "Custom", "editURL": "https://override"}
	file := docs.DocFile{Repository: "repo1", Name: "custom"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Existing: existing, Config: cfg, Now: fixedTime()})
	if fm["title"] != "Custom" {
		t.Fatalf("existing title should be preserved, got %v", fm["title"])
	}
	if fm["editURL"] != "https://override" {
		t.Fatalf("existing editURL should be preserved, got %v", fm["editURL"])
	}
}

func TestBuildFrontMatter_IncludesForge(t *testing.T) {
	cfg := &config.Config{Repositories: []config.Repository{{Name: "repo"}}}
	file := docs.DocFile{Repository: "repo", Name: "guide", Forge: "github"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if got, ok := fm["forge"]; !ok || got != "github" {
		t.Fatalf("expected forge field 'github', got %v (present=%v)", got, ok)
	}
}

// TestTestForgeFrontmatterIntegration demonstrates TestForge integration for frontmatter generation
func TestTestForgeFrontmatterIntegration(t *testing.T) {
	// Test frontmatter generation with TestForge-generated repositories across platforms
	platforms := []struct {
		name      string
		forgeType config.ForgeType
	}{
		{"github", config.ForgeGitHub},
		{"gitlab", config.ForgeGitLab},
		{"forgejo", config.ForgeForgejo},
	}

	for _, platform := range platforms {
		t.Run(platform.name+"_frontmatter", func(t *testing.T) {
			// Create TestForge for the platform
			forge := testforge.NewTestForge(platform.name+"-fm-test", platform.forgeType)
			repositories := forge.ToConfigRepositories()

			if len(repositories) == 0 {
				t.Fatalf("TestForge should generate repositories for %s", platform.name)
			}

			testRepo := repositories[0]
			cfg := &config.Config{
				Repositories: repositories,
			}

			// Test frontmatter generation with realistic repository data
			file := docs.DocFile{
				Repository:   testRepo.Name,
				Name:         "testforge-integration",
				RelativePath: "api/testforge-integration.md",
				DocsBase:     "docs",
				Section:      "api",
				Forge:        platform.name,
			}

			fm := BuildFrontMatter(FrontMatterInput{
				File:   file,
				Config: cfg,
				Now:    fixedTime(),
			})

			// Validate basic frontmatter fields
			if fm["title"] != "Testforge Integration" {
				t.Errorf("Expected title 'Testforge Integration', got %v", fm["title"])
			}
			if fm["repository"] != testRepo.Name {
				t.Errorf("Expected repository %s, got %v", testRepo.Name, fm["repository"])
			}
			if fm["section"] != "api" {
				t.Errorf("Expected section 'api', got %v", fm["section"])
			}
			if fm["forge"] != platform.name {
				t.Errorf("Expected forge %s, got %v", platform.name, fm["forge"])
			}

			// Validate editURL generation with TestForge repository URLs
			if editURL, ok := fm["editURL"]; ok {
				editURLStr, isString := editURL.(string)
				if !isString {
					t.Errorf("editURL should be a string, got %T", editURL)
				} else if len(editURLStr) == 0 {
					t.Errorf("editURL should not be empty")
				} else {
					t.Logf("✓ Generated editURL: %s", editURLStr)
				}
			}

			// Validate date field
			if fm["date"] == nil {
				t.Error("date field should be set")
			}

			t.Logf("✓ %s frontmatter: repo=%s, title=%v, editURL present=%v",
				platform.name, testRepo.Name, fm["title"], fm["editURL"] != nil)
		})
	}
}

// TestTestForgeRepositoryMetadataInFrontmatter validates that TestForge repository metadata is accessible
func TestTestForgeRepositoryMetadataInFrontmatter(t *testing.T) {
	forge := testforge.NewTestForge("metadata-test", config.ForgeGitHub)
	repositories := forge.ToConfigRepositories()

	if len(repositories) == 0 {
		t.Fatal("TestForge should generate repositories")
	}

	testRepo := repositories[0]
	cfg := &config.Config{
		Repositories: repositories,
	}

	file := docs.DocFile{
		Repository:   testRepo.Name,
		Name:         "metadata-test",
		RelativePath: "metadata-test.md",
		DocsBase:     "docs",
	}

	fm := BuildFrontMatter(FrontMatterInput{
		File:   file,
		Config: cfg,
		Now:    fixedTime(),
	})

	// Validate that TestForge repository metadata is reflected
	if fm["repository"] != testRepo.Name {
		t.Errorf("Expected repository %s, got %v", testRepo.Name, fm["repository"])
	}

	// The repository should have realistic metadata from TestForge
	if testRepo.Tags != nil {
		if description, ok := testRepo.Tags["description"]; ok && len(description) > 0 {
			t.Logf("✓ TestForge repository description: %s", description)
		}
		if language, ok := testRepo.Tags["language"]; ok && len(language) > 0 {
			t.Logf("✓ TestForge repository language: %s", language)
		}
	}

	t.Logf("✓ TestForge metadata integration: repository %s with URL %s", testRepo.Name, testRepo.URL)
}
