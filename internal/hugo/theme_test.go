package hugo

import (
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGenerateHugoConfig_DocsyModuleImport checks that the Docsy
// theme is imported as the module it is: the theme lives in the
// "theme" subdirectory of the Docsy repository and is its own Go
// module.
func TestGenerateHugoConfig_DocsyModuleImport(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Title: "Test", BaseURL: "/", Theme: config.ThemeDocsy,
	}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	mod, ok := conf["module"].(map[string]any)
	if !ok {
		t.Fatalf("expected module imports for docsy")
	}
	imports := mod["imports"].([]any)
	found := false
	for _, im := range imports {
		m, ok := im.(map[string]any)
		if !ok {
			continue
		}
		if m["path"] == "github.com/docsy/docsy/theme" {
			found = true
			if m["version"] != "v0.18.1" {
				t.Errorf("docsy version = %v, want v0.18.1", m["version"])
			}
		}
	}
	if !found {
		t.Fatalf("docsy module import missing: %v", imports)
	}
}

// TestGenerateHugoConfig_DocsyHasNoSidebarMenus checks that the
// Docsy configuration carries no Relearn sidebar menus: Docsy's
// navigation is the content tree, and the categories stay a taxonomy.
func TestGenerateHugoConfig_DocsyHasNoSidebarMenus(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Title: "Test", BaseURL: "/", Theme: config.ThemeDocsy,
	}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	params, ok := conf["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params for docsy")
	}
	if _, has := params["sidebarmenus"]; has {
		t.Errorf("the docsy config must not carry Relearn sidebar menus: %v", params["sidebarmenus"])
	}
	if _, has := params["themeVariant"]; has {
		t.Errorf("the docsy config must not carry Relearn theme variants: %v", params["themeVariant"])
	}
}

// TestGenerateHugoConfig_DocsyHasNoSharedNavigation checks that the
// Docsy configuration does not publish the shared navigation
// fragment: Docsy renders its own navigation per page.
func TestGenerateHugoConfig_DocsyHasNoSharedNavigation(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Title: "Test", BaseURL: "/", Theme: config.ThemeDocsy,
	}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	if _, ok := conf["outputFormats"]; ok {
		t.Errorf("the docsy config must not define the shared navigation output: %v", conf["outputFormats"])
	}
}
