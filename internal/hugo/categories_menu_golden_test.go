package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
)

// docWithFrontMatter builds a docs.DocFile with the given in-memory
// content, so the categories-menu stage can read categories from
// f.Content via frontmatterops.
func docWithFrontMatter(repo, name, content string) docs.DocFile {
	rel := name + ".md"
	return docs.DocFile{
		Path:         filepath.Join("/tmp/src", repo, "docs", rel),
		RelativePath: "docs/" + rel,
		DocsBase:     "docs",
		Repository:   repo,
		Section:      "",
		Name:         name,
		Extension:    ".md",
		Content:      []byte(content),
		IsAsset:      false,
	}
}

// TestHugoConfigGolden_CategoriesMode builds a DocFile set that
// reproduces the user's real scenario (two repos, both with intro.md
// titled "Introduction" and category "Documentation") and asserts the
// emitted hugo.yaml wires the categories menus correctly so the
// Relearn sidebar renders them as a three-level tree (Category >
// Project > Document) with no colliding link text.
func TestHugoConfigGolden_CategoriesMode(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Aggregator Site",
			Sidebar: &config.SidebarConfig{
				Mode: config.SidebarModeCategories,
			},
		},
		Repositories: []config.Repository{
			{Name: "project_a", DisplayName: "Project A"},
			{Name: "project_b", DisplayName: "Project B"},
		},
	}
	g := NewGenerator(cfg, out)

	// Build docs that carry front matter the categories-menu stage can
	// read from DocFile.Content.
	docA1 := docWithFrontMatter("project_a", "intro", "---\ntitle: Introduction\ncategories: [Documentation]\n---\n\n# Introduction\n")
	docA2 := docWithFrontMatter("project_a", "setup", "---\ntitle: Setup\ncategories: [Documentation]\n---\n\n# Setup\n")
	docB1 := docWithFrontMatter("project_b", "intro", "---\ntitle: Introduction\ncategories: [Documentation]\n---\n\n# Introduction\n")
	docB2 := docWithFrontMatter("project_b", "config", "---\ntitle: Configuration\ncategories: [Documentation]\n---\n\n# Configuration\n")

	// Build the categories menu from the docs.
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{
			Files:        []docs.DocFile{docA1, docA2, docB1, docB2},
			IsSingleRepo: false,
		},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	if cm == nil {
		t.Fatal("expected non-nil categories menu in categories mode")
	}
	g.AttachCategoriesMenu(cm)

	// Emit hugo.yaml.
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}

	// Parse the output and assert the key structures.
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read hugo.yaml: %v", err)
	}
	var parsed map[string]any
	if unmarshalErr := yaml.Unmarshal(data, &parsed); unmarshalErr != nil {
		t.Fatalf("unmarshal hugo.yaml: %v", unmarshalErr)
	}

	// 1) The Documentation menu exists and is keyed at root.menu.Documentation.
	menus, ok := parsed["menu"].(map[string]any)
	if !ok {
		t.Fatalf("expected root.menu to be a map; got %T (yaml:\n%s)", parsed["menu"], data)
	}
	docMenu, isList := menus["Documentation"].([]any)
	if !isList {
		t.Fatalf("expected menu.Documentation to be a list; got %T (yaml:\n%s)",
			menus["Documentation"], data)
	}
	if got, want := len(docMenu), 7; got != want {
		t.Fatalf("Documentation menu length = %d, want %d; got: %v", got, want, docMenu)
	}

	// 2) Each project has a parent entry with no URL/pageRef (Relearn
	// renders it as a non-clickable expandable header) and child
	// entries that reference that parent.
	introParents := map[string]string{}
	for _, raw := range docMenu {
		entry, isMap := raw.(map[string]any)
		if !isMap {
			t.Fatalf("expected menu entry to be a map; got %T", raw)
		}
		if name, _ := entry["name"].(string); name == "Introduction" {
			parent, _ := entry["parent"].(string)
			if parent == "" {
				t.Fatalf("Introduction entry must have a parent; entry: %+v", entry)
			}
			if prev, dup := introParents[parent]; dup {
				t.Fatalf("two Introduction entries share parent %q (this is the collision case)", prev)
			}
			introParents[parent] = parent
		}
	}
	if len(introParents) != 2 {
		t.Fatalf("expected two distinct Introduction entries with distinct parents; got: %v", introParents)
	}

	// 3) The sidebarmenus block has a Documentation entry of type
	// "menu" followed by the default "main" page menu.
	params, ok := parsed["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected root.params to be a map")
	}
	sidebars, ok := params["sidebarmenus"].([]any)
	if !ok {
		t.Fatalf("expected params.sidebarmenus to be a list; got %T (yaml:\n%s)",
			params["sidebarmenus"], data)
	}
	if len(sidebars) < 2 {
		t.Fatalf("expected at least 2 sidebarmenus blocks (Documentation + main), got %d: %v",
			len(sidebars), sidebars)
	}
	first := sidebars[0].(map[string]any)
	if first["type"] != "menu" {
		t.Fatalf("first sidebar block should be type=menu; got %v", first)
	}
	if first["identifier"] != "Documentation" {
		t.Fatalf("first sidebar block identifier = %v, want Documentation", first["identifier"])
	}
	// The last block must be the default main page menu.
	last := sidebars[len(sidebars)-1].(map[string]any)
	if last["identifier"] != "main" || last["type"] != "page" {
		t.Fatalf("last sidebar block should be the default main page menu; got %v", last)
	}
}

