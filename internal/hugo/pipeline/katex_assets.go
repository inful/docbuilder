package pipeline

import (
	"embed"
	"fmt"
	"path/filepath"
)

// The vendored KaTeX stylesheet and the fonts it asks for.
//
// The theme fetches them from a CDN with resources.GetRemote. A build
// without a network fails on it, so the site serves its own copies.
//
//go:embed assets/katex/katex.min.css
var katexStylesheet []byte

//go:embed assets/katex/fonts/*.woff2
var katexFonts embed.FS

// katexVendoredAssets returns the vendored KaTeX files as static
// assets, keyed by their path relative to the Hugo project root.
//
// The stylesheet lives under assets/ so Hugo publishes it through its
// resource pipeline, and the fonts live under static/ because the
// stylesheet asks for them by a relative path.
func katexVendoredAssets() ([]*StaticAsset, error) {
	assets := []*StaticAsset{
		{Path: filepath.Join("assets", "katex", "katex.min.css"), Content: katexStylesheet},
	}

	entries, err := katexFonts.ReadDir("assets/katex/fonts")
	if err != nil {
		return nil, fmt.Errorf("read the vendored KaTeX fonts: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := katexFonts.ReadFile("assets/katex/fonts/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read the vendored KaTeX font %s: %w", entry.Name(), err)
		}
		assets = append(assets, &StaticAsset{
			Path:    filepath.Join("static", "katex", "fonts", entry.Name()),
			Content: content,
		})
	}

	return assets, nil
}
