package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func readYaml(t *testing.T, path string) map[string]any {
	t.Helper()
	// #nosec G304 -- test utility reading from testdata directory
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestGenerateHugoConfig_DocsyModuleImport(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	mod, ok := conf["module"].(map[string]any)
	if !ok {
		t.Fatalf("expected module imports for the theme")
	}
	imports := mod["imports"].([]any)
	found := false
	for _, im := range imports {
		if m, ok := im.(map[string]any); ok && m["path"] == "github.com/docsy/docsy/theme" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docsy module import missing: %v", imports)
	}
}

func TestGenerateHugoConfig_ThemeParams(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	params, ok := conf["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params for the theme")
	}
	if _, has := params["offlineSearch"]; !has {
		t.Errorf("expected the offline search to be enabled")
	}
	ui, ok := params["ui"].(map[string]any)
	if !ok {
		t.Fatalf("expected the ui params; got %v", params["ui"])
	}
	if _, ok := ui["showLightDarkModeMenu"]; !ok {
		t.Errorf("expected the light/dark menu to be enabled")
	}
	if _, ok := ui["sidebar_menu_foldable"]; !ok {
		t.Errorf("expected the sidebar to fold")
	}
}

// TestGenerateHugoConfig_ChromeMode checks that the shared chrome is
// opt-in: the theme's own default renders the navigation on every
// page.
func TestGenerateHugoConfig_ChromeMode(t *testing.T) {
	generate := func(t *testing.T, chrome string) map[string]any {
		t.Helper()
		out := t.TempDir()
		gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
			Title:  "Test",
			Chrome: chrome,
		}}, out)
		if err := gen.GenerateHugoConfig(); err != nil {
			t.Fatalf("generate config: %v", err)
		}
		return readYaml(t, filepath.Join(out, "hugo.yaml"))
	}

	t.Run("full by default", func(t *testing.T) {
		conf := generate(t, "")
		params := conf["params"].(map[string]any)
		if _, ok := params["td"]; ok {
			t.Errorf("the default must not share the chrome: %v", params["td"])
		}
	})

	t.Run("shared on request", func(t *testing.T) {
		conf := generate(t, config.ChromeShared)
		params := conf["params"].(map[string]any)
		td, ok := params["td"].(map[string]any)
		if !ok {
			t.Fatalf("expected the td params; got %v", params["td"])
		}
		if td["chrome"] != config.ChromeShared {
			t.Errorf("chrome = %v, want %v", td["chrome"], config.ChromeShared)
		}
	})
}
