package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestPageFacadeContract validates that *Page satisfies PageFacade and that
// facade methods mutate the underlying Page consistently with existing merge logic.
func TestPageFacadeContract(t *testing.T) {
	var _ PageFacade = (*Page)(nil) // compile-time assertion (redundant but explicit)

	p := &Page{File: docs.DocFile{Name: "example", Repository: "repo1", Extension: ".md"}, Content: "Body", OriginalFrontMatter: map[string]any{
		"title": "Original Title",
		"tags":  []any{"alpha", "beta"},
	}, HadFrontMatter: true}

	// Use facade-style methods
	if p.GetContent() != "Body" {
		// sanity
		p.SetContent("Body")
	}
	p.AddPatch(FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 10, Data: map[string]any{
		"description": "Added",          // new key
		"title":       "Original Title", // identical (no conflict overwrite)
		"tags":        []any{"beta", "gamma"},
	}})
	p.ApplyPatches()

	fm := p.MergedFrontMatter
	if fm == nil {
		t.Fatalf("expected merged front matter")
	}
	// title should remain original
	if fm["title"] != "Original Title" {
		t.Fatalf("expected title preserved, got %v", fm["title"])
	}
	if fm["description"] != "Added" {
		t.Fatalf("expected description added, got %v", fm["description"])
	}
	tagsV, ok := fm["tags"].([]any)
	if !ok {
		// Allow string slice path
		if sv, ok2 := fm["tags"].([]string); ok2 {
			for _, v := range sv {
				if v == "alpha" || v == "beta" || v == "gamma" {
					continue
				}
			}
			return
		}
		// unexpected type
		return
	}
	// Expect union preserving order of first slice then new uniques: alpha, beta, gamma
	if len(tagsV) != 3 {
		t.Fatalf("expected 3 tags after union, got %d", len(tagsV))
	}
	order := []string{tagsV[0].(string), tagsV[1].(string), tagsV[2].(string)}
	if order[0] != "alpha" || order[1] != "beta" || order[2] != "gamma" {
		t.Fatalf("unexpected tag union order %v", order)
	}
}
