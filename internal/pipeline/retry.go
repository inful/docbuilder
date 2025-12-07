package pipeline

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// RetryPolicy defines retry behavior for failed handlers.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
	IsRetryable func(error) bool
}

// DefaultRetryPolicy provides sensible defaults: 3 attempts, exponential backoff starting at 1s.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		Backoff:     time.Second,
		IsRetryable: func(err error) bool {
			// Retry on network, timeout, or temporary errors; skip validation errors
			var tempErr interface{ Temporary() bool }
			if errors.As(err, &tempErr) && tempErr.Temporary() {
				return true
			}
			// Add custom retryable error types as needed
			return false
		},
	}
}

// WithRetry wraps a handler with retry logic according to the policy.
func WithRetry(h Handler, policy RetryPolicy, dlq *DeadLetterQueue) Handler {
	return func(e Event) error {
		var lastErr error
		for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
			lastErr = h(e)
			if lastErr == nil {
				return nil
			}
			if !policy.IsRetryable(lastErr) {
				slog.Warn("Non-retryable error encountered", "event", e.Name(), "error", lastErr)
				break
			}
			if attempt < policy.MaxAttempts {
				backoff := policy.Backoff * time.Duration(1<<uint(attempt-1)) // exponential
				slog.Info("Retrying after failure", "event", e.Name(), "attempt", attempt, "backoff", backoff, "error", lastErr)
				time.Sleep(backoff)
			}
		}
		// Exhausted retries; send to DLQ
		slog.Error("Handler failed after retries", "event", e.Name(), "attempts", policy.MaxAttempts, "error", lastErr)
		if dlq != nil {
			dlq.Enqueue(FailedEvent{Event: e, Error: lastErr, Timestamp: time.Now()})
		}
		return fmt.Errorf("handler failed after %d attempts: %w", policy.MaxAttempts, lastErr)
	}
}
