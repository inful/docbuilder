package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadConfig reads a configuration file.
func loadConfig(t *testing.T, content string) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// Nobody chooses a provider in the file, so nobody chooses one there.
// Who builds the site decides: the daemon serves the site it builds,
// and a plain build produces a static tree.
func TestSearchProviderIsChosenByTheBuilder(t *testing.T) {
	cfg := loadConfig(t, "version: \"2.0\"\ndaemon:\n  http:\n    docs_port: 8080\nrepositories:\n  - name: docs\n    url: https://example.com/docs.git\n")
	if cfg.Search == nil {
		t.Fatal("the search configuration must have defaults")
	}
	if cfg.Search.Provider != "" {
		t.Fatalf("provider = %q, want it unset", cfg.Search.Provider)
	}
	if cfg.Search.MaxResults != 10 {
		t.Fatalf("max_results = %d, want 10", cfg.Search.MaxResults)
	}
}

// The operator chooses.
func TestSearchProviderCanBeSet(t *testing.T) {
	cfg := loadConfig(t, "version: \"2.0\"\nsearch:\n  provider: lunr\n  max_results: 25\nrepositories:\n  - name: docs\n    url: https://example.com/docs.git\n")
	if cfg.Search.Provider != SearchProviderLunr {
		t.Fatalf("provider = %q, want %q", cfg.Search.Provider, SearchProviderLunr)
	}
	if cfg.Search.MaxResults != 25 {
		t.Fatalf("max_results = %d, want 25", cfg.Search.MaxResults)
	}
}

// A provider nobody knows is a mistake: the browser-side search is the
// one that works without a server.
func TestSearchProviderUnknownFallsBackToTheBrowser(t *testing.T) {
	cfg := loadConfig(t, "version: \"2.0\"\nsearch:\n  provider: nonsense\nrepositories:\n  - name: docs\n    url: https://example.com/docs.git\n")
	if cfg.Search.Provider != SearchProviderLunr {
		t.Fatalf("provider = %q, want %q", cfg.Search.Provider, SearchProviderLunr)
	}
}
