package state

import (
	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// Service is a service adapter that wraps the new state management system
// and integrates it with the service orchestrator. This bridges the gap between
// the monolithic StateManager and the new composable state stores.
type Service struct {
	store Store
}

// NewService creates a new state service with the default JSON store.
func NewService(dataDir string) foundation.Result[*Service, error] {
	store := NewJSONStore(dataDir)
	if store.IsErr() {
		return foundation.Err[*Service, error](store.UnwrapErr())
	}

	return foundation.Ok[*Service, error](&Service{
		store: store.Unwrap(),
	})
}

// Store returns the underlying state store for direct access.
// This allows other services to interact with state through the interfaces.
func (ss *Service) Store() Store {
	return ss.store
}

// GetRepositoryStore provides typed access to repository operations.
func (ss *Service) GetRepositoryStore() RepositoryStore {
	return ss.store.Repositories()
}

// GetStatisticsStore provides typed access to statistics operations.
func (ss *Service) GetStatisticsStore() StatisticsStore {
	return ss.store.Statistics()
}

// GetConfigurationStore provides typed access to configuration operations.
func (ss *Service) GetConfigurationStore() ConfigurationStore {
	return ss.store.Configuration()
}
