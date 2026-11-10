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

func TestGenerateHugoConfig_RelearnModuleImport(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	mod, ok := conf["module"].(map[string]any)
	if !ok {
		t.Fatalf("expected module imports for relearn")
	}
	imports := mod["imports"].([]any)
	found := false
	for _, im := range imports {
		if m, ok := im.(map[string]any); ok && m["path"] == "github.com/McShelby/hugo-theme-relearn" {
			found = true
		}
	}
	if !found {
		t.Fatalf("relearn module import missing: %v", imports)
	}
}

func TestGenerateHugoConfig_RelearnParams(t *testing.T) {
	out := t.TempDir()
	gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, out)
	if err := gen.GenerateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	params, ok := conf["params"].(map[string]any)
	if !ok {
		t.Fatalf("expected params for relearn")
	}
	// Verify Relearn-specific params are set
	if _, ok := params["themeVariant"]; !ok {
		t.Errorf("expected themeVariant param")
	}
	if _, ok := params["collapsibleMenu"]; !ok {
		t.Errorf("expected collapsibleMenu param")
	}
	if _, ok := params["showVisitedLinks"]; !ok {
		t.Errorf("expected showVisitedLinks param")
	}
}

// TestGenerateHugoConfig_SharedSidebarRegistersFragment pins that the
// shared sidebar mode registers the output format that carries the
// navigation fragment, and that the default mode does not.
func TestGenerateHugoConfig_SharedSidebarRegistersFragment(t *testing.T) {
	generate := func(t *testing.T, mode config.SidebarMode) map[string]any {
		t.Helper()
		out := t.TempDir()
		gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
			Title:   "Test",
			BaseURL: "/",
			Sidebar: &config.SidebarConfig{Mode: mode},
		}}, out)
		if err := gen.GenerateHugoConfig(); err != nil {
			t.Fatalf("generate config: %v", err)
		}
		return readYaml(t, filepath.Join(out, "hugo.yaml"))
	}

	t.Run("shared", func(t *testing.T) {
		conf := generate(t, config.SidebarModeShared)

		formats, ok := conf["outputFormats"].(map[string]any)
		if !ok {
			t.Fatalf("expected outputFormats in the shared sidebar config; got %v", conf)
		}
		if _, hasFormat := formats[SharedNavOutputFormat]; !hasFormat {
			t.Errorf("missing %q output format: %v", SharedNavOutputFormat, formats)
		}

		outputs, ok := conf["outputs"].(map[string]any)
		if !ok {
			t.Fatalf("expected outputs in the shared sidebar config; got %v", conf)
		}
		home, ok := outputs["home"].([]any)
		if !ok {
			t.Fatalf("expected home outputs; got %v", outputs)
		}
		found := false
		for _, o := range home {
			if o == SharedNavOutputFormat {
				found = true
			}
		}
		if !found {
			t.Errorf("home outputs do not include %q: %v", SharedNavOutputFormat, home)
		}
	})

	t.Run("default", func(t *testing.T) {
		conf := generate(t, config.SidebarModeAuto)

		if _, ok := conf["outputFormats"]; ok {
			t.Errorf("default mode must not define output formats: %v", conf["outputFormats"])
		}
		outputs, _ := conf["outputs"].(map[string]any)
		home, _ := outputs["home"].([]any)
		for _, o := range home {
			if o == SharedNavOutputFormat {
				t.Errorf("default mode must not publish the %q output: %v", SharedNavOutputFormat, home)
			}
		}
	})
}
