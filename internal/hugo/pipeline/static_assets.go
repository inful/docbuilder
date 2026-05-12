package pipeline

import (
	"bytes"
	_ "embed"
)

// Embedded View Transitions API assets

//go:embed assets/view-transitions.css
var viewTransitionsCSS []byte

//go:embed assets/view-transitions-head.html
var viewTransitionsHeadPartial []byte

//go:embed assets/template-metadata-head.html
var templateMetadataHeadPartial []byte

// StaticAsset represents a static file to be copied to the Hugo site.
// Unlike Document, StaticAsset doesn't go through the transform pipeline.
type StaticAsset struct {
	Path    string // Relative path from Hugo root (e.g., "static/view-transitions.css")
	Content []byte // Raw file content
}

// StaticAssetGenerator creates static assets based on configuration.
// Returns a list of assets to be copied to the Hugo site root.
type StaticAssetGenerator func(ctx *GenerationContext) ([]*StaticAsset, error)

// generateViewTransitionsAssets generates custom header assets for template metadata
// and optionally includes View Transitions assets when enabled.
//
// Template metadata is always emitted in layouts/partials/custom-header.html because
// template discovery/parsing depends on these meta tags regardless of transitions.
func generateViewTransitionsAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	transitionsEnabled := ctx != nil && ctx.Config != nil && ctx.Config.Hugo.EnablePageTransitions
	assets := make([]*StaticAsset, 0, 2)

	customHeaderContent := templateMetadataHeadPartial
	if transitionsEnabled {
		assets = append(assets, &StaticAsset{
			Path:    "static/view-transitions.css",
			Content: viewTransitionsCSS,
		})

		customHeaderContent = bytes.Join([][]byte{
			viewTransitionsHeadPartial,
			[]byte("\n"),
			templateMetadataHeadPartial,
		}, nil)
	}

	assets = append(assets, &StaticAsset{
		Path:    "layouts/partials/custom-header.html",
		Content: customHeaderContent,
	})

	return assets, nil
}
