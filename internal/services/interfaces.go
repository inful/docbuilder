// Package services provides interfaces for service orchestration and lifecycle management.
package services

import "time"

// StateManager defines the minimal interface for persistent state lifecycle and status.
// Components that need state persistence operations type assert to this interface
// or more specific interfaces like state.DaemonStateManager.
type StateManager interface {
	Load() error
	Save() error
	IsLoaded() bool
	LastSaved() *time.Time
}
