package hugo

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// TestBuildCategoriesMenu_CollidingIntroFiles is the reproducer for the
// "two projects both have an intro.md with the same title and category"
// case. The categories menu wiring must keep each entry distinguishable
// in the rendered Relearn sidebar.
//
// The function-under-test is pure: it takes the DocFile set and the
// repository registry, and returns the Relearn menu definition map
// keyed by category name. No filesystem, no Hugo binary.
func TestBuildCategoriesMenu_CollidingIntroFiles(t *testing.T) {
	repos := []config.Repository{
		{Name: "project_a", DisplayName: "Project A"},
		{Name: "project_b", DisplayName: "Project B"},
	}

	// Two docs from different repos with identical filenames, titles,
	// and category. This is the collision scenario.
	docs := []categoryDoc{
		{
			Repository: "project_a",
			Title:      "Introduction",
			Path:       "/project_a/intro/",
			Weight:     1,
			Categories: []string{"Documentation"},
		},
		{
			Repository: "project_b",
			Title:      "Introduction",
			Path:       "/project_b/intro/",
			Weight:     1,
			Categories: []string{"Documentation"},
		},
		// And a second doc per repo to make sure the menu groups under
		// the project parent rather than emitting flat entries.
		{
			Repository: "project_a",
			Title:      "Setup",
			Path:       "/project_a/setup/",
			Weight:     2,
			Categories: []string{"Documentation"},
		},
		{
			Repository: "project_b",
			Title:      "Configuration",
			Path:       "/project_b/config/",
			Weight:     1,
			Categories: []string{"Documentation"},
		},
	}

	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	if cm == nil {
		t.Fatal("buildCategoriesMenu returned nil")
	}

	// The Documentation menu must exist.
	docMenu, ok := cm.Menus["documentation"]
	if !ok {
		t.Fatalf("expected a documentation menu, got keys: %v", keys(cm.Menus))
	}

	// The menu must contain a title-wrapper entry, one parent
	// entry per project (so the tree groups children under their
	// repo) and one child entry per doc. Total = 1 wrapper +
	// 2 parents + 4 children = 7.
	if got, want := len(docMenu), 7; got != want {
		t.Fatalf("Documentation menu length = %d, want %d; entries: %+v", got, want, docMenu)
	}

	// The two docs titled "Introduction" must NOT collide: each must
	// have a distinct parent (project_a vs project_b). If both were
	// emitted as siblings, Relearn would render two indistinguishable
	// "Introduction" links at the same level.
	introParents := map[string]string{}
	for _, e := range docMenu {
		for _, c := range docs {
			if e.PageRef == c.Path && e.Name == c.Title {
				if prev, dup := introParents[c.Title]; dup && prev == e.Parent {
					t.Fatalf("two docs titled %q share the same parent %q; "+
						"they would render as indistinguishable sidebar entries",
						c.Title, e.Parent)
				}
				introParents[c.Title] = e.Parent
			}
		}
	}

	// And the sidebar must list exactly one block per category.
	if got, want := len(cm.SidebarEntries), 1; got != want {
		t.Fatalf("sidebar entries = %d, want %d; got: %+v", got, want, cm.SidebarEntries)
	}
	if cm.SidebarEntries[0].Identifier != "documentation" {
		t.Fatalf("sidebar entry identifier = %q, want %q",
			cm.SidebarEntries[0].Identifier, "documentation")
	}
	if cm.SidebarEntries[0].Type != "menu" {
		t.Fatalf("sidebar entry type = %q, want \"menu\"", cm.SidebarEntries[0].Type)
	}
}

// TestBuildCategoriesMenu_SkipsDocsWithoutCategory ensures docs with
// no categories front matter do not produce any menu entries.
func TestBuildCategoriesMenu_SkipsDocsWithoutCategory(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{Repository: "project_a", Title: "Untagged", Path: "/project_a/untagged/", Categories: nil},
		{Repository: "project_a", Title: "Tagged", Path: "/project_a/tagged/", Categories: []string{"Docs"}},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	if _, ok := cm.Menus["docs"]; !ok {
		t.Fatalf("expected docs menu, got: %v", keys(cm.Menus))
	}
	if _, ok := cm.Menus[""]; ok {
		t.Fatalf("empty category must not produce a menu")
	}
	// Sidebar must list only Docs.
	if len(cm.SidebarEntries) != 1 {
		t.Fatalf("sidebar entries = %d, want 1", len(cm.SidebarEntries))
	}
}

