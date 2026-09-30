package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/frontmatter"
)

// ragabastUploadPath and ragabastPreflightPath are the stable endpoint
// paths ragabast exposes. Both are appended to a single base URL so
// operators only configure the host. Hard-coding them here is safe as
// long as ragabast's API contract holds; if either path changes
// upstream we'll need a config override.
const (
	ragabastUploadPath    = "/api/ingest/file"
	ragabastPreflightPath = "/api/documents/%s/fingerprint"
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
//
// A single baseURL drives both endpoints: the upload path is derived as
// {baseURL}/api/ingest/file and the preflight path is derived as
// {baseURL}/api/documents/<uid>/fingerprint. Operators configure the host
// once and cannot accidentally POST to "/" by forgetting a path component.
//
// When preflight is enabled (default when a base URL can be resolved), the
// worker consults the preflight endpoint before POSTing. A matching stored
// fingerprint means the document is unchanged since the last ingest and the
// upload is skipped (counter increments). Mismatch, 404, transient HTTP
// errors, and parse failures fall through to the upload (fail-open) —
// ragabast's own dedup catches duplicates on the server side. Auth failures
// (401/403) do NOT upload; they count as a regular fail.
type OutboundDispatcher struct {
	baseURL string
	token   string
	client  *http.Client

	// preflightBase is the URL prefix used to build preflight requests.
	// It equals baseURL by default and may be overridden via
	// DispatcherConfig.PreflightBaseURL for environments where preflight
	// routes through a different host. Empty means preflight is disabled.
	preflightBase string

	queue    chan ingestJob
	workers  int
	queueCap int

	wg sync.WaitGroup

	// Observability counters. Read via accessor methods; atomic so
	// concurrent increments from workers are safe.
	sendsTotal          atomic.Int64
	dropsTotal          atomic.Int64
	failsTotal          atomic.Int64
	preflightSkipsTotal atomic.Int64
	preflightErrorsTotal atomic.Int64
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
	// BaseURL is the preferred field: the ragabast host (scheme+host).
	// Both the upload path ({BaseURL}/api/ingest/file) and the preflight
	// path ({BaseURL}/api/documents/<uid>/fingerprint) are derived from
	// this. Path components are stripped — operators only configure the
	// host.
	BaseURL string
	// IngestURL is the deprecated full endpoint URL. When BaseURL is empty
	// and IngestURL is set, the dispatcher derives scheme+host from
	// IngestURL and uses that as the base; the path component is silently
	// discarded. Logged as a deprecation warning at construction time.
	// Migrate by replacing `ingest_url: …/api/ingest/file` with
	// `ragabast_base_url: …`.
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
	// PreflightBaseURL overrides the preflight base when set, allowing
	// preflight to hit a different host than the upload. When empty, the
	// preflight base is the resolved base URL (BaseURL or derived from
	// IngestURL).
	PreflightBaseURL string
	// PreflightEnabled toggles preflight. Nil defaults to true whenever a
	// base URL is resolvable. Explicit false disables preflight.
	PreflightEnabled *bool
}

// NewOutboundDispatcher constructs a dispatcher. It resolves the ragabast
// base URL from the (preferred) BaseURL field or, falling back, derives
// it from the deprecated IngestURL. Both the upload and preflight
// endpoints are then derived from this single base URL — see
// ragabastUploadPath / ragabastPreflightPath. Defaults are applied for
// zero-valued Workers/QueueSize/Timeout.
func NewOutboundDispatcher(cfg DispatcherConfig) (*OutboundDispatcher, error) {
	// Resolve the base URL. BaseURL wins; if only IngestURL is set, derive
	// scheme+host from it (path is discarded) and log a deprecation
	// warning so operators migrate.
	baseURL := cfg.BaseURL
	if baseURL == "" && cfg.IngestURL != "" {
		derived, err := deriveBaseURL(cfg.IngestURL)
		if err != nil {
			return nil, fmt.Errorf("outbound dispatcher: invalid ingest_url: %w", err)
		}
		baseURL = derived
		slog.Warn("outbound dispatcher: ingest_url is deprecated; prefer ragabast_base_url",
			slog.String("hint", "set ragabast_base_url to the host (e.g. https://ragabast.example.com); path is no longer needed since the dispatcher derives /api/ingest/file and /api/documents/<uid>/fingerprint automatically"))
	}
	if baseURL == "" {
		return nil, errors.New("outbound dispatcher: ragabast_base_url (or deprecated ingest_url) is required")
	}
	// Validate the resolved base URL is parseable. We already accept the
	// result of deriveBaseURL which guarantees scheme+host, but if the
	// operator set BaseURL directly it could be anything.
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("outbound dispatcher: invalid ragabast_base_url %q: %w", baseURL, err)
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

	// Preflight base URL resolution.
	//
	// Priority:
	//   1. Explicit PreflightBaseURL in config (escape hatch: preflight
	//      hits a different host than the upload).
	//   2. Otherwise, share the resolved base URL.
	//   3. PreflightEnabled==false disables preflight outright.
	preflightBase := ""
	preflightWanted := cfg.PreflightEnabled == nil || *cfg.PreflightEnabled
	if preflightWanted {
		if cfg.PreflightBaseURL != "" {
			preflightBase = cfg.PreflightBaseURL
		} else {
			preflightBase = baseURL
		}
	}

	return &OutboundDispatcher{
		baseURL:       baseURL,
		token:         cfg.Token,
		client:        &http.Client{Timeout: timeout},
		queue:         make(chan ingestJob, queueCap),
		workers:       workers,
		queueCap:      queueCap,
		preflightBase: preflightBase,
	}, nil
}

// deriveBaseURL returns scheme+host from a full URL, stripping any path,
// query, and fragment. Used to derive the ragabast API base from the
// configured IngestURL when no PreflightBaseURL is supplied.
func deriveBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse ingest_url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("ingest_url must include scheme and host")
	}
	return u.Scheme + "://" + u.Host, nil
}

