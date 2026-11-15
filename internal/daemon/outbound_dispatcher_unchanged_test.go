package daemon

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A document this dispatcher already uploaded, unchanged, needs no
// request at all. Without this, a rebuild that changed one page still
// cost two HTTP calls for every page of the site.
func TestOutboundDispatcher_UnchangedDocumentCostsNoRequest(t *testing.T) {
	const fp = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// The preflight reports a stale fingerprint, so the upload happens.
	const storedFP = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.Method + " " + r.URL.Path {
		case "GET /api/documents/uid-1/fingerprint":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uid":"uid-1","fingerprint":"` + storedFP + `"}`))
		case "POST /api/ingest/file":
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	d.Start(t.Context())
	defer d.Stop()

	// First build: the document is new, so it is uploaded.
	d.Enqueue(docWithUIDAndFingerprint(fp), "docs/x.md")
	waitFor(t, "the upload", func() bool { return d.SendsTotal() == 1 })

	// Second build: the same content. No request may be made.
	d.Enqueue(docWithUIDAndFingerprint(fp), "docs/x.md")
	waitFor(t, "the unchanged skip", func() bool { return d.UnchangedTotal() == 1 })

	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2 (the first build's preflight and upload, nothing after)", got)
	}
	if d.UnchangedTotal() != 1 {
		t.Fatalf("UnchangedTotal = %d, want 1", d.UnchangedTotal())
	}
	if d.SendsTotal() != 1 {
		t.Fatalf("SendsTotal = %d, want 1", d.SendsTotal())
	}
}

// A document whose content changed is uploaded again.
func TestOutboundDispatcher_ChangedDocumentIsUploadedAgain(t *testing.T) {
	const firstFP = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secondFP = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// The preflight reports a stale fingerprint, so the upload happens.
	const storedFP = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	var uploads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /api/documents/uid-1/fingerprint":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uid":"uid-1","fingerprint":"` + storedFP + `"}`))
		case "POST /api/ingest/file":
			uploads.Add(1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL:        srv.URL + "/api/ingest/file",
		PreflightBaseURL: srv.URL,
		Workers:          1,
		QueueSize:        4,
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(docWithUIDAndFingerprint(firstFP), "docs/x.md")
	waitFor(t, "the first upload", func() bool { return d.SendsTotal() == 1 })

	d.Enqueue(docWithUIDAndFingerprint(secondFP), "docs/x.md")
	waitFor(t, "the second upload", func() bool { return d.SendsTotal() == 2 })

	if uploads.Load() != 2 {
		t.Fatalf("uploads = %d, want 2", uploads.Load())
	}
}
