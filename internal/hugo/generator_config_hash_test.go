package hugo

import (
	"testing"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGeneratorConfigHashUsesSnapshot ensures generator config hash matches Config.Snapshot.
func TestGeneratorConfigHashUsesSnapshot(t *testing.T) {
	c := &cfg.Config{Hugo: cfg.HugoConfig{BaseURL: "https://ex", Title: "Docs"}}
	// simulate normalization + defaults if needed (no-op for these fields currently)
	if _, err := cfg.NormalizeConfig(c); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	gen := NewGenerator(c, t.TempDir())
	if gen.ComputeConfigHash() != c.Snapshot() {
		t.Fatalf("generator config hash mismatch snapshot\nwant=%s\ngot=%s", c.Snapshot(), gen.ComputeConfigHash())
	}
}
