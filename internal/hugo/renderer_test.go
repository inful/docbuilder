package hugo

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TestNoopRenderer ensures that when a NoopRenderer is injected the RunHugo stage
// marks the site as rendered without invoking the external hugo binary (which may
// not be installed in CI). We approximate this by injecting NoopRenderer and using
// render_mode=always so the stage attempts to render unconditionally.
func TestNoopRenderer(t *testing.T) {
	// Temp output dir
	dir := t.TempDir()

	cfg := &config.Config{}
	cfg.Hugo.Title = "Test"
	cfg.Hugo.BaseURL = "https://example.test"
	cfg.Build.RenderMode = "always"

	g := NewGenerator(&config.Config{Hugo: config.HugoConfig{Title: "Test", BaseURL: "/"}}, dir).WithRenderer(&NoopRenderer{})

	// Minimal doc file to drive pipeline through content stages.
	doc := docs.DocFile{Repository: "repo1", Name: "intro", RelativePath: "intro.md", DocsBase: "docs", Extension: ".md", Content: []byte("# Intro\n")}
	report, err := g.GenerateSiteWithReportContext(context.Background(), []docs.DocFile{doc})
	if err != nil {
		// Any hugo invocation attempt (if NoopRenderer not used) could fail here if binary missing.
		// Surface error for visibility.
		t.Fatalf("generation failed: %v", err)
	}

	if !report.StaticRendered {
		t.Fatalf("expected report.StaticRendered=true with NoopRenderer, got false")
	}

	// With NoopRenderer no static site is produced; we only assert that the pipeline considered rendering done.
}
