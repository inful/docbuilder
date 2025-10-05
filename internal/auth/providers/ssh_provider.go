package providers

import (
	"fmt"
	"os"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// SSHProvider handles SSH key authentication.
type SSHProvider struct{}

// NewSSHProvider creates a new SSH authentication provider.
func NewSSHProvider() *SSHProvider {
	return &SSHProvider{}
}

// Type returns the authentication type this provider handles.
func (p *SSHProvider) Type() config.AuthType {
	return config.AuthTypeSSH
}

// CreateAuth creates SSH authentication from the configuration.
func (p *SSHProvider) CreateAuth(authConfig *config.AuthConfig) (transport.AuthMethod, error) {
	keyPath := authConfig.KeyPath
	if keyPath == "" {
		keyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	}

	publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
	}

	return publicKeys, nil
}

// ValidateConfig validates the SSH authentication configuration.
func (p *SSHProvider) ValidateConfig(authConfig *config.AuthConfig) error {
	keyPath := authConfig.KeyPath
	if keyPath == "" {
		keyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
	}

	// Check if the key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", keyPath)
	}

	return nil
}

// Name returns a human-readable name for this provider.
func (p *SSHProvider) Name() string {
	return "SSHProvider"
}

// CreateAuthWithContext creates SSH authentication with additional context.
// This implements EnhancedAuthProvider to allow for context-aware SSH key selection.
func (p *SSHProvider) CreateAuthWithContext(authConfig *config.AuthConfig, ctx AuthContext) (transport.AuthMethod, error) {
	// For now, SSH provider doesn't use context, but this allows for future enhancements
	// like per-repository SSH keys or operation-specific key selection
	return p.CreateAuth(authConfig)
}