// Start launches worker goroutines bound to the given context. The context is
// propagated to in-flight HTTP requests so daemon shutdown cancels them.
// Idempotency is the caller's responsibility — the daemon lifecycle pairs
// Start with Stop.
func (d *OutboundDispatcher) Start(ctx context.Context) {
	if d == nil {
		return
	}
	for range d.workers {
		d.wg.Add(1)
		go d.worker(ctx)
	}
	slog.Info("outbound dispatcher started",
		slog.String("base_url", d.baseURL),
		slog.Int("workers", d.workers),
		slog.Int("queue_capacity", d.queueCap),
		slog.Bool("auth_enabled", d.token != ""),
		slog.Bool("preflight_enabled", d.preflightBase != ""))
}

// Stop closes the queue and waits for workers to drain. Canceling the
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
		slog.Int64("fails_total", d.failsTotal.Load()),
		slog.Int64("preflight_skips_total", d.preflightSkipsTotal.Load()),
		slog.Int64("preflight_errors_total", d.preflightErrorsTotal.Load()))
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

// PreflightSkipsTotal returns the number of preflight checks that matched
// ragabast's stored fingerprint and skipped the upload.
func (d *OutboundDispatcher) PreflightSkipsTotal() int64 {
	if d == nil {
		return 0
	}
	return d.preflightSkipsTotal.Load()
}

// PreflightErrorsTotal returns the number of preflight checks that failed
// transiently (network, 5xx, parse) and fell through to the upload.
func (d *OutboundDispatcher) PreflightErrorsTotal() int64 {
	if d == nil {
		return 0
	}
	return d.preflightErrorsTotal.Load()
}

func (d *OutboundDispatcher) worker(ctx context.Context) {
	defer d.wg.Done()
	for job := range d.queue {
		d.send(ctx, job)
	}
}

// send runs preflight (when enabled) and then conditionally POSTs the
// document. The preflight URL is constructed from d.preflightBase plus
// the uid parsed from the document's frontmatter. The frontmatter
// fingerprint is the authoritative local value to compare against.
func (d *OutboundDispatcher) send(ctx context.Context, job ingestJob) {
	if d.preflightBase != "" {
		uid, fp := parseUIDAndFingerprint(job.content)
		if uid != "" && fp != "" {
			if !d.preflight(ctx, uid, fp, job.path) {
				// preflight returned false → skip the upload
				return
			}
		}
		// Anything missing (uid or fingerprint, or no frontmatter at all)
		// means we have nothing to ask ragabast about. Fall through to
		// the upload so we still push the doc.
	}
	d.upload(ctx, job)
}

// upload POSTs the document to ragabast's ingest endpoint as
// multipart/form-data with a single `file` field. This matches the
// /api/ingest/file contract in the ragabast example script; the older
// /api/ingest/async JSON path was retired upstream.
func (d *OutboundDispatcher) upload(ctx context.Context, job ingestJob) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", job.path)
	if err != nil {
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: create form file failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}
	if _, err := fw.Write(job.content); err != nil {
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: write form file failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}
	// Closes the multipart writer and writes its terminating boundary.
	// Must be called before sending so the boundary is appended to the body.
	if err := mw.Close(); err != nil {
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: close multipart writer failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+ragabastUploadPath, &body)
	if err != nil {
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: build request failed",
			slog.String("path", job.path),
			slog.String("error", err.Error()))
		return
	}
	// multipart.Writer.FormDataContentType() returns the value with the
	// boundary parameter — do NOT set "application/json" or any other
	// Content-Type, or the boundary will be missing/wrong.
	req.Header.Set("Content-Type", mw.FormDataContentType())
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

