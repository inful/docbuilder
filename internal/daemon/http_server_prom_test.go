package daemon

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/build/queue"
)

// The Prometheus gauges must report the daemon they belong to. They used to
// read a package global, so a second daemon in the same process overwrote
// the numbers of the first one.
func TestPrometheusGaugesAreBoundToTheirDaemon(t *testing.T) {
	first := &Daemon{build: &buildStack{buildQueue: queue.NewBuildQueue(10, 0, &stubBuilder{})}}
	second := &Daemon{build: &buildStack{buildQueue: queue.NewBuildQueue(10, 0, &stubBuilder{})}}

	for _, id := range []string{"a"} {
		if err := first.build.buildQueue.Enqueue(&queue.BuildJob{ID: id}); err != nil {
			t.Fatalf("enqueue first: %v", err)
		}
	}
	for _, id := range []string{"a", "b"} {
		if err := second.build.buildQueue.Enqueue(&queue.BuildJob{ID: id}); err != nil {
			t.Fatalf("enqueue second: %v", err)
		}
	}

	firstGauge := scrapeGauge(t, prometheusOptionalHandler(first), "docbuilder_daemon_queue_length")
	secondGauge := scrapeGauge(t, prometheusOptionalHandler(second), "docbuilder_daemon_queue_length")

	if firstGauge != "1" {
		t.Fatalf("first daemon queue length = %s, want 1", firstGauge)
	}
	if secondGauge != "2" {
		t.Fatalf("second daemon queue length = %s, want 2", secondGauge)
	}
}

// scrapeGauge returns the value of a gauge from a Prometheus scrape.
func scrapeGauge(t *testing.T, handler http.Handler, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(rec, req)
	for line := range strings.SplitSeq(rec.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, name+" "); ok {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("gauge %s not found in the scrape:\n%s", name, rec.Body.String())
	return ""
}
