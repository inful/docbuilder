package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A document that arrives after the dispatcher was stopped must not take
// the daemon down. The queue of a stopped dispatcher is closed, and a send
// on a closed channel panics - in a build worker, outside the recovery of
// the HTTP server.
func TestOutboundDispatcher_DoesNotPanicAfterItWasStopped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL + "/api/ingest/file",
		Token:     "t",
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	d.Stop()

	// The panic would fail the test on its own.
	d.Enqueue([]byte("---\nuid: u1\n---\n\n# Doc\n"), "doc.md")
}

// A dispatcher that runs again after a stop has to accept documents again:
// the queue of the last run is closed, and its workers are gone.
func TestOutboundDispatcher_AcceptsDocumentsAfterItWasRestarted(t *testing.T) {
	received := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL + "/api/ingest/file",
		Token:     "t",
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	d.Start(context.Background())
	d.Stop()
	d.Start(context.Background())
	defer d.Stop()

	d.Enqueue([]byte("---\nuid: u1\n---\n\n# Doc\n"), "doc.md")

	select {
	case path := <-received:
		if path == "" {
			t.Fatal("the document did not arrive")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the document did not arrive")
	}
}
