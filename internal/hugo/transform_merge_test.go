package hugo

import (
	"testing"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func testDocFile(name string, meta map[string]string) docs.DocFile {
	return docs.DocFile{Repository: "repo", Name: name, Metadata: meta}
}

func TestFrontMatterDeepMergeNested(t *testing.T) {
	p := &Page{OriginalFrontMatter: map[string]any{"a": map[string]any{"b": 1, "c": map[string]any{"d": 2}}, "tags": []any{"x"}}}
	p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 10, Data: map[string]any{
		"a": map[string]any{"c": map[string]any{"e": 3}, "f": 4},
		"tags": []any{"y", "x"},
	}})
	p.applyPatches()
	a := p.MergedFrontMatter["a"].(map[string]any)
	if a["b"].(int) != 1 || a["f"].(int) != 4 { t.Fatalf("expected preserved and added keys in nested merge: %#v", a) }
	inner := a["c"].(map[string]any)
	if inner["d"].(int) != 2 || inner["e"].(int) != 3 { t.Fatalf("expected deep nested merge: %#v", inner) }
	tags := p.MergedFrontMatter["tags"].([]any)
	if len(tags) != 2 { t.Fatalf("expected union of tags, got %v", tags) }
}

func TestReservedKeyProtection(t *testing.T) {
	p := &Page{OriginalFrontMatter: map[string]any{"title": "Original", "description": "Desc"}}
	p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 5, Data: map[string]any{"title": "NewTitle", "description": "NewDesc"}})
	p.applyPatches()
	if p.MergedFrontMatter["title"].(string) != "Original" { t.Fatalf("reserved key title should be protected") }
	if p.MergedFrontMatter["description"].(string) != "Desc" { t.Fatalf("reserved key description should be protected") }
	if len(p.Conflicts) == 0 { t.Fatalf("expected conflicts recorded") }
}

func TestSetIfMissingAndReplace(t *testing.T) {
	p := &Page{OriginalFrontMatter: map[string]any{"slug": "keep"}}
	p.Patches = append(p.Patches,
		FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 10, Data: map[string]any{"slug": "ignored", "newkey": "val"}},
		FrontMatterPatch{Source: "override", Mode: MergeReplace, Priority: 20, Data: map[string]any{"slug": "override"}},
	)
	p.applyPatches()
	if p.MergedFrontMatter["slug"].(string) != "override" { t.Fatalf("expected replace to override slug") }
	if p.MergedFrontMatter["newkey"].(string) != "val" { t.Fatalf("expected newkey set") }
}

func TestArrayStrategies(t *testing.T) {
	p := &Page{OriginalFrontMatter: map[string]any{"tags": []any{"a", "b"}}}
	// union (default for tags) then append via explicit patch
	p.Patches = append(p.Patches,
		FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 5, Data: map[string]any{"tags": []any{"b", "c"}}},
		FrontMatterPatch{Source: "append", Mode: MergeDeep, Priority: 10, ArrayStrategy: ArrayAppend, Data: map[string]any{"tags": []any{"d"}}},
	)
	p.applyPatches()
	tags := p.MergedFrontMatter["tags"].([]any)
	expectedOrder := []any{"a", "b", "c", "d"}
	if len(tags) != len(expectedOrder) { t.Fatalf("unexpected tags length: %v", tags) }
	for i, v := range expectedOrder { if tags[i] != v { t.Fatalf("unexpected order %v", tags) } }
}

func TestBuilderIntegrationConflictRecording(t *testing.T) {
	p := &Page{OriginalFrontMatter: map[string]any{"title": "Keep"}}
	p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 10, Data: map[string]any{"title": "Change", "other": 1}})
	p.applyPatches()
	if p.MergedFrontMatter["title"].(string) != "Keep" { t.Fatalf("title should remain original") }
	if p.MergedFrontMatter["other"].(int) != 1 { t.Fatalf("other key should be set") }
	found := false
	for _, c := range p.Conflicts { if c.Key == "title" && c.Action == "kept_original" { found = true } }
	if !found { t.Fatalf("expected kept_original conflict for title") }
}

// Ensure BuildFrontMatter still produces baseline fields integrated via patch.
func TestFrontMatterBuilderPatchFlow(t *testing.T) {
	cfg := &config.Config{}
	gen := &Generator{config: cfg, outputDir: "out"}
	p := &Page{File: testDocFile("sample", nil), OriginalFrontMatter: map[string]any{}, Content: ""}
	fb := &FrontMatterBuilder{ConfigProvider: func() *Generator { return gen }}
	if err := fb.Transform(p); err != nil { t.Fatalf("builder transform error: %v", err) }
	if len(p.Patches) == 0 { t.Fatalf("expected patch emitted") }
	p.applyPatches()
	if p.MergedFrontMatter["title"] == nil { t.Fatalf("expected title set from builder") }
}
