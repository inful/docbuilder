package providers

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// BasicProvider handles basic username/password authentication.
type BasicProvider struct{}

// NewBasicProvider creates a new basic authentication provider.
func NewBasicProvider() *BasicProvider {
	return &BasicProvider{}
}

// Type returns the authentication type this provider handles.
func (p *BasicProvider) Type() config.AuthType {
	return config.AuthTypeBasic
}

// CreateAuth creates basic authentication from the configuration.
func (p *BasicProvider) CreateAuth(authConfig *config.AuthConfig) (transport.AuthMethod, error) {
	if authConfig.Username == "" || authConfig.Password == "" {
		return nil, fmt.Errorf("basic authentication requires username and password")
	}

	return &http.BasicAuth{
		Username: authConfig.Username,
		Password: authConfig.Password,
	}, nil
}

// ValidateConfig validates the basic authentication configuration.
func (p *BasicProvider) ValidateConfig(authConfig *config.AuthConfig) error {
	if authConfig.Username == "" {
		return fmt.Errorf("basic authentication requires a username")
	}

	if authConfig.Password == "" {
		return fmt.Errorf("basic authentication requires a password")
	}

	return nil
}

// Name returns a human-readable name for this provider.
func (p *BasicProvider) Name() string {
	return "BasicProvider"
}
