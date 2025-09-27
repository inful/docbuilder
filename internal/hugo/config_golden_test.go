package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"gopkg.in/yaml.v3"
)

// normalizeConfig removes volatile fields (dates) and sorts maps for stable serialization.
func normalizeConfig(t *testing.T, path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read: %v", err) }
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil { t.Fatalf("unmarshal: %v", err) }
	if params, ok := m["params"].(map[string]any); ok {
		if _, exists := params["build_date"]; exists { params["build_date"] = "IGNORE" }
	}
	out, err := yaml.Marshal(m)
	if err != nil { t.Fatalf("marshal: %v", err) }
	return out
}

func TestHugoConfigGolden_Hextra(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Title: "Hextra Site", Theme: "hextra"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
	g := NewGenerator(cfg, out)
	if err := g.generateHugoConfig(); err != nil { t.Fatalf("generate: %v", err) }
	actual := normalizeConfig(t, filepath.Join(out, "hugo.yaml"))
	golden := filepath.Join("testdata", "hugo_config", "hextra.yaml")
	want, err := os.ReadFile(golden)
	if err != nil { t.Fatalf("read golden: %v", err) }
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(actual)) {
		// write diff friendly output
		writeMismatch(t, want, actual)
		// update mode via env var
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(golden, actual, 0644); err != nil { t.Fatalf("update golden: %v", err) }
			return
		}
		// fail after optional update attempt
		 t.Fatalf("hextra hugo.yaml mismatch; run UPDATE_GOLDEN=1 go test ./internal/hugo -run TestHugoConfigGolden_Hextra to accept")
	}
}

func TestHugoConfigGolden_Docsy(t *testing.T) {
	out := t.TempDir()
	cfg := &config.V2Config{Hugo: config.HugoConfig{Title: "Docsy Site", Theme: "docsy"}, Repositories: []config.Repository{{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}}}}
	g := NewGenerator(cfg, out)
	if err := g.generateHugoConfig(); err != nil { t.Fatalf("generate: %v", err) }
	actual := normalizeConfig(t, filepath.Join(out, "hugo.yaml"))
	golden := filepath.Join("testdata", "hugo_config", "docsy.yaml")
	want, err := os.ReadFile(golden)
	if err != nil { t.Fatalf("read golden: %v", err) }
	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(actual)) {
		writeMismatch(t, want, actual)
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(golden, actual, 0644); err != nil { t.Fatalf("update golden: %v", err) }
			return
		}
		 t.Fatalf("docsy hugo.yaml mismatch; run UPDATE_GOLDEN=1 go test ./internal/hugo -run TestHugoConfigGolden_Docsy to accept")
	}
}

// writeMismatch writes a simple diff-ish output to help debugging mismatches.
func writeMismatch(t *testing.T, want, got []byte) {
	// naive line diff for brevity
	wantLines := lines(string(want))
	gotLines := lines(string(got))
	// collect into deterministic sets for quick visual (order matters in YAML but we just show extras/missing)
	wantSet := map[string]struct{}{}
	gotSet := map[string]struct{}{}
	for _, l := range wantLines { wantSet[l] = struct{}{} }
	for _, l := range gotLines { gotSet[l] = struct{}{} }
	var missing, extra []string
	for l := range wantSet { if _, ok := gotSet[l]; !ok { missing = append(missing, l) } }
	for l := range gotSet { if _, ok := wantSet[l]; !ok { extra = append(extra, l) } }
	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing); sort.Strings(extra)
		 t.Logf("missing lines: %v", missing)
		 t.Logf("extra lines: %v", extra)
	}
}

func lines(s string) []string {
	var out []string
	start := 0
	for i, c := range s { if c == '\n' { out = append(out, s[start:i]); start = i+1 } }
	if start < len(s) { out = append(out, s[start:]) }
	return out
}
