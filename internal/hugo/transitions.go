package hugo

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

//go:embed assets/view-transitions.css
var transitionCSS []byte

//go:embed assets/view-transitions-head.html
var transitionHeadPartial []byte

// copyTransitionAssets copies View Transitions API assets to the static directory
// and the head partial to the appropriate location for each theme
func (g *Generator) copyTransitionAssets() error {
	// Only copy if transitions are enabled
	if g.config == nil || !g.config.Hugo.EnableTransitions {
		return nil
	}

	slog.Debug("View Transitions API enabled, copying transition assets")

	staticDir := filepath.Join(g.buildRoot(), "static")
	if err := os.MkdirAll(staticDir, 0o750); err != nil {
		return err
	}

	// Copy CSS file
	cssPath := filepath.Join(staticDir, "view-transitions.css")
	// #nosec G306 -- static CSS file is a public asset
	if err := os.WriteFile(cssPath, transitionCSS, 0644); err != nil {
		return err
	}

	// Determine head partial path based on theme
	var headPartialPath string
	themeType := g.config.Hugo.ThemeType()
	switch themeType {
	case config.ThemeHextra:
		// Hextra uses layouts/_partials/custom/head-end.html
		customPartialsDir := filepath.Join(g.buildRoot(), "layouts", "_partials", "custom")
		if err := os.MkdirAll(customPartialsDir, 0o750); err != nil {
			return err
		}
		headPartialPath = filepath.Join(customPartialsDir, "head-end.html")
		slog.Debug("Using Hextra head partial path", "path", "layouts/_partials/custom/head-end.html")

	case config.ThemeDocsy:
		// Docsy uses layouts/partials/hooks/head-end.html
		hooksDir := filepath.Join(g.buildRoot(), "layouts", "partials", "hooks")
		if err := os.MkdirAll(hooksDir, 0o750); err != nil {
			return err
		}
		headPartialPath = filepath.Join(hooksDir, "head-end.html")
		slog.Debug("Using Docsy head partial path", "path", "layouts/partials/hooks/head-end.html")

	case config.ThemeRelearn:
		// Relearn uses layouts/partials/custom-header.html
		partialsDir := filepath.Join(g.buildRoot(), "layouts", "partials")
		if err := os.MkdirAll(partialsDir, 0o750); err != nil {
			return err
		}
		headPartialPath = filepath.Join(partialsDir, "custom-header.html")
		slog.Debug("Using Relearn head partial path", "path", "layouts/partials/custom-header.html")

	default:
		// Fallback for unknown themes - use Hextra path
		customPartialsDir := filepath.Join(g.buildRoot(), "layouts", "_partials", "custom")
		if err := os.MkdirAll(customPartialsDir, 0o750); err != nil {
			return err
		}
		headPartialPath = filepath.Join(customPartialsDir, "head-end.html")
		slog.Debug("Using fallback head partial path", "path", "layouts/_partials/custom/head-end.html", "theme", themeType)
	}

	// Write the head partial
	// #nosec G306 -- HTML partial is a public template file
	if err := os.WriteFile(headPartialPath, transitionHeadPartial, 0644); err != nil {
		return err
	}

	slog.Debug("View Transitions assets copied",
		"css", "view-transitions.css",
		"partial", headPartialPath)

	return nil
}
