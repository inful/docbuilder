package forge

import (
	"fmt"

	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
)

// NewForgeClient creates a new forge client based on the configuration.
func NewForgeClient(config *Config) (Client, error) {
	switch config.Type {
	case cfg.ForgeGitHub:
		return NewGitHubClient(config)
	case cfg.ForgeGitLab:
		return NewGitLabClient(config)
	case cfg.ForgeForgejo:
		return NewForgejoClient(config)
	case cfg.ForgeLocal:
		return NewLocalClient(config)
	default:
		return nil, fmt.Errorf("unsupported forge type: %s", config.Type)
	}
}

// CreateForgeManager creates a forge manager with the provided configurations.
func CreateForgeManager(configs []*Config) (*Manager, error) {
	manager := NewForgeManager()

	for _, config := range configs {
		client, err := NewForgeClient(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create forge client for %s: %w", config.Name, err)
		}

		manager.AddForge(config, client)
	}

	return manager, nil
}
