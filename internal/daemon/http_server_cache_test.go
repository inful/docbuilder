package daemon

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestCacheControlHeaders verifies that appropriate Cache-Control headers are set for different asset types.
func TestCacheControlHeaders(t *testing.T) {
	tests := []struct {
		path          string
		expectedCache string
	}{
		// CSS and JavaScript - immutable, 1 year
		{"/assets/main.css", "public, max-age=31536000, immutable"},
		{"/js/bundle.js", "public, max-age=31536000, immutable"},
		{"/static/app.min.js", "public, max-age=31536000, immutable"},

		// Web fonts - immutable, 1 year
		{"/fonts/roboto.woff2", "public, max-age=31536000, immutable"},
		{"/fonts/icons.woff", "public, max-age=31536000, immutable"},
		{"/static/font.ttf", "public, max-age=31536000, immutable"},

		// Images - 1 week
		{"/images/logo.png", "public, max-age=604800"},
		{"/assets/hero.jpg", "public, max-age=604800"},
		{"/static/icon.svg", "public, max-age=604800"},
		{"/favicon.ico", "public, max-age=604800"},

		// Downloadable files - 1 day
		{"/downloads/manual.pdf", "public, max-age=86400"},
		{"/files/archive.zip", "public, max-age=86400"},

		// JSON (non-search) - 5 minutes
		{"/data/config.json", "public, max-age=300"},
		{"/api-data.json", "public, max-age=300"},

		// XML - 1 hour
		{"/sitemap.xml", "public, max-age=3600"},
		{"/feed.xml", "public, max-age=3600"},

		// HTML and root - no cache
		{"/index.html", "no-cache, must-revalidate"},
		{"/docs/guide.html", "no-cache, must-revalidate"},
		{"/", "no-cache, must-revalidate"},
		{"/docs/", "no-cache, must-revalidate"},

		// Search index - no cache header (special case)
		{"/search-index.json", ""},
		{"/idx.search.json", ""},
	}

	// Create a minimal HTTP server instance
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{},
	}
	srv := &HTTPServer{
		config: cfg,
	}

	// Simple handler that just returns 200 OK
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with cache control middleware
	handler := srv.addCacheControlHeaders(simpleHandler)

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			gotCache := rec.Header().Get("Cache-Control")
			if gotCache != tt.expectedCache {
				t.Errorf("path %s: expected Cache-Control %q, got %q",
					tt.path, tt.expectedCache, gotCache)
			}
		})
	}
}

// TestCacheControlNoInterferenceWithLiveReload ensures cache headers don't interfere with other middleware.
func TestCacheControlNoInterferenceWithLiveReload(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{},
		Build: config.BuildConfig{
			LiveReload: true,
		},
	}
	srv := &HTTPServer{
		config: cfg,
	}

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set a custom header to verify the handler was called
		w.Header().Set("X-Test-Handler", "called")
		w.WriteHeader(http.StatusOK)
	})

	handler := srv.addCacheControlHeaders(simpleHandler)

	req := httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify both headers are present
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("expected Cache-Control header, got %q", got)
	}
	if got := rec.Header().Get("X-Test-Handler"); got != "called" {
		t.Errorf("expected custom header from wrapped handler, got %q", got)
	}
}
