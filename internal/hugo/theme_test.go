package hugo

import (
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGenerateHugoConfig_DocsyHasNoSidebarMenus checks that the
// The configuration carries no sidebar menus: the theme's
// navigation is the content tree, and the categories stay a taxonomy.
func TestGenerateHugoConfig_DocsyHasNoSidebarMenus(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Title: "Test", BaseURL: "/",
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
		t.Errorf("the config must not carry sidebar menus: %v", params["sidebarmenus"])
	}
	if _, has := params["themeVariant"]; has {
		t.Errorf("the config must not carry theme variants: %v", params["themeVariant"])
	}
}

// TestGenerateHugoConfig_DocsyHasNoSharedNavigation checks that the
// Docsy configuration does not publish the shared navigation
// fragment: Docsy renders its own navigation per page.
func TestGenerateHugoConfig_DocsyHasNoSharedNavigation(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
		Title: "Test", BaseURL: "/",
	}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	if _, ok := conf["outputFormats"]; ok {
		t.Errorf("the docsy config must not define the shared navigation output: %v", conf["outputFormats"])
	}
}
