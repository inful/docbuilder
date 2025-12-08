package hugo

import (
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestUnknownThemeFallback ensures that specifying an unsupported theme string results
// in a successful config generation with minimal defaults (no panic, no module block).
func TestUnknownThemeFallback(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.Hugo.Title = "Test"
	cfg.Hugo.Description = "Desc"
	cfg.Hugo.BaseURL = "http://example.test/"
	cfg.Hugo.Theme = "some-future-theme" // unknown

	g := NewGenerator(cfg, filepath.Join(tmp, "out"))
	g.stageDir = filepath.Join(tmp, "stage")
	if err := os.MkdirAll(g.stageDir, 0o750); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}
	if err := g.generateHugoConfig(); err != nil {
		t.Fatalf("generate config: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(g.buildRoot(), "hugo.yaml"))
	if err != nil {
		t.Fatalf("read hugo.yaml: %v", err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatalf("empty hugo.yaml for unknown theme")
	}
	if contains(content, "module:") {
		t.Fatalf("unexpected module block for unknown theme:\n%s", content)
	}
}

// contains is a small helper avoiding bringing in strings for a single call; duplicated intentionally for test locality.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool { return stringIndex(s, sub) >= 0 })()
}

// stringIndex reimplements strings.Index minimally (naive) to reduce imports in this small test file.
func stringIndex(s, sep string) int {
	if len(sep) == 0 {
		return 0
	}
	outer := len(s) - len(sep)
	for i := 0; i <= outer; i++ {
		if s[i] == sep[0] {
			match := true
			for j := 1; j < len(sep); j++ {
				if s[i+j] != sep[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}
