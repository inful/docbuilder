package eventstore

// Package errors provides sentinel errors for event store operations.
// These enable consistent classification and improved error handling for event sourcing stage failures.

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

var (
	// ErrDatabaseOpenFailed indicates the SQLite database could not be opened.
	ErrDatabaseOpenFailed = errors.EventStoreError("could not open event store database").Build()

	// ErrInitializeSchemaFailed indicates the database schema could not be initialized.
	ErrInitializeSchemaFailed = errors.EventStoreError("failed to initialize event store schema").Build()

	// ErrEventAppendFailed indicates appending an event failed.
	ErrEventAppendFailed = errors.EventStoreError("failed to append event to store").Build()

	// ErrEventQueryFailed indicates querying events failed.
	ErrEventQueryFailed = errors.EventStoreError("failed to query events from store").Build()

	// ErrEventScanFailed indicates scanning event rows failed.
	ErrEventScanFailed = errors.EventStoreError("failed to scan event rows").Build()

	// ErrMarshalPayloadFailed indicates JSON marshaling of event payload failed.
	ErrMarshalPayloadFailed = errors.EventStoreError("failed to marshal event payload").Build()

	// ErrUnmarshalPayloadFailed indicates JSON unmarshaling of event payload failed.
	ErrUnmarshalPayloadFailed = errors.EventStoreError("failed to unmarshal event payload").Build()

	// ErrProjectionRebuildFailed indicates rebuilding a projection failed.
	ErrProjectionRebuildFailed = errors.EventStoreError("failed to rebuild projection").Build()
)
