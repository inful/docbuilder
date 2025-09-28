package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// shouldRunHugo determines if we should invoke the external hugo binary.
// Enabled when DOCBUILDER_RUN_HUGO=1 and hugo binary exists in PATH, unless DOCBUILDER_SKIP_HUGO=1.
func shouldRunHugo() bool {
	if os.Getenv("DOCBUILDER_SKIP_HUGO") == "1" {
		return false
	}
	if os.Getenv("DOCBUILDER_RUN_HUGO") != "1" {
		return false
	}
	_, err := exec.LookPath("hugo")
	return err == nil
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
