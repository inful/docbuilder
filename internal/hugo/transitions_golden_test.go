package hugo

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestHugoConfigGolden_Transitions(t *testing.T) {
	out := t.TempDir()
	cfg := &config.Config{
		Hugo: config.HugoConfig{
			Title:              "Transitions Test Site",
			Theme:              "hextra",
			EnableTransitions:  true,
			TransitionDuration: "400ms",
		},
		Repositories: []config.Repository{
			{Name: "repo1", URL: "https://github.com/org/repo1.git", Branch: "main", Paths: []string{"docs"}},
		},
	}

	g := NewGenerator(cfg, out)
	if err := g.generateHugoConfig(); err != nil {
		t.Fatalf("generate: %v", err)
	}

	actual := normalizeConfig(t, filepath.Join(out, "hugo.yaml"))
	golden := filepath.Join("testdata", "hugo_config", "transitions.yaml")

	// #nosec G304 - test file
	want, err := os.ReadFile(golden)
	if err != nil {
		// If golden file doesn't exist and UPDATE_GOLDEN=1, create it
		if os.IsNotExist(err) && os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatalf("create golden dir: %v", err)
			}
			if err := os.WriteFile(golden, actual, 0o600); err != nil {
				t.Fatalf("create golden: %v", err)
			}
			t.Logf("Created golden file: %s", golden)
			return
		}
		t.Fatalf("read golden: %v", err)
	}

	if !bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(actual)) {
		writeMismatch(t, want, actual)
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(golden, actual, 0o600); err != nil {
				t.Fatalf("update golden: %v", err)
			}
			t.Logf("Updated golden file: %s", golden)
			return
		}
		t.Fatalf("transitions hugo.yaml mismatch; run UPDATE_GOLDEN=1 go test ./internal/hugo -run TestHugoConfigGolden_Transitions to accept")
	}
}
