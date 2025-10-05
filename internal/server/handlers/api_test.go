package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

type stubDaemon struct{}

func (s *stubDaemon) GetStatus() interface{}  { return "ready" }
func (s *stubDaemon) GetStartTime() time.Time { return time.Now().Add(-time.Hour) }

func TestHandleDocsStatus_OK(t *testing.T) {
	cfg := &config.Config{}
	h := NewAPIHandlers(cfg, &stubDaemon{})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
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
