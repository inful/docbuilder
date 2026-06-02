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
	if got, want := len(docMenu), 6; got != want {
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
	if first["identifier"] != "cat-Documentation" {
		t.Fatalf("first sidebar block identifier = %v, want cat-Documentation", first["identifier"])
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

// TestHugoConfigGolden_DefaultModeNoCategoriesMenu is a regression
// guard: when the sidebar mode is unset (the default), the
// categories-menu stage must be a no-op and hugo.yaml must not
// include any sidebarmenus or Documentation menu entries.
func TestHugoConfigGolden_DefaultModeNoCategoriesMenu(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title: "Default Site",
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
		t.Fatalf("default mode must not emit a categories menu; got:\n%s", data)
	}
	if strings.Contains(string(data), "sidebarmenus:") {
		t.Fatalf("default mode must not emit sidebarmenus; got:\n%s", data)
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
