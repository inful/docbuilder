package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A document and a section can want the same page.
//
// The document `x.md` publishes at the page of the section `x`, and
// the generated index of the section publishes at the same page. Hugo
// keeps one of them and says nothing about the other: the index was a
// file in the content tree that published nowhere, and in the tree of
// the metadata it took the page of the document away.
func TestTheIndexIsNotGeneratedForASectionWithADocumentOfTheSameName(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{
			Path:       "content/repo/plan/x.md",
			Name:       "x",
			Extension:  ".md",
			Repository: "repo",
			Section:    "plan",
		},
		{
			Path:       "content/repo/plan/x/y.md",
			Name:       "y",
			Extension:  ".md",
			Repository: "repo",
			Section:    "plan/x",
		},
	}

	processed, err := processor.ProcessContent(documents, map[string]RepositoryInfo{}, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	for _, doc := range processed {
		if doc == nil || !doc.Generated {
			continue
		}
		if doc.Path == "content/repo/plan/x/_index.md" {
			t.Fatalf("the index of the section was generated, it wants the page of the document: %s", doc.Path)
		}
	}

	// The document itself stays: it is the page of the section.
	found := false
	for _, doc := range processed {
		if doc != nil && doc.Path == "content/repo/plan/x.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("the document was lost")
	}
}
