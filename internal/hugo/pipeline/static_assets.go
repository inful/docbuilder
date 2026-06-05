package pipeline

import (
	"bytes"
	_ "embed"
	"strings"
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

// generateCustomHeaderAssets builds the project's
// layouts/partials/custom-header.html partial that the Relearn
// theme includes in every page head. The partial contains three
// pieces, in this order:
//
//  1. A <style> block with the resolved hugo.custom_css (or the
//     built-in default that tones down Relearn's loud sidebar
//     section titles).
//  2. When EnablePageTransitions is true, the View Transitions
//     API <link> and the accompanying view-transitions.css asset.
//  3. The template-metadata <meta> tags that template discovery
//     depends on (always present).
//
// Relearn explicitly documents custom-header.html as the override
// point for adding custom CSS. We do that here so the project's
// custom CSS ships automatically without the operator having to
// maintain a Hugo template override.
//
// When hugo.custom_css is unset, the built-in default is written.
// When set to a non-empty value, that value replaces the default
// verbatim. A whitespace-only value suppresses the <style> block
// entirely so operators who want the unmodified Relearn styling
// can opt out.
func generateCustomHeaderAssets(ctx *GenerationContext) ([]*StaticAsset, error) {
	transitionsEnabled := ctx != nil && ctx.Config != nil && ctx.Config.Hugo.EnablePageTransitions

	// Resolve the custom CSS: empty -> default, otherwise the
	// configured value. A whitespace-only value suppresses the
	// <style> block.
	css := ""
	if ctx != nil && ctx.Config != nil {
		css = ctx.Config.Hugo.CustomCSS
	}
	if css == "" {
		css = defaultCustomCSS
	}
	includeCSS := strings.TrimSpace(css) != ""

	assets := make([]*StaticAsset, 0, 2)

	// Assemble the partial: optional <style>, optional view-
	// transitions <link>, template-metadata <meta>s.
	var blocks [][]byte
	if includeCSS {
		blocks = append(blocks, []byte("<style>\n"+css+"\n</style>"))
	}
	if transitionsEnabled {
		blocks = append(blocks, viewTransitionsHeadPartial)
		assets = append(assets, &StaticAsset{
			Path:    "static/view-transitions.css",
			Content: viewTransitionsCSS,
		})
	}
	blocks = append(blocks, templateMetadataHeadPartial)
	customHeaderContent := bytes.Join(blocks, []byte("\n"))

	assets = append(assets, &StaticAsset{
		Path:    "layouts/partials/custom-header.html",
		Content: customHeaderContent,
	})

	return assets, nil
}
