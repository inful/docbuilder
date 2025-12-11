package auth

import (
	"git.home.luguber.info/inful/docbuilder/internal/auth/providers"
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
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

// NewManagerWithRegistry creates a new authentication manager with a custom provider registry.
func NewManagerWithRegistry(registry *providers.AuthProviderRegistry) *Manager {
	return &Manager{
		registry: registry,
	}
}

// CreateAuth creates authentication for the given configuration.
// This is the main entry point for git operations needing authentication.
func (m *Manager) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	result, err := m.registry.CreateAuth(authCfg)
	if err != nil {
		return nil, err
	}
	return result.Auth, nil
}

// CreateAuthWithContext creates authentication with additional context.
// This allows for more sophisticated authentication strategies based on the repository or operation.
func (m *Manager) CreateAuthWithContext(authCfg *config.AuthConfig, repoURL, operation string) (transport.AuthMethod, error) {
	ctx := providers.AuthContext{
		RepositoryURL: repoURL,
		Operation:     operation,
	}

	result, err := m.registry.CreateAuthWithContext(authCfg, ctx)
	if err != nil {
		return nil, err
	}
	return result.Auth, nil
}

// ValidateAuthConfig validates an authentication configuration without creating the authentication.
// This is useful for configuration validation phases.
func (m *Manager) ValidateAuthConfig(authCfg *config.AuthConfig) error {
	if authCfg == nil {
		return nil // None auth is valid
	}

	provider, exists := m.registry.GetProvider(authCfg.Type)
	if !exists {
		return &providers.AuthError{
			Type:    authCfg.Type,
			Message: "unsupported authentication type",
		}
	}

	return provider.ValidateConfig(authCfg)
}

// GetSupportedTypes returns all supported authentication types.
func (m *Manager) GetSupportedTypes() []config.AuthType {
	// This is a simple implementation - could be made more dynamic
	return []config.AuthType{
		config.AuthTypeNone,
		config.AuthTypeSSH,
		config.AuthTypeToken,
		config.AuthTypeBasic,
	}
}

// DefaultManager is a package-level instance for convenience.
var DefaultManager = NewManager()

// CreateAuth is a convenience function that uses the default manager.
func CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	return DefaultManager.CreateAuth(authCfg)
}

// CreateAuthWithContext is a convenience function that uses the default manager.
func CreateAuthWithContext(authCfg *config.AuthConfig, repoURL, operation string) (transport.AuthMethod, error) {
	return DefaultManager.CreateAuthWithContext(authCfg, repoURL, operation)
}

// ValidateAuthConfig is a convenience function that uses the default manager.
func ValidateAuthConfig(authCfg *config.AuthConfig) error {
	return DefaultManager.ValidateAuthConfig(authCfg)
}
