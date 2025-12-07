package pipeline

import (
	"errors"
	"testing"
	"time"
)

var errRetryable = errors.New("retryable error")
var errPermanent = errors.New("permanent error")

type tempError struct{ error }

func (t tempError) Temporary() bool { return true }

// TestRetrySuccess validates that retries succeed after transient failure.
func TestRetrySuccess(t *testing.T) {
	attempts := 0
	handler := func(e Event) error {
		attempts++
		if attempts < 2 {
			return tempError{errRetryable}
		}
		return nil
	}
	dlq := NewDeadLetterQueue()
	policy := RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond, IsRetryable: func(err error) bool {
		var te tempError
		return errors.As(err, &te)
	}}
	wrapped := WithRetry(handler, policy, dlq)

	if err := wrapped(SimpleEvent{E: "test"}); err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if dlq.Count() != 0 {
		t.Errorf("expected DLQ to be empty, got %d", dlq.Count())
	}
}

// TestRetryExhaustion validates that persistent failures go to DLQ after max attempts.
func TestRetryExhaustion(t *testing.T) {
	attempts := 0
	handler := func(e Event) error {
		attempts++
		return tempError{errRetryable}
	}
	dlq := NewDeadLetterQueue()
	policy := RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond, IsRetryable: func(err error) bool {
		var te tempError
		return errors.As(err, &te)
	}}
	wrapped := WithRetry(handler, policy, dlq)

	if err := wrapped(SimpleEvent{E: "test"}); err == nil {
		t.Fatal("expected error after retry exhaustion")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if dlq.Count() != 1 {
		t.Fatalf("expected 1 event in DLQ, got %d", dlq.Count())
	}
	failed := dlq.GetAll()[0]
	if failed.Event.Name() != "test" {
		t.Errorf("expected event name 'test', got %q", failed.Event.Name())
	}
}

// TestNonRetryableError validates that non-retryable errors skip retries and go straight to DLQ.
func TestNonRetryableError(t *testing.T) {
	attempts := 0
	handler := func(e Event) error {
		attempts++
		return errPermanent
	}
	dlq := NewDeadLetterQueue()
	policy := RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond, IsRetryable: func(err error) bool {
		return errors.Is(err, errRetryable)
	}}
	wrapped := WithRetry(handler, policy, dlq)

	if err := wrapped(SimpleEvent{E: "test"}); err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retries), got %d", attempts)
	}
	if dlq.Count() != 1 {
		t.Fatalf("expected 1 event in DLQ, got %d", dlq.Count())
	}
}

// TestDLQClear validates that clearing the DLQ removes all failed events.
func TestDLQClear(t *testing.T) {
	dlq := NewDeadLetterQueue()
	dlq.Enqueue(FailedEvent{Event: SimpleEvent{E: "test1"}, Error: errRetryable, Timestamp: time.Now()})
	dlq.Enqueue(FailedEvent{Event: SimpleEvent{E: "test2"}, Error: errPermanent, Timestamp: time.Now()})

	if dlq.Count() != 2 {
		t.Fatalf("expected 2 events, got %d", dlq.Count())
	}
	dlq.Clear()
	if dlq.Count() != 0 {
		t.Errorf("expected DLQ to be empty after clear, got %d", dlq.Count())
	}
}
