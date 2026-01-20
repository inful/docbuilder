package forge

import (
	cfg "git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
		return nil, errors.ConfigError("unsupported forge type").
			WithContext("type", config.Type).
			Fatal().
			Build()
	}
}

// CreateForgeManager creates a forge manager with the provided configurations.
func CreateForgeManager(configs []*Config) (*Manager, error) {
	manager := NewForgeManager()

	for _, config := range configs {
		client, err := NewForgeClient(config)
		if err != nil {
			return nil, errors.ForgeError("failed to create forge client").
				WithCause(err).
				WithContext("name", config.Name).
				Build()
		}

		manager.AddForge(config, client)
	}

	return manager, nil
}
