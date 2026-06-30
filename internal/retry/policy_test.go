package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// TestDo_SuccessFirstTry ensures fn succeeding on first try returns nil without retry hooks firing.
func TestDo_SuccessFirstTry(t *testing.T) {
	p := NewPolicy(config.RetryBackoffLinear, 1*time.Millisecond, 5*time.Millisecond, 3)
	calls := 0
	hooks := RetryHooks{
		OnRetry: func(int, error) { t.Errorf("OnRetry should not run when fn succeeds") },
	}
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	}, hooks)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestDo_RetriesUntilSuccess ensures fn failing twice then succeeding returns nil and
// OnRetry fires for each retry.
func TestDo_RetriesUntilSuccess(t *testing.T) {
	p := NewPolicy(config.RetryBackoffFixed, 1*time.Millisecond, 5*time.Millisecond, 5)
	calls := 0
	retryNotices := 0
	hooks := RetryHooks{
		IsRetryable: func(err error) bool { return true },
		OnRetry: func(attempt int, err error) {
			retryNotices++
			if attempt < 1 {
				t.Errorf("attempt should be 1-based")
			}
		},
	}
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}, hooks)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if retryNotices != 2 {
		t.Fatalf("expected 2 OnRetry invocations (after attempt 1 and 2), got %d", retryNotices)
	}
}

// TestDo_NonRetryableHook ensures IsRetryable=false returns the error without further retries.
func TestDo_NonRetryableHook(t *testing.T) {
	p := NewPolicy(config.RetryBackoffLinear, 1*time.Millisecond, 5*time.Millisecond, 5)
	calls := 0
	hooks := RetryHooks{
		IsRetryable: func(err error) bool { return false },
	}
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errors.New("permanent")
	}, hooks)
	if err == nil || err.Error() != "permanent" {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestDo_ExhaustsMaxRetries ensures fn failing MaxRetries+1 times returns the last error.
func TestDo_ExhaustsMaxRetries(t *testing.T) {
	p := NewPolicy(config.RetryBackoffFixed, 1*time.Millisecond, 5*time.Millisecond, 2)
	calls := 0
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		return errors.New("always fails")
	}, RetryHooks{IsRetryable: func(error) bool { return true }})
	if err == nil || err.Error() != "always fails" {
		t.Fatalf("expected 'always fails', got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (1 + MaxRetries=2 retries), got %d", calls)
	}
}

// TestDo_DefaultRetryableUsesClassifier ensures that without an IsRetryable hook,
// a classified error with RetryNever stops the loop while RetryBackoff continues.
func TestDo_DefaultRetryableUsesClassifier(t *testing.T) {
	permanent := derrors.NewError(derrors.CategoryNotFound, "nope").Build()
	transient := derrors.NewError(derrors.CategoryNetwork, "blip").Retryable().Build()

	t.Run("RetryNever stops", func(t *testing.T) {
		p := NewPolicy(config.RetryBackoffFixed, 1*time.Millisecond, 5*time.Millisecond, 5)
		calls := 0
		err := p.Do(context.Background(), func(_ context.Context) error {
			calls++
			return permanent
		}, RetryHooks{})
		if err == nil {
			t.Fatalf("expected error to surface, got nil")
		}
		if calls != 1 {
			t.Fatalf("permanent classified must short-circuit, got %d calls", calls)
		}
	})

	t.Run("RetryBackoff continues", func(t *testing.T) {
		p := NewPolicy(config.RetryBackoffFixed, 1*time.Millisecond, 5*time.Millisecond, 2)
		calls := 0
		err := p.Do(context.Background(), func(_ context.Context) error {
			calls++
			if calls < 3 {
				return transient
			}
			return nil
		}, RetryHooks{})
		if err != nil {
			t.Fatalf("expected nil on eventual success, got %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected 3 calls, got %d", calls)
		}
	})
}

// TestDo_AdjustDelay ensures AdjustDelay can override the base delay before sleeping.
func TestDo_AdjustDelay(t *testing.T) {
	p := NewPolicy(config.RetryBackoffFixed, 50*time.Millisecond, 100*time.Millisecond, 1)
	adjusted := false
	hooks := RetryHooks{
		IsRetryable: func(error) bool { return true },
		AdjustDelay: func(_ error, base time.Duration) time.Duration {
			adjusted = true
			return time.Millisecond // force the sleep to be tiny for the test
		},
	}
	start := time.Now()
	err := p.Do(context.Background(), func(_ context.Context) error { return errors.New("x") }, hooks)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !adjusted {
		t.Fatalf("AdjustDelay was not invoked")
	}
	if elapsed > 30*time.Millisecond {
		t.Fatalf("AdjustDelay didn't compress the sleep, elapsed=%v", elapsed)
	}
}

// TestDo_RespectsContextCancellation ensures ctx cancellation mid-sleep returns ctx.Err.
func TestDo_RespectsContextCancellation(t *testing.T) {
	p := NewPolicy(config.RetryBackoffFixed, 50*time.Millisecond, time.Second, 5)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := p.Do(ctx, func(_ context.Context) error { return errors.New("x") }, RetryHooks{
		IsRetryable: func(error) bool { return true },
	})
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("should have returned quickly after cancellation, elapsed=%v", elapsed)
	}
}

// TestDo_ZeroMaxRetries ensures MaxRetries<=0 short-circuits to a single fn call.
func TestDo_ZeroMaxRetries(t *testing.T) {
	hooks := RetryHooks{}
	for _, max := range []int{-1, 0} {
		p := Policy{Mode: config.RetryBackoffFixed, Initial: time.Second, Max: 10 * time.Second, MaxRetries: max}
		calls := 0
		err := p.Do(context.Background(), func(_ context.Context) error {
			calls++
			return errors.New("once")
		}, hooks)
		if err == nil {
			t.Fatalf("expected error for MaxRetries=%d", max)
		}
		if calls != 1 {
			t.Fatalf("expected 1 call for MaxRetries=%d, got %d", max, calls)
		}
	}
}

// TestDo_NilHooksAllowed ensures callers can pass an empty RetryHooks struct.
func TestDo_NilHooksAllowed(t *testing.T) {
	p := NewPolicy(config.RetryBackoffFixed, 1*time.Millisecond, 5*time.Millisecond, 2)
	calls := 0
	err := p.Do(context.Background(), func(_ context.Context) error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	}, RetryHooks{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}
