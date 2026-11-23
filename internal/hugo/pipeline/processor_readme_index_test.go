package pipeline

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// A README next to an index keeps its name.
//
// The discovery maps an index.md to _index.md, and the pipeline renames
// a README to _index as well: the two of them are the same page, and one
// of them was lost. A repository with a README and an index in the same
// directory is a common layout.
func TestAReadmeNextToAnIndexKeepsItsName(t *testing.T) {
	cfg := &config.Config{}
	processor := NewProcessor(cfg)

	documents := []*Document{
		{Path: "repo/_index.md", Name: "index", Extension: ".md", Content: "# Index\n", Repository: "repo"},
		{Path: "repo/readme.md", Name: "README", Extension: ".md", Content: "# Readme\n", Repository: "repo"},
	}

	processed, err := processor.ProcessContent(documents, map[string]RepositoryInfo{}, false)
	if err != nil {
		t.Fatalf("the pipeline failed: %v", err)
	}

	paths := make(map[string]string, len(processed))
	for _, doc := range processed {
		if doc == nil {
			continue
		}
		paths[doc.Path] = doc.Name
	}

	if _, ok := paths["repo/readme.md"]; !ok {
		t.Fatalf("the readme was renamed into the page of the index: %v", paths)
	}
}
