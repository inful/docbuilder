package providers

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// TokenProvider handles token-based authentication.
type TokenProvider struct{}

// NewTokenProvider creates a new token authentication provider.
func NewTokenProvider() *TokenProvider {
	return &TokenProvider{}
}

// Type returns the authentication type this provider handles.
func (p *TokenProvider) Type() config.AuthType {
	return config.AuthTypeToken
}

// CreateAuth creates token authentication from the configuration.
func (p *TokenProvider) CreateAuth(authConfig *config.AuthConfig) (transport.AuthMethod, error) {
	if authConfig.Token == "" {
		return nil, fmt.Errorf("token authentication requires a token")
	}

	// Most Git hosting services use "token" as the username for token auth
	return &http.BasicAuth{
		Username: "token",
		Password: authConfig.Token,
	}, nil
}

// ValidateConfig validates the token authentication configuration.
func (p *TokenProvider) ValidateConfig(authConfig *config.AuthConfig) error {
	if authConfig.Token == "" {
		return fmt.Errorf("token authentication requires a token")
	}

	return nil
}

// Name returns a human-readable name for this provider.
func (p *TokenProvider) Name() string {
	return "TokenProvider"
}

// CreateAuthWithContext creates token authentication with additional context.
// This implements EnhancedAuthProvider to allow for context-aware token handling.
func (p *TokenProvider) CreateAuthWithContext(authConfig *config.AuthConfig, ctx AuthContext) (transport.AuthMethod, error) {
	// Future enhancement: Could use different usernames based on the forge type
	// For example, some services might use different username conventions
	return p.CreateAuth(authConfig)
}
