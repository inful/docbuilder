package config

import (
	derrors "git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// CompositeDefaultApplier applies defaults across all configuration domains.
type CompositeDefaultApplier struct {
	appliers []DefaultApplier
}

// NewDefaultApplier creates a composite default applier with all domain appliers.
func NewDefaultApplier() *CompositeDefaultApplier {
	return &CompositeDefaultApplier{
		appliers: []DefaultApplier{
			&BuildDefaultApplier{},
			&HugoDefaultApplier{},
			&OutputDefaultApplier{},
			&DaemonDefaultApplier{},
			&FilteringDefaultApplier{},
			&VersioningDefaultApplier{},
			&MonitoringDefaultApplier{},
			&RepositoryDefaultApplier{},
		},
	}
}

// ApplyDefaults applies defaults for all configuration domains.
func (c *CompositeDefaultApplier) ApplyDefaults(cfg *Config) error {
	for _, applier := range c.appliers {
		if err := applier.ApplyDefaults(cfg); err != nil {
			return derrors.WrapError(err, derrors.CategoryConfig, "applying defaults").WithContext("domain", applier.Domain()).Build()
		}
	}
	return nil
}
