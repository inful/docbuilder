package state

import (
	"context"
	"sync"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// ServiceAdapter wraps a state.Service and implements the narrow interfaces
// defined in narrow_interfaces.go (DaemonStateManager). This is the canonical
// implementation for daemon state management.
//
// The adapter translates simple method signatures (no context, no Result types)
// to the typed Store method signatures (context + Result types).
//
// The implementation is split across multiple files by capability:
//   - lifecycle.go:        Load, Save, IsLoaded, LastSaved
//   - repository.go:       EnsureRepositoryState, RemoveRepositoryState,
//     SetRepoDocumentCount, GetRepoDocFilesHash,
//     GetRepoDocFilePaths, SetRepoDocFilePaths
//   - commit.go:           SetRepoLastCommit, GetRepoLastCommit
//   - build_counter.go:    IncrementRepoBuild
//   - configuration.go:    Set/Get* (config hash, report checksum,
//     global doc files hash)
//   - discovery.go:        RecordDiscovery
//   - lookup.go:           GetRepository (test helper)
type ServiceAdapter struct {
	service *Service
	mu      sync.RWMutex

	// Cached values for lifecycle methods
	loaded    bool
	lastSaved *time.Time
}

// NewServiceAdapter creates an adapter that wraps a state.Service.
func NewServiceAdapter(svc *Service) *ServiceAdapter {
	return &ServiceAdapter{
		service: svc,
		loaded:  false,
	}
}

// --- LifecycleManager interface ---

// Load loads state from the underlying store.
func (a *ServiceAdapter) Load() error {
	ctx := context.Background()
	// The typed store loads on creation, so this is mostly a health check
	health := a.service.Store().Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	if health.Unwrap().Status != healthyStatus {
		return errors.InternalError("state store unhealthy").
			WithContext("status", health.Unwrap().Status).
			Build()
	}
	a.mu.Lock()
	a.loaded = true
	a.mu.Unlock()
	return nil
}

// Save persists state to the underlying store.
func (a *ServiceAdapter) Save() error {
	ctx := context.Background()
	// The typed JSON store auto-saves, but we trigger a close/reopen cycle
	// for explicit save semantics, or just mark save time.
	// For now, just update the save timestamp since JSONStore auto-persists.
	a.mu.Lock()
	now := time.Now()
	a.lastSaved = &now
	a.mu.Unlock()

	// Optionally flush by checking health (which internally persists)
	health := a.service.Store().Health(ctx)
	if health.IsErr() {
		return health.UnwrapErr()
	}
	return nil
}

// IsLoaded returns whether the state has been loaded.
func (a *ServiceAdapter) IsLoaded() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loaded
}

// LastSaved returns the last save timestamp.
func (a *ServiceAdapter) LastSaved() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastSaved
}

// Compile-time verification that ServiceAdapter implements the canonical
// state interfaces consumed by the daemon, build, and hugo packages.
var (
	_ LifecycleManager   = (*ServiceAdapter)(nil)
	_ DaemonStateManager = (*ServiceAdapter)(nil)
)

// Compile-time assertion that *ServiceAdapter satisfies validation.SkipStateAccess.
// (validation lives in a different package; we re-export the assertion here to
// avoid making the state package depend on build/validation.)
// We assert each method individually rather than importing the interface.
var (
	_ = (*ServiceAdapter)(nil).GetRepoLastCommit
	_ = (*ServiceAdapter)(nil).GetLastConfigHash
	_ = (*ServiceAdapter)(nil).GetLastReportChecksum
	_ = (*ServiceAdapter)(nil).SetLastReportChecksum
	_ = (*ServiceAdapter)(nil).GetRepoDocFilesHash
	_ = (*ServiceAdapter)(nil).GetLastGlobalDocFilesHash
	_ = (*ServiceAdapter)(nil).SetLastGlobalDocFilesHash
)
