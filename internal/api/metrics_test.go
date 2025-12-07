package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/observability"
)

func TestMetricsEndpoint(t *testing.T) {
	// Reset and create fresh metrics collector
	observability.ResetMetricsCollector()
	mc := observability.InitMetricsCollector()

	// Add some test data
	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordCacheHit("test")
	mc.RecordStorageOperation("put", 1024)

	// Create server and request
	server := NewServer(":0")
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain content type, got %s", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if !strings.Contains(body, "DocBuilder Metrics") {
		t.Error("expected metrics output to contain 'DocBuilder Metrics'")
	}
	if !strings.Contains(body, "Total Builds: 1") {
		t.Error("expected build count in metrics")
	}

	observability.ResetMetricsCollector()
}

func TestMetricsEndpointEmpty(t *testing.T) {
	// Reset metrics for clean slate
	observability.ResetMetricsCollector()
	observability.InitMetricsCollector()

	server := NewServer(":0")
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "DocBuilder Metrics") {
		t.Error("expected metrics output even when empty")
	}

	observability.ResetMetricsCollector()
}

func TestMetricsEndpointWithDetailedData(t *testing.T) {
	observability.ResetMetricsCollector()
	mc := observability.InitMetricsCollector()

	// Add more complex data
	mc.RecordBuildStart("build-1", "tenant-1")
	mc.RecordBuildStart("build-2", "tenant-2")
	mc.RecordCacheHit("sig1")
	mc.RecordCacheHit("sig2")
	mc.RecordCacheMiss("miss1")
	mc.RecordStage("clone", 100, true)
	mc.RecordStage("discover", 50, true)
	mc.RecordStorageOperation("put", 1024)
	mc.RecordStorageOperation("get", 512)
	mc.RecordDLQEvent()

	server := NewServer(":0")
	req, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify all metrics are present
	if !strings.Contains(body, "Total Builds: 2") {
		t.Error("expected 2 builds in metrics")
	}
	if !strings.Contains(body, "Cache Hits: 2") {
		t.Error("expected 2 cache hits in metrics")
	}
	if !strings.Contains(body, "DLQ Size: 1") {
		t.Error("expected DLQ size in metrics")
	}

	observability.ResetMetricsCollector()
}
