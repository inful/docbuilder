package providers

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
func (p *TokenProvider) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg.Token == "" {
		return nil, errors.New("token authentication requires a token")
	}

	// Most Git hosting services use "token" as the username for token auth
	return &http.BasicAuth{
		Username: "token",
		Password: authCfg.Token,
	}, nil
}

// ValidateConfig validates the token authentication configuration.
func (p *TokenProvider) ValidateConfig(authCfg *config.AuthConfig) error {
	if authCfg.Token == "" {
		return errors.New("token authentication requires a token")
	}

	return nil
}

// Name returns a human-readable name for this provider.
func (p *TokenProvider) Name() string {
	return "TokenProvider"
}
