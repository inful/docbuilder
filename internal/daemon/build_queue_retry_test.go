package daemon

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	bld "git.home.luguber.info/inful/docbuilder/internal/build"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
	"git.home.luguber.info/inful/docbuilder/internal/metrics"
)

// fakeRecorder captures retry metrics for assertions.
type fakeRecorder struct {
	mu        sync.Mutex
	retries   map[string]int
	exhausted map[string]int
}

func newFakeRecorder() *fakeRecorder {
	return &fakeRecorder{retries: map[string]int{}, exhausted: map[string]int{}}
}

// Implement metrics.Recorder (only retry-related methods record state; others noop)
func (f *fakeRecorder) ObserveStageDuration(string, time.Duration)           {}
func (f *fakeRecorder) ObserveBuildDuration(time.Duration)                   {}
func (f *fakeRecorder) IncStageResult(string, metrics.ResultLabel)           {}
func (f *fakeRecorder) IncBuildOutcome(string)                               {}
func (f *fakeRecorder) ObserveCloneRepoDuration(string, time.Duration, bool) {}
func (f *fakeRecorder) IncCloneRepoResult(bool)                              {}
func (f *fakeRecorder) SetCloneConcurrency(int)                              {}
func (f *fakeRecorder) IncBuildRetry(stage string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retries[stage]++
}
func (f *fakeRecorder) IncBuildRetryExhausted(stage string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.exhausted[stage]++
}

func (f *fakeRecorder) getRetry(stage string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.retries[stage]
}

func (f *fakeRecorder) getExhausted(stage string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.exhausted[stage]
}

// mockBuilder allows scripted outcomes: sequence of (report,error) pairs returned per Build invocation.
type mockBuilder struct {
	mu  sync.Mutex
	seq []struct {
		rep *hugo.BuildReport
		err error
	}
	idx int
}

func (m *mockBuilder) Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx >= len(m.seq) {
		return &hugo.BuildReport{}, nil
	}
	cur := m.seq[m.idx]
	m.idx++
	return cur.rep, cur.err
}

// helper to create a transient StageError in a report
func transientReport(stage string) (*hugo.BuildReport, error) {
	// Use sentinel errors from internal/build to trigger transient classification.
	var underlying error
	switch stage {
	case "clone_repos":
		underlying = bld.ErrClone
	case "run_hugo":
		underlying = bld.ErrHugo
	case "discover_docs":
		underlying = bld.ErrDiscovery
	default:
		underlying = errors.New("transient")
	}
	se := &hugo.StageError{Stage: stage, Kind: hugo.StageErrorWarning, Err: underlying}
	r := &hugo.BuildReport{StageDurations: map[string]time.Duration{}, StageErrorKinds: map[string]string{}}
	r.Errors = append(r.Errors, se)
	return r, se
}

// helper to create a fatal (non-transient) StageError report
func fatalReport(stage string) (*hugo.BuildReport, error) {
	se := &hugo.StageError{Stage: stage, Kind: hugo.StageErrorFatal, Err: errors.New("fatal")}
	r := &hugo.BuildReport{StageDurations: map[string]time.Duration{}, StageErrorKinds: map[string]string{}}
	r.Errors = append(r.Errors, se)
	return r, se
}

// newJob creates a minimal BuildJob
func newJob(id string) *BuildJob {
	return &BuildJob{ID: id, Type: BuildTypeManual, CreatedAt: time.Now()}
}

