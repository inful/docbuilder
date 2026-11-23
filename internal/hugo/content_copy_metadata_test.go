package hugo

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// The metadata of the repositories is built from the repositories of the
// build: the versioning expansion adds a repository for every version,
// and the versioned pages look their metadata up by that name. Without
// them they have no edit link and no landing page.
func TestBuildRepositoryMetadataHoldsTheVersionedRepositories(t *testing.T) {
	cfg := &config.Config{
		Repositories: []config.Repository{{
			Name:   "repo",
			URL:    "https://example.com/repo.git",
			Paths:  []string{"docs"},
			Tags:   map[string]string{"forge_type": "github"},
			Branch: "main",
		}},
	}

	g := NewGenerator(cfg, t.TempDir())
	bs := models.NewBuildState(g, []docs.DocFile{
		{Repository: "repo-v1.0", Name: "page", Extension: ".md", Forge: "github"},
	}, models.NewBuildReport(context.Background(), 0, 0))
	bs.Git.Repositories = []config.Repository{{
		Name:   "repo-v1.0",
		URL:    "https://example.com/repo.git",
		Paths:  []string{"docs"},
		Branch: "v1.0",
		Tags:   map[string]string{"forge_type": "github", "version": "v1.0"},
	}}

	metadata := g.buildRepositoryMetadata(bs)

	info, ok := metadata["repo-v1.0"]
	if !ok {
		t.Fatalf("the versioned repository is missing from the metadata: %+v", metadata)
	}
	// The forge is the one the discovery gave: the content path of the
	// generated index has to be the one of the pages.
	if info.Forge != "github" {
		t.Fatalf("the forge of the versioned repository is %q", info.Forge)
	}
	if info.Branch != "v1.0" {
		t.Fatalf("the branch of the versioned repository is %q", info.Branch)
	}
}
