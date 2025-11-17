package providers

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// NoneProvider handles "none" authentication (no authentication).
type NoneProvider struct{}

// NewNoneProvider creates a new none authentication provider.
func NewNoneProvider() *NoneProvider {
	return &NoneProvider{}
}

// Type returns the authentication type this provider handles.
func (p *NoneProvider) Type() config.AuthType {
	return config.AuthTypeNone
}

// CreateAuth creates no authentication (returns nil).
func (p *NoneProvider) CreateAuth(_ *config.AuthConfig) (transport.AuthMethod, error) {
	return nil, nil
}

// ValidateConfig validates that no authentication is properly configured.
func (p *NoneProvider) ValidateConfig(_ *config.AuthConfig) error {
	// No validation needed for none auth
	return nil
}

// Name returns a human-readable name for this provider.
func (p *NoneProvider) Name() string {
	return "NoneProvider"
}
