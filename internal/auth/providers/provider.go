package providers

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// AuthProvider defines the interface for authentication providers.
// Each provider handles a specific authentication method (SSH, token, basic, none).
type AuthProvider interface {
	// Type returns the authentication type this provider handles.
	Type() config.AuthType

	// CreateAuth creates a transport.AuthMethod from the given configuration.
	// Returns nil, nil for no authentication (AuthTypeNone).
	CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error)

	// ValidateConfig validates the authentication configuration for this provider.
	// This allows each provider to enforce its own requirements.
	ValidateConfig(authCfg *config.AuthConfig) error

	// Name returns a human-readable name for this provider (for logging/debugging).
	Name() string
}

// AuthProviderRegistry manages the collection of available auth providers.
type AuthProviderRegistry struct {
	providers map[config.AuthType]AuthProvider
}

// NewAuthProviderRegistry creates a new registry with the standard providers.
func NewAuthProviderRegistry() *AuthProviderRegistry {
	registry := &AuthProviderRegistry{
		providers: make(map[config.AuthType]AuthProvider),
	}

	// Register standard providers
	registry.Register(NewNoneProvider())
	registry.Register(NewSSHProvider())
	registry.Register(NewTokenProvider())
	registry.Register(NewBasicProvider())

	return registry
}

// Register adds a provider to the registry.
func (r *AuthProviderRegistry) Register(provider AuthProvider) {
	r.providers[provider.Type()] = provider
}

// CreateAuth creates authentication using the appropriate provider.
func (r *AuthProviderRegistry) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg == nil {
		authCfg = &config.AuthConfig{Type: config.AuthTypeNone}
	}

	provider, exists := r.providers[authCfg.Type]
	if !exists {
		return nil, &AuthError{
			Type:    authCfg.Type,
			Message: "unsupported authentication type",
		}
	}

	// Validate configuration first
	if err := provider.ValidateConfig(authCfg); err != nil {
		return nil, &AuthError{
			Type:    authCfg.Type,
			Message: "configuration validation failed",
			Cause:   err,
		}
	}

	// Create authentication
	auth, err := provider.CreateAuth(authCfg)
	if err != nil {
		return nil, &AuthError{
			Type:    authCfg.Type,
			Message: "failed to create authentication",
			Cause:   err,
		}
	}

	return auth, nil
}

// AuthError represents an authentication-related error.
type AuthError struct {
	Type    config.AuthType
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("auth error (%s): %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("auth error (%s): %s", e.Type, e.Message)
}

// Unwrap returns the underlying error.
func (e *AuthError) Unwrap() error {
	return e.Cause
}

// Temporary returns true if the error is temporary and the operation can be retried.
func (e *AuthError) Temporary() bool {
	// Most auth errors are permanent (bad credentials, missing files, etc.)
	// but some network-related errors during auth might be temporary
	return false
}
