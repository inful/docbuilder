package hugo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// createHugoStructure creates the basic Hugo directory structure.
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
		if err := os.MkdirAll(path, 0o750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	slog.Debug("Created Hugo directory structure", "root", root)
	return nil
}

// beginStaging creates an isolated staging directory for atomic build output.
func (g *Generator) beginStaging() error {
	// Determine staging directory based on base_directory configuration
	var stage string
	if g.config.Output.BaseDirectory != "" {
		// When base_directory is set, create staging as {base}/staging
		// and output will be at {base}/{directory}
		stage = filepath.Join(g.config.Output.BaseDirectory, "staging")
		slog.Debug("Using base_directory for staging",
			slog.String("base_directory", g.config.Output.BaseDirectory),
			slog.String("staging", stage))
	} else {
		// Default: create sibling staging dir: <output>_stage (not inside output)
		// For example: if outputDir is "site", create "site_stage" as a sibling
		stage = g.outputDir + "_stage"
		slog.Debug("Using default staging location",
			slog.String("staging", stage))
	}
	slog.Info("Creating staging directory for atomic build",
		slog.String("staging", stage),
		slog.String("output", g.outputDir))
	if err := os.MkdirAll(stage, 0o750); err != nil {
		slog.Error("Failed to create staging directory",
			slog.String("path", stage),
			slog.String("error", err.Error()))
		return err
	}
	g.stageDir = stage
	slog.Info("Staging directory initialized successfully",
		slog.String("staging", stage),
		slog.String("output", g.outputDir))
	return nil
}

// finalizeStaging atomically promotes staging directory to final output location.
// Strategy:
//  1. Move existing outputDir (if exists) to outputDir.prev (overwrite if already there).
//  2. Rename staging -> outputDir.
//  3. Remove previous backup asynchronously best-effort.
func (g *Generator) finalizeStaging() error {
	slog.Info("Starting atomic staging finalization",
		slog.String("staging", g.stageDir),
		slog.String("output", g.outputDir))

	if g.stageDir == "" {
		return errors.New("no staging directory initialized")
	}

	// Check if staging directory still exists
	if stat, err := os.Stat(g.stageDir); err != nil {
		slog.Error("Staging directory missing at finalize",
			slog.String("staging", g.stageDir),
			slog.String("output", g.outputDir),
			slog.String("error", err.Error()))
		return fmt.Errorf("staging directory missing: %w", err)
	} else {
		slog.Debug("Staging directory verified",
			slog.String("path", g.stageDir),
			slog.Bool("is_dir", stat.IsDir()),
			slog.Time("modified", stat.ModTime()))
	}

	prev := g.outputDir + ".prev"
	// Remove old backup if present
	if stat, err := os.Stat(prev); err == nil {
		slog.Info("Removing old backup directory",
			slog.String("path", prev),
			slog.Time("modified", stat.ModTime()))
		// Try multiple times to remove previous backup (may be locked/in-use)
		for i := range 3 {
			if err := os.RemoveAll(prev); err == nil {
				slog.Debug("Successfully removed old backup",
					slog.String("path", prev),
					slog.Int("attempts", i+1))
				break
			} else if i == 2 {
				slog.Warn("Failed to remove old backup after retries",
					slog.String("path", prev),
					slog.String("error", err.Error()))
			}
			if i < 2 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		// If still exists, try to force remove any remaining files
		if _, err := os.Stat(prev); err == nil {
			slog.Warn("Old backup still present, attempting force removal",
				slog.String("path", prev))
			// Last resort: remove with chmod
			_ = filepath.Walk(prev, func(path string, _ os.FileInfo, err error) error {
				if err == nil {
					_ = os.Chmod(path, 0o600)
				}
				return nil
			})
			if err := os.RemoveAll(prev); err != nil {
				slog.Warn("Failed to remove previous backup", logfields.Path(prev), "error", err)
				// Continue anyway - rename will fail if prev still exists
			}
		}
	} else {
		slog.Debug("No old backup directory to remove")
	}

	// Step 1: Backup current output (if exists)
	if stat, err := os.Stat(g.outputDir); err == nil {
		slog.Info("Backing up current output directory",
			slog.String("from", g.outputDir),
			slog.String("to", prev),
			slog.Time("modified", stat.ModTime()))
		if err := os.Rename(g.outputDir, prev); err != nil {
			slog.Error("Failed to backup current output",
				slog.String("from", g.outputDir),
				slog.String("to", prev),
				slog.String("error", err.Error()))
			return fmt.Errorf("backup existing output: %w", err)
		}
		slog.Debug("Successfully backed up current output")
	} else {
		slog.Debug("No existing output directory to backup",
			slog.String("path", g.outputDir))
	}

	// Step 2: Promote staging to output
	slog.Info("Promoting staging directory to output",
		slog.String("from", g.stageDir),
		slog.String("to", g.outputDir))
	if err := os.Rename(g.stageDir, g.outputDir); err != nil {
		slog.Error("Failed to promote staging directory",
			slog.String("from", g.stageDir),
			slog.String("to", g.outputDir),
			slog.String("error", err.Error()))
		return fmt.Errorf("promote staging: %w", err)
	}
	g.stageDir = ""
	slog.Info("Successfully promoted staging directory",
		slog.String("output", g.outputDir))

	// Step 3: Remove previous backup asynchronously (non-critical)
	go func(p string) {
		slog.Debug("Starting async cleanup of backup directory",
			slog.String("path", p))
		if err := os.RemoveAll(p); err != nil {
			slog.Warn("Failed to remove previous backup in async cleanup",
				logfields.Path(p),
				slog.String("error", err.Error()))
		} else {
			slog.Debug("Successfully cleaned up backup directory",
				slog.String("path", p))
		}
	}(prev)

	return nil
}

// abortStaging removes any existing staging directory after a failed build to avoid orphaned temp dirs.
func (g *Generator) abortStaging() {
	if g.stageDir == "" {
		return
	}
	// Preserve staging directory for debugging if requested
	if g.keepStaging {
		slog.Info("Build failed - staging directory preserved for debugging",
			slog.String("staging", g.stageDir),
			slog.String("output", g.outputDir))
		slog.Debug("Staging directory preserved for debugging: " + g.stageDir)
		return
	}
	slog.Warn("Aborting build - cleaning up staging directory",
		slog.String("staging", g.stageDir),
		slog.String("output", g.outputDir))
	dir := g.stageDir
	g.stageDir = "" // prevent double cleanup
	if err := os.RemoveAll(dir); err != nil {
		slog.Error("Failed to remove staging directory on abort",
			logfields.Path(dir),
			slog.String("error", err.Error()))
	} else {
		slog.Info("Successfully cleaned up staging directory after build abort",
			slog.String("path", dir))
	}
}
