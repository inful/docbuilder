package hugo

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
