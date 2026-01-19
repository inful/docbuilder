package config

import (
	"fmt"
	"path/filepath"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// NormalizationResult captures adjustments & warnings from normalization pass.
type NormalizationResult struct{ Warnings []string }

// NormalizeConfig performs canonicalization on enumerated and bounded fields prior to default application.
// It mutates the provided config in-place and returns a result describing any coercions.
func NormalizeConfig(c *Config) (*NormalizationResult, error) {
	if c == nil {
		return nil, errors.NewError(errors.CategoryConfig, "config nil").Build()
	}
	res := &NormalizationResult{}
	normalizeBuildConfig(&c.Build, res)
	normalizeMonitoring(&c.Monitoring, res)
	normalizeVersioning(c.Versioning, res)
	normalizeOutput(&c.Output, res)
	normalizeFiltering(c.Filtering, res)

	// Cross-domain normalization and warnings
	normalizeCrossDomain(c, res)

	return res, nil
}

func normalizeCrossDomain(cfg *Config, res *NormalizationResult) {
	// Warn if user configured workspace_dir equal to output directory
	if cfg.Build.WorkspaceDir != "" && cfg.Output.Directory != "" {
		wd := filepath.Clean(cfg.Build.WorkspaceDir)
		od := filepath.Clean(cfg.Output.Directory)
		if wd == od {
			res.Warnings = append(res.Warnings, fmt.Sprintf("build.workspace_dir (%s) matches output.directory (%s); this may mix git working trees with generated site artifacts", wd, od))
		}
	}
}

// Domain-specific normalization functions live in separate files for maintainability.
// (build: normalize_build.go, monitoring: normalize_monitoring.go, versioning: normalize_versioning.go,
//  output: normalize_output.go, filtering: normalize_filtering.go)

// Helper constructors retained for existing warning string formats expected by tests.
func warnChanged(field string, from, to any) string {
	return fmt.Sprintf("normalized %s from '%v' to '%v'", field, from, to)
}

func warnUnknown(field, value, def string) string {
	return fmt.Sprintf("unknown %s '%s', defaulting to %s", field, value, def)
}
