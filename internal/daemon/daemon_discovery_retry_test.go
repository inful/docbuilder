package daemon

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRunDiscoveryWithRetry_RetriesUntilSuccess pins that a failed discovery
// is retried instead of leaving the daemon without a build until the next
// cron tick.
func TestRunDiscoveryWithRetry_RetriesUntilSuccess(t *testing.T) {
	attempts := 0
	backoff := []time.Duration{time.Millisecond, time.Millisecond}

	runDiscoveryWithRetry(t.Context(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("discovery failed")
		}
		return nil
	}, backoff)

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

// TestRunDiscoveryWithRetry_StopsOnContextCancel pins that the retry loop
// gives up when the daemon shuts down.
func TestRunDiscoveryWithRetry_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	attempts := 0
	runDiscoveryWithRetry(ctx, func(context.Context) error {
		attempts++
		return errors.New("discovery failed")
	}, []time.Duration{time.Hour})

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

// TestRunDiscoveryWithRetry_UsesLastBackoffEntry pins that the ladder's last
// entry repeats once the ladder is exhausted.
func TestRunDiscoveryWithRetry_UsesLastBackoffEntry(t *testing.T) {
	attempts := 0
	backoff := []time.Duration{time.Millisecond}

	start := time.Now()
	runDiscoveryWithRetry(t.Context(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("discovery failed")
		}
		return nil
	}, backoff)

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("retry loop took %s, want < 5s", elapsed)
	}
}
