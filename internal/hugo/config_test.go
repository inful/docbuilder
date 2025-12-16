package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func readYaml(t *testing.T, path string) map[string]any {
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
	if err := gen.generateHugoConfig(); err != nil {
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
	if err := gen.generateHugoConfig(); err != nil {
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
