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
	Snapshot    map[string]string // optional: repoURL -> commitSHA
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

// WebhookReceived represents an accepted/validated webhook that may result in a repo update and build.
//
// This event is intentionally "thin": it carries only webhook inputs. Downstream workers are
// responsible for:
// - matching the webhook to a known repository
// - optional docs-change filtering
// - publishing RepoUpdateRequested (and subsequent build requests).
type WebhookReceived struct {
	JobID        string
	ForgeName    string
	RepoFullName string
	Branch       string
	ChangedFiles []string
	ReceivedAt   time.Time
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

// RepoRemoved is emitted when a previously discovered repository is no longer present
// in the latest discovery result.
//
// This is an orchestration event used by the daemon's in-process control flow.
// It is not durable and is not written to internal/eventstore.
type RepoRemoved struct {
	RepoURL    string
	RepoName   string
	RemovedAt  time.Time
	Discovered bool // true when removal was detected via forge discovery
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
	Snapshot      map[string]string // optional: repoURL -> commitSHA
	FirstRequest  time.Time
	LastRequest   time.Time
	DebounceCause string // "quiet" or "max_delay" or "after_running"
}
