package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// statusRuntime is a Runtime whose status is configurable per test.
type statusRuntime struct {
	stubRuntime
	status string
}

func (r statusRuntime) GetStatus() string { return r.status }

func TestHandleReadiness_ReportsDaemonStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantStatus int
	}{
		{name: "running", status: "running", wantStatus: http.StatusOK},
		{name: "starting", status: "starting", wantStatus: http.StatusServiceUnavailable},
		{name: "error", status: "error", wantStatus: http.StatusServiceUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New(&config.Config{}, statusRuntime{status: tc.status}, Options{})

			rec := httptest.NewRecorder()
			s.handleReadiness(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ready", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestHandleSiteReadiness_DependsOnRenderedSite(t *testing.T) {
	outDir := t.TempDir()
	s := New(&config.Config{Output: config.OutputConfig{Directory: outDir}}, statusRuntime{status: "running"}, Options{})

	// No rendered site yet.
	rec := httptest.NewRecorder()
	s.handleSiteReadiness(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/site-ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	// A rendered site exists.
	if err := os.MkdirAll(filepath.Join(outDir, "public"), 0o750); err != nil {
		t.Fatalf("create public dir: %v", err)
	}
	rec = httptest.NewRecorder()
	s.handleSiteReadiness(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/site-ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// TestHandleSiteReadiness_IgnoresDaemonStatus pins that the render-state
// check is independent of the daemon status: a running daemon without a
// rendered site is still "site not ready".
func TestHandleSiteReadiness_IgnoresDaemonStatus(t *testing.T) {
	outDir := t.TempDir()
	s := New(&config.Config{Output: config.OutputConfig{Directory: outDir}}, statusRuntime{status: "starting"}, Options{})

	rec := httptest.NewRecorder()
	s.handleSiteReadiness(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/site-ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
