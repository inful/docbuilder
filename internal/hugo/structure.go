package hugo

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
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
    for _, dir := range dirs {
        path := filepath.Join(g.outputDir, dir)
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create directory %s: %w", path, err)
        }
    }
    slog.Debug("Created Hugo directory structure", "output", g.outputDir)
    return nil
}
