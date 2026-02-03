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
// This merges view transitions content with the existing custom-header.html
// (which contains template metadata from generateTemplateMetadataAssets).
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
	// The template metadata partial should already exist from generateTemplateMetadataAssets
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

// generateTemplateMetadataAssets creates the custom-header.html partial that injects
// template metadata as HTML meta tags. This is always generated to support template discovery.
// Note: This must run before generateViewTransitionsAssets so the view transitions generator
// can merge its content with the template metadata partial.
func generateTemplateMetadataAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	// Always generate template metadata partial (required for template discovery)
	return []*StaticAsset{
		{
			Path:    "layouts/partials/custom-header.html",
			Content: templateMetadataHeadPartial,
		},
	}, nil
}
