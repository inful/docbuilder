package eventstore

import "time"

// Event represents a domain event in the build pipeline.
type Event interface {
	// ID returns the unique identifier for this event.
	ID() int64
	// BuildID returns the build identifier this event belongs to.
	BuildID() string
	// Type returns the event type name.
	Type() string
	// Timestamp returns when the event occurred.
	Timestamp() time.Time
	// Payload returns the event data as bytes.
	Payload() []byte
	// Metadata returns optional event metadata.
	Metadata() map[string]string
}

// BaseEvent provides a default implementation of Event.
type BaseEvent struct {
	EventID        int64
	EventBuildID   string
	EventType      string
	EventTimestamp time.Time
	EventPayload   []byte
	EventMetadata  map[string]string
}

func (e *BaseEvent) ID() int64                   { return e.EventID }
func (e *BaseEvent) BuildID() string             { return e.EventBuildID }
func (e *BaseEvent) Type() string                { return e.EventType }
func (e *BaseEvent) Timestamp() time.Time        { return e.EventTimestamp }
func (e *BaseEvent) Payload() []byte             { return e.EventPayload }
func (e *BaseEvent) Metadata() map[string]string { return e.EventMetadata }
