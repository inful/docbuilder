package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The generated index belongs next to the documents it describes.
//
// Two repositories can share a name: the same name on two forges. The
// generated index was then written in the tree of one of them, and the
// landing page of the other one had no index at all: Hugo rendered it
// as a bare section page, with no content and no navigation.
func TestTheGeneratedIndexLandsInTheTreeOfItsOwnDocuments(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "",
		},
		{
			Path:       "content/forgejo/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "forgejo",
		},
	}

	// The metadata knows one name, so it knows one forge: the first one
	// it finds for that name.
	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	indexes := map[string]bool{}
	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		indexes[doc.Path] = true
	}

	for _, want := range []string{"content/repo/_index.md", "content/forgejo/repo/_index.md"} {
		if !indexes[want] {
			t.Fatalf("the index %s is missing, the build wrote %v", want, indexes)
		}
	}
}

// The metadata of a repository can name a forge that its documents do
// not live in.
//
// The index was then written in the tree of the metadata, and the
// landing page of the repository had no index: an empty page without
// the navigation.
func TestTheGeneratedIndexFollowsTheDocumentsAndNotTheMetadata(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/repo/guide.md",
			Name:       "guide",
			Extension:  ".md",
			Repository: "repo",
			Forge:      "",
		},
	}

	metadata := map[string]RepositoryInfo{
		"repo": {Name: "repo", Forge: "forgejo"},
	}

	processed, err := processor.ProcessContent(documents, metadata, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	found := ""
	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		found = doc.Path
	}

	if found != "content/repo/_index.md" {
		t.Fatalf("the index is at %s, want content/repo/_index.md", found)
	}
}
