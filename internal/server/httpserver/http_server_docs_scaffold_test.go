package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// The docs server answers on a public port. Before the first build the
// build tree is there: the sources of the site and the configuration of
// Hugo. They are not public.
func TestTheBuildTreeIsNotServedBeforeTheFirstBuild(t *testing.T) {
	tmpDir := t.TempDir()
	content := filepath.Join(tmpDir, "content", "docs")
	if err := os.MkdirAll(content, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(content, "secret.md"), []byte("# SECRET SOURCE"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "hugo.toml"), []byte("baseURL = 'https://example.invalid/'"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := &config.Config{
		Output: config.OutputConfig{Directory: tmpDir},
		Build:  config.BuildConfig{LiveReload: false},
	}

	srv := New(cfg, testRuntime{}, Options{})

	for _, path := range []string{"/content/docs/secret.md", "/hugo.toml"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		root := srv.resolveDocsRoot()
		srv.handleStatusPage(rec, req, root)

		if rec.Code != http.StatusNotFound {
			t.Errorf("path %s: status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		body, _ := io.ReadAll(rec.Body)
		if strings.Contains(string(body), "SECRET SOURCE") || strings.Contains(string(body), "example.invalid") {
			t.Errorf("path %s: the build tree was served: %s", path, body)
		}
	}
}
