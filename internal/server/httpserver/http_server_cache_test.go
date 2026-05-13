package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

const immutableCacheControl = "public, max-age=31536000, immutable"

// TestCacheControlHeaders verifies that appropriate Cache-Control headers are set for different asset types.
func TestCacheControlHeaders(t *testing.T) {
	tests := []struct {
		path          string
		expectedCache string
	}{
		// CSS and JavaScript - immutable, 1 year
		{"/assets/main.css", immutableCacheControl},
		{"/js/bundle.js", immutableCacheControl},
		{"/static/app.min.js", immutableCacheControl},

		// Web fonts - immutable, 1 year
		{"/fonts/roboto.woff2", immutableCacheControl},
		{"/fonts/icons.woff", immutableCacheControl},
		{"/static/font.ttf", immutableCacheControl},

		// Images - 1 week
		{"/images/logo.png", cacheControlWeek},
		{"/assets/hero.jpg", cacheControlWeek},
		{"/static/icon.svg", cacheControlWeek},
		{"/favicon.ico", cacheControlWeek},

		// Downloadable files - 1 day
		{"/downloads/manual.pdf", cacheControlDay},
		{"/files/archive.zip", cacheControlDay},

		// JSON (non-search) - 5 minutes
		{"/data/config.json", cacheControlFiveMinute},
		{"/api-data.json", cacheControlFiveMinute},

		// XML - 1 hour
		{"/sitemap.xml", cacheControlHour},
		{"/feed.xml", cacheControlHour},

		// HTML and root - no cache
		{"/index.html", cacheControlNoCache},
		{"/docs/guide.html", cacheControlNoCache},
		{"/", cacheControlNoCache},
		{"/docs/", cacheControlNoCache},

		// Search index - no cache header (special case)
		{"/search-index.json", ""},
		{"/idx.search.json", ""},
	}

	// Create a minimal server instance
	cfg := &config.Config{Daemon: &config.DaemonConfig{}}
	srv := &Server{cfg: cfg}

	// Simple handler that just returns 200 OK
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with cache control middleware
	handler := srv.addCacheControlHeaders(simpleHandler)

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			gotCache := rec.Header().Get("Cache-Control")
			if gotCache != tt.expectedCache {
				t.Errorf("path %s: expected Cache-Control %q, got %q", tt.path, tt.expectedCache, gotCache)
			}
		})
	}
}

// TestCacheControlNoInterferenceWithLiveReload ensures cache headers don't interfere with other middleware.
func TestCacheControlNoInterferenceWithLiveReload(t *testing.T) {
	cfg := &config.Config{
		Daemon: &config.DaemonConfig{},
		Build:  config.BuildConfig{LiveReload: true},
	}
	srv := &Server{cfg: cfg}

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set a custom header to verify the handler was called
		w.Header().Set("X-Test-Handler", "called")
		w.WriteHeader(http.StatusOK)
	})

	handler := srv.addCacheControlHeaders(simpleHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/static/app.css", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify both headers are present
	if got := rec.Header().Get("Cache-Control"); got != immutableCacheControl {
		t.Errorf("expected Cache-Control header, got %q", got)
	}
	if got := rec.Header().Get("X-Test-Handler"); got != "called" {
		t.Errorf("expected custom header from wrapped handler, got %q", got)
	}
}