// TestBuildCategoriesMenu_EmptyInput is a no-op sanity check.
func TestBuildCategoriesMenu_EmptyInput(t *testing.T) {
	cm, err := buildCategoriesMenu(nil, nil, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	if cm == nil {
		t.Fatal("expected non-nil CategoriesMenu for empty input")
	}
	if len(cm.Menus) != 0 {
		t.Fatalf("expected no menus, got: %v", keys(cm.Menus))
	}
	if len(cm.SidebarEntries) != 0 {
		t.Fatalf("expected no sidebar entries, got: %+v", cm.SidebarEntries)
	}
}

// TestBuildCategoriesMenu_ProjectLabelUsesDisplayName asserts the
// project parent entry uses DisplayName when set, otherwise Name.
func TestBuildCategoriesMenu_ProjectLabelUsesDisplayName(t *testing.T) {
	repos := []config.Repository{
		{Name: "platform-services", DisplayName: "Platform Services"},
	}
	docs := []categoryDoc{
		{Repository: "platform-services", Title: "Overview", Path: "/platform-services/overview/", Categories: []string{"Reference"}},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entry := cm.Menus["reference"][1]
	if entry.Name != "Platform Services" {
		t.Fatalf("project entry name = %q, want %q (DisplayName should win over Name)", entry.Name, "Platform Services")
	}
	if entry.URL != "" {
		t.Fatalf("project parent entry must not have a URL, got %q", entry.URL)
	}
}

func keys(m map[string][]models.MenuEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestCategoriesMenu_EmitsUncategorizedBucketForUntaggedDocs asserts
// that a doc with no categories front matter is bucketed under the
// synthetic _uncategorized category, while a doc with a real
// category stays under its user-declared category. This is the core
// "easy to find uncategorized" use case.
func TestCategoriesMenu_EmitsUncategorizedBucketForUntaggedDocs(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		// Categorized doc: appears under Documentation only.
		{
			Repository: "project_a", Title: "Setup", Path: "/project_a/setup/",
			Categories: []string{"Documentation"},
		},
		// Untagged doc: appears under _uncategorized only.
		{
			Repository: "project_a", Title: "Orphan", Path: "/project_a/orphan/",
			Categories: []string{UncategorizedCategory},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	// Both keys must be present.
	if _, ok := cm.Menus["documentation"]; !ok {
		t.Fatalf("expected documentation menu; got keys: %v", keys(cm.Menus))
	}
	if _, ok := cm.Menus[UncategorizedCategory]; !ok {
		t.Fatalf("expected %q menu; got keys: %v", UncategorizedCategory, keys(cm.Menus))
	}
	// Documentation: only the categorized doc.
	for _, e := range cm.Menus["documentation"] {
		if e.Name == "Orphan" {
			t.Fatalf("Orphan must not appear in documentation menu")
		}
	}
	// _uncategorized: only the orphan doc.
	for _, e := range cm.Menus[UncategorizedCategory] {
		if e.Name == "Setup" {
			t.Fatalf("Setup must not appear in _uncategorized menu")
		}
	}
	// Two sidebar blocks: one per category.
	if got := len(cm.SidebarEntries); got != 2 {
		t.Fatalf("sidebar entries = %d, want 2; got: %+v", got, cm.SidebarEntries)
	}
}

// TestCategoriesMenu_UncategorizedSidebarBlockRendersLast asserts
// that the synthetic _uncategorized sidebar block carries a higher
// weight than user categories so it renders last in the Relearn
// sidebar regardless of alphabetical sort order.
func TestCategoriesMenu_UncategorizedSidebarBlockRendersLast(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "A1", Path: "/project_a/a1/",
			Categories: []string{"Documentation"},
		},
		{
			Repository: "project_a", Title: "B1", Path: "/project_a/b1/",
			Categories: []string{"Operations"},
		},
		{
			Repository: "project_a", Title: "O1", Path: "/project_a/o1/",
			Categories: []string{UncategorizedCategory},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	var uncategorized, others []models.SidebarEntry
	for _, e := range cm.SidebarEntries {
		if e.Weight == uncategorizedSidebarWeightValue {
			uncategorized = append(uncategorized, e)
		} else {
			others = append(others, e)
		}
	}
	if len(uncategorized) != 1 {
		t.Fatalf("expected exactly one _uncategorized sidebar block; got %d", len(uncategorized))
	}
	if len(others) != 2 {
		t.Fatalf("expected exactly two user-category sidebar blocks; got %d", len(others))
	}
	if uncategorized[0].Weight <= others[0].Weight {
		t.Fatalf("_uncategorized weight (%d) must be higher than user-category weight (%d) so it renders last",
			uncategorized[0].Weight, others[0].Weight)
	}
	if uncategorized[0].Weight != uncategorizedSidebarWeightValue {
		t.Fatalf("_uncategorized weight = %d, want %d", uncategorized[0].Weight, uncategorizedSidebarWeightValue)
	}
}

// TestReadCategoriesMenuDocs_EmitsNothingForEmptyDocSet locks down
// the truly-empty-build path: with no documents, the function
// returns an empty slice so the categories-menu stage falls back to
// the default sidebar (and emits the one-line INFO log).
func TestReadCategoriesMenuDocs_EmitsNothingForEmptyDocSet(t *testing.T) {
	got := readCategoriesMenuDocs(nil, false, false, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result for empty doc set; got %d entries", len(got))
	}
}

// TestReadCategoriesMenuDocs_BucketsUntaggedUnderSynthetic asserts
// the read-side bucketing: a doc with no categories front matter
// emits exactly one categoryDoc with the synthetic category. A doc
// with two user-declared categories emits two categoryDocs.
func TestReadCategoriesMenuDocs_BucketsUntaggedUnderSynthetic(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/a.md",
			Name:       "a",
			Repository: "project_a",
			Content:    []byte("---\ntitle: A\ncategories: [Documentation, Reference]\n---\n"),
		},
		{
			Path:       "/tmp/b.md",
			Name:       "b",
			Repository: "project_a",
			Content:    []byte("---\ntitle: B\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, false, nil)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries (1 doc × 2 categories + 1 doc × 1 synthetic); got %d", len(got))
	}
	var synth, userCat int
	for _, cd := range got {
		if len(cd.Categories) != 1 {
			t.Fatalf("each emitted entry must have exactly one category; got %v", cd.Categories)
		}
		switch cd.Categories[0] {
		case UncategorizedCategory:
			synth++
		default:
			userCat++
		}
	}
	if synth != 1 || userCat != 2 {
		t.Fatalf("got synth=%d userCat=%d; want synth=1 userCat=2", synth, userCat)
	}
}

// TestReadCategoriesMenuDocs_LoadsContentFromDisk is the regression
// test for the bug where the categories-menu stage ran BEFORE the
// copy-content stage and therefore saw f.Content == nil for every
// doc, producing no menu. The fix: readCategoriesMenuDocs now
// triggers the same lazy LoadContent() that the copy-content stage
// uses, so a doc with an empty Content field but a real on-disk
// Path is read and bucketed correctly.
func TestReadCategoriesMenuDocs_LoadsContentFromDisk(t *testing.T) {
	dir := t.TempDir()
	// Write a real markdown file with categories front matter.
	mdPath := filepath.Join(dir, "intro.md")
	if err := os.WriteFile(mdPath, []byte("---\ntitle: Introduction\ncategories: [Documentation]\n---\n\n# Introduction\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	files := []docs.DocFile{
		{
			Path:       mdPath,
			Repository: "project_a",
			// Content intentionally left nil to simulate the
			// pre-copy-content state.
			Content: nil,
		},
	}
	got := readCategoriesMenuDocs(files, false, false, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry from on-disk doc; got %d", len(got))
	}
	if got[0].Categories[0] != "documentation" {
		t.Fatalf("expected canonical lowercase documentation bucket; got %q", got[0].Categories[0])
	}
	if got[0].CategoryDisplay != "Documentation" {
		t.Fatalf("expected CategoryDisplay to preserve original spelling; got %q", got[0].CategoryDisplay)
	}
	if got[0].Title != "Introduction" {
		t.Fatalf("expected title from front matter; got %q", got[0].Title)
	}
}

// TestReadCategoriesMenuDocs_ToleratesMalformedFrontmatter asserts
// that a doc whose front matter is unparseable does not fail the
// build. The doc is bucketed under the synthetic _uncategorized
// category with a debug log, mirroring how the rest of the
// pipeline tolerates malformed front matter.
func TestReadCategoriesMenuDocs_ToleratesMalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "broken.md")
	if err := os.WriteFile(mdPath, []byte("---\ntitle: Broken [unterminated\n---\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	files := []docs.DocFile{
		{Path: mdPath, Repository: "project_a", Name: "broken"},
	}
	got := readCategoriesMenuDocs(files, false, false, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry from malformed doc; got %d", len(got))
	}
	if got[0].Categories[0] != UncategorizedCategory {
		t.Fatalf("malformed doc must be bucketed under _uncategorized; got %q", got[0].Categories[0])
	}
}

// TestBuildCategoriesMenu_PrependsTitleWrapperEntry locks down the
// title-wrapper convention: each category's menu opens with a single
// nameless top-level entry whose `name` becomes the sidebar block's
// title (Relearn's i18n-free title path). Every project parent
// entry is then re-parented under that wrapper so the wrapper is the
// menu's sole root.
func TestBuildCategoriesMenu_PrependsTitleWrapperEntry(t *testing.T) {
	repos := []config.Repository{
		{Name: "project_a", DisplayName: "Project A"},
		{Name: "project_b", DisplayName: "Project B"},
	}
	docs := []categoryDoc{
		{Repository: "project_a", Title: "Setup", Path: "/project_a/setup/", Categories: []string{"Documentation"}},
		{Repository: "project_b", Title: "Configuration", Path: "/project_b/config/", Categories: []string{"Documentation"}},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entries := cm.Menus["documentation"]
	if len(entries) == 0 {
		t.Fatal("expected non-empty documentation menu")
	}
	wrapper := entries[0]
	wantWrapperID := categoryTitleIdentifier("documentation")
	if wrapper.Identifier != wantWrapperID {
		t.Fatalf("wrapper identifier = %q, want %q", wrapper.Identifier, wantWrapperID)
	}
	if wrapper.Name != "Documentation" {
		t.Fatalf("wrapper name = %q, want %q (first-seen spelling)", wrapper.Name, "Documentation")
	}
	if wrapper.URL != "" || wrapper.PageRef != "" {
		t.Fatalf("wrapper must not be clickable; got url=%q pageRef=%q", wrapper.URL, wrapper.PageRef)
	}
	if wrapper.Parent != "" {
		t.Fatalf("wrapper must be at top level; got parent=%q", wrapper.Parent)
	}
	// Every project-parent entry must reference the wrapper.
	for _, e := range entries[1:] {
		// Project parents have an Identifier set; doc children do not.
		if e.Identifier == "" {
			continue
		}
		if e.Parent != wantWrapperID {
			t.Fatalf("project parent %q parent = %q, want wrapper %q",
				e.Identifier, e.Parent, wantWrapperID)
		}
	}
}

// TestBuildCategoriesMenu_UncategorizedWrapperShowsHumanLabel asserts
// the synthetic _uncategorized bucket renders with the literal
// display label "Uncategorized" rather than the raw "_uncategorized"
// front-matter token.
func TestBuildCategoriesMenu_UncategorizedWrapperShowsHumanLabel(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "Orphan", Path: "/project_a/orphan/",
			Categories: []string{UncategorizedCategory},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wrapper := cm.Menus[UncategorizedCategory][0]
	if wrapper.Name != "Uncategorized" {
		t.Fatalf("uncategorized wrapper name = %q, want %q", wrapper.Name, "Uncategorized")
	}
}

// TestCategoryDisplayLabel checks the synthetic-vs-user category
// label rules: the synthetic bucket always renders as "Uncategorized";
// every other category renders with the supplied display spelling
// verbatim; an empty display falls back to the canonical key.
func TestCategoryDisplayLabel(t *testing.T) {
	cases := []struct {
		key, display, want string
	}{
		{UncategorizedCategory, "anything", "Uncategorized"},
		{UncategorizedCategory, "", "Uncategorized"},
		{"minutes", "Minutes", "Minutes"},
		{"minutes", "minutes", "minutes"},
		{"ios", "iOS", "iOS"},
		{"ebpf", "eBPF", "eBPF"},
		{"documentation", "", "documentation"},
	}
	for _, c := range cases {
		if got := categoryDisplayLabel(c.key, c.display); got != c.want {
			t.Errorf("categoryDisplayLabel(%q, %q) = %q, want %q",
				c.key, c.display, got, c.want)
		}
	}
}

// TestReadCategoriesMenuDocs_LowercasesCategoryKey is the read-side
// regression for case-insensitive grouping: the canonical Categories[0]
// key must be lowercased, while CategoryDisplay must preserve the
// author's original spelling so the sidebar label can use it.
func TestReadCategoriesMenuDocs_LowercasesCategoryKey(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/m.md",
			Name:       "m",
			Repository: "project_a",
			Content:    []byte("---\ntitle: M\ncategories: [Minutes]\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, false, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry; got %d", len(got))
	}
	if got[0].Categories[0] != "minutes" {
		t.Fatalf("canonical key = %q, want %q", got[0].Categories[0], "minutes")
	}
	if got[0].CategoryDisplay != "Minutes" {
		t.Fatalf("CategoryDisplay = %q, want %q", got[0].CategoryDisplay, "Minutes")
	}
}

// TestBuildCategoriesMenu_MergesCategoriesCaseInsensitively is the
// core fix for the user-reported bug: two docs that declare the same
// category with different capitalisations must end up in a single
// sidebar block, with the first-seen original spelling shown as the
// wrapper label.
func TestBuildCategoriesMenu_MergesCategoriesCaseInsensitively(t *testing.T) {
	repos := []config.Repository{
		{Name: "project_a", DisplayName: "Project A"},
		{Name: "project_b", DisplayName: "Project B"},
	}
	// Mimic the shape readCategoriesMenuDocs would produce: lowercased
	// Categories[0] paired with the author's original CategoryDisplay.
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "Setup", Path: "/project_a/setup/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
		},
		{
			Repository: "project_b", Title: "Standup", Path: "/project_b/standup/",
			Categories: []string{"minutes"}, CategoryDisplay: "minutes",
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	// Exactly one menu bucket, one sidebar entry.
	if got := len(cm.Menus); got != 1 {
		t.Fatalf("len(Menus) = %d, want 1; keys=%v", got, keys(cm.Menus))
	}
	entries, ok := cm.Menus["minutes"]
	if !ok {
		t.Fatalf("expected canonical key %q; got: %v", "minutes", keys(cm.Menus))
	}
	if got := len(cm.SidebarEntries); got != 1 {
		t.Fatalf("len(SidebarEntries) = %d, want 1; got %+v", got, cm.SidebarEntries)
	}
	if id := cm.SidebarEntries[0].Identifier; id != "minutes" {
		t.Fatalf("sidebar identifier = %q, want %q", id, "minutes")
	}
	// Wrapper carries the first-seen original spelling.
	wrapper := entries[0]
	if wrapper.Identifier != "cat_minutes_top" {
		t.Fatalf("wrapper identifier = %q, want %q", wrapper.Identifier, "cat_minutes_top")
	}
	if wrapper.Name != "Minutes" {
		t.Fatalf("wrapper name = %q, want %q (first-seen spelling)", wrapper.Name, "Minutes")
	}
	// Both project parents must be present and reference the wrapper.
	wantParents := map[string]bool{
		"cat_minutes_project_a": true,
		"cat_minutes_project_b": true,
	}
	seen := map[string]bool{}
	for _, e := range entries[1:] {
		if e.Identifier == "" {
			continue
		}
		if e.Parent != "cat_minutes_top" {
			t.Errorf("project parent %q parent = %q, want %q",
				e.Identifier, e.Parent, "cat_minutes_top")
		}
		seen[e.Identifier] = true
	}
	for id := range wantParents {
		if !seen[id] {
			t.Errorf("missing project parent %q; got entries %+v", id, entries)
		}
	}
	// Both docs must be grouped under their respective project parents.
	docsByParent := map[string]string{}
	for _, e := range entries {
		if e.PageRef != "" {
			docsByParent[e.Parent] = e.Name
		}
	}
	if docsByParent["cat_minutes_project_a"] != "Setup" {
		t.Errorf("project_a doc = %q, want %q", docsByParent["cat_minutes_project_a"], "Setup")
	}
	if docsByParent["cat_minutes_project_b"] != "Standup" {
		t.Errorf("project_b doc = %q, want %q", docsByParent["cat_minutes_project_b"], "Standup")
	}
}

// TestBuildCategoriesMenu_IdentifiersStableAcrossCaseFlips verifies
// that all emitted identifiers (wrapper, project parents, sidebar)
// are derived from the canonical lowercase key, so they stay
// byte-identical regardless of which spelling appeared first.
func TestBuildCategoriesMenu_IdentifiersStableAcrossCaseFlips(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	mk := func(display string) []categoryDoc {
		return []categoryDoc{
			{
				Repository: "project_a", Title: "T", Path: "/project_a/t/",
				Categories: []string{"minutes"}, CategoryDisplay: display,
			},
		}
	}
	a, err := buildCategoriesMenu(mk("Minutes"), repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu(Minutes): %v", err)
	}
	b, err := buildCategoriesMenu(mk("minutes"), repos, "repo", nil)
	if err != nil {
		t.Fatalf("buildCategoriesMenu(minutes): %v", err)
	}
	idsA := idsOf(a.Menus["minutes"])
	idsB := idsOf(b.Menus["minutes"])
	if !equalStringSlices(idsA, idsB) {
		t.Fatalf("identifiers diverge across casings:\n Minutes -> %v\n minutes -> %v",
			idsA, idsB)
	}
	if a.SidebarEntries[0].Identifier != b.SidebarEntries[0].Identifier {
		t.Fatalf("sidebar identifier diverges: %q vs %q",
			a.SidebarEntries[0].Identifier, b.SidebarEntries[0].Identifier)
	}
}

func idsOf(entries []models.MenuEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Identifier)
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestReadCategoriesMenuDocs_PublicOnly_DropsPrivateDocs is the
// reproducer for the public_only menu-leak: when daemon.content.public_only
// is on, only docs whose front matter carries `public: true` may
// contribute to the categories menu. A private doc tagged with a
// category must not surface in the sidebar at all (not even as
// _uncategorized).
func TestReadCategoriesMenuDocs_PublicOnly_DropsPrivateDocs(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/pub.md",
			Name:       "pub",
			Repository: "project_a",
			Content: []byte("---\ntitle: Public Doc\npublic: true\n" +
				"categories: [Minutes]\n---\n"),
		},
		{
			Path:       "/tmp/priv.md",
			Name:       "priv",
			Repository: "project_a",
			Content: []byte("---\ntitle: Private Doc\n" +
				"categories: [Secret]\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, true, nil)
	if len(got) != 1 {
		t.Fatalf("expected exactly the public doc; got %d entries: %+v", len(got), got)
	}
	if got[0].Title != "Public Doc" {
		t.Fatalf("surviving entry title = %q, want %q", got[0].Title, "Public Doc")
	}
	if got[0].Categories[0] != "minutes" {
		t.Fatalf("surviving entry canonical category = %q, want %q",
			got[0].Categories[0], "minutes")
	}
	for _, cd := range got {
		if cd.Categories[0] == "secret" {
			t.Fatalf("private doc's category %q leaked into menu: %+v", "secret", cd)
		}
	}
}

// TestReadCategoriesMenuDocs_PublicOnly_NoPublicDocsForCategoryDropsBucket
// asserts that a category referenced only by private docs disappears
// entirely from the menu input set.
func TestReadCategoriesMenuDocs_PublicOnly_NoPublicDocsForCategoryDropsBucket(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/priv.md",
			Name:       "priv",
			Repository: "project_a",
			Content: []byte("---\ntitle: Private\n" +
				"categories: [Minutes]\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, true, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result (only private doc); got %d entries: %+v", len(got), got)
	}
}

// TestReadCategoriesMenuDocs_PublicOnly_KeepsPublicUncategorized
// verifies that the public_only filter only drops *private* docs:
// a public doc with no `categories:` front matter still ends up
// in the synthetic _uncategorized bucket.
func TestReadCategoriesMenuDocs_PublicOnly_KeepsPublicUncategorized(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/orphan.md",
			Name:       "orphan",
			Repository: "project_a",
			Content:    []byte("---\ntitle: Public Orphan\npublic: true\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, true, nil)
	if len(got) != 1 {
		t.Fatalf("expected the public orphan to survive; got %d entries", len(got))
	}
	if got[0].Categories[0] != UncategorizedCategory {
		t.Fatalf("public orphan must be bucketed under %q; got %q",
			UncategorizedCategory, got[0].Categories[0])
	}
}

// TestReadCategoriesMenuDocs_PublicOnly_MalformedFrontmatterDropped
// pins the fail-closed behavior for malformed front matter under
// public_only: the doc is dropped rather than bucketed, because we
// cannot prove it is meant to be public.
func TestReadCategoriesMenuDocs_PublicOnly_MalformedFrontmatterDropped(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "broken.md")
	if err := os.WriteFile(mdPath, []byte("---\ntitle: Broken [unterminated\n---\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	files := []docs.DocFile{
		{Path: mdPath, Repository: "project_a", Name: "broken"},
	}
	got := readCategoriesMenuDocs(files, false, true, nil)
	if len(got) != 0 {
		t.Fatalf("expected malformed doc to be dropped under public_only; got %+v", got)
	}
}

// TestComputeCategoriesMenu_PublicOnly_FiltersDownstream is the
// stage-level end-to-end check: with daemon.content.public_only on,
// the resulting CategoriesMenu must only contain buckets whose
// members include at least one public doc, and each surviving bucket
// must hold only the public docs.
func TestComputeCategoriesMenu_PublicOnly_FiltersDownstream(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:   "Public Only Build",
			Sidebar: &config.SidebarConfig{Mode: config.SidebarModeCategories},
		},
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	// Three docs: two public in distinct categories, one private in
	// a third category. The private doc's category must not surface.
	docPub1 := docWithFrontMatter("project_a", "alpha",
		"---\ntitle: Alpha\npublic: true\ncategories: [Documentation]\n---\n")
	docPub2 := docWithFrontMatter("project_a", "ops",
		"---\ntitle: Ops\npublic: true\ncategories: [Operations]\n---\n")
	docPriv := docWithFrontMatter("project_a", "secret",
		"---\ntitle: Secret\ncategories: [Internal]\n---\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{docPub1, docPub2, docPriv}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	if _, ok := cm.Menus["documentation"]; !ok {
		t.Errorf("expected documentation bucket; got keys: %v", keys(cm.Menus))
	}
	if _, ok := cm.Menus["operations"]; !ok {
		t.Errorf("expected operations bucket; got keys: %v", keys(cm.Menus))
	}
	if _, ok := cm.Menus["internal"]; ok {
		t.Errorf("private-only category 'internal' leaked into menu: %v", keys(cm.Menus))
	}
	for _, entries := range cm.Menus {
		for _, e := range entries {
			if e.Name == "Secret" {
				t.Errorf("private doc 'Secret' leaked into a category bucket: %+v", e)
			}
		}
	}
}

// TestReadCategoriesMenuDocs_ExtractsGroupingFields is the read-side
// regression for hugo.sidebar.group_by: when the read path is given
// a non-empty list of grouping field names, the doc's front matter
// for each named field must end up on categoryDoc.GroupFields keyed
// by that field name.
func TestReadCategoriesMenuDocs_ExtractsGroupingFields(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/m.md",
			Name:       "m",
			Repository: "project_a",
			Content: []byte("---\ntitle: M\ncategories: [Minutes]\n" +
				"projects: [team_alpha]\n---\n"),
		},
	}
	// The read path is given the per-category axis map (already
	// lowercased by SidebarConfig.Normalize at config load).
	got := readCategoriesMenuDocs(files, false, false, map[string][]string{
		"minutes": {"projects"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry; got %d", len(got))
	}
	if got[0].GroupFields == nil {
		t.Fatalf("GroupFields must be populated when grouping fields are requested")
	}
	if got[0].GroupFields["projects"] != "team_alpha" {
		t.Fatalf("GroupFields[projects] = %q, want %q",
			got[0].GroupFields["projects"], "team_alpha")
	}
}

// TestReadCategoriesMenuDocs_NoGroupingFieldsNoMap confirms the
// common case: when the read path is called with nil grouping
// fields, no GroupFields map is allocated.
func TestReadCategoriesMenuDocs_NoGroupingFieldsNoMap(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/m.md",
			Name:       "m",
			Repository: "project_a",
			Content:    []byte("---\ntitle: M\ncategories: [Minutes]\nprojects: team_alpha\n---\n"),
		},
	}
	got := readCategoriesMenuDocs(files, false, false, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 entry; got %d", len(got))
	}
	if got[0].GroupFields != nil {
		t.Fatalf("GroupFields must be nil when no grouping fields are requested; got %v",
			got[0].GroupFields)
	}
}

// TestBuildCategoriesMenu_GroupByFrontMatterField is the headline
// test for the new feature: docs in a category configured with
// `group_by: [project]` are bucketed under their `project:` value
// instead of under their source Repository.
func TestBuildCategoriesMenu_GroupByFrontMatterField(t *testing.T) {
	repos := []config.Repository{
		{Name: "project_a"},
		{Name: "project_b"},
	}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "alpha1", Path: "/project_a/alpha1/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
		{
			Repository: "project_b", Title: "alpha2", Path: "/project_b/alpha2/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
		{
			Repository: "project_a", Title: "beta1", Path: "/project_a/beta1/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_beta"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entries := cm.Menus["minutes"]
	// Expect: wrapper + 2 project parents (alpha, beta) + 3 docs = 6.
	if got, want := len(entries), 6; got != want {
		t.Fatalf("minutes menu length = %d, want %d; got: %+v", got, want, entries)
	}
	// Both alpha docs must share the same parent identifier.
	wantAlphaID := categoryParentIdentifier("minutes", "team_alpha")
	wantBetaID := categoryParentIdentifier("minutes", "team_beta")
	wantTitleID := categoryTitleIdentifier("minutes")
	got := map[string]string{}
	for _, e := range entries {
		switch e.Identifier {
		case wantTitleID:
			if e.Parent != "" {
				t.Errorf("wrapper must be at top level; got parent=%q", e.Parent)
			}
		case wantAlphaID:
			if e.Parent != wantTitleID {
				t.Errorf("alpha parent must point at wrapper; got parent=%q", e.Parent)
			}
			if e.Name != "team_alpha" {
				t.Errorf("alpha parent label = %q, want %q", e.Name, "team_alpha")
			}
		case wantBetaID:
			if e.Parent != wantTitleID {
				t.Errorf("beta parent must point at wrapper; got parent=%q", e.Parent)
			}
		}
		if e.PageRef != "" {
			got[e.Parent] = e.Name
		}
	}
	// Confirm doc parents are correct.
	alphaDocs := 0
	betaDocs := 0
	for _, e := range entries {
		if e.PageRef == "" {
			continue
		}
		switch e.Parent {
		case wantAlphaID:
			alphaDocs++
		case wantBetaID:
			betaDocs++
		}
	}
	if alphaDocs != 2 {
		t.Errorf("alphaDocs = %d, want 2", alphaDocs)
	}
	if betaDocs != 1 {
		t.Errorf("betaDocs = %d, want 1", betaDocs)
	}
}

// TestBuildCategoriesMenu_GroupByFirstSeenSpelling confirms the
// first-seen original spelling rule applies to the new project level
// the same way it does for category wrappers: two casings collapse to
// one bucket labeled with whichever appeared first.
func TestBuildCategoriesMenu_GroupByFirstSeenSpelling(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "a1", Path: "/project_a/a1/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "Team_Alpha"},
		},
		{
			Repository: "project_a", Title: "a2", Path: "/project_a/a2/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entries := cm.Menus["minutes"]
	wantID := categoryParentIdentifier("minutes", "team_alpha")
	for _, e := range entries {
		if e.Identifier == wantID && e.Name != "Team_Alpha" {
			t.Errorf("first-seen project label = %q, want %q", e.Name, "Team_Alpha")
		}
	}
}

// TestBuildCategoriesMenu_GroupByFallsBackToRepository asserts that
// a doc in a configured category with no value for the configured
// grouping field is bucketed under its source Repository.
func TestBuildCategoriesMenu_GroupByFallsBackToRepository(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "no-project-field", Path: "/project_a/x/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			// GroupFields is nil: no `project:` declared.
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "project_a")
	found := false
	for _, e := range cm.Menus["minutes"] {
		if e.Identifier == wantID {
			found = true
			if e.Name != "project_a" {
				t.Errorf("fallback parent label = %q, want %q", e.Name, "project_a")
			}
		}
	}
	if !found {
		t.Fatalf("expected fallback to project_a parent; got entries: %+v", cm.Menus["minutes"])
	}
}

// TestBuildCategoriesMenu_GroupByScopedToConfiguredCategories
// asserts that the group_by map only affects categories listed in
// it; other categories continue to use Repository-based grouping.
func TestBuildCategoriesMenu_GroupByScopedToConfiguredCategories(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "minute", Path: "/project_a/m/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
		{
			Repository: "project_a", Title: "doc", Path: "/project_a/d/",
			Categories: []string{"operations"}, CategoryDisplay: "Operations",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"}, // operations is NOT in the map
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	// "minutes" must use the project field; expect a team_alpha parent.
	if id := categoryParentIdentifier("minutes", "team_alpha"); !entryByID(cm.Menus["minutes"], id) {
		t.Errorf("minutes menu missing team_alpha parent: %+v", cm.Menus["minutes"])
	}
	// "operations" must NOT use the project field; it should fall
	// back to project_a (the source Repository).
	if id := categoryParentIdentifier("operations", "team_alpha"); entryByID(cm.Menus["operations"], id) {
		t.Errorf("operations menu should not have a team_alpha parent: %+v", cm.Menus["operations"])
	}
	if id := categoryParentIdentifier("operations", "project_a"); !entryByID(cm.Menus["operations"], id) {
		t.Errorf("operations menu missing project_a parent: %+v", cm.Menus["operations"])
	}
}

// TestBuildCategoriesMenu_GroupByPublicOnlyFilter applies the
// public_only contract to docs that carry a grouping field: a
// private doc is dropped, and its group key must not surface.
func TestBuildCategoriesMenu_GroupByPublicOnlyFilter(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Public Only Build",
			Sidebar: &config.SidebarConfig{
				Mode:    config.SidebarModeCategories,
				GroupBy: map[string][]string{"minutes": {"projects"}},
			},
		},
		Daemon: &config.DaemonConfig{
			Content: config.DaemonContentConfig{PublicOnly: true},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	docPub := docWithFrontMatter("project_a", "pub",
		"---\ntitle: Public\npublic: true\ncategories: [Minutes]\n"+
			"projects: team_alpha\n---\n")
	docPriv := docWithFrontMatter("project_a", "priv",
		"---\ntitle: Private\ncategories: [Minutes]\n"+
			"projects: secret_team\n---\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{docPub, docPriv}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	if id := categoryParentIdentifier("minutes", "secret_team"); entryByID(cm.Menus["minutes"], id) {
		t.Errorf("private group's parent leaked into menu: %+v", cm.Menus["minutes"])
	}
	if id := categoryParentIdentifier("minutes", "team_alpha"); !entryByID(cm.Menus["minutes"], id) {
		t.Errorf("public group's parent missing: %+v", cm.Menus["minutes"])
	}
}

func entryByID(entries []models.MenuEntry, id string) bool {
	for _, e := range entries {
		if e.Identifier == id {
			return true
		}
	}
	return false
}

// TestReadCategoriesMenuDocs_CaseInsensitiveFrontMatter is the
// regression for the case where the user's config says
// group_by: { minutes: [Project] } (or any case) but the doc's
// front matter uses lowercase `projects:`. Both should match.
func TestReadCategoriesMenuDocs_CaseInsensitiveFrontMatter(t *testing.T) {
	files := []docs.DocFile{
		{
			Path:       "/tmp/m.md",
			Name:       "m",
			Repository: "project_a",
			// Front matter uses capital P.
			Content: []byte("---\ntitle: M\ncategories: [Minutes]\n" +
				"Projects: [team_alpha]\n---\n"),
		},
	}
	// Read path is given the per-category axis map (already
	// lowercased by SidebarConfig.Normalize at config load).
	got := readCategoriesMenuDocs(files, false, false, map[string][]string{
		"minutes": {"projects"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry; got %d", len(got))
	}
	if got[0].GroupFields["projects"] != "team_alpha" {
		t.Fatalf("GroupFields[projects] = %q, want %q (case-insensitive lookup)",
			got[0].GroupFields["projects"], "team_alpha")
	}
}

// TestComputeCategoriesMenu_GroupByMixedCaseConfig is the
// end-to-end regression for the user-reported bug: when the user
// writes the group_by map with capitalised keys (e.g. "Minutes")
// and the doc's front matter field uses a different case, the
// categories menu must still use the projects field for grouping.
func TestComputeCategoriesMenu_GroupByMixedCaseConfig(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Mixed Case Group By",
			Sidebar: &config.SidebarConfig{
				Mode: config.SidebarModeCategories,
				// Capitalised key and field name on purpose.
				GroupBy: map[string][]string{
					"Minutes": {"Projects"},
				},
			},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	// Force the Normalize pass (mimicking what the config loader
	// does at startup).
	cfg.Hugo.Sidebar.Normalize(&config.NormalizationResult{})

	g := NewGenerator(cfg, out)
	// Doc uses lowercase categories and lowercase project.
	doc := docWithFrontMatter("project_a", "alpha",
		"---\ntitle: Alpha\ncategories: [Minutes]\n"+
			"projects: [team_alpha]\n---\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{doc}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "team_alpha")
	if !entryByID(cm.Menus["minutes"], wantID) {
		t.Fatalf("expected team_alpha parent under minutes; got: %+v", cm.Menus["minutes"])
	}
	// The doc must be the child of the team_alpha parent, NOT the
	// project_a (Repository) parent.
	for _, e := range cm.Menus["minutes"] {
		if e.PageRef == "" || e.Name != "Alpha" {
			continue
		}
		if e.Parent != wantID {
			t.Errorf("doc parent = %q, want %q (group_by must override Repository)", e.Parent, wantID)
		}
	}
}

// TestBuildCategoriesMenu_GroupByThreeLevels is the regression for
// N-level max-depth nesting. With group_by: [project, year] the
// rendered tree is three levels deep:
//   - Level 1: project (from `project:` front matter or Repository)
//   - Level 2: year    (from `year:`    front matter or Repository)
//   - Leaf:   the doc
//
// Each level's identifier is path-based, and the empty-level rule
// (A1) is verified: a doc with a `project` but no `year` lands
// under `<repo>` at level 2.
func TestBuildCategoriesMenu_GroupByThreeLevels(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			// Full path: project_a > team_alpha > 2024.
			Repository: "project_a", Title: "q1-2024", Path: "/q1-2024/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha", "year": "2024"},
		},
		{
			// Missing year -> year falls back to Repository (A1).
			Repository: "project_a", Title: "no-year", Path: "/no-year/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
		{
			// No project, no year -> both fall back to Repository.
			Repository: "project_a", Title: "neither", Path: "/neither/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "year"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entries := cm.Menus["minutes"]

	wantWrapperID := categoryTitleIdentifier("minutes")
	wantL1AlphaID := categoryParentIdentifier("minutes", "team_alpha")
	wantL2Alpha2024ID := categoryParentIdentifier("minutes", "team_alpha", "2024")
	wantL2AlphaRepoID := categoryParentIdentifier("minutes", "team_alpha", "project_a")
	wantL1RepoID := categoryParentIdentifier("minutes", "project_a")
	wantL2RepoRepoID := categoryParentIdentifier("minutes", "project_a", "project_a")

	wantIDs := map[string]string{
		"wrapper":      wantWrapperID,
		"l1-alpha":     wantL1AlphaID,
		"l2-2024":      wantL2Alpha2024ID,
		"l2-repo":      wantL2AlphaRepoID,
		"l1-repo":      wantL1RepoID,
		"l2-repo-repo": wantL2RepoRepoID,
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Identifier] = true
	}
	for k, id := range wantIDs {
		if !seen[id] {
			t.Errorf("missing %s identifier %q in entries: %+v", k, id, entries)
		}
	}

	// Spot-check parents: wrapper is top-level; each level-1 entry
	// points at the wrapper; each level-2 entry points at its
	// level-1 parent.
	for _, e := range entries {
		switch e.Identifier {
		case wantWrapperID:
			if e.Parent != "" {
				t.Errorf("wrapper must be at top level; got parent=%q", e.Parent)
			}
		case wantL1AlphaID, wantL1RepoID:
			if e.Parent != wantWrapperID {
				t.Errorf("level-1 %q parent = %q, want %q", e.Identifier, e.Parent, wantWrapperID)
			}
		case wantL2Alpha2024ID, wantL2AlphaRepoID, wantL2RepoRepoID:
			if e.Parent == "" {
				t.Errorf("level-2 %q must have a parent", e.Identifier)
			}
		}
	}

	// Each doc must be a child of the right level-2 node.
	wantDocParent := map[string]string{
		"q1-2024": wantL2Alpha2024ID,
		"no-year": wantL2AlphaRepoID,
		"neither": wantL2RepoRepoID,
	}
	for _, e := range entries {
		want, ok := wantDocParent[e.Name]
		if !ok {
			continue
		}
		if e.PageRef == "" {
			t.Errorf("doc entry %q has no PageRef", e.Name)
			continue
		}
		if e.Parent != want {
			t.Errorf("doc %q parent = %q, want %q (level-2 path)", e.Name, e.Parent, want)
		}
	}
}

// TestBuildCategoriesMenu_GroupByThreeLevelFirstSeenSpelling pins the
// first-seen raw-spelling rule at every level: team_alpha and
// Team_Alpha collapse to one bucket labeled "Team_Alpha" at level
// 1, and 2024 / 2024-Q1 collapse at level 2.
func TestBuildCategoriesMenu_GroupByThreeLevelFirstSeenSpelling(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "first", Path: "/first/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "Team_Alpha", "year": "2024-Q1"},
		},
		{
			Repository: "project_a", Title: "second", Path: "/second/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha", "year": "2024-q1"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "year"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantL1ID := categoryParentIdentifier("minutes", "team_alpha")
	wantL2ID := categoryParentIdentifier("minutes", "team_alpha", "2024-q1")
	for _, e := range cm.Menus["minutes"] {
		switch e.Identifier {
		case wantL1ID:
			if e.Name != "Team_Alpha" {
				t.Errorf("level-1 first-seen = %q, want %q", e.Name, "Team_Alpha")
			}
		case wantL2ID:
			if e.Name != "2024-Q1" {
				t.Errorf("level-2 first-seen = %q, want %q", e.Name, "2024-Q1")
			}
		}
	}
}

// TestBuildCategoriesMenu_GroupBySiblingCategory is the headline
// test for the sibling-category axis. Config:
//
//	group_by:
//	  minutes: [project, team]
//	  team: [project]
//
// The chain for `minutes` is [front-matter `project`, sibling
// category `team`]. A doc with categories: [Minutes, Team-Alpha]
// must land under Minutes > project > Team-Alpha, with the original
// spelling preserved on the level-2 node label.
func TestBuildCategoriesMenu_GroupBySiblingCategory(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "alpha-doc", Path: "/alpha-doc/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "Team-Alpha",
			},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "team"},
		"team":    {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entries := cm.Menus["minutes"]
	wantL1ID := categoryParentIdentifier("minutes", "team_alpha")
	wantL2ID := categoryParentIdentifier("minutes", "team_alpha", "team-alpha")
	for _, e := range entries {
		switch e.Identifier {
		case wantL1ID:
			if e.Name != "team_alpha" {
				t.Errorf("level-1 label = %q, want %q", e.Name, "team_alpha")
			}
		case wantL2ID:
			if e.Name != "Team-Alpha" {
				t.Errorf("level-2 label = %q, want %q (first-seen original spelling)",
					e.Name, "Team-Alpha")
			}
			if e.Parent != wantL1ID {
				t.Errorf("level-2 parent = %q, want %q", e.Parent, wantL1ID)
			}
		}
	}
	for _, e := range entries {
		if e.PageRef == "" {
			continue
		}
		if e.Parent != wantL2ID {
			t.Errorf("doc parent = %q, want %q (sibling-category level)", e.Parent, wantL2ID)
		}
	}
}

// TestBuildCategoriesMenu_GroupBySiblingCategoryMissingFallsBackToRepo
// asserts the A1 fallback for a sibling-category axis: a doc in
// `minutes` but NOT in the sibling `team` must land under the
// Repository at that level, so the path stays fully populated.
func TestBuildCategoriesMenu_GroupBySiblingCategoryMissingFallsBackToRepo(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "no-team", Path: "/no-team/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields:       map[string]string{"projects": "team_alpha"},
			SiblingCategories: map[string]string{"minutes": "Minutes"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "team"},
		"team":    {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantL2ID := categoryParentIdentifier("minutes", "team_alpha", "project_a")
	if !entryByID(cm.Menus["minutes"], wantL2ID) {
		t.Errorf("missing Repository-fallback level-2 node %q: %+v",
			wantL2ID, cm.Menus["minutes"])
	}
}

// TestBuildCategoriesMenu_GroupBySiblingCategoryCaseInsensitive
// pins the case-insensitive match against the doc's categories list
// and the first-seen-spelling rule for the rendered label: docs that
// declare the sibling under different casings all collapse into one
// bucket labeled with whichever spelling the renderer saw first.
func TestBuildCategoriesMenu_GroupBySiblingCategoryCaseInsensitive(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "first", Path: "/first/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "Team-Alpha",
			},
		},
		{
			Repository: "project_a", Title: "second", Path: "/second/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "team-alpha",
			},
		},
		{
			Repository: "project_a", Title: "third", Path: "/third/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "TEAM-ALPHA",
			},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"team"},
		"team":    {},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "team-alpha")
	wantName := "Team-Alpha"
	matches := 0
	for _, e := range cm.Menus["minutes"] {
		if e.Identifier == wantID {
			matches++
			if e.Name != wantName {
				t.Errorf("level-1 label = %q, want %q (first-seen wins)",
					e.Name, wantName)
			}
		}
	}
	if matches != 1 {
		t.Errorf("expected exactly 1 level-1 node; got %d", matches)
	}
}