// TestHugoConfigGolden_CategoriesModeNoKeepDefault asserts the user
// can opt out of the default main page menu.
func TestHugoConfigGolden_CategoriesModeNoKeepDefault(t *testing.T) {
	out := t.TempDir()
	f := false
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Aggregator Site",
			Sidebar: &config.SidebarConfig{
				Mode:            config.SidebarModeCategories,
				KeepDefaultMenu: &f,
			},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	doc := docWithFrontMatter("project_a", "intro",
		"---\ntitle: Introduction\ncategories: [Documentation]\n---\n\n# Introduction\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{
			Files:        []docs.DocFile{doc},
			IsSingleRepo: false,
		},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	g.AttachCategoriesMenu(cm)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read hugo.yaml: %v", err)
	}
	if bytes.Contains(data, []byte("identifier: main")) {
		t.Fatalf("expected no default main page menu when KeepDefaultMenu=false; got:\n%s", data)
	}
}

// TestHugoConfigGolden_ExplicitDefaultModeOptOut is a regression
// guard for the explicit opt-out: when hugo.sidebar.mode is set to
// "default", DocBuilder must NOT emit the categories sidebar even if
// the build contains categorized docs. The docs remain reachable
// through the default main page menu.
func TestHugoConfigGolden_ExplicitDefaultModeOptOut(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Opted-out Site",
			Sidebar: &config.SidebarConfig{
				Mode: config.SidebarModeDefault,
			},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "cat-Documentation") {
		t.Fatalf("explicit default mode must not emit a categories menu; got:\n%s", data)
	}
	if strings.Contains(string(data), "sidebarmenus:") {
		t.Fatalf("explicit default mode must not emit sidebarmenus; got:\n%s", data)
	}
}

// TestHugoConfigGolden_UnsetModeWithUntaggedDocsEmitsUncategorizedSidebar
// locks down the new default behavior: when hugo.sidebar.mode is unset
// (the new default is "auto") and the build contains only un-categorized
// docs, the categories sidebar must still be emitted, with the
// synthetic _uncategorized block as its sole entry.
func TestHugoConfigGolden_UnsetModeWithUntaggedDocsEmitsUncategorizedSidebar(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Aggregator Site",
			// Sidebar: nil — the new default is auto, which routes
			// to categories whenever the build has any docs.
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	// Doc with no categories front matter.
	doc := docWithFrontMatter("project_a", "orphan",
		"---\ntitle: Orphan\n---\n\n# Orphan\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{doc}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	if cm == nil {
		t.Fatal("expected non-nil categories menu in auto mode with untagged docs")
	}
	g.AttachCategoriesMenu(cm)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	parsed, perr := parseHugoYAML(data)
	if perr != nil {
		t.Fatalf("parse: %v", perr)
	}
	// The synthetic _uncategorized menu must exist.
	menus, ok := parsed["menu"].(map[string]any)
	if !ok {
		t.Fatalf("expected root.menu; got: %v", parsed["menu"])
	}
	if _, ok := menus[slugForIdentifier(config.SidebarUncategorizedCategory)]; !ok {
		t.Fatalf("expected menu[%q]; got: %v", slugForIdentifier(config.SidebarUncategorizedCategory), keysMenu(menus))
	}
	// The sidebar block must carry the high weight.
	sidebars, _ := parsed["params"].(map[string]any)["sidebarmenus"].([]any)
	if len(sidebars) == 0 {
		t.Fatalf("expected at least one sidebar block; got: %v", sidebars)
	}
	first := sidebars[0].(map[string]any)
	if first["type"] != "menu" {
		t.Fatalf("first sidebar block should be type=menu; got: %v", first)
	}
	if w, _ := first["weight"].(int); w == 0 {
		t.Fatalf("expected weight on synthetic block; got: %v", first)
	}
}

