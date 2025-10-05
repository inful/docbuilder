package config

import "fmt"

// CompositeDefaultApplier applies defaults across all configuration domains
type CompositeDefaultApplier struct {
	appliers []ConfigDefaultApplier
}

// NewDefaultApplier creates a composite default applier with all domain appliers
func NewDefaultApplier() *CompositeDefaultApplier {
	return &CompositeDefaultApplier{
		appliers: []ConfigDefaultApplier{
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

// ApplyDefaults applies defaults for all configuration domains
func (c *CompositeDefaultApplier) ApplyDefaults(cfg *Config) error {
	for _, applier := range c.appliers {
		if err := applier.ApplyDefaults(cfg); err != nil {
			return fmt.Errorf("applying defaults for %s: %w", applier.Domain(), err)
		}
	}
	return nil
}

// GetApplierByDomain returns a specific domain applier (useful for testing)
func (c *CompositeDefaultApplier) GetApplierByDomain(domain string) ConfigDefaultApplier {
	for _, applier := range c.appliers {
		if applier.Domain() == domain {
			return applier
		}
	}
	return nil
}
