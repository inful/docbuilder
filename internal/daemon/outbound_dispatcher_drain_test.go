package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The documents that are still in the queue when the daemon stops must
// still be uploaded. The context of the run is canceled before the
// dispatcher is stopped: every upload with a canceled context fails.
func TestTheDrainOfTheDispatcherUploadsTheDocumentsThatAreLeft(t *testing.T) {
	received := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		BaseURL:   srv.URL,
		Token:     "t",
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	// The context of the run is canceled when the daemon stops.
	ctx, cancel := context.WithCancel(context.Background())
	d.Start(ctx)

	d.Enqueue([]byte("---\nuid: u1\nfingerprint: abc123\n---\n\n# Doc\n"), "doc.md")

	// The daemon stops: the run context is canceled first.
	cancel()
	d.Stop()

	select {
	case path := <-received:
		if path == "" {
			t.Fatal("the document did not arrive")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the document that was in the queue never arrived")
	}
}
