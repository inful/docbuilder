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
	Branch      string
	RequestedAt time.Time
}

// RepoUpdateRequested asks for a repository refresh/check before triggering a build.
//
// This is an orchestration event used by the daemon's in-process control flow.
// It is not durable and is not written to internal/eventstore.
type RepoUpdateRequested struct {
	JobID       string
	Immediate   bool
	RepoURL     string
	Branch      string
	RequestedAt time.Time
}

// RepoUpdated is emitted after a repository update/check completes.
//
// When Changed is true, consumers may request a build.
type RepoUpdated struct {
	JobID     string
	RepoURL   string
	Branch    string
	CommitSHA string
	Changed   bool
	UpdatedAt time.Time
	Immediate bool
}

// BuildNow is emitted by the BuildDebouncer once it decides to start a build.
// Consumers should enqueue a canonical full-site build job.
type BuildNow struct {
	JobID         string
	TriggeredAt   time.Time
	RequestCount  int
	LastReason    string
	LastRepoURL   string
	LastBranch    string
	FirstRequest  time.Time
	LastRequest   time.Time
	DebounceCause string // "quiet" or "max_delay" or "after_running"
}
