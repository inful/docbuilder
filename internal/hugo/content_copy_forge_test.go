package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// The forge of a repository is the one the discovery gave it.
//
// The discovery names a repository only when the configuration asks for
// it: a portal whose repositories all come from one forge has no
// namespace, the pages live in content/<repo>/, and the generated index
// of the repository has to live there as well. The namespace of the
// repository is not the answer: the index landed in another tree than
// the pages.
func TestTheForgeOfARepositoryIsTheOneOfTheDiscovery(t *testing.T) {
	bs := models.NewBuildState(nil, []docs.DocFile{
		{Repository: "repoA", Name: "page", Extension: ".md"},
	}, nil)

	repo := &config.Repository{
		Name:      "repoA",
		Namespace: "github",
		Tags:      map[string]string{"forge_type": "github"},
	}

	if forge := forgeOfRepository(bs, repo); forge != "" {
		t.Fatalf("the forge of the repository is %q, want the one of the discovery (none)", forge)
	}

	// A repository that the discovery named keeps its forge.
	bs.Docs.Files[0].Forge = "github"
	if forge := forgeOfRepository(bs, repo); forge != "github" {
		t.Fatalf("the forge of the repository is %q, want github", forge)
	}
}
