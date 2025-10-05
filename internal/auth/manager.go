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
func (m *Manager) CreateAuth(authConfig *config.AuthConfig) (transport.AuthMethod, error) {
	result, err := m.registry.CreateAuth(authConfig)
	if err != nil {
		return nil, err
	}
	return result.Auth, nil
}

// CreateAuthWithContext creates authentication with additional context.
// This allows for more sophisticated authentication strategies based on the repository or operation.
func (m *Manager) CreateAuthWithContext(authConfig *config.AuthConfig, repositoryURL, operation string) (transport.AuthMethod, error) {
	ctx := providers.AuthContext{
		RepositoryURL: repositoryURL,
		Operation:     operation,
	}

	result, err := m.registry.CreateAuthWithContext(authConfig, ctx)
	if err != nil {
		return nil, err
	}
	return result.Auth, nil
}

// ValidateAuthConfig validates an authentication configuration without creating the authentication.
// This is useful for configuration validation phases.
func (m *Manager) ValidateAuthConfig(authConfig *config.AuthConfig) error {
	if authConfig == nil {
		return nil // None auth is valid
	}

	provider, exists := m.registry.GetProvider(authConfig.Type)
	if !exists {
		return &providers.AuthError{
			Type:    authConfig.Type,
			Message: "unsupported authentication type",
		}
	}

	return provider.ValidateConfig(authConfig)
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
func CreateAuth(authConfig *config.AuthConfig) (transport.AuthMethod, error) {
	return DefaultManager.CreateAuth(authConfig)
}

// CreateAuthWithContext is a convenience function that uses the default manager.
func CreateAuthWithContext(authConfig *config.AuthConfig, repositoryURL, operation string) (transport.AuthMethod, error) {
	return DefaultManager.CreateAuthWithContext(authConfig, repositoryURL, operation)
}

// ValidateAuthConfig is a convenience function that uses the default manager.
func ValidateAuthConfig(authConfig *config.AuthConfig) error {
	return DefaultManager.ValidateAuthConfig(authConfig)
}
