package hugo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/models"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/stages"
)

// TestStageLayouts_SharedSidebarWritesOverrides pins that the shared
// sidebar mode writes the overrides into the Hugo project root, and that
// the default mode writes nothing.
func TestStageLayouts_SharedSidebarWritesOverrides(t *testing.T) {
	t.Run("shared", func(t *testing.T) {
		out := t.TempDir()
		gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{
			Sidebar: &config.SidebarConfig{Mode: config.SidebarModeShared},
		}}, out)
		bs := models.NewBuildState(gen, nil, models.NewBuildReport(context.Background(), 0, 0))

		if err := stages.StageLayouts(context.Background(), bs); err != nil {
			t.Fatalf("stage layouts: %v", err)
		}

		for rel := range SharedNavFiles() {
			if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
				t.Errorf("expected %s to be written: %v", rel, err)
			}
		}
	})

	t.Run("default", func(t *testing.T) {
		out := t.TempDir()
		gen := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "T"}}, out)
		bs := models.NewBuildState(gen, nil, models.NewBuildReport(context.Background(), 0, 0))

		if err := stages.StageLayouts(context.Background(), bs); err != nil {
			t.Fatalf("stage layouts: %v", err)
		}

		for rel := range SharedNavFiles() {
			if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
				t.Errorf("default mode must not write %s", rel)
			}
		}
	})
}
