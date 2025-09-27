package daemon

import (
	"context"
	"strconv"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo"
)

// TestRetryFlakinessSmoke runs multiple iterations of transient-then-success and fatal-no-retry
// scenarios to surface timing or race related flakiness in the BuildQueue retry logic.
func TestRetryFlakinessSmoke(t *testing.T) {
	const iterations = 25
	// Transient then success scenario loop
	for i := 0; i < iterations; i++ {
		t.Run("transient_then_success_iter_"+strconv.Itoa(i), func(t *testing.T) {
			fr := newFakeRecorder()
			tr, terr := transientReport(hugo.StageCloneRepos)
			mb := &mockBuilder{seq: []struct {
				rep *hugo.BuildReport
				err error
			}{{tr, terr}, {&hugo.BuildReport{}, nil}}}
			bq := NewBuildQueue(5, 1)
			bq.builder = mb
			bq.ConfigureRetry(config.BuildConfig{MaxRetries: 3, RetryBackoff: "fixed", RetryInitialDelay: "1ms", RetryMaxDelay: "2ms"})
			bq.SetRecorder(fr)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			bq.Start(ctx)
			job := newJob("txs_" + strconv.Itoa(i))
			if err := bq.Enqueue(job); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			for {
				time.Sleep(5 * time.Millisecond)
				snap, ok := bq.JobSnapshot(job.ID)
				if ok && snap.CompletedAt != nil {
					break
				}
				if ctx.Err() != nil {
					t.Fatalf("timeout waiting (transient success) iter %d", i)
				}
			}
			if got := fr.getRetry(string(hugo.StageCloneRepos)); got != 1 {
				t.Fatalf("expected 1 retry got %d", got)
			}
		})
	}
	// Fatal no retry scenario loop
	for i := 0; i < iterations; i++ {
		t.Run("fatal_no_retry_iter_"+strconv.Itoa(i), func(t *testing.T) {
			fr := newFakeRecorder()
			frpt, ferr := fatalReport(hugo.StageCloneRepos)
			mb := &mockBuilder{seq: []struct {
				rep *hugo.BuildReport
				err error
			}{{frpt, ferr}}}
			bq := NewBuildQueue(5, 1)
			bq.builder = mb
			bq.ConfigureRetry(config.BuildConfig{MaxRetries: 3, RetryBackoff: "linear", RetryInitialDelay: "1ms", RetryMaxDelay: "2ms"})
			bq.SetRecorder(fr)
			ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
			defer cancel()
			bq.Start(ctx)
			job := newJob("fnr_" + strconv.Itoa(i))
			if err := bq.Enqueue(job); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			for {
				time.Sleep(5 * time.Millisecond)
				snap, ok := bq.JobSnapshot(job.ID)
				if ok && snap.CompletedAt != nil {
					break
				}
				if ctx.Err() != nil {
					t.Fatalf("timeout waiting (fatal no retry) iter %d", i)
				}
			}
			if got := fr.getRetry(string(hugo.StageCloneRepos)); got != 0 {
				t.Fatalf("expected 0 retries got %d", got)
			}
		})
	}
}
