package daemon

import (
    "context"
    "errors"
    "sync"
    "testing"
    "time"

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

func newFakeRecorder() *fakeRecorder { return &fakeRecorder{retries: map[string]int{}, exhausted: map[string]int{}} }

// Implement metrics.Recorder (only retry-related methods record state; others noop)
func (f *fakeRecorder) ObserveStageDuration(string, time.Duration)          {}
func (f *fakeRecorder) ObserveBuildDuration(time.Duration)                  {}
func (f *fakeRecorder) IncStageResult(string, metrics.ResultLabel)          {}
func (f *fakeRecorder) IncBuildOutcome(string)                              {}
func (f *fakeRecorder) ObserveCloneRepoDuration(string, time.Duration, bool) {}
func (f *fakeRecorder) IncCloneRepoResult(bool)                             {}
func (f *fakeRecorder) SetCloneConcurrency(int)                             {}
func (f *fakeRecorder) IncBuildRetry(stage string) {
    f.mu.Lock(); defer f.mu.Unlock(); f.retries[stage]++
}
func (f *fakeRecorder) IncBuildRetryExhausted(stage string) {
    f.mu.Lock(); defer f.mu.Unlock(); f.exhausted[stage]++
}

// mockBuilder allows scripted outcomes: sequence of (report,error) pairs returned per Build invocation.
type mockBuilder struct {
    mu   sync.Mutex
    seq  []struct{ rep *hugo.BuildReport; err error }
    idx  int
}

func (m *mockBuilder) Build(ctx context.Context, job *BuildJob) (*hugo.BuildReport, error) {
    m.mu.Lock(); defer m.mu.Unlock()
    if m.idx >= len(m.seq) {
        return &hugo.BuildReport{}, nil
    }
    cur := m.seq[m.idx]
    m.idx++
    return cur.rep, cur.err
}

// helper to create a transient StageError in a report
func transientReport(stage string) (*hugo.BuildReport, error) {
    se := &hugo.StageError{Stage: stage, Kind: hugo.StageErrorWarning, Err: errors.New("transient")}
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
func newJob(id string) *BuildJob { return &BuildJob{ID: id, Type: BuildTypeManual, CreatedAt: time.Now()} }

func TestRetrySucceedsAfterTransient(t *testing.T) {
    fr := newFakeRecorder()
    // First attempt transient failure, second succeeds
    tr, terr := transientReport("clone_repos")
    mb := &mockBuilder{seq: []struct{ rep *hugo.BuildReport; err error }{
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
    if err := bq.Enqueue(job); err != nil { t.Fatalf("enqueue: %v", err) }
    // wait until job finishes
    for {
        time.Sleep(10 * time.Millisecond)
        if job.CompletedAt != nil { break }
        if ctx.Err() != nil { t.Fatalf("timeout waiting for job completion") }
    }
    if job.Status != BuildStatusCompleted { t.Fatalf("expected completed, got %s", job.Status) }
    if fr.retries["clone_repos"] != 1 { t.Fatalf("expected 1 retry metric, got %d", fr.retries["clone_repos"]) }
    if fr.exhausted["clone_repos"] != 0 { t.Fatalf("expected 0 exhausted, got %d", fr.exhausted["clone_repos"]) }
}

func TestRetryExhausted(t *testing.T) {
    fr := newFakeRecorder()
    // Always transient failure, exceed retries
    tr1, terr1 := transientReport("clone_repos")
    tr2, terr2 := transientReport("clone_repos")
    tr3, terr3 := transientReport("clone_repos")
    mb := &mockBuilder{seq: []struct{ rep *hugo.BuildReport; err error }{
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
    if err := bq.Enqueue(job); err != nil { t.Fatalf("enqueue: %v", err) }
    for {
        time.Sleep(10 * time.Millisecond)
        if job.CompletedAt != nil { break }
        if ctx.Err() != nil { t.Fatalf("timeout waiting for job completion") }
    }
    if job.Status != BuildStatusFailed { t.Fatalf("expected failed, got %s", job.Status) }
    if fr.retries["clone_repos"] != 2 { t.Fatalf("expected 2 retry attempts metric, got %d", fr.retries["clone_repos"]) }
    if fr.exhausted["clone_repos"] != 1 { t.Fatalf("expected 1 exhausted metric, got %d", fr.exhausted["clone_repos"]) }
}

func TestNoRetryOnPermanent(t *testing.T) {
    fr := newFakeRecorder()
    frpt, ferr := fatalReport("clone_repos")
    mb := &mockBuilder{seq: []struct{ rep *hugo.BuildReport; err error }{{frpt, ferr}}}
    bq := NewBuildQueue(10, 1)
    bq.builder = mb
    bq.ConfigureRetry(config.BuildConfig{MaxRetries: 3, RetryBackoff: "exponential", RetryInitialDelay: "1ms", RetryMaxDelay: "4ms"})
    bq.SetRecorder(fr)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    bq.Start(ctx)
    job := newJob("job3")
    if err := bq.Enqueue(job); err != nil { t.Fatalf("enqueue: %v", err) }
    for {
        time.Sleep(10 * time.Millisecond)
        if job.CompletedAt != nil { break }
        if ctx.Err() != nil { t.Fatalf("timeout waiting for job completion") }
    }
    if fr.retries["clone_repos"] != 0 { t.Fatalf("expected 0 retries, got %d", fr.retries["clone_repos"]) }
    if fr.exhausted["clone_repos"] != 0 { t.Fatalf("expected 0 exhausted, got %d", fr.exhausted["clone_repos"]) }
}

func TestExponentialBackoffCapped(t *testing.T) {
    // Validate exponential growth and cap respect without sleeping real exponential durations by using very small intervals.
    initial := 1 * time.Millisecond
    max := 4 * time.Millisecond
    // retryCount: 1->1ms,2->2ms,3->4ms,4->cap 4ms
    cases := []struct{ retry int; want time.Duration }{{1, 1 * time.Millisecond}, {2, 2 * time.Millisecond}, {3, 4 * time.Millisecond}, {4, 4 * time.Millisecond}}
    for _, c := range cases {
        got := computeBackoffDelay("exponential", initial, max, c.retry)
        if got != c.want {
            t.Fatalf("retry %d: expected %v got %v", c.retry, c.want, got)
        }
    }
}
