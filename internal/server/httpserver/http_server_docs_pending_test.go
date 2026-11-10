package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestDocsHandlerPendingPageForDocPaths pins that every documentation URL
// gets the pending page while no rendered site exists — not just the root.
// Falling through to the file server would serve the Hugo scaffold, which
// 404s for every real documentation path.
func TestDocsHandlerPendingPageForDocPaths(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Output: config.OutputConfig{Directory: tmpDir},
		Build:  config.BuildConfig{LiveReload: false},
	}

	srv := New(cfg, testRuntime{}, Options{})

	for _, path := range []string{"/", "/tutorials/getting-started", "/reference/configuration"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		root := srv.resolveDocsRoot()
		srv.handleStatusPage(rec, req, root)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("path %s: status = %d, want %d", path, rec.Code, http.StatusServiceUnavailable)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), "Documentation is being prepared") {
			t.Errorf("path %s: expected pending page, got: %s", path, body)
		}
	}
}

// TestDocsHandlerAssetPathFallsThroughToFileServer pins that asset-looking
// paths keep the file server (404) instead of returning an HTML page.
func TestDocsHandlerAssetPathFallsThroughToFileServer(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Output: config.OutputConfig{Directory: tmpDir},
		Build:  config.BuildConfig{LiveReload: false},
	}

	srv := New(cfg, testRuntime{}, Options{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/css/main.css", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	srv.handleStatusPage(rec, req, root)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "Documentation is being prepared") {
		t.Errorf("expected no pending page for an asset path, got: %s", body)
	}
}

// TestDocsHandlerPendingPageShowsDaemonState pins that the pending page
// reports what the daemon is doing, so it reads as "working on it" rather
// than a dead end.
func TestDocsHandlerPendingPageShowsDaemonState(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Output: config.OutputConfig{Directory: tmpDir},
		Build:  config.BuildConfig{LiveReload: false},
	}

	srv := New(cfg, testRuntime{}, Options{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	root := srv.resolveDocsRoot()
	srv.handleStatusPage(rec, req, root)

	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "Daemon:") {
		t.Errorf("expected daemon state on the pending page, got: %s", body)
	}
}
