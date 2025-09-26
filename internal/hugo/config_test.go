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

func TestGenerateHugoConfig_Docsy(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Title: "Docsy Site", Theme: "docsy"}}
	gen := NewGenerator(cfg, out)
	if err := gen.generateHugoConfig(); err != nil {
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
		if m, ok := im.(map[string]any); ok && m["path"] == "github.com/google/docsy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("docsy module import missing: %v", imports)
	}
	outs := conf["outputs"].(map[string]any)
	home := outs["home"].([]any)
	hasJSON := false
	for _, v := range home {
		if v == "JSON" {
			hasJSON = true
		}
	}
	if !hasJSON {
		t.Fatalf("docsy home outputs missing JSON: %v", home)
	}
}

func TestGenerateHugoConfig_Hextra(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Title: "Hextra Site", Theme: "hextra"}}
	gen := NewGenerator(cfg, out)
	if err := gen.generateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	conf := readYaml(t, filepath.Join(out, "hugo.yaml"))
	mod, ok := conf["module"].(map[string]any)
	if !ok {
		t.Fatalf("expected module imports for hextra")
	}
	imports := mod["imports"].([]any)
	found := false
	for _, im := range imports {
		if m, ok := im.(map[string]any); ok && m["path"] == "github.com/imfing/hextra" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hextra module import missing: %v", imports)
	}
	if _, ok := conf["outputs"]; ok {
		t.Fatalf("hextra should not configure outputs.home JSON by default")
	}
	markup := conf["markup"].(map[string]any)
	gold := markup["goldmark"].(map[string]any)
	ext := gold["extensions"].(map[string]any)
	if _, ok := ext["passthrough"]; !ok {
		t.Fatalf("expected math passthrough extension for hextra")
	}
}
