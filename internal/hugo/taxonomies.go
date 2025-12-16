package hugo

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
)

//go:embed assets/taxonomy-terms.html
var taxonomyTermsLayout []byte

//go:embed assets/taxonomy-term.html
var taxonomyTermLayout []byte

// copyTaxonomyLayouts copies taxonomy layout templates to the layouts directory
// for rendering taxonomy list pages (e.g., /tags/) and individual term pages (e.g., /tags/api/)
func (g *Generator) copyTaxonomyLayouts() error {
	// Only copy if config exists
	if g.config == nil {
		return nil
	}

	// Always copy taxonomy layouts since we always set default taxonomies (tags, categories)
	// in generateHugoConfig even when user hasn't explicitly configured them
	slog.Debug("Copying taxonomy layouts for default or custom taxonomies")

	// Create layouts for _default
	defaultLayoutsDir := filepath.Join(g.buildRoot(), "layouts", "_default")
	if err := os.MkdirAll(defaultLayoutsDir, 0o750); err != nil {
		return err
	}

	// Copy terms.html layout (for taxonomy list pages like /tags/, /categories/)
	termsPath := filepath.Join(defaultLayoutsDir, "terms.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(termsPath, taxonomyTermsLayout, 0644); err != nil {
		return err
	}

	// Copy term.html layout (for individual term pages like /tags/api/, /tags/guide/)
	termPath := filepath.Join(defaultLayoutsDir, "term.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(termPath, taxonomyTermLayout, 0644); err != nil {
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
	if err := os.WriteFile(taxonomyTermsPath, taxonomyTermsLayout, 0644); err != nil {
		return err
	}

	// Copy to taxonomy/term.html for higher priority
	taxonomyTermPath := filepath.Join(taxonomyLayoutsDir, "term.html")
	// #nosec G306 -- HTML layout is a public template file
	if err := os.WriteFile(taxonomyTermPath, taxonomyTermLayout, 0644); err != nil {
		return err
	}

	slog.Debug("Taxonomy layouts copied",
		"default_terms", "layouts/_default/terms.html",
		"default_term", "layouts/_default/term.html",
		"taxonomy_terms", "layouts/taxonomy/terms.html",
		"taxonomy_term", "layouts/taxonomy/term.html")

	return nil
}