// TestBuildCategoriesMenu_GroupBySiblingCategoryDeepChain mixes a
// front-matter field (project), a sibling category (team), and a
// second front-matter field (quarter) in a 3-level chain.
func TestBuildCategoriesMenu_GroupBySiblingCategoryDeepChain(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "q1", Path: "/q1/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{
				"projects": "team_alpha",
				"quarter":  "Q1",
			},
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "Team-Alpha",
			},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "team", "quarter"},
		"team":    {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantL1ID := categoryParentIdentifier("minutes", "team_alpha")
	wantL2ID := categoryParentIdentifier("minutes", "team_alpha", "team-alpha")
	wantL3ID := categoryParentIdentifier("minutes", "team_alpha", "team-alpha", "q1")
	for _, e := range cm.Menus["minutes"] {
		switch e.Identifier {
		case wantL1ID:
			if e.Parent != categoryTitleIdentifier("minutes") {
				t.Errorf("L1 parent = %q, want wrapper", e.Parent)
			}
		case wantL2ID:
			if e.Parent != wantL1ID {
				t.Errorf("L2 parent = %q, want %q", e.Parent, wantL1ID)
			}
		case wantL3ID:
			if e.Parent != wantL2ID {
				t.Errorf("L3 parent = %q, want %q", e.Parent, wantL2ID)
			}
		}
	}
}

