package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// createHugoStructure creates the basic Hugo directory structure
func (g *Generator) createHugoStructure() error {
	dirs := []string{
		"content",
		"layouts",
		"layouts/_default",
		"layouts/partials",
		"static",
		"data",
		"assets",
		"archetypes",
	}
	root := g.buildRoot()
	for _, dir := range dirs {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	slog.Debug("Created Hugo directory structure", "root", root)
	return nil
}

// beginStaging creates an isolated staging directory for atomic build output.
func (g *Generator) beginStaging() error {
	// Create sibling staging dir: <output>_stage (not inside output)
	// For example: if outputDir is "site", create "site_stage" as a sibling
	stage := g.outputDir + "_stage"
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	g.stageDir = stage
	slog.Debug("Initialized staging directory", "staging", stage, "final", g.outputDir)
	return nil
}

// finalizeStaging atomically promotes staging directory to final output location.
// Strategy:
//  1. Move existing outputDir (if exists) to outputDir.prev (overwrite if already there).
//  2. Rename staging -> outputDir.
//  3. Remove previous backup asynchronously best-effort.
func (g *Generator) finalizeStaging() error {
	slog.Debug("finalizeStaging called", "stageDir", g.stageDir, "outputDir", g.outputDir)

	if g.stageDir == "" {
		return fmt.Errorf("no staging directory initialized")
	}

	// Check if staging directory still exists
	if stat, err := os.Stat(g.stageDir); err != nil {
		slog.Error("Staging directory missing at finalize", "stageDir", g.stageDir, "outputDir", g.outputDir, "error", err)
		return fmt.Errorf("staging directory missing: %w", err)
	} else {
		slog.Debug("Staging directory exists at finalize", "stageDir", g.stageDir, "is_dir", stat.IsDir())
	}

	prev := g.outputDir + ".prev"
	// Remove old backup if present
	if _, err := os.Stat(prev); err == nil {
		// Try multiple times to remove previous backup (may be locked/in-use)
		for i := 0; i < 3; i++ {
			if err := os.RemoveAll(prev); err == nil {
				break
			}
			if i < 2 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		// If still exists, try to force remove any remaining files
		if _, err := os.Stat(prev); err == nil {
			// Last resort: remove with chmod
			_ = filepath.Walk(prev, func(path string, _ os.FileInfo, err error) error {
				if err == nil {
					_ = os.Chmod(path, 0o755)
				}
				return nil
			})
			if err := os.RemoveAll(prev); err != nil {
				slog.Warn("Failed to remove previous backup", logfields.Path(prev), "error", err)
				// Continue anyway - rename will fail if prev still exists
			}
		}
	}
	if _, err := os.Stat(g.outputDir); err == nil {
		if err := os.Rename(g.outputDir, prev); err != nil {
			return fmt.Errorf("backup existing output: %w", err)
		}
	}
	if err := os.Rename(g.stageDir, g.outputDir); err != nil {
		return fmt.Errorf("promote staging: %w", err)
	}
	g.stageDir = ""
	// Remove previous backup asynchronously (non-critical)
	go func(p string) {
		if err := os.RemoveAll(p); err != nil {
			slog.Warn("Failed to remove previous backup", logfields.Path(p), "error", err)
		}
	}(prev)
	slog.Info("Promoted staging directory", "output", g.outputDir)
	return nil
}

// abortStaging removes any existing staging directory after a failed build to avoid orphaned temp dirs.
func (g *Generator) abortStaging() {
	if g.stageDir == "" {
		return
	}
	dir := g.stageDir
	g.stageDir = "" // prevent double cleanup
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("Failed to remove staging directory after abort", "staging", dir, "error", err)
	} else {
		slog.Debug("Removed staging directory after abort", "staging", dir)
	}
}
