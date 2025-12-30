package providers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"git.home.luguber.info/inful/docbuilder/internal/config"
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
func (p *SSHProvider) CreateAuth(authCfg *config.AuthConfig) (transport.AuthMethod, error) {
	keyPath := authCfg.KeyPath
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
func (p *SSHProvider) ValidateConfig(authCfg *config.AuthConfig) error {
	keyPath := authCfg.KeyPath
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