func TestRetrySucceedsAfterTransient(t *testing.T) {
	fr := newFakeRecorder()
	// First attempt transient failure, second succeeds
	tr, terr := transientReport("clone_repos")
	mb := &mockBuilder{seq: []struct {
		rep *hugo.BuildReport
		err error
	}{
		{tr, terr},
		{&hugo.BuildReport{}, nil},
	}}
	bq := NewBuildQueue(10, 1)
	bq.builder = mb
	bq.ConfigureRetry(config.BuildConfig{MaxRetries: 3, RetryBackoff: "fixed", RetryInitialDelay: "1ms", RetryMaxDelay: "5ms"})
	bq.SetRecorder(fr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bq.Start(ctx)
	job := newJob("job1")
	if err := bq.Enqueue(job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	// wait until job finishes
	for {
		time.Sleep(10 * time.Millisecond)
		snap, ok := bq.JobSnapshot(job.ID)
		if ok && snap.CompletedAt != nil {
			if snap.Status != BuildStatusCompleted {
				t.Fatalf("expected completed, got %s", snap.Status)
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("timeout waiting for job completion")
		}
	}
	if fr.getRetry("clone_repos") != 1 {
		t.Fatalf("expected 1 retry metric, got %d", fr.getRetry("clone_repos"))
	}
	if fr.getExhausted("clone_repos") != 0 {
		t.Fatalf("expected 0 exhausted, got %d", fr.getExhausted("clone_repos"))
	}
}

func TestRetryExhausted(t *testing.T) {
	fr := newFakeRecorder()
	// Always transient failure, exceed retries
	tr1, terr1 := transientReport("clone_repos")
	tr2, terr2 := transientReport("clone_repos")
	tr3, terr3 := transientReport("clone_repos")
	mb := &mockBuilder{seq: []struct {
		rep *hugo.BuildReport
		err error
	}{
		{tr1, terr1}, {tr2, terr2}, {tr3, terr3},
	}}
	bq := NewBuildQueue(10, 1)
	bq.builder = mb
	bq.ConfigureRetry(config.BuildConfig{MaxRetries: 2, RetryBackoff: "linear", RetryInitialDelay: "1ms", RetryMaxDelay: "5ms"})
	bq.SetRecorder(fr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bq.Start(ctx)
	job := newJob("job2")
	if err := bq.Enqueue(job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	for {
		time.Sleep(10 * time.Millisecond)
		snap, ok := bq.JobSnapshot(job.ID)
		if ok && snap.CompletedAt != nil {
			if snap.Status != BuildStatusFailed {
				t.Fatalf("expected failed, got %s", snap.Status)
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("timeout waiting for job completion")
		}
	}
	if fr.getRetry("clone_repos") != 2 {
		t.Fatalf("expected 2 retry attempts metric, got %d", fr.getRetry("clone_repos"))
	}
	if fr.getExhausted("clone_repos") != 1 {
		t.Fatalf("expected 1 exhausted metric, got %d", fr.getExhausted("clone_repos"))
	}
}

func TestNoRetryOnPermanent(t *testing.T) {
	fr := newFakeRecorder()
	frpt, ferr := fatalReport("clone_repos")
	mb := &mockBuilder{seq: []struct {
		rep *hugo.BuildReport
		err error
	}{{frpt, ferr}}}
	bq := NewBuildQueue(10, 1)
	bq.builder = mb
	bq.ConfigureRetry(config.BuildConfig{MaxRetries: 3, RetryBackoff: "exponential", RetryInitialDelay: "1ms", RetryMaxDelay: "4ms"})
	bq.SetRecorder(fr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bq.Start(ctx)
	job := newJob("job3")
	if err := bq.Enqueue(job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	for {
		time.Sleep(10 * time.Millisecond)
		snap, ok := bq.JobSnapshot(job.ID)
		if ok && snap.CompletedAt != nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("timeout waiting for job completion")
		}
	}
	if fr.getRetry("clone_repos") != 0 {
		t.Fatalf("expected 0 retries, got %d", fr.getRetry("clone_repos"))
	}
	if fr.getExhausted("clone_repos") != 0 {
		t.Fatalf("expected 0 exhausted, got %d", fr.getExhausted("clone_repos"))
	}
}

func TestExponentialBackoffCapped(t *testing.T) {
	// Validate exponential growth and cap respect without sleeping real exponential durations by using very small intervals.
	initial := 1 * time.Millisecond
	max := 4 * time.Millisecond
	// retryCount: 1->1ms,2->2ms,3->4ms,4->cap 4ms
	cases := []struct {
		retry int
		want  time.Duration
	}{{1, 1 * time.Millisecond}, {2, 2 * time.Millisecond}, {3, 4 * time.Millisecond}, {4, 4 * time.Millisecond}}
	pol := NewRetryPolicy("exponential", initial, max, 5)
	for _, c := range cases {
		got := pol.Delay(c.retry)
		if got != c.want { t.Fatalf("retry %d: expected %v got %v", c.retry, c.want, got) }
	}
}

func TestRetryPolicyValidationAndModes(t *testing.T) {
    p := NewRetryPolicy("", 0, 0, -1) // triggers defaults except maxRetries negative ignored
    if err := p.Validate(); err != nil { t.Fatalf("default policy should validate: %v", err) }
    if p.Mode != "linear" { t.Fatalf("expected default mode linear got %s", p.Mode) }
    fixed := NewRetryPolicy("fixed", 10*time.Millisecond, 20*time.Millisecond, 3)
    if d := fixed.Delay(2); d != 10*time.Millisecond { t.Fatalf("fixed mode should not scale: got %v", d) }
    linear := NewRetryPolicy("linear", 5*time.Millisecond, 12*time.Millisecond, 3)
    if d := linear.Delay(3); d != 12*time.Millisecond { t.Fatalf("linear capping failed expected 12ms got %v", d) }
    exp := NewRetryPolicy("exponential", 2*time.Millisecond, 10*time.Millisecond, 5)
    if exp.Delay(4) != 10*time.Millisecond { t.Fatalf("exponential cap failed: %v", exp.Delay(4)) }
}
