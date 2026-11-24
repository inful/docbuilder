package pipeline

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The build must not ask the network for the assets of the theme: the
// operator builds the site without a network, and the build died with
// "Could not retrieve KaTeX css file from CDN" (issue #129).
func TestTheSiteCarriesItsOwnKatexAndMermaid(t *testing.T) {
	ctx := &GenerationContext{Config: &config.Config{}}

	assets, err := generateThemeAssets(ctx)
	if err != nil {
		t.Fatalf("build the assets: %v", err)
	}

	paths := make(map[string]string, len(assets))
	for _, asset := range assets {
		paths[asset.Path] = string(asset.Content)
	}

	// The stylesheet of KaTeX and its fonts.
	if _, ok := paths["assets/katex/katex.min.css"]; !ok {
		t.Error("the site does not carry the KaTeX stylesheet")
	}
	fonts := 0
	for path := range paths {
		if strings.HasPrefix(path, "assets/katex/fonts/") {
			fonts++
		}
		if strings.HasPrefix(path, "static/katex/") {
			// The fonts under static/ are published in every build,
			// and no page asks for them.
			t.Errorf("the files of KaTeX are published in every build: %s", path)
		}
	}
	if fonts == 0 {
		t.Error("the site does not carry the fonts of KaTeX")
	}

	// The partials that replace the ones of the theme.
	katex, ok := paths["layouts/_partials/scripts/katex.html"]
	if !ok {
		t.Fatal("the site does not replace the KaTeX partial of the theme")
	}
	if strings.Contains(katex, "GetRemote") || strings.Contains(katex, "unpkg.com") {
		t.Errorf("the KaTeX partial still asks the network:\n%s", katex)
	}

	mermaid, ok := paths["layouts/_partials/scripts/plugins/mermaid.html"]
	if !ok {
		t.Fatal("the site does not replace the mermaid partial of the theme")
	}
	if strings.Contains(mermaid, "GetRemote") {
		t.Errorf("the mermaid partial still fetches the script:\n%s", mermaid)
	}
}
