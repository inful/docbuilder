package config

import (
	"fmt"
	"time"
)

// ValidateConfig validates the complete configuration structure using the new validation system.
// This function is now implemented directly here to avoid import cycles.
func ValidateConfig(cfg *Config) error {
	// Create a simple validation orchestrator without the separate package
	// to avoid circular dependencies while maintaining the decomposed approach
	validator := newConfigurationValidator(cfg)
	return validator.validate()
}

// configurationValidator coordinates validation across all configuration domains.
type configurationValidator struct {
	config *Config
}

// newConfigurationValidator creates a comprehensive configuration validator.
func newConfigurationValidator(config *Config) *configurationValidator {
	return &configurationValidator{config: config}
}

// validate performs comprehensive configuration validation using domain-specific methods.
func (cv *configurationValidator) validate() error {
	// Validate in order of dependencies
	if err := cv.validateForges(); err != nil {
		return err
	}
	if err := cv.validateRepositories(); err != nil {
		return err
	}
	if err := cv.validateBuild(); err != nil {
		return err
	}
	if err := cv.validateVersioning(); err != nil {
		return err
	}
	return nil
}

// validateForges validates forge configuration.
func (cv *configurationValidator) validateForges() error {
	// Check if at least one forge is configured
	if len(cv.config.Forges) == 0 {
		return fmt.Errorf("at least one forge must be configured")
	}

	// Track forge names for duplicates
	forgeNames := make(map[string]bool)

	for _, forge := range cv.config.Forges {
		// Validate forge name
		if forge.Name == "" {
			return fmt.Errorf("forge name cannot be empty")
		}
		if forgeNames[forge.Name] {
			return fmt.Errorf("duplicate forge name: %s", forge.Name)
		}
		forgeNames[forge.Name] = true

		// Validate forge type
		if err := cv.validateForgeType(forge); err != nil {
			return err
		}

		// Validate authentication
		if err := cv.validateForgeAuth(forge); err != nil {
			return err
		}

		// Validate organizations/groups requirement
		if err := cv.validateForgeScopes(forge); err != nil {
			return err
		}
	}

	return nil
}

// validateForgeType validates the forge type field.
func (cv *configurationValidator) validateForgeType(forge *ForgeConfig) error {
	// Empty forge type is explicitly invalid
	if forge.Type == "" {
		return fmt.Errorf("unsupported forge type: %s", forge.Type)
	}

	// Attempt normalization - if it returns empty, the type was invalid
	norm := NormalizeForgeType(string(forge.Type))
	if norm == "" {
		return fmt.Errorf("unsupported forge type: %s", forge.Type)
	}

	// Apply the normalized value (this maintains existing behavior)
	forge.Type = norm

	return nil
}

// validateForgeAuth validates the forge authentication configuration.
func (cv *configurationValidator) validateForgeAuth(forge *ForgeConfig) error {
	if forge.Auth == nil {
		return fmt.Errorf("forge %s must have authentication configured", forge.Name)
	}

	switch forge.Auth.Type {
	case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
		// Valid auth types - semantic checks done by individual clients
	default:
		return fmt.Errorf("forge %s: unsupported auth type: %s", forge.Name, forge.Auth.Type)
	}

	return nil
}

// validateForgeScopes validates that forge has organizations/groups or auto-discovery enabled.
func (cv *configurationValidator) validateForgeScopes(forge *ForgeConfig) error {
	emptyScopes := len(forge.Organizations) == 0 && len(forge.Groups) == 0
	if !emptyScopes {
		return nil // Has scopes, no need to check auto-discovery
	}

	// Check if auto-discovery is enabled
	allowAuto := forge.AutoDiscover
	if !allowAuto && forge.Options != nil {
		// Check legacy options-based flag
		if v, ok := forge.Options["auto_discover"]; ok {
			if b, ok2 := v.(bool); ok2 && b {
				allowAuto = true
			}
		}
	}

	if !allowAuto {
		return fmt.Errorf("forge %s must have at least one organization or group configured (or set auto_discover=true)", forge.Name)
	}

	return nil
}

