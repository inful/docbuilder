package config

import (
	"fmt"
)

// NormalizationResult captures adjustments & warnings from normalization pass.
type NormalizationResult struct{ Warnings []string }

// NormalizeConfig performs canonicalization on enumerated and bounded fields prior to default application.
// It mutates the provided config in-place and returns a result describing any coercions.
func NormalizeConfig(c *Config) (*NormalizationResult, error) {
	if c == nil {
		return nil, fmt.Errorf("config nil")
	}
	res := &NormalizationResult{}
	normalizeBuildConfig(&c.Build, res)
	normalizeMonitoring(&c.Monitoring, res)
	normalizeVersioning(c.Versioning, res)
	normalizeOutput(&c.Output, res)
	normalizeFiltering(c.Filtering, res)
	return res, nil
}
// Domain-specific normalization functions live in separate files for maintainability.
// (build: normalize_build.go, monitoring: normalize_monitoring.go, versioning: normalize_versioning.go,
//  output: normalize_output.go, filtering: normalize_filtering.go)

// Helper constructors retained for existing warning string formats expected by tests.
func warnChanged(field string, from, to interface{}) string { return fmt.Sprintf("normalized %s from '%v' to '%v'", field, from, to) }
func warnUnknown(field, value, def string) string { return fmt.Sprintf("unknown %s '%s', defaulting to %s", field, value, def) }
