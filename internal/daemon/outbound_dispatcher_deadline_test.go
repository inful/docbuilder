package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The deadline of the drain belongs to the drain. Armed at the start-up
// of the dispatcher, it ended the outbound ingest of a daemon that lives
// longer than it: every document after that failed with "context
// deadline exceeded".
func TestTheDispatcherUploadsLongAfterItStarted(t *testing.T) {
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
	// The deadline of the drain is short in this test: the upload has
	// to work after it passed.
	d.drainTimeout = 200 * time.Millisecond

	d.Start(context.Background())
	defer d.Stop()

	time.Sleep(400 * time.Millisecond)

	d.Enqueue([]byte("---\nuid: u1\nfingerprint: abc123\n---\n\n# Doc\n"), "doc.md")

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("the document was not uploaded: the deadline of the drain ended the ingest")
	}
}