// validateRepositories validates repository-specific configuration.
func (cv *configurationValidator) validateRepositories() error {
	for _, repo := range cv.config.Repositories {
		if repo.Auth != nil {
			if err := cv.validateRepoAuth(repo); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateRepoAuth validates repository authentication configuration.
func (cv *configurationValidator) validateRepoAuth(repo Repository) error {
	switch repo.Auth.Type {
	case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
		// Valid auth type
	default:
		return fmt.Errorf("repository %s: unsupported auth type: %s", repo.Name, repo.Auth.Type)
	}

	// Validate basic auth requirements
	if repo.Auth.Type == AuthTypeBasic {
		if repo.Auth.Username == "" || repo.Auth.Password == "" {
			return fmt.Errorf("repository %s: basic auth requires username and password", repo.Name)
		}
	}

	return nil
}

// validateBuild validates build configuration settings.
func (cv *configurationValidator) validateBuild() error {
	// Validate retry configuration
	if err := cv.validateRetryBackoff(); err != nil {
		return err
	}
	if err := cv.validateCloneStrategy(); err != nil {
		return err
	}
	if err := cv.validateRetryDelays(); err != nil {
		return err
	}
	if err := cv.validateMaxRetries(); err != nil {
		return err
	}

	return nil
}

// validateRetryBackoff validates the retry backoff strategy.
func (cv *configurationValidator) validateRetryBackoff() error {
	switch cv.config.Build.RetryBackoff {
	case RetryBackoffFixed, RetryBackoffLinear, RetryBackoffExponential:
		// Valid backoff strategies
	default:
		return fmt.Errorf("invalid retry_backoff: %s (allowed: fixed|linear|exponential)", cv.config.Build.RetryBackoff)
	}
	return nil
}

// validateCloneStrategy validates the clone strategy.
func (cv *configurationValidator) validateCloneStrategy() error {
	switch cv.config.Build.CloneStrategy {
	case CloneStrategyFresh, CloneStrategyUpdate, CloneStrategyAuto:
		// Valid clone strategies
	default:
		return fmt.Errorf("invalid clone_strategy: %s (allowed: fresh|update|auto)", cv.config.Build.CloneStrategy)
	}
	return nil
}

// validateRetryDelays validates retry delay durations and their relationship.
func (cv *configurationValidator) validateRetryDelays() error {
	// Validate initial delay format
	initDur, err := time.ParseDuration(cv.config.Build.RetryInitialDelay)
	if err != nil {
		return fmt.Errorf("invalid retry_initial_delay: %s: %w", cv.config.Build.RetryInitialDelay, err)
	}

	// Validate max delay format
	maxDur, err := time.ParseDuration(cv.config.Build.RetryMaxDelay)
	if err != nil {
		return fmt.Errorf("invalid retry_max_delay: %s: %w", cv.config.Build.RetryMaxDelay, err)
	}

	// Validate relationship between delays
	if maxDur < initDur {
		return fmt.Errorf("retry_max_delay (%s) must be >= retry_initial_delay (%s)",
			cv.config.Build.RetryMaxDelay, cv.config.Build.RetryInitialDelay)
	}

	return nil
}

// validateMaxRetries validates the maximum retry count.
func (cv *configurationValidator) validateMaxRetries() error {
	if cv.config.Build.MaxRetries < 0 {
		return fmt.Errorf("max_retries cannot be negative: %d", cv.config.Build.MaxRetries)
	}
	return nil
}

// validateVersioning validates versioning configuration.
func (cv *configurationValidator) validateVersioning() error {
	// Only validate if versioning is configured
	if cv.config.Versioning == nil {
		return nil
	}

	switch cv.config.Versioning.Strategy {
	case StrategyBranchesAndTags, StrategyBranchesOnly, StrategyTagsOnly:
		// Valid versioning strategies
	default:
		return fmt.Errorf("invalid versioning strategy: %s", cv.config.Versioning.Strategy)
	}

	return nil
}
