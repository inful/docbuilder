package hugo

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

//go:embed assets/taxonomy-terms.html
var taxonomyTermsLayoutHextra []byte

//go:embed assets/taxonomy-term.html
var taxonomyTermLayoutHextra []byte

// copyTaxonomyLayouts copies taxonomy layout templates to the layouts directory
// for rendering taxonomy list pages (e.g., /tags/) and individual term pages (e.g., /tags/api/)
// Only copies layouts for themes that don't provide their own (e.g., Hextra).
// Themes like Relearn and Docsy have built-in taxonomy support.
func (g *Generator) copyTaxonomyLayouts() error {
	// Only copy if config exists
	if g.config == nil {
		return nil
	}

	theme := config.Theme(g.config.Hugo.Theme)
	
	// Skip for themes that provide their own taxonomy layouts
	switch theme {
	case config.ThemeRelearn:
		slog.Debug("Skipping taxonomy layout copy - Relearn theme provides its own")
		return nil
	case config.ThemeDocsy:
		slog.Debug("Skipping taxonomy layout copy - Docsy theme provides its own")
		return nil
	case config.ThemeHextra:
		// Hextra needs taxonomy layouts
		slog.Debug("Copying taxonomy layouts for Hextra theme")
	default:
		// For unknown themes, provide Hextra-style layouts as fallback
		slog.Debug("Copying default taxonomy layouts for unknown theme", "theme", theme)
	}

	// Use Hextra layouts for themes that need them
	termsLayout := taxonomyTermsLayoutHextra
	termLayout := taxonomyTermLayoutHextra

	// Create layouts for _default
	defaultLayoutsDir := filepath.Join(g.buildRoot(), "layouts", "_default")
	if err := os.MkdirAll(defaultLayoutsDir, 0o750); err != nil {
		return err
	}

	// Copy terms.html layout (for taxonomy list pages like /tags/, /categories/)
	termsPath := filepath.Join(defaultLayoutsDir, "terms.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(termsPath, termsLayout, 0644); err != nil {
		return err
	}

	// Copy term.html layout (for individual term pages like /tags/api/, /tags/guide/)
	termPath := filepath.Join(defaultLayoutsDir, "term.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(termPath, termLayout, 0644); err != nil {
		return err
	}

	// Also create taxonomy-specific layouts for tags and categories
	// These have higher priority in Hugo's lookup order
	taxonomyLayoutsDir := filepath.Join(g.buildRoot(), "layouts", "taxonomy")
	if err := os.MkdirAll(taxonomyLayoutsDir, 0o750); err != nil {
		return err
	}

	// Copy to taxonomy/terms.html for higher priority
	taxonomyTermsPath := filepath.Join(taxonomyLayoutsDir, "terms.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(taxonomyTermsPath, termsLayout, 0644); err != nil {
		return err
	}

	// Copy to taxonomy/term.html for higher priority
	taxonomyTermPath := filepath.Join(taxonomyLayoutsDir, "term.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(taxonomyTermPath, termLayout, 0644); err != nil {
		return err
	}

	slog.Debug("Taxonomy layouts copied",
		"theme", theme,
		"default_terms", "layouts/_default/terms.html",
		"default_term", "layouts/_default/term.html",
		"taxonomy_terms", "layouts/taxonomy/terms.html",
		"taxonomy_term", "layouts/taxonomy/term.html")

	return nil
}
