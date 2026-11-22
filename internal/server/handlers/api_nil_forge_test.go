package handlers

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// An empty entry in the list of forges is a nil pointer. The summary of
// the configuration must not fall over it.
func TestSanitizeConfigSkipsAnEmptyForge(t *testing.T) {
	h := &APIHandlers{}

	cfg := &config.Config{
		Daemon: &config.DaemonConfig{},
		Forges: []*config.ForgeConfig{
			{Name: "first", Type: config.ForgeGitHub},
			nil,
			{Name: "second", Type: config.ForgeGitLab},
		},
	}

	summary := h.sanitizeConfig(cfg)
	if len(summary.Forges) != 2 {
		t.Fatalf("the summary holds %d forges, want 2", len(summary.Forges))
	}
	if summary.Forges[0].Name != "first" || summary.Forges[1].Name != "second" {
		t.Fatalf("the summary holds %+v", summary.Forges)
	}
}
