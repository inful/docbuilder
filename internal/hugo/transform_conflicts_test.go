package hugo

import "testing"

// TestFrontMatterConflicts exercises merge logic to ensure conflict actions are stable.
func TestFrontMatterConflicts(t *testing.T) {
	p := &Page{
		OriginalFrontMatter: map[string]any{
			"title":       "Existing Title", // protected key
			"description": "Existing Desc",  // protected key but will be MergeReplace
			"weight":      10,               // protected key, used with set-if-missing
			"tags":        []any{"alpha"},   // taxonomy array (union default)
		},
	}
	p.Patches = []FrontMatterPatch{
		{Source: "builder", Mode: MergeDeep, Priority: 10, Data: map[string]any{
			// Attempt to override protected title -> should be kept_original
			"title": "New Title",
			// Add new non-protected key to ensure normal overwrite recorded later
			"keywords": []string{"k1"},
		}},
		{Source: "replace", Mode: MergeReplace, Priority: 20, Data: map[string]any{
			// description with MergeReplace should overwrite and record overwritten
			"description": "Replaced Desc",
		}},
		{Source: "set_if_missing", Mode: MergeSetIfMissing, Priority: 30, Data: map[string]any{
			// weight exists; action kept_original under set-if-missing path
			"weight": 42,
			// introducing a new key with set-if-missing should be set_if_missing action
			"summary": "Short summary",
		}},
		{Source: "deep_array", Mode: MergeDeep, Priority: 40, Data: map[string]any{
			// taxonomy union; should merge without producing conflict entries (union path doesn't append conflict)
			"tags": []string{"alpha", "beta"},
		}},
	}

	p.applyPatches()

	// Build index of conflict actions by key for assertions; some keys may have multiple entries if overwritten twice.
	byKey := map[string][]FrontMatterConflict{}
	for _, c := range p.Conflicts {
		byKey[c.Key] = append(byKey[c.Key], c)
	}

	// title: protected & attempted overwrite via MergeDeep => kept_original
	if cs := byKey["title"]; len(cs) != 1 || cs[0].Action != "kept_original" {
		t.Fatalf("expected title kept_original conflict, got %#v", cs)
	}
	// description: replaced via MergeReplace => overwritten
	if cs := byKey["description"]; len(cs) != 1 || cs[0].Action != "overwritten" {
		t.Fatalf("expected description overwritten conflict, got %#v", cs)
	}
	// weight: set-if-missing with existing value => kept_original
	if cs := byKey["weight"]; len(cs) != 1 || cs[0].Action != "kept_original" {
		t.Fatalf("expected weight kept_original under set-if-missing, got %#v", cs)
	}
	// summary: newly added with set-if-missing => set_if_missing
	if cs := byKey["summary"]; len(cs) != 1 || cs[0].Action != "set_if_missing" {
		t.Fatalf("expected summary set_if_missing action, got %#v", cs)
	}
	// tags: taxonomy union merge should not create a conflict entry
	if _, ok := byKey["tags"]; ok {
		t.Fatalf("did not expect conflict entry for tags union merge, got %#v", byKey["tags"])
	}
	// keywords: newly added by builder deep patch without existing value -> no conflict (only conflicts on overwrite, keep, or set_if_missing)
	if _, ok := byKey["keywords"]; ok {
		t.Fatalf("did not expect conflict entry for new keywords, got %#v", byKey["keywords"])
	}

	// Validate final merged state reflects intended outcomes
	if p.MergedFrontMatter["title"] != "Existing Title" {
		t.Fatalf("title changed unexpectedly: %#v", p.MergedFrontMatter["title"])
	}
	if p.MergedFrontMatter["description"] != "Replaced Desc" {
		t.Fatalf("description not replaced: %#v", p.MergedFrontMatter["description"])
	}
	if p.MergedFrontMatter["weight"].(int) != 10 {
		t.Fatalf("weight should remain original (10), got %#v", p.MergedFrontMatter["weight"])
	}
	if p.MergedFrontMatter["summary"] != "Short summary" {
		t.Fatalf("summary missing: %#v", p.MergedFrontMatter["summary"])
	}
	// tags union order stable: [alpha beta]
	tags, _ := p.MergedFrontMatter["tags"].([]any)
	if len(tags) != 2 || tags[0] != "alpha" || tags[1] != "beta" {
		t.Fatalf("unexpected tags union result: %#v", tags)
	}
}