// preflight consults ragabast's fingerprint endpoint and reports whether the
// caller should upload the document. Returns:
//
//	false → fingerprint matches stored value, upload skipped (counter++)
//	true  → caller should proceed with the upload
//
// Failure modes:
//
//	200 + matching fingerprint  → false (skip)
//	200 + mismatched fingerprint → true (upload; ragabast will update)
//	200 + unparseable body       → true (fail-open, preflight_errors++)
//	404                          → true (new doc on ragabast)
//	401 / 403                    → false (caller counts it as fail via
//	                                 d.failsTotal; auth is broken, retry
//	                                 won't help)
//	5xx / network / timeout      → true (fail-open, preflight_errors++)
//	empty UID or fingerprint     → true (caller already short-circuited;
//	                                 we don't get here in normal flow)
func (d *OutboundDispatcher) preflight(ctx context.Context, uid, fp, path string) bool {
	target := d.preflightBase + fmt.Sprintf(ragabastPreflightPath, url.PathEscape(uid))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		// Path construction failed — extremely unlikely since the base
		// was already validated in NewOutboundDispatcher. Fail-open.
		d.preflightErrorsTotal.Add(1)
		slog.Warn("outbound dispatcher: preflight build request failed",
			slog.String("path", path),
			slog.String("error", err.Error()))
		return true
	}
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		d.preflightErrorsTotal.Add(1)
		slog.Warn("outbound dispatcher: preflight GET failed",
			slog.String("path", path),
			slog.String("error", err.Error()))
		return true
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		body, _ := io.ReadAll(resp.Body)
		var stored struct {
			Fingerprint string `json:"fingerprint"`
		}
		if err := json.Unmarshal(body, &stored); err != nil {
			d.preflightErrorsTotal.Add(1)
			slog.Warn("outbound dispatcher: preflight body parse failed",
				slog.String("path", path),
				slog.String("error", err.Error()))
			return true
		}
		if stored.Fingerprint == fp {
			d.preflightSkipsTotal.Add(1)
			slog.Debug("outbound dispatcher: preflight match, skipping upload",
				slog.String("path", path),
				slog.String("uid", uid))
			return false
		}
		slog.Debug("outbound dispatcher: preflight fingerprint mismatch, will upload",
			slog.String("path", path),
			slog.String("uid", uid),
			slog.String("stored", stored.Fingerprint),
			slog.String("local", fp))
		return true

	case resp.StatusCode == http.StatusNotFound:
		// Document not present on the server yet. Upload.
		return true

	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		// Auth is broken; retrying the upload would just waste bandwidth
		// and potentially leak a doc with wrong auth. Count as a fail so
		// operators see the regression in metrics.
		d.failsTotal.Add(1)
		slog.Error("outbound dispatcher: preflight auth failed; not uploading",
			slog.String("path", path),
			slog.Int("status", resp.StatusCode))
		return false

	default:
		// 5xx and any other unexpected status: fail-open so transient
		// ragabast hiccups don't silently drop coverage. Counter makes
		// the rate visible.
		d.preflightErrorsTotal.Add(1)
		slog.Warn("outbound dispatcher: preflight non-2xx, falling through to upload",
			slog.String("path", path),
			slog.Int("status", resp.StatusCode))
		return true
	}
}

// parseUIDAndFingerprint pulls uid and fingerprint out of the document's
// YAML frontmatter. Returns empty strings for any field that is missing or
// cannot be parsed. UID and fingerprint are the only fields docbuilder needs
// for the ragabast preflight contract.
func parseUIDAndFingerprint(content []byte) (uid, fingerprint string) {
	fmBytes, _, had, _, err := frontmatter.Split(content)
	if err != nil || !had || len(fmBytes) == 0 {
		return "", ""
	}
	fields, err := frontmatter.ParseYAML(fmBytes)
	if err != nil {
		return "", ""
	}
	if v, ok := fields["uid"].(string); ok {
		uid = v
	}
	if v, ok := fields["fingerprint"].(string); ok {
		fingerprint = v
	}
	return uid, fingerprint
}

// String returns a human-readable representation of the dispatcher showing
// the configured URL, worker count, and queue capacity. Implements
// fmt.Stringer for logs/debug.
func (d *OutboundDispatcher) String() string {
	if d == nil {
		return "<nil>"
	}
	return fmt.Sprintf("OutboundDispatcher(base_url=%s, workers=%d, queue_capacity=%d, preflight=%v)",
		d.baseURL, d.workers, d.queueCap, d.preflightBase != "")
}
