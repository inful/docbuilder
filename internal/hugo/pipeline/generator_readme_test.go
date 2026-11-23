package pipeline

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The landing page of a single repository build is the generated index.
//
// The pipeline renames the readme of the repository to _index, and the
// generated index is written over it: the home page of a portal with
// one repository is the page of the portal, with the title and the
// description of the configuration, and not the readme of the
// repository. The readme of a repository is the landing page of the
// repository in a portal with many repositories.
func TestTheGeneratedIndexIsTheLandingPageOfASingleRepositoryBuild(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/readme.md",
			Name:       "README",
			Extension:  ".md",
			Content:    "# The repository\n\nThe readme of the repository.\n",
			Repository: "repo",
			Section:    "",
		},
	}

	processed, err := processor.ProcessContent(documents, map[string]RepositoryInfo{}, true)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	generated := 0
	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		generated++
		if !strings.Contains(doc.Content, "Documentation hub") {
			t.Fatalf("the landing page is not the generated index: %q", doc.Content)
		}
	}
	if generated != 1 {
		t.Fatalf("the build generated %d indexes, want 1", generated)
	}
}
