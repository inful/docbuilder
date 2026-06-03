package hugo

import (
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

	cm, err := buildCategoriesMenu(docs, repos, "repo")
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	if cm == nil {
		t.Fatal("buildCategoriesMenu returned nil")
	}

	// The Documentation menu must exist.
	docMenu, ok := cm.Menus["Documentation"]
	if !ok {
		t.Fatalf("expected a Documentation menu, got keys: %v", keys(cm.Menus))
	}

	// The menu must contain one parent entry per project (so the
	// tree groups children under their repo) and one child entry per
	// doc. Total = 2 parents + 4 children = 6.
	if got, want := len(docMenu), 6; got != want {
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
	if cm.SidebarEntries[0].Identifier != "cat-Documentation" {
		t.Fatalf("sidebar entry identifier = %q, want %q",
			cm.SidebarEntries[0].Identifier, "cat-Documentation")
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
	cm, err := buildCategoriesMenu(docs, repos, "repo")
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	if _, ok := cm.Menus["Docs"]; !ok {
		t.Fatalf("expected Docs menu, got: %v", keys(cm.Menus))
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
	cm, err := buildCategoriesMenu(nil, nil, "repo")
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
	cm, err := buildCategoriesMenu(docs, repos, "repo")
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	entry := cm.Menus["Reference"][0]
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
	cm, err := buildCategoriesMenu(docs, repos, "repo")
	if err != nil {
		t.Fatalf("buildCategoriesMenu: %v", err)
	}
	// Both keys must be present.
	if _, ok := cm.Menus["Documentation"]; !ok {
		t.Fatalf("expected Documentation menu; got keys: %v", keys(cm.Menus))
	}
	if _, ok := cm.Menus[UncategorizedCategory]; !ok {
		t.Fatalf("expected %q menu; got keys: %v", UncategorizedCategory, keys(cm.Menus))
	}
	// Documentation: only the categorized doc.
	for _, e := range cm.Menus["Documentation"] {
		if e.Name == "Orphan" {
			t.Fatalf("Orphan must not appear in Documentation menu")
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
	cm, err := buildCategoriesMenu(docs, repos, "repo")
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
	got, err := readCategoriesMenuDocs(nil, false)
	if err != nil {
		t.Fatalf("readCategoriesMenuDocs(nil): %v", err)
	}
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
	got, err := readCategoriesMenuDocs(files, false)
	if err != nil {
		t.Fatalf("readCategoriesMenuDocs: %v", err)
	}
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
