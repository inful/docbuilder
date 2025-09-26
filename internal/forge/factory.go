package forge

import (
	"fmt"
)

// NewForgeClient creates a new forge client based on the configuration
func NewForgeClient(config *ForgeConfig) (ForgeClient, error) {
	switch config.Type {
	case string(ForgeTypeGitHub):
		return NewGitHubClient(config)
	case string(ForgeTypeGitLab):
		return NewGitLabClient(config)
	case string(ForgeTypeForgejo):
		return NewForgejoClient(config)
	default:
		return nil, fmt.Errorf("unsupported forge type: %s", config.Type)
	}
}

// CreateForgeManager creates a forge manager with the provided configurations
func CreateForgeManager(configs []*ForgeConfig) (*ForgeManager, error) {
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
