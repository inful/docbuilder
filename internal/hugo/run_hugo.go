package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// shouldRunHugo determines if we should invoke the external hugo binary under RenderModeAuto.
// It preserves backward compatibility with legacy env vars but logs a deprecation notice when they drive the decision.
var legacyEnvWarnOnce sync.Once

func shouldRunHugo(cfg *config.Config) bool {
	mode := config.ResolveEffectiveRenderMode(cfg)
	switch mode {
	case config.RenderModeNever:
		return false
	case config.RenderModeAlways:
		if _, err := exec.LookPath("hugo"); err != nil {
			slog.Warn("Hugo binary not found while in render_mode=always; skipping execution", "error", err)
			return false
		}
		return true
	case config.RenderModeAuto:
		// Legacy env gating path (auto means: only run when DOCBUILDER_RUN_HUGO=1 and not DOCBUILDER_SKIP_HUGO=1)
		if os.Getenv("DOCBUILDER_SKIP_HUGO") == "1" {
			legacyEnvWarnOnce.Do(func() {
				slog.Warn("Legacy env DOCBUILDER_SKIP_HUGO is deprecated; prefer build.render_mode=never or --render-mode never")
			})
			slog.Info("Skipping Hugo due to DOCBUILDER_SKIP_HUGO=1 (render_mode=auto)")
			return false
		}
		if os.Getenv("DOCBUILDER_RUN_HUGO") == "1" {
			if _, err := exec.LookPath("hugo"); err != nil {
				slog.Warn("DOCBUILDER_RUN_HUGO=1 set but Hugo binary not found; skipping", "error", err)
				return false
			}
			legacyEnvWarnOnce.Do(func() {
				slog.Warn("Legacy env DOCBUILDER_RUN_HUGO is deprecated; prefer build.render_mode=always or --render-mode always")
			})
			slog.Info("Running Hugo due to DOCBUILDER_RUN_HUGO=1 (render_mode=auto)")
			return true
		}
		return false
	default:
		return false
	}
}

// runHugoBuild executes `hugo` inside the output directory to produce the static site under public/.
func (g *Generator) runHugoBuild() error {
	// IMPORTANT: At this pipeline stage we are still in staging. We must run Hugo
	// inside the staging directory (buildRoot) so that generated artifacts (public/)
	// are promoted atomically along with the rest of the site. Running in finalRoot
	// caused the observed bug where hugo.yaml was not found.
	root := g.buildRoot()
	configPath := filepath.Join(root, "hugo.yaml")
	if fi, err := os.Stat(configPath); err != nil || fi.IsDir() {
		entries, _ := os.ReadDir(root)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		slog.Warn("Hugo config file missing before run; build will likely fail", "expected", configPath, "root", root, "dir_entries", names, "error", err)
	}
	cmd := exec.Command("hugo")
	cmd.Dir = root // run against staging directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	slog.Info("Running Hugo binary to render static site", "dir", root)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hugo command failed: %w", err)
	}
	return nil
}
