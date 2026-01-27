package daemon

import (
	"context"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/daemon/events"
	ferrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

type BuildDebouncerConfig struct {
	QuietWindow time.Duration
	MaxDelay    time.Duration

	// CheckBuildRunning reports whether a build is currently running.
	// When true, the debouncer will avoid emitting BuildNow and will instead
	// schedule exactly one follow-up build after the running build finishes.
	CheckBuildRunning func() bool

	// PollInterval controls how often the debouncer polls for build completion
	// after it has detected that a build is running.
	PollInterval time.Duration
}

// BuildDebouncer coalesces bursts of BuildRequested events into a single BuildNow.
//
// It implements the key daemon behavior required by ADR-021:
//   - quiet window debounce
//   - max delay (cannot postpone indefinitely)
//   - if a build is already running, queue exactly one follow-up
//
// It is safe to run as a single goroutine.
type BuildDebouncer struct {
	bus *events.Bus
	cfg BuildDebouncerConfig

	mu        sync.Mutex
	readyOnce sync.Once
	ready     chan struct{}

	pending         bool
	pendingAfterRun bool
	firstRequestAt  time.Time
	lastRequestAt   time.Time
	lastReason      string
	lastRepoURL     string
	requestCount    int
	pollingAfterRun bool
}

func NewBuildDebouncer(bus *events.Bus, cfg BuildDebouncerConfig) (*BuildDebouncer, error) {
	if bus == nil {
		return nil, ferrors.ValidationError("bus is required").Build()
	}
	if cfg.QuietWindow <= 0 {
		return nil, ferrors.ValidationError("quiet window must be > 0").Build()
	}
	if cfg.MaxDelay <= 0 {
		return nil, ferrors.ValidationError("max delay must be > 0").Build()
	}
	if cfg.CheckBuildRunning == nil {
		cfg.CheckBuildRunning = func() bool { return false }
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}

	return &BuildDebouncer{bus: bus, cfg: cfg, ready: make(chan struct{})}, nil
}

// Ready is closed once Run has fully initialized and subscribed to events.
// This is primarily intended for tests and deterministic startup sequencing.
func (d *BuildDebouncer) Ready() <-chan struct{} {
	return d.ready
}

func (d *BuildDebouncer) Run(ctx context.Context) error {
	if ctx == nil {
		return ferrors.ValidationError("context cannot be nil").Build()
	}

	reqCh, unsubscribe := events.Subscribe[events.BuildRequested](d.bus, 64)
	defer unsubscribe()

	d.readyOnce.Do(func() { close(d.ready) })

	quietTimer := time.NewTimer(time.Hour)
	if !quietTimer.Stop() {
		select {
		case <-quietTimer.C:
		default:
		}
	}
	maxTimer := time.NewTimer(time.Hour)
	if !maxTimer.Stop() {
		select {
		case <-maxTimer.C:
		default:
		}
	}
	pollTimer := time.NewTimer(time.Hour)
	if !pollTimer.Stop() {
		select {
		case <-pollTimer.C:
		default:
		}
	}

	var (
		quietC <-chan time.Time
		maxC   <-chan time.Time
		pollC  <-chan time.Time
	)

	resetTimer := func(t *time.Timer, after time.Duration) {
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(after)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case req, ok := <-reqCh:
			if !ok {
				return nil
			}
			d.onRequest(req)

			resetTimer(quietTimer, d.cfg.QuietWindow)
			quietC = quietTimer.C

			if d.shouldStartMaxTimer() {
				resetTimer(maxTimer, d.cfg.MaxDelay)
				maxC = maxTimer.C
			}

		case <-quietC:
			if d.tryEmit(ctx, "quiet") {
				quietC = nil
				maxC = nil
			}
			// else: build running; we keep pollingAfterRun until completion.

		case <-maxC:
			if d.tryEmit(ctx, "max_delay") {
				quietC = nil
				maxC = nil
			}

		case <-pollC:
			if d.tryEmitAfterRunning(ctx) {
				pollC = nil
				quietC = nil
				maxC = nil
				continue
			}
			resetTimer(pollTimer, d.cfg.PollInterval)
			pollC = pollTimer.C
		}

		// Start polling only when we have pendingAfterRun.
		if d.shouldPollAfterRun() && pollC == nil {
			resetTimer(pollTimer, d.cfg.PollInterval)
			pollC = pollTimer.C
		}
	}
}

func (d *BuildDebouncer) onRequest(req events.BuildRequested) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := req.RequestedAt
	if now.IsZero() {
		now = time.Now()
	}

	if !d.pending {
		d.pending = true
		d.firstRequestAt = now
		d.requestCount = 0
	}

	d.lastRequestAt = now
	d.lastReason = req.Reason
	d.lastRepoURL = req.RepoURL
	d.requestCount++
}

func (d *BuildDebouncer) shouldStartMaxTimer() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pending && d.requestCount == 1
}

func (d *BuildDebouncer) shouldPollAfterRun() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pendingAfterRun && !d.pollingAfterRun
}

func (d *BuildDebouncer) tryEmit(ctx context.Context, cause string) bool {
	d.mu.Lock()
	pending := d.pending
	first := d.firstRequestAt
	last := d.lastRequestAt
	count := d.requestCount
	reason := d.lastReason
	repoURL := d.lastRepoURL
	if !pending {
		d.mu.Unlock()
		return true
	}

	if d.cfg.CheckBuildRunning() {
		d.pendingAfterRun = true
		d.mu.Unlock()
		return false
	}

	d.pending = false
	d.pendingAfterRun = false
	d.pollingAfterRun = false
	d.mu.Unlock()

	evt := events.BuildNow{
		TriggeredAt:   time.Now(),
		RequestCount:  count,
		LastReason:    reason,
		LastRepoURL:   repoURL,
		FirstRequest:  first,
		LastRequest:   last,
		DebounceCause: cause,
	}

	_ = d.bus.Publish(ctx, evt)
	return true
}

func (d *BuildDebouncer) tryEmitAfterRunning(ctx context.Context) bool {
	d.mu.Lock()
	if !d.pendingAfterRun {
		d.mu.Unlock()
		return true
	}
	d.pollingAfterRun = true
	d.mu.Unlock()

	if d.cfg.CheckBuildRunning() {
		return false
	}

	// Build finished; emit exactly one follow-up.
	return d.tryEmit(ctx, "after_running")
}
