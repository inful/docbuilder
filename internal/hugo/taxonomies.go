package hugo

import (
	"log/slog"
)

// copyTaxonomyLayouts is a no-op for Relearn theme.
// Relearn provides its own built-in taxonomy support and layouts.
// This function is kept for backward compatibility.
func (g *Generator) copyTaxonomyLayouts() error {
	slog.Debug("Skipping taxonomy layout copy - Relearn theme provides its own")
	return nil
}
