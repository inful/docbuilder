package daemon

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestOutboundDispatcher_PostsContentAndAuthHeader verifies the happy path:
// Enqueue submits a job, the worker POSTs {content: ...} with the configured
// bearer token, and the sends counter increments.
func TestOutboundDispatcher_PostsContentAndAuthHeader(t *testing.T) {
	var (
		gotMethod   atomic.Value
		gotPath     atomic.Value
		gotAuth     atomic.Value
		gotContent  atomic.Value
		gotCT       atomic.Value
		requestsHit atomic.Int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsHit.Add(1)
		gotMethod.Store(r.Method)
		gotPath.Store(r.URL.Path)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotCT.Store(r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		gotContent.Store(string(body))
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"j-1","status":"pending","status_url":"/api/ingest/jobs/j-1"}`))
	}))
	defer srv.Close()

	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL + "/api/ingest/async",
		Token:     "test-token-xyz",
		Workers:   1,
		QueueSize: 4,
		Timeout:   2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue([]byte("---\nuid: doc-1\n---\n# Hello\n"), "docs/index.md")

	// Wait for the request to land.
	deadline := time.Now().Add(2 * time.Second)
	for requestsHit.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if requestsHit.Load() == 0 {
		t.Fatalf("expected at least 1 request, got 0; sends=%d fails=%d", d.SendsTotal(), d.FailsTotal())
	}

	if got, _ := gotMethod.Load().(string); got != http.MethodPost {
		t.Errorf("method = %q, want POST", got)
	}
	if got, _ := gotPath.Load().(string); got != "/api/ingest/async" {
		t.Errorf("path = %q, want /api/ingest/async", got)
	}
	if got, _ := gotAuth.Load().(string); got != "Bearer test-token-xyz" {
		t.Errorf("auth = %q, want Bearer test-token-xyz", got)
	}
	if got, _ := gotCT.Load().(string); got != "application/json" {
		t.Errorf("content-type = %q, want application/json", got)
	}

	// Verify body shape: {content: <raw markdown>}
	bodyStr, _ := gotContent.Load().(string)
	var parsed struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &parsed); err != nil {
		t.Fatalf("body is not JSON: %v (body=%q)", err, bodyStr)
	}
	if !strings.HasPrefix(parsed.Content, "---\nuid: doc-1") {
		t.Errorf("content payload = %q, want to start with frontmatter", parsed.Content)
	}

	if d.SendsTotal() != 1 {
		t.Errorf("SendsTotal = %d, want 1", d.SendsTotal())
	}
	if d.FailsTotal() != 0 {
		t.Errorf("FailsTotal = %d, want 0", d.FailsTotal())
	}
	if d.DropsTotal() != 0 {
		t.Errorf("DropsTotal = %d, want 0", d.DropsTotal())
	}
}

// TestOutboundDispatcher_NoAuthHeaderWhenTokenEmpty verifies that empty token
// produces no Authorization header (ragabast accepts anonymous when
// auth_token is empty).
func TestOutboundDispatcher_NoAuthHeaderWhenTokenEmpty(t *testing.T) {
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 1,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue([]byte("content"), "p.md")
	time.Sleep(100 * time.Millisecond)

	if got, _ := gotAuth.Load().(string); got != "" {
		t.Errorf("auth header = %q, want empty", got)
	}
}

// TestOutboundDispatcher_ServerError_LogsAndDrops verifies 5xx responses
// increment FailsTotal and do not retry (ragabast's queue handles durability).
func TestOutboundDispatcher_ServerError_LogsAndDrops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue([]byte("content"), "p.md")
	time.Sleep(150 * time.Millisecond)

	if d.FailsTotal() != 1 {
		t.Errorf("FailsTotal = %d, want 1", d.FailsTotal())
	}
	if d.SendsTotal() != 0 {
		t.Errorf("SendsTotal = %d, want 0", d.SendsTotal())
	}
}

// TestOutboundDispatcher_RateLimited_LogsAndDrops verifies 429 responses
// increment FailsTotal and are not retried within docbuilder.
func TestOutboundDispatcher_RateLimited_LogsAndDrops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue([]byte("content"), "p.md")
	time.Sleep(150 * time.Millisecond)

	if d.FailsTotal() != 1 {
		t.Errorf("FailsTotal = %d, want 1", d.FailsTotal())
	}
}

// TestOutboundDispatcher_QueueFull_Drops verifies that when the buffer is
// saturated, additional Enqueue calls drop with a counter increment rather
// than blocking. The server handler blocks on a channel so the worker stays
// busy and the queue stays full.
func TestOutboundDispatcher_QueueFull_Drops(t *testing.T) {
	blockHandler := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		<-blockHandler
		w.WriteHeader(http.StatusAccepted)
	}))

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 2,
	})
	d.Start(t.Context())

	// First enqueue: worker takes it and blocks on the server.
	d.Enqueue([]byte("content-1"), "p1.md")

	// Wait for the worker to actually be in-flight so the buffer is empty.
	deadline := time.Now().Add(1 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hits.Load() == 0 {
		close(blockHandler)
		srv.Close()
		d.Stop()
		t.Fatal("server never received the first request")
	}

	// Fill the buffer (capacity 2): these two are queued behind the worker.
	d.Enqueue([]byte("content-2"), "p2.md")
	d.Enqueue([]byte("content-3"), "p3.md")

	// These 10 must overflow and be dropped at enqueue time.
	for i := 0; i < 10; i++ {
		d.Enqueue([]byte("overflow"), "over.md")
	}

	if d.DropsTotal() == 0 {
		t.Errorf("DropsTotal = 0, want >0; buffer should have overflowed")
	}

	// Cleanup in the order that lets the dispatcher drain cleanly:
	// 1. Unblock the in-flight handler so it can finish.
	// 2. Close the server to kill any leftover connections after drain.
	// 3. Stop the dispatcher.
	close(blockHandler)
	srv.Close()
	d.Stop()
}

// TestOutboundDispatcher_NilSafe verifies Enqueue, Stop, and counter methods
// on a nil pointer don't panic. This is the contract call sites rely on.
func TestOutboundDispatcher_NilSafe(t *testing.T) {
	var d *OutboundDispatcher // nil

	// Must not panic.
	d.Enqueue([]byte("content"), "p.md")
	d.Start(t.Context())
	d.Stop()

	if got := d.SendsTotal(); got != 0 {
		t.Errorf("nil SendsTotal = %d, want 0", got)
	}
	if got := d.DropsTotal(); got != 0 {
		t.Errorf("nil DropsTotal = %d, want 0", got)
	}
	if got := d.FailsTotal(); got != 0 {
		t.Errorf("nil FailsTotal = %d, want 0", got)
	}
	if got := d.String(); got != "<nil>" {
		t.Errorf("nil String = %q, want <nil>", got)
	}
}

// TestOutboundDispatcher_EmptyContent_NoOp verifies that zero-byte content is
// dropped silently at enqueue time (nothing to send).
func TestOutboundDispatcher_EmptyContent_NoOp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 4,
	})
	d.Start(t.Context())
	defer d.Stop()

	d.Enqueue(nil, "p.md")
	d.Enqueue([]byte{}, "p.md")
	time.Sleep(50 * time.Millisecond)

	if hits.Load() != 0 {
		t.Errorf("server hits = %d, want 0 for empty content", hits.Load())
	}
	if d.SendsTotal() != 0 || d.FailsTotal() != 0 || d.DropsTotal() != 0 {
		t.Errorf("counters incremented for empty content: sends=%d fails=%d drops=%d",
			d.SendsTotal(), d.FailsTotal(), d.DropsTotal())
	}
}

// TestOutboundDispatcher_StopDrainsQueue verifies Stop blocks until the queue
// is empty. With a fast server, all enqueued jobs should be flushed before
// Stop returns.
func TestOutboundDispatcher_StopDrainsQueue(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   2,
		QueueSize: 16,
	})
	d.Start(t.Context())

	const n = 5
	for i := 0; i < n; i++ {
		d.Enqueue([]byte("content"), "p.md")
	}

	// Stop should drain the queue.
	d.Stop()

	if got := hits.Load(); got != int32(n) {
		t.Errorf("server hits after Stop = %d, want %d", got, n)
	}
	if got := d.SendsTotal(); got != int64(n) {
		t.Errorf("SendsTotal = %d, want %d", got, n)
	}
}

// TestOutboundDispatcher_StopMultipleTimesSafe verifies that calling Stop
// after Stop is a no-op (closes the queue only once).
func TestOutboundDispatcher_StopMultipleTimesSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   1,
		QueueSize: 1,
	})
	d.Start(t.Context())

	// Must not panic on repeated close.
	d.Stop()
	d.Stop()
}

// TestOutboundDispatcher_RequiresIngestURL verifies that constructing without
// a URL is rejected at config time, not at first send.
func TestOutboundDispatcher_RequiresIngestURL(t *testing.T) {
	if _, err := NewOutboundDispatcher(DispatcherConfig{}); err == nil {
		t.Fatal("expected error for empty IngestURL, got nil")
	}
}

// TestOutboundDispatcher_DefaultsApplied verifies that zero-valued
// workers/queue/timeout get sensible defaults.
func TestOutboundDispatcher_DefaultsApplied(t *testing.T) {
	d, err := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: "http://example.invalid", // unused in this test
	})
	if err != nil {
		t.Fatalf("NewOutboundDispatcher: %v", err)
	}
	if d.workers != 4 {
		t.Errorf("workers default = %d, want 4", d.workers)
	}
	if d.queueCap != 256 {
		t.Errorf("queueCap default = %d, want 256", d.queueCap)
	}
	if d.client.Timeout != 10*time.Second {
		t.Errorf("client.Timeout default = %v, want 10s", d.client.Timeout)
	}
}

// TestOutboundDispatcher_ConcurrentEnqueue verifies thread-safety of the
// counter and queue under concurrent enqueues from multiple goroutines.
func TestOutboundDispatcher_ConcurrentEnqueue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	d, _ := NewOutboundDispatcher(DispatcherConfig{
		IngestURL: srv.URL,
		Workers:   4,
		QueueSize: 64,
	})
	d.Start(t.Context())

	const goroutines = 8
	const perGoroutine = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				d.Enqueue([]byte("content"), "p.md")
			}
		}()
	}
	wg.Wait()

	d.Stop()

	total := d.SendsTotal() + d.FailsTotal() + d.DropsTotal()
	if total != int64(goroutines*perGoroutine) {
		t.Errorf("total enqueued = sends(%d)+fails(%d)+drops(%d) = %d, want %d",
			d.SendsTotal(), d.FailsTotal(), d.DropsTotal(), total, goroutines*perGoroutine)
	}
}
