package httpserver

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The output lives under the base directory when one is set. Everything
// that looks for the output must look there: the status page that
// replaces the site while it is built, and the content of the site.
func TestDocsHandlerLooksInTheBaseDirectory(t *testing.T) {
	base := t.TempDir()
	cfg := &config.Config{
		Output: config.OutputConfig{
			BaseDirectory: base,
			Directory:     "site",
		},
	}

	srv := New(cfg, testRuntime{}, Options{})

	want := base + "/site"
	if got := srv.resolveAbsoluteOutputDir(); got != want {
		t.Fatalf("output dir = %q, want %q", got, want)
	}

	// The site is not built yet, so the daemon shows the status page
	// instead of serving an empty tree.
	root := srv.resolveDocsRoot()
	if !srv.shouldShowStatusPage(root) {
		t.Fatalf("the status page must be shown when the site is not built (root %q)", root)
	}
}