// TestHugoConfigGolden_UnsetModeWithEmptyBuildLogsAndFallsBack locks
// down the truly-empty-build path. With no docs at all, the
// categories-menu stage must log a one-line INFO and fall back to
// the default sidebar (no _uncategorized, no sidebarmenus).
func TestHugoConfigGolden_UnsetModeWithEmptyBuildLogsAndFallsBack(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Empty Site",
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	// No docs in the build state. computeCategoriesMenu returns
	// ErrCategoriesMenuSkipped; the config writer produces no
	// categories menu and no sidebarmenus.
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: nil},
	})
	if err == nil {
		t.Fatal("expected ErrCategoriesMenuSkipped for empty build")
	}
	if cm != nil {
		t.Fatalf("expected nil cm; got: %+v", cm)
	}
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), "cat-") {
		t.Fatalf("empty build must not emit any categories sidebar; got:\n%s", data)
	}
	if strings.Contains(string(data), "sidebarmenus:") {
		t.Fatalf("empty build must not emit sidebarmenus; got:\n%s", data)
	}
}

// TestSidebarConfig_IsCategories_TruthTable is a table-driven test
// covering the (mode, expected) pairs that drive the new default
// behavior. Locks the config-layer decision.
func TestSidebarConfig_IsCategories_TruthTable(t *testing.T) {
	trues := []config.SidebarMode{
		"", // zero value, including nil-config case
		config.SidebarModeAuto,
		config.SidebarModeCategories,
	}
	falses := []config.SidebarMode{
		config.SidebarModeDefault,
		"unknown-mode-xyz",
	}
	for _, m := range trues {
		s := &config.SidebarConfig{Mode: m}
		if !s.IsCategories() {
			t.Errorf("IsCategories() = false for mode %q; want true", m)
		}
	}
	for _, m := range falses {
		s := &config.SidebarConfig{Mode: m}
		if s.IsCategories() {
			t.Errorf("IsCategories() = true for mode %q; want false", m)
		}
	}
	// Nil config also returns true (the new default is "auto", and
	// nil is the zero-value state that should route to the default).
	var nilCfg *config.SidebarConfig
	if !nilCfg.IsCategories() {
		t.Error("IsCategories() = false for nil config; want true (the default)")
	}
}