// TestBuildCategoriesMenu_GroupBySiblingCategoryOrderIndependent
// asserts the renderer matches the sibling category by name, not by
// position in the doc's categories: list.
func TestBuildCategoriesMenu_GroupBySiblingCategoryOrderIndependent(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	mk := func(swap bool) []categoryDoc {
		_ = swap
		return []categoryDoc{
			{
				Repository: "project_a", Title: "doc", Path: "/doc/",
				Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
				GroupFields:       map[string]string{"projects": "team_alpha"},
				SiblingCategories: map[string]string{"minutes": "Minutes", "team": "Team"},
			},
		}
	}
	configMap := map[string][]string{
		"minutes": {"projects", "team"},
		"team":    {"projects"},
	}
	cm1, err := buildCategoriesMenu(mk(false), repos, "repo", configMap)
	if err != nil {
		t.Fatalf("buildCategoriesMenu(order A): %v", err)
	}
	cm2, err := buildCategoriesMenu(mk(true), repos, "repo", configMap)
	if err != nil {
		t.Fatalf("buildCategoriesMenu(order B): %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "team_alpha", "team")
	if !entryByID(cm1.Menus["minutes"], wantID) {
		t.Errorf("order A: missing sibling level-2 node %q: %+v",
			wantID, cm1.Menus["minutes"])
	}
	if !entryByID(cm2.Menus["minutes"], wantID) {
		t.Errorf("order B: missing sibling level-2 node %q: %+v",
			wantID, cm2.Menus["minutes"])
	}
}

// TestReadCategoriesMenuDocs_SiblingCategoriesExtracted confirms the
// read path populates SiblingCategories when grouping fields are
// requested, and leaves it nil otherwise.
func TestReadCategoriesMenuDocs_SiblingCategoriesExtracted(t *testing.T) {
	withGrouping := []docs.DocFile{
		docWithFrontMatter("project_a", "m",
			"---\ntitle: M\ncategories: [Minutes, Team, Extra]\n---\n"),
	}
	got := readCategoriesMenuDocs(withGrouping, false, false, map[string][]string{
		"minutes": {"projects"},
	})
	if len(got) == 0 {
		t.Fatalf("expected entries; got 0")
	}
	for _, e := range got {
		if e.SiblingCategories == nil {
			t.Errorf("SiblingCategories must be populated when grouping is requested")
		}
		if e.SiblingCategories["minutes"] != "Minutes" {
			t.Errorf("SiblingCategories[minutes] = %q, want %q",
				e.SiblingCategories["minutes"], "Minutes")
		}
		if e.SiblingCategories["team"] != "Team" {
			t.Errorf("SiblingCategories[team] = %q, want %q",
				e.SiblingCategories["team"], "Team")
		}
		if e.SiblingCategories["extra"] != "Extra" {
			t.Errorf("SiblingCategories[extra] = %q, want %q",
				e.SiblingCategories["extra"], "Extra")
		}
	}
	noGrouping := readCategoriesMenuDocs(withGrouping, false, false, nil)
	for _, e := range noGrouping {
		if e.SiblingCategories != nil {
			t.Errorf("SiblingCategories must be nil when no grouping is requested; got %v",
				e.SiblingCategories)
		}
	}
}

// TestBuildCategoriesMenu_GroupByCategoryPrefersSibling pins the
// disambiguation rule: when an axis name is BOTH a sibling category
// (a key in the same groupBy map) AND a front-matter field on the
// doc, the renderer must prefer the sibling-category resolution.
func TestBuildCategoriesMenu_GroupByCategoryPrefersSibling(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "doc", Path: "/doc/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{
				"projects": "team_alpha",
				"team":     "frontmatter_value",
			},
			SiblingCategories: map[string]string{
				"minutes": "Minutes",
				"team":    "Team",
			},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects", "team"},
		"team":    {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "team_alpha", "team")
	if !entryByID(cm.Menus["minutes"], wantID) {
		t.Errorf("missing sibling level-2 node %q (sibling must win over front-matter field): %+v",
			wantID, cm.Menus["minutes"])
	}
	badID := categoryParentIdentifier("minutes", "team_alpha", "frontmatter_value")
	if entryByID(cm.Menus["minutes"], badID) {
		t.Errorf("front-matter level-2 node %q leaked into menu; sibling must win: %+v",
			badID, cm.Menus["minutes"])
	}
}

// TestExtractAxisValues covers the four shapes the helper handles:
// scalar string, []string, []any of strings, and missing/empty.
// These are the same shapes Hugo's front-matter parser can produce
// depending on the YAML form the author used.
func TestExtractAxisValues(t *testing.T) {
	cases := []struct {
		name string
		fm   map[string]any
		axis string
		want []string
	}{
		{
			name: "scalar string",
			fm:   map[string]any{"projects": "team_alpha"},
			axis: "projects",
			want: []string{"team_alpha"},
		},
		{
			name: "list of strings",
			fm:   map[string]any{"projects": []string{"team_alpha", "team_beta"}},
			axis: "projects",
			want: []string{"team_alpha", "team_beta"},
		},
		{
			name: "list of any (Hugo's typical parse for YAML lists)",
			fm:   map[string]any{"projects": []any{"team_alpha", "team_beta"}},
			axis: "projects",
			want: []string{"team_alpha", "team_beta"},
		},
		{
			name: "missing field",
			fm:   map[string]any{},
			axis: "projects",
			want: []string{""},
		},
		{
			name: "empty list",
			fm:   map[string]any{"projects": []any{}},
			axis: "projects",
			want: []string{""},
		},
		{
			name: "list with whitespace and empties (filtered)",
			fm:   map[string]any{"projects": []any{"  team_alpha  ", "", "team_beta"}},
			axis: "projects",
			want: []string{"team_alpha", "team_beta"},
		},
		{
			name: "case-insensitive lookup",
			fm:   map[string]any{"Projects": "team_alpha"},
			axis: "projects",
			want: []string{"team_alpha"},
		},
		{
			name: "non-string element in []any (filtered)",
			fm:   map[string]any{"projects": []any{"team_alpha", 42, "team_beta"}},
			axis: "projects",
			want: []string{"team_alpha", "team_beta"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAxisValues(tc.fm, []string{tc.axis})
			if len(got) != 1 {
				t.Fatalf("expected 1 axis, got %d", len(got))
			}
			if !reflect.DeepEqual(got[0].values, tc.want) {
				t.Errorf("values = %#v, want %#v", got[0].values, tc.want)
			}
		})
	}
}

// TestReadCategoriesMenuDocs_FanOutOnListAxis is the read-path
// regression for list-valued axes: a doc with `projects: [a, b]`
// produces two categoryDoc entries, one per value, each with a
// single-value GroupFields map.
func TestReadCategoriesMenuDocs_FanOutOnListAxis(t *testing.T) {
	files := []docs.DocFile{
		docWithFrontMatter("project_a", "doc",
			"---\ntitle: Doc\ncategories: [Minutes]\n"+
				"projects: [team_alpha, team_beta]\n---\n"),
	}
	got := readCategoriesMenuDocs(files, false, false, map[string][]string{
		"minutes": {"projects"},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 entries (fan-out); got %d", len(got))
	}
	sort.Slice(got, func(i, j int) bool {
		return got[i].GroupFields["projects"] < got[j].GroupFields["projects"]
	})
	if got[0].GroupFields["projects"] != "team_alpha" {
		t.Errorf("entry 0 project = %q, want %q", got[0].GroupFields["projects"], "team_alpha")
	}
	if got[1].GroupFields["projects"] != "team_beta" {
		t.Errorf("entry 1 project = %q, want %q", got[1].GroupFields["projects"], "team_beta")
	}
}

// TestReadCategoriesMenuDocs_CrossProductCategoryAndProject asserts
// the cross-product: a doc with `categories: [A, B]` and
// `projects: [x, y]` produces 4 categoryDoc entries (one per
// (cat, project) combination).
func TestReadCategoriesMenuDocs_CrossProductCategoryAndProject(t *testing.T) {
	files := []docs.DocFile{
		docWithFrontMatter("project_a", "doc",
			"---\ntitle: Doc\ncategories: [A, B]\n"+
				"projects: [x, y]\n---\n"),
	}
	got := readCategoriesMenuDocs(files, false, false, map[string][]string{
		"a": {"projects"},
		"b": {"projects"},
	})
	if len(got) != 4 {
		t.Fatalf("expected 4 entries (2 cats × 2 projects); got %d", len(got))
	}
	seen := map[string]bool{}
	for _, e := range got {
		cat := e.Categories[0]
		seen[cat+"|"+e.GroupFields["projects"]] = true
	}
	want := []string{"a|x", "a|y", "b|x", "b|y"}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing (cat, project) combination %q; got %v", w, seen)
		}
	}
}

