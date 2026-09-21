package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// OutboundDispatcher forwards per-document content to an external ragabast
// ingest endpoint. Documents are pushed asynchronously after docbuilder writes
// them to disk; ragabast is expected to handle its own persistent job queue
// and idempotency (via uid+fingerprint from the frontmatter).
//
// The dispatcher is daemon-mode-only and opt-in via configuration. When
// disabled or unconfigured, this type is not constructed and Enqueue is never
// called. The dispatcher itself is nil-safe so callers can pass a nil
// reference without guarding every call site.
type OutboundDispatcher struct {
	url    string
	token  string
	client *http.Client

	queue    chan ingestJob
	workers  int
	queueCap int

	wg sync.WaitGroup

	// Observability counters. Read via accessor methods; atomic so
	// concurrent increments from workers are safe.
	sendsTotal atomic.Int64
	dropsTotal atomic.Int64
	failsTotal atomic.Int64
}

// ingestJob is a single document pending ingest.
type ingestJob struct {
	content []byte
	path    string
}

// DispatcherConfig is the runtime configuration for OutboundDispatcher.
// Wiring code in NewDaemonWithConfigFile extracts these fields from
// config.Daemon.Outbound.RagabastConfig and resolves the bearer token from
// the named environment variable at startup.
type DispatcherConfig struct {
	// IngestURL is the full ragabast async ingest endpoint, e.g.
	// "https://ragabast.example.com/api/ingest/async".
	IngestURL string
	// Token is the bearer token to send in the Authorization header.
	// Empty is allowed (ragabast with auth_token: "" accepts anonymous).
	Token string
	// Workers is the number of concurrent POST goroutines.
	Workers int
	// QueueSize is the in-memory buffer capacity between Enqueue and POST.
	QueueSize int
	// Timeout is the per-POST HTTP timeout.
	Timeout time.Duration
}

// NewOutboundDispatcher constructs a dispatcher. It validates the URL is
// present and applies defaults for zero-valued Workers/QueueSize/Timeout.
func NewOutboundDispatcher(cfg DispatcherConfig) (*OutboundDispatcher, error) {
	if cfg.IngestURL == "" {
		return nil, errors.New("outbound dispatcher: ingest_url is required")
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = 4
	}
	queueCap := cfg.QueueSize
	if queueCap <= 0 {
		queueCap = 256
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &OutboundDispatcher{
		url:      cfg.IngestURL,
		token:    cfg.Token,
		client:   &http.Client{Timeout: timeout},
		queue:    make(chan ingestJob, queueCap),
		workers:  workers,
		queueCap: queueCap,
	}, nil
}

// Start launches worker goroutines bound to the given context. The context is
// propagated to in-flight HTTP requests so daemon shutdown cancels them.
// Idempotency is the caller's responsibility — the daemon lifecycle pairs
// Start with Stop.
func (d *OutboundDispatcher) Start(ctx context.Context) {
	if d == nil {
		return
	}
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker(ctx)
	}
	slog.Info("outbound dispatcher started",
		slog.String("url", d.url),
		slog.Int("workers", d.workers),
		slog.Int("queue_capacity", d.queueCap),
		slog.Bool("auth_enabled", d.token != ""))
}

// Stop closes the queue and waits for workers to drain. Cancelling the
// context passed to Start is the caller's responsibility — typically via the
// daemon's runCancel. Safe to call multiple times; subsequent calls are
// no-ops.
func (d *OutboundDispatcher) Stop() {
	if d == nil {
		return
	}
	// Drain-safe close: workers exit when the channel is closed and drained.
	// We intentionally do not close d.queue while workers might still be
	// sending on it — but workers only send to ragabast (external), never
	// back into d.queue, so a single close is safe.
	d.closeQueueOnce()
	d.wg.Wait()
	slog.Info("outbound dispatcher stopped",
		slog.Int64("sends_total", d.sendsTotal.Load()),
		slog.Int64("drops_total", d.dropsTotal.Load()),
		slog.Int64("fails_total", d.failsTotal.Load()))
}

// closeQueueOnce closes the job channel exactly once. Uses sync.Once via a
// flag because Stop can be called from multiple paths in tests.
var (
	closeOnce sync.Map // *OutboundDispatcher -> *sync.Once
)

func (d *OutboundDispatcher) closeQueueOnce() {
	v, _ := closeOnce.LoadOrStore(d, &sync.Once{})
	once := v.(*sync.Once)
	once.Do(func() {
		close(d.queue)
	})
}

// Enqueue submits a document for async ingest. Non-blocking: if the queue is
// full, the document is dropped with a counter increment. ragabast's
// uid+fingerprint dedup plus docbuilder's per-build re-emission make drops
// self-healing. Nil-safe so call sites don't need to guard.
func (d *OutboundDispatcher) Enqueue(content []byte, path string) {
	if d == nil {
		return
	}
	if len(content) == 0 {
		return
	}
	select {
	case d.queue <- ingestJob{content: content, path: path}:
	default:
		n := d.dropsTotal.Add(1)
		slog.Warn("outbound dispatcher queue full, dropping doc",
			slog.String("path", path),
			slog.Int64("queue_drops_total", n),
			slog.Int("queue_capacity", d.queueCap))
	}
}

// SendsTotal returns the number of successful POSTs (2xx response).
func (d *OutboundDispatcher) SendsTotal() int64 {
	if d == nil {
		return 0
	}
	return d.sendsTotal.Load()
}

// DropsTotal returns the number of enqueue drops (queue full at enqueue time).
func (d *OutboundDispatcher) DropsTotal() int64 {
	if d == nil {
		return 0
	}
	return d.dropsTotal.Load()
}

// FailsTotal returns the number of POSTs that returned non-2xx or errored.
func (d *OutboundDispatcher) FailsTotal() int64 {
	if d == nil {
		return 0
	}
	return d.failsTotal.Load()
}

func (d *OutboundDispatcher) worker(ctx context.Context) {
	defer d.wg.Done()
	for job := range d.queue {
		d.send(ctx, job)
	}
}

func (d *OutboundDispatcher) send(ctx context.Context, job ingestJob) {
	payload, err := json.Marshal(map[string]string{"content": string(job.content)})
	if err != nil {
		// Should never happen for a string->string map; log and bail.
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: marshal failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(payload))
	if err != nil {
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: build request failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.failsTotal.Add(1)
		slog.Warn("outbound dispatcher: POST failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		d.sendsTotal.Add(1)
	default:
		d.failsTotal.Add(1)
		slog.Warn("outbound dispatcher: non-2xx response, dropping",
			slog.String("path", job.path),
			slog.Int("status", resp.StatusCode))
	}
}

// Format implements fmt.Stringer for logs/debug. Returns the URL the
// dispatcher posts to so operators can confirm the target.
func (d *OutboundDispatcher) String() string {
	if d == nil {
		return "<nil>"
	}
	return fmt.Sprintf("OutboundDispatcher(url=%s, workers=%d, queue_capacity=%d)", d.url, d.workers, d.queueCap)
}
