package daemon

import "git.home.luguber.info/inful/docbuilder/internal/build/queue"

// Type and constant aliases keep the daemon package API stable while the
// implementation lives in internal/build/queue.

type (
	BuildType         = queue.BuildType
	BuildPriority     = queue.BuildPriority
	BuildStatus       = queue.BuildStatus
	BuildJob          = queue.BuildJob
	BuildJobMetadata  = queue.BuildJobMetadata
	BuildQueue        = queue.BuildQueue
	BuildEventEmitter = queue.BuildEventEmitter
	Builder           = queue.Builder
)

const (
	BuildTypeManual    = queue.BuildTypeManual
	BuildTypeScheduled = queue.BuildTypeScheduled
	BuildTypeWebhook   = queue.BuildTypeWebhook
	BuildTypeDiscovery = queue.BuildTypeDiscovery

	PriorityLow    = queue.PriorityLow
	PriorityNormal = queue.PriorityNormal
	PriorityHigh   = queue.PriorityHigh
	PriorityUrgent = queue.PriorityUrgent

	BuildStatusQueued    = queue.BuildStatusQueued
	BuildStatusRunning   = queue.BuildStatusRunning
	BuildStatusCompleted = queue.BuildStatusCompleted
	BuildStatusFailed    = queue.BuildStatusFailed
	BuildStatusCancelled = queue.BuildStatusCancelled
)

func EnsureTypedMeta(job *BuildJob) *BuildJobMetadata { return queue.EnsureTypedMeta(job) }

func NewBuildQueue(maxSize, workers int, builder Builder) *BuildQueue {
	return queue.NewBuildQueue(maxSize, workers, builder)
}
