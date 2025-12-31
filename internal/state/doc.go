// Package state provides domain models and interfaces for daemon state management.
//
// This package replaces the monolithic StateManager with focused, well-typed components
// that provide narrow, composable interfaces for state operations.
//
// Key components:
//   - Repository metadata and build tracking
//   - Configuration state (hashes, checksums)
//   - Daemon information and statistics
//   - Schedule management
//   - JSON-based persistence layer
//
// The package defines narrow interfaces that bridge legacy and typed state implementations,
// allowing for gradual migration and testing.
package state
