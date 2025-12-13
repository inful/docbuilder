package hugo

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
)

//go:embed assets/view-transitions.css
var transitionCSS []byte

//go:embed assets/view-transitions.js
var transitionJS []byte

// copyTransitionAssets copies View Transitions API assets to the static directory
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

	// Copy JS file
	jsPath := filepath.Join(staticDir, "view-transitions.js")
	// #nosec G306 -- static JS file is a public asset
	if err := os.WriteFile(jsPath, transitionJS, 0644); err != nil {
		return err
	}

	slog.Debug("View Transitions assets copied",
		"css", "view-transitions.css",
		"js", "view-transitions.js",
		"duration", g.config.Hugo.TransitionDuration)

	return nil
}
