package pipeline

// Event is a domain event published by stages and consumed by handlers.
type Event interface{ Name() string }

// SimpleEvent is a lightweight event implementation for scaffolding.
type SimpleEvent struct{ E string }

func (s SimpleEvent) Name() string { return s.E }

// Event names used in the pipeline.
const (
	EventCloneRequested    = "CloneRequested"
	EventDiscoverRequested = "DiscoverRequested"
	EventDiscoverCompleted = "DiscoverCompleted"
	EventGenerateRequested = "GenerateRequested"
	EventGenerateCompleted = "GenerateCompleted"
)
