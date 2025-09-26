package hugo

import (
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

func fixedTime() time.Time { return time.Date(2025, 9, 26, 12, 34, 56, 0, time.UTC) }

func TestBuildFrontMatter_TitleAndBasicFields(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
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
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	file := docs.DocFile{Repository: "repo1", Name: "index"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if _, exists := fm["title"]; exists {
		t.Fatalf("index file should not auto-generate title, got %v", fm["title"])
	}
}

func TestBuildFrontMatter_MetadataPassthrough(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
	file := docs.DocFile{Repository: "repo1", Name: "ref", Metadata: map[string]string{"product": "alpha"}}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if fm["product"] != "alpha" {
		t.Fatalf("metadata not passed through: %v", fm["product"])
	}
}

func TestBuildFrontMatter_EditURL_GitHub(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/project.git", Branch: "develop"}}}
	file := docs.DocFile{Repository: "repo1", Name: "intro", RelativePath: "intro.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://github.com/org/project/edit/develop/docs/intro.md"
	if fm["editURL"] != want {
		t.Fatalf("expected editURL %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_GitLabSSH(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}, Repositories: []config.Repository{{Name: "r", URL: "git@gitlab.com:group/proj.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "r", Name: "guide", RelativePath: "dir/guide.md", DocsBase: "documentation"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://gitlab.com/group/proj/-/edit/main/documentation/dir/guide.md"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_Bitbucket(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}, Repositories: []config.Repository{{Name: "bb", URL: "https://bitbucket.org/team/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "bb", Name: "page", RelativePath: "page.md", DocsBase: "."}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://bitbucket.org/team/repo/src/main/page.md?mode=edit"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_Gitea(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}, Repositories: []config.Repository{{Name: "gt", URL: "https://git.home.luguber.info/org/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "gt", Name: "usage", RelativePath: "nested/usage.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	want := "https://git.home.luguber.info/org/repo/_edit/main/docs/nested/usage.md"
	if fm["editURL"] != want {
		t.Fatalf("expected %s got %v", want, fm["editURL"])
	}
}

func TestBuildFrontMatter_EditURL_SiteBaseSuppressesPerPage(t *testing.T) {
	params := map[string]any{"editURL": map[string]any{"base": "https://example.com/edit"}}
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra", Params: params}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo.git", Branch: "main"}}}
	file := docs.DocFile{Repository: "repo1", Name: "conf", RelativePath: "conf.md", DocsBase: "docs"}
	fm := BuildFrontMatter(FrontMatterInput{File: file, Config: cfg, Now: fixedTime()})
	if _, exists := fm["editURL"]; exists {
		t.Fatalf("per-page editURL should be suppressed when site base provided, got %v", fm["editURL"])
	}
}

func TestBuildFrontMatter_ExistingPreserved(t *testing.T) {
	cfg := &config.V2Config{Hugo: config.HugoConfig{Theme: "hextra"}}
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
