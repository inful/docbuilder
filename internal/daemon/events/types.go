package events

import "time"

// BuildRequested indicates that a coherent full-site build should happen soon.
//
// This is an orchestration event used by the daemon's in-process control flow.
// It is not a durable event and is not written to internal/eventstore.
type BuildRequested struct {
	JobID       string
	Immediate   bool
	Reason      string
	RepoURL     string
	RequestedAt time.Time
}

// BuildNow is emitted by the BuildDebouncer once it decides to start a build.
// Consumers should enqueue a canonical full-site build job.
type BuildNow struct {
	JobID         string
	TriggeredAt   time.Time
	RequestCount  int
	LastReason    string
	LastRepoURL   string
	FirstRequest  time.Time
	LastRequest   time.Time
	DebounceCause string // "quiet" or "max_delay" or "after_running"
}
