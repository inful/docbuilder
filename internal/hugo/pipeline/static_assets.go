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

// generateViewTransitionsAssets creates View Transitions API static assets
// if enable_page_transitions is enabled in the Hugo configuration.
// This also includes template metadata meta tags in the generated custom-header.html.
func generateViewTransitionsAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	// Check if transitions are enabled
	if ctx.Config == nil || !ctx.Config.Hugo.EnablePageTransitions {
		return nil, nil
	}

	assets := make([]*StaticAsset, 0, 2)
	assets = append(assets, &StaticAsset{
		Path:    "static/view-transitions.css",
		Content: viewTransitionsCSS,
	})

	// Merge view transitions with template metadata in custom-header.html
	mergedHeader := bytes.Join([][]byte{
		viewTransitionsHeadPartial,
		[]byte("\n"),
		templateMetadataHeadPartial,
	}, nil)

	assets = append(assets, &StaticAsset{
		Path:    "layouts/partials/custom-header.html",
		Content: mergedHeader,
	})

	return assets, nil
}
