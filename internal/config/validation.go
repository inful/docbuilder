package config

import (
	"fmt"
	"time"
)

// ValidateConfig validates the complete configuration structure
func ValidateConfig(cfg *Config) error {
	// Validate forges
	if len(cfg.Forges) == 0 {
		return fmt.Errorf("at least one forge must be configured")
	}

	forgeNames := make(map[string]bool)
	for _, forge := range cfg.Forges {
		if forge.Name == "" {
			return fmt.Errorf("forge name cannot be empty")
		}
		if forgeNames[forge.Name] {
			return fmt.Errorf("duplicate forge name: %s", forge.Name)
		}
		forgeNames[forge.Name] = true

		// Normalize & validate forge type
		if forge.Type == "" { // empty is invalid
			return fmt.Errorf("unsupported forge type: %s", forge.Type)
		}
		norm := NormalizeForgeType(string(forge.Type))
		if norm == "" {
			return fmt.Errorf("unsupported forge type: %s", forge.Type)
		}
		forge.Type = norm

		// Validate authentication
		if forge.Auth == nil {
			return fmt.Errorf("forge %s must have authentication configured", forge.Name)
		}
		if forge.Auth != nil {
			switch forge.Auth.Type {
			case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
				// ok; semantic checks done by individual clients
			default:
				return fmt.Errorf("forge %s: unsupported auth type: %s", forge.Name, forge.Auth.Type)
			}
			// Minimal semantic validation now (clients perform stricter checks when constructing)
			// Token presence is validated lazily by forge clients / git operations to permit env placeholders.
		}

		// Require at least one organization or group to be specified. This keeps discovery bounded
		// Validate explicit repository auth blocks (if provided)
		for _, repo := range cfg.Repositories {
			if repo.Auth != nil {
				switch repo.Auth.Type {
				case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
					// valid
				default:
					return fmt.Errorf("repository %s: unsupported auth type: %s", repo.Name, repo.Auth.Type)
				}
				// Token emptiness allowed (environment may supply later)
				if repo.Auth.Type == AuthTypeBasic && (repo.Auth.Username == "" || repo.Auth.Password == "") {
					return fmt.Errorf("repository %s: basic auth requires username and password", repo.Name)
				}
			}
		}
		// and matches test expectations for explicit configuration (auto-discovery can be added
		// later behind a dedicated flag to avoid surprising large scans). We now allow an empty
		// set if options.auto_discover is explicitly true.
		emptyScopes := len(forge.Organizations) == 0 && len(forge.Groups) == 0
		if emptyScopes {
			allowAuto := forge.AutoDiscover
			if !allowAuto && forge.Options != nil { // legacy/options-based flag
				if v, ok := forge.Options["auto_discover"]; ok {
					if b, ok2 := v.(bool); ok2 && b {
						allowAuto = true
					}
				}
			}
			if !allowAuto {
				return fmt.Errorf("forge %s must have at least one organization or group configured (or set auto_discover=true)", forge.Name)
			}
		}
	}

	// Validate versioning strategy
	if cfg.Versioning != nil {
		if cfg.Versioning.Strategy != StrategyBranchesAndTags && cfg.Versioning.Strategy != StrategyBranchesOnly && cfg.Versioning.Strategy != StrategyTagsOnly {
			return fmt.Errorf("invalid versioning strategy: %s", cfg.Versioning.Strategy)
		}
	}

	// Validate retry configuration
	switch cfg.Build.RetryBackoff {
	case RetryBackoffFixed, RetryBackoffLinear, RetryBackoffExponential:
	default:
		return fmt.Errorf("invalid retry_backoff: %s (allowed: fixed|linear|exponential)", cfg.Build.RetryBackoff)
	}
	// Validate clone strategy
	switch cfg.Build.CloneStrategy {
	case CloneStrategyFresh, CloneStrategyUpdate, CloneStrategyAuto:
	default:
		return fmt.Errorf("invalid clone_strategy: %s (allowed: fresh|update|auto)", cfg.Build.CloneStrategy)
	}
	if _, err := time.ParseDuration(cfg.Build.RetryInitialDelay); err != nil {
		return fmt.Errorf("invalid retry_initial_delay: %s: %w", cfg.Build.RetryInitialDelay, err)
	}
	if _, err := time.ParseDuration(cfg.Build.RetryMaxDelay); err != nil {
		return fmt.Errorf("invalid retry_max_delay: %s: %w", cfg.Build.RetryMaxDelay, err)
	}
	initDur, _ := time.ParseDuration(cfg.Build.RetryInitialDelay)
	maxDur, _ := time.ParseDuration(cfg.Build.RetryMaxDelay)
	if maxDur < initDur {
		return fmt.Errorf("retry_max_delay (%s) must be >= retry_initial_delay (%s)", cfg.Build.RetryMaxDelay, cfg.Build.RetryInitialDelay)
	}
	if cfg.Build.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative: %d", cfg.Build.MaxRetries)
	}

	return nil
}