// TestReadCategoriesMenuDocs_FanOutRespectsGroupByScope: a list-
// valued field for a category NOT in group_by must not fan out
// (the per-category Repository grouping applies, single entry).
func TestReadCategoriesMenuDocs_FanOutRespectsGroupByScope(t *testing.T) {
	files := []docs.DocFile{
		docWithFrontMatter("project_a", "doc",
			"---\ntitle: Doc\ncategories: [Operations]\n"+
				"projects: [team_alpha, team_beta]\n---\n"),
	}
	// Only `minutes` is in group_by; `operations` is not, so the
	// project list must NOT drive a fan-out for this doc.
	got := readCategoriesMenuDocs(files, false, false, map[string][]string{
		"minutes": {"projects"},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry (no fan-out for non-grouped category); got %d", len(got))
	}
	if got[0].GroupFields != nil {
		t.Errorf("GroupFields = %v, want nil (non-grouped category must not see project)",
			got[0].GroupFields)
	}
}

// TestBuildCategoriesMenu_MultiValueProjectInSidebar: end-to-end
// pipeline assertion. A doc with `projects: [a, b]` and
// `group_by: minutes: [project]` lands the doc under both
// `Minutes > a` and `Minutes > b`.
func TestBuildCategoriesMenu_MultiValueProjectInSidebar(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "shared", Path: "/shared/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_alpha"},
		},
		{
			Repository: "project_a", Title: "shared", Path: "/shared/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team_beta"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantA := categoryParentIdentifier("minutes", "team_alpha")
	wantB := categoryParentIdentifier("minutes", "team_beta")
	if !entryByID(cm.Menus["minutes"], wantA) {
		t.Errorf("missing team_alpha parent: %+v", cm.Menus["minutes"])
	}
	if !entryByID(cm.Menus["minutes"], wantB) {
		t.Errorf("missing team_beta parent: %+v", cm.Menus["minutes"])
	}
	docCount := 0
	for _, e := range cm.Menus["minutes"] {
		if e.PageRef == "/shared/" {
			docCount++
			if e.Parent != wantA && e.Parent != wantB {
				t.Errorf("doc parent = %q, want one of {%q, %q}",
					e.Parent, wantA, wantB)
			}
		}
	}
	if docCount != 2 {
		t.Errorf("doc appears %d times in menu, want 2 (once per project value)", docCount)
	}
}

