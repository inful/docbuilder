package pipeline

import (
	"sync"
	"time"
)

// FailedEvent wraps an event with its error and timestamp for DLQ storage.
type FailedEvent struct {
	Event     Event
	Error     error
	Timestamp time.Time
}

// DeadLetterQueue stores events that failed after retry exhaustion.
type DeadLetterQueue struct {
	mu     sync.RWMutex
	failed []FailedEvent
}

// NewDeadLetterQueue creates a new DLQ.
func NewDeadLetterQueue() *DeadLetterQueue {
	return &DeadLetterQueue{failed: []FailedEvent{}}
}

// Enqueue adds a failed event to the queue.
func (dlq *DeadLetterQueue) Enqueue(fe FailedEvent) {
	dlq.mu.Lock()
	dlq.failed = append(dlq.failed, fe)
	dlq.mu.Unlock()
}

// GetAll returns all failed events (for inspection/replay).
func (dlq *DeadLetterQueue) GetAll() []FailedEvent {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	result := make([]FailedEvent, len(dlq.failed))
	copy(result, dlq.failed)
	return result
}

// Clear removes all failed events from the queue.
func (dlq *DeadLetterQueue) Clear() {
	dlq.mu.Lock()
	dlq.failed = []FailedEvent{}
	dlq.mu.Unlock()
}

// Count returns the number of failed events in the queue.
func (dlq *DeadLetterQueue) Count() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.failed)
}
