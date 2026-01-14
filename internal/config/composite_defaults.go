package config

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
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
			return errors.WrapError(err, errors.CategoryConfig, fmt.Sprintf("applying defaults for %s", applier.Domain())).WithSeverity(errors.SeverityError).WithContext("domain", applier.Domain()).Build()
		}
	}
	return nil
}
