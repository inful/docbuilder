package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"

	"git.home.luguber.info/inful/docbuilder/internal/auth/providers"
	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// Manager provides a high-level interface for authentication operations.
type Manager struct {
	registry *providers.AuthProviderRegistry
}

// NewManager creates a new authentication manager with the standard providers.
func NewManager() *Manager {
	return &Manager{
		registry: providers.NewAuthProviderRegistry(),
	}
}

// CreateAuth creates authentication for the given configuration.
// This is the main entry point for git operations needing authentication.
func (m *Manager) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	return m.registry.CreateAuth(authCfg)
}

// DefaultManager is a package-level instance for convenience.
var DefaultManager = NewManager()

// CreateAuth is a convenience function that uses the default manager.
func CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	return DefaultManager.CreateAuth(authCfg)
}
