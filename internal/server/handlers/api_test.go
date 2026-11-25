package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/server/responses"
)

type stubDaemon struct{}

func (s *stubDaemon) GetStatus() string       { return "ready" }
func (s *stubDaemon) GetStartTime() time.Time { return time.Now().Add(-time.Hour) }
func (s *stubDaemon) BuildStats() (responses.BuildStatistics, *responses.BuildInfo) {
	return responses.BuildStatistics{}, nil
}

func TestHandleDocsStatus_OK(t *testing.T) {
	cfg := &config.Config{}
	h := NewAPIHandlers(cfg, &stubDaemon{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	h.HandleDocsStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct == "" || ct[:16] != "application/json" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
}

// The docs port is public: the answer does not tell where the daemon
// writes the site.
func TestHandleDocsStatus_DoesNotExposeTheOutputDirectory(t *testing.T) {
	cfg := &config.Config{Output: config.OutputConfig{Directory: "/srv/site"}}
	h := NewAPIHandlers(cfg, &stubDaemon{})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	h.HandleDocsStatus(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "output_dir") || strings.Contains(body, "/srv/site") {
		t.Fatalf("the answer holds the output directory: %s", body)
	}
}
