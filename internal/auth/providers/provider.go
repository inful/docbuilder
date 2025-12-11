package providers

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
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

// AuthContext provides additional context for authentication operations.
type AuthContext struct {
	RepositoryURL string // The repository URL being authenticated against
	Operation     string // The operation being performed (clone, fetch, push, etc.)
}

// EnhancedAuthProvider extends AuthProvider with context-aware operations.
// This is optional and providers can implement it for more sophisticated auth handling.
type EnhancedAuthProvider interface {
	AuthProvider

	// CreateAuthWithContext creates authentication with additional context.
	// This allows providers to customize behavior based on the operation or repository.
	CreateAuthWithContext(authCfg *config.AuthConfig, ctx AuthContext) (transport.AuthMethod, error)
}

// ProviderResult wraps the result of authentication creation with metadata.
type ProviderResult struct {
	Auth     transport.AuthMethod
	Provider string // Name of the provider that created this auth
	Type     config.AuthType
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

// GetProvider returns the provider for the given auth type.
func (r *AuthProviderRegistry) GetProvider(authType config.AuthType) (AuthProvider, bool) {
	provider, exists := r.providers[authType]
	return provider, exists
}

// CreateAuth creates authentication using the appropriate provider.
func (r *AuthProviderRegistry) CreateAuth(authCfg *config.AuthConfig) (*ProviderResult, error) {
	if authCfg == nil {
		authCfg = &config.AuthConfig{Type: config.AuthTypeNone}
	}

	provider, exists := r.GetProvider(authCfg.Type)
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

	return &ProviderResult{
		Auth:     auth,
		Provider: provider.Name(),
		Type:     provider.Type(),
	}, nil
}

// CreateAuthWithContext creates authentication with context using enhanced providers when available.
func (r *AuthProviderRegistry) CreateAuthWithContext(authCfg *config.AuthConfig, ctx AuthContext) (*ProviderResult, error) {
	if authCfg == nil {
		authCfg = &config.AuthConfig{Type: config.AuthTypeNone}
	}

	provider, exists := r.GetProvider(authCfg.Type)
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

	// Try enhanced provider first
	if enhancedProvider, ok := provider.(EnhancedAuthProvider); ok {
		auth, err := enhancedProvider.CreateAuthWithContext(authCfg, ctx)
		if err != nil {
			return nil, &AuthError{
				Type:    authCfg.Type,
				Message: "failed to create authentication with context",
				Cause:   err,
			}
		}

		return &ProviderResult{
			Auth:     auth,
			Provider: provider.Name(),
			Type:     provider.Type(),
		}, nil
	}

	// Fall back to regular provider
	return r.CreateAuth(authCfg)
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