// TestHugoConfigGolden_UncategorizedWeighedLastInSidebar asserts
// that when both user categories and the synthetic _uncategorized
// block are emitted, the synthetic block carries weight:999 and
// appears at the END of the sidebarmenus list.
func TestHugoConfigGolden_UncategorizedWeighedLastInSidebar(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Multi-Category Site",
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	docA := docWithFrontMatter("project_a", "alpha",
		"---\ntitle: Alpha\ncategories: [Documentation]\n---\n\n# Alpha\n")
	docB := docWithFrontMatter("project_a", "ops",
		"---\ntitle: Ops\ncategories: [Operations]\n---\n\n# Ops\n")
	docC := docWithFrontMatter("project_a", "orphan",
		"---\ntitle: Orphan\n---\n\n# Orphan\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{docA, docB, docC}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	g.AttachCategoriesMenu(cm)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	parsed, perr := parseHugoYAML(data)
	if perr != nil {
		t.Fatalf("parse: %v", perr)
	}
	sidebars, _ := parsed["params"].(map[string]any)["sidebarmenus"].([]any)
	if len(sidebars) < 2 {
		t.Fatalf("expected at least 2 sidebar blocks (Documentation, Operations, _uncategorized, main); got %d: %v",
			len(sidebars), sidebars)
	}
	// Find the index of the _uncategorized block and the "main" block.
	uncIdx, mainIdx := -1, -1
	for i, e := range sidebars {
		m := e.(map[string]any)
		id, _ := m["identifier"].(string)
		if id == slugForIdentifier(config.SidebarUncategorizedCategory) {
			uncIdx = i
			if w, _ := m["weight"].(int); w != uncategorizedSidebarWeightValue {
				t.Fatalf("_uncategorized block weight = %d, want %d", w, uncategorizedSidebarWeightValue)
			}
		}
		if id == "main" {
			mainIdx = i
		}
	}
	if uncIdx == -1 {
		t.Fatalf("_uncategorized sidebar block not found; got: %v", sidebars)
	}
	if mainIdx == -1 {
		t.Fatalf("main sidebar block not found; got: %v", sidebars)
	}
	// With keep_default_menu true, the default main block sits at the
	// end. The _uncategorized block sits at the end of the
	// category-menus portion, just before the main block.
	if uncIdx >= mainIdx {
		t.Fatalf("_uncategorized block (idx %d) must appear before the main block (idx %d) in sidebarmenus",
			uncIdx, mainIdx)
	}
}

// TestHugoConfigGolden_CategoriesModePreservesUserShortcuts asserts
// that a user-defined `hugo.menu.shortcuts` is preserved and the
// corresponding sidebar block is emitted after the category menus
// and the main page menu.
func TestHugoConfigGolden_CategoriesModePreservesUserShortcuts(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Site",
			Sidebar: &config.SidebarConfig{
				Mode: config.SidebarModeCategories,
			},
			Menu: map[string][]config.Menu{
				"shortcuts": {{Name: "GitHub", URL: "https://github.com/org", Weight: 1}},
			},
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	doc := docWithFrontMatter("project_a", "intro",
		"---\ntitle: Introduction\ncategories: [Documentation]\n---\n\n# Introduction\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{doc}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	g.AttachCategoriesMenu(cm)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	// The shortcuts entry must survive.
	if !strings.Contains(s, "GitHub") {
		t.Fatalf("expected user-defined shortcuts entry to be preserved; got:\n%s", s)
	}
	// The shortcuts sidebar block must be present.
	if !strings.Contains(s, "identifier: shortcuts") {
		t.Fatalf("expected a shortcuts sidebar block; got:\n%s", s)
	}
}

// TestHugoConfigGolden_CategoriesMenuKeyMatchesSidebarIdentifier is
// the regression test for the bug where the categories menu was
// emitted in hugo.yaml but did not appear in the rendered
// navigation. Root cause: the Relearn sidebar template looks up the
// menu via `index site.Menus $config.identifier`, so the menu key
// in `menu:` and the sidebar block's `identifier` field MUST be
// identical. This test asserts that alignment for every emitted
// category menu.
func TestHugoConfigGolden_CategoriesMenuKeyMatchesSidebarIdentifier(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Alignment Test",
			// Two user categories plus the synthetic _uncategorized.
		},
		Repositories: []config.Repository{{Name: "project_a"}},
	}
	g := NewGenerator(cfg, out)
	docA := docWithFrontMatter("project_a", "alpha",
		"---\ntitle: Alpha\ncategories: [Documentation]\n---\n\n# Alpha\n")
	docB := docWithFrontMatter("project_a", "ops",
		"---\ntitle: Ops\ncategories: [Operations]\n---\n\n# Ops\n")
	docC := docWithFrontMatter("project_a", "orphan",
		"---\ntitle: Orphan\n---\n\n# Orphan\n")
	cm, err := g.computeCategoriesMenu(&models.BuildState{
		Docs: models.DocsState{Files: []docs.DocFile{docA, docB, docC}},
	})
	if err != nil {
		t.Fatalf("computeCategoriesMenu: %v", err)
	}
	g.AttachCategoriesMenu(cm)
	if genErr := g.GenerateHugoConfig(); genErr != nil {
		t.Fatalf("GenerateHugoConfig: %v", genErr)
	}
	// #nosec G304 -- test reading from t.TempDir() output
	data, err := os.ReadFile(filepath.Join(out, "hugo.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	parsed, perr := parseHugoYAML(data)
	if perr != nil {
		t.Fatalf("parse: %v", perr)
	}
	menus, ok := parsed["menu"].(map[string]any)
	if !ok {
		t.Fatalf("expected root.menu; got: %v", parsed["menu"])
	}
	sidebars, ok := parsed["params"].(map[string]any)["sidebarmenus"].([]any)
	if !ok {
		t.Fatalf("expected sidebarmenus; got: %v", parsed["params"])
	}
	// Build a set of sidebarmenus identifiers (only menu-type
	// blocks, not page-type).
	sidebarIDs := map[string]bool{}
	for _, s := range sidebars {
		m := s.(map[string]any)
		if m["type"] == "menu" {
			id, _ := m["identifier"].(string)
			sidebarIDs[id] = true
		}
	}
	// For every Hugo menu key emitted by the categories menu,
	// there must be a matching sidebar identifier.
	for menuKey := range menus {
		// Skip user-defined menus (e.g. "shortcuts") that the
		// user supplied outside of our categories-menu wiring.
		// The categories menu is responsible only for its own
		// emitted keys. We identify them by checking the
		// associated sidebar block.
		if !sidebarIDs[menuKey] {
			t.Errorf("menu key %q has no matching sidebar block (sidebarIDs: %v)",
				menuKey, sidebarIDs)
		}
	}
	// And vice versa: every menu-type sidebar block must point
	// at a real menu key.
	for id := range sidebarIDs {
		if _, ok := menus[id]; !ok {
			t.Errorf("sidebar block identifier %q has no matching menu key (menus: %v)",
				id, keysMenu(menus))
		}
	}
}

// parseHugoYAML is a small test helper that parses a Hugo YAML file
// into a generic map. The Golden test code uses it instead of
// re-implementing the unmarshal + cast dance.
func parseHugoYAML(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func keysMenu(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
