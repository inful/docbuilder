package providers

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
func (p *BasicProvider) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	if authCfg.Username == "" || authCfg.Password == "" {
		return nil, errors.New("basic authentication requires username and password")
	}

	return &http.BasicAuth{
		Username: authCfg.Username,
		Password: authCfg.Password,
	}, nil
}

// ValidateConfig validates the basic authentication configuration.
func (p *BasicProvider) ValidateConfig(authCfg *config.AuthConfig) error {
	if authCfg.Username == "" {
		return errors.New("basic authentication requires a username")
	}

	if authCfg.Password == "" {
		return errors.New("basic authentication requires a password")
	}

	return nil
}

// Name returns a human-readable name for this provider.
func (p *BasicProvider) Name() string {
	return "BasicProvider"
}