// TestBuildCategoriesMenu_MultiValueFirstSeenSpelling: the first-
// seen spelling rule applies within a list. Two values that differ
// only in case collapse to one bucket labeled with the first-seen
// spelling.
func TestBuildCategoriesMenu_MultiValueFirstSeenSpelling(t *testing.T) {
	repos := []config.Repository{{Name: "project_a"}}
	docs := []categoryDoc{
		{
			Repository: "project_a", Title: "first", Path: "/first/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "Team-Alpha"},
		},
		{
			Repository: "project_a", Title: "second", Path: "/second/",
			Categories: []string{"minutes"}, CategoryDisplay: "Minutes",
			GroupFields: map[string]string{"projects": "team-alpha"},
		},
	}
	cm, err := buildCategoriesMenu(docs, repos, "repo", map[string][]string{
		"minutes": {"projects"},
	})
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	wantID := categoryParentIdentifier("minutes", "team-alpha")
	matches := 0
	for _, e := range cm.Menus["minutes"] {
		if e.Identifier == wantID {
			matches++
			if e.Name != "Team-Alpha" && e.Name != "team-alpha" {
				t.Errorf("level-1 label = %q, want one of {Team-Alpha, team-alpha}",
					e.Name)
			}
		}
	}
	if matches != 1 {
		t.Errorf("expected exactly 1 level-1 node; got %d", matches)
	}
}
