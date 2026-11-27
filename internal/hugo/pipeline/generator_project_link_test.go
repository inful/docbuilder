package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The landing page of a repository and the landing page of a section
// link to the page of the project in the forge.
//
// The reader of such a page wants to see the project the page belongs
// to, and the address of the project is the address of the repository
// in the forge: the clone address without the ".git" suffix.
func TestTheGeneratedIndexesLinkToTheProjectInTheForge(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/forgejo/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
		},
		{
			Path:       "content/forgejo/repo/api/endpoint.md",
			Name:       "endpoint",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
			Section:    "api",
		},
	}
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo", URL: "https://git.example.com/org/repo.git"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	want := map[string]string{
		"content/forgejo/repo/_index.md":     "https://git.example.com/org/repo",
		"content/forgejo/repo/api/_index.md": "https://git.example.com/org/repo",
	}
	found := map[string]string{}
	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		if url, ok := doc.FrontMatter["projectURL"].(string); ok {
			found[doc.Path] = url
		}
	}

	for path, url := range want {
		if found[path] != url {
			t.Fatalf("the index %s links to %q, want %q", path, found[path], url)
		}
	}
}

// The clone address of a repository can be the address of SSH. The
// page of the project in the forge is a page of the web.
func TestTheProjectLinkOfAnSSHCloneAddressIsAWebAddress(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/forgejo/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
		},
	}
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo", URL: "git@git.example.com:org/repo.git"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	for _, doc := range processed {
		if doc == nil || !doc.Generated || doc.Repository != "repo" {
			continue
		}
		url, _ := doc.FrontMatter["projectURL"].(string)
		if url != "https://git.example.com/org/repo" {
			t.Fatalf("the index %s links to %q, want https://git.example.com/org/repo", doc.Path, url)
		}
	}
}

// A local repository has no page in a forge, and the index carries no
// link to one.
func TestTheGeneratedIndexOfALocalRepositoryHasNoProjectLink(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
		},
	}
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", URL: "/var/lib/docbuilder/repos/repo"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		if url, ok := doc.FrontMatter["projectURL"].(string); ok {
			t.Fatalf("the index %s links to %q, want no link", doc.Path, url)
		}
	}
}

// The landing page of the site belongs to no project: a site that
// gathers the pages of many repositories has no single project page.
func TestTheLandingPageOfTheSiteHasNoProjectLink(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/forgejo/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
		},
	}
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo", URL: "https://git.example.com/org/repo.git"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	for _, doc := range processed {
		if doc == nil || !doc.Generated || doc.Path != contentIndexPath {
			continue
		}
		if url, ok := doc.FrontMatter["projectURL"].(string); ok {
			t.Fatalf("the landing page of the site links to %q, want no link", url)
		}
	}
}

// The operator can name the address of the project themselves. The
// link points there, and not at the address of the clone.
func TestTheProjectLinkFollowsTheAddressOfTheOperator(t *testing.T) {
	cfg := &config.Config{}
	cfg.Build.EditURLBase = "https://github.com/org/other"
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/forgejo/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
		},
	}
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo", URL: "https://git.example.com/org/repo.git"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	for _, doc := range processed {
		if doc == nil || !doc.Generated || doc.Repository != "repo" {
			continue
		}
		url, _ := doc.FrontMatter["projectURL"].(string)
		if url != "https://github.com/org/other" {
			t.Fatalf("the index %s links to %q, want https://github.com/org/other", doc.Path, url)
		}
	}
}
