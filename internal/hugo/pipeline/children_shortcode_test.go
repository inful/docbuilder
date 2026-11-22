package pipeline

import (
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The site must carry the children shortcode. The documentation of the
// repositories uses it, and a shortcode without a template fails the
// build of the whole site.
func TestTheSiteCarriesTheChildrenShortcode(t *testing.T) {
	ctx := &GenerationContext{Config: &config.Config{}}

	assets, err := generateThemeAssets(ctx)
	if err != nil {
		t.Fatalf("build the assets: %v", err)
	}

	found := false
	for _, asset := range assets {
		if asset.Path != docsyChildrenShortcodePath {
			continue
		}
		found = true
		body := string(asset.Content)
		if !strings.Contains(body, ".Pages") {
			t.Errorf("the shortcode does not list the pages below the current one:\n%s", body)
		}
	}
	if !found {
		t.Fatalf("the site does not carry %s", docsyChildrenShortcodePath)
	}
}

// The shortcode must not break the build of a site that does not use it.
func TestTheSiteBuildsWithoutTheChildrenShortcode(t *testing.T) {
	ctx := &GenerationContext{Config: &config.Config{}}

	assets, err := generateThemeAssets(ctx)
	if err != nil {
		t.Fatalf("build the assets: %v", err)
	}
	for _, asset := range assets {
		if asset.Path == docsyChildrenShortcodePath && len(asset.Content) == 0 {
			t.Fatal("the shortcode is empty")
		}
	}
}
