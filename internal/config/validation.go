package config

import (
	"path/filepath"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

const defaultOutputDir = "./site"

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
	if err := cv.validatePaths(); err != nil {
		return err
	}
	if err := cv.validateVersioning(); err != nil {
		return err
	}
	return nil
}

// validateForges validates forge configuration.
func (cv *configurationValidator) validateForges() error {
	// If repositories are explicitly configured, forges are optional
	if len(cv.config.Forges) == 0 && len(cv.config.Repositories) == 0 {
		return errors.NewError(errors.CategoryValidation, "either forges or repositories must be configured").Build()
	}

	// Skip forge validation if no forges configured (direct repository mode)
	if len(cv.config.Forges) == 0 {
		return nil
	}

	// Track forge names for duplicates
	forgeNames := make(map[string]bool)

	for _, forge := range cv.config.Forges {
		// Validate forge name
		if forge.Name == "" {
			return errors.NewError(errors.CategoryValidation, "forge name cannot be empty").Build()
		}
		if forgeNames[forge.Name] {
			return errors.NewError(errors.CategoryValidation, "duplicate forge name").
				WithContext("name", forge.Name).
				Build()
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
		return errors.NewError(errors.CategoryValidation, "unsupported forge type").
			WithContext("type", string(forge.Type)).
			Build()
	}

	// Attempt normalization - if it returns empty, the type was invalid
	norm := NormalizeForgeType(string(forge.Type))
	if norm == "" {
		return errors.NewError(errors.CategoryValidation, "unsupported forge type").
			WithContext("type", string(forge.Type)).
			Build()
	}

	// Apply the normalized value (this maintains existing behavior)
	forge.Type = norm

	return nil
}

// validateForgeAuth validates the forge authentication configuration.
func (cv *configurationValidator) validateForgeAuth(forge *ForgeConfig) error {
	if forge.Auth == nil {
		return errors.NewError(errors.CategoryValidation, "forge must have authentication configured").
			WithContext("forge", forge.Name).
			Build()
	}

	switch forge.Auth.Type {
	case AuthTypeToken, AuthTypeSSH, AuthTypeBasic, AuthTypeNone, "":
		// Valid auth types - semantic checks done by individual clients
	default:
		return errors.NewError(errors.CategoryValidation, "forge has unsupported auth type").
			WithContext("forge", forge.Name).
			WithContext("type", string(forge.Auth.Type)).
			Build()
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
		return errors.NewError(errors.CategoryValidation, "forge must have at least one organization or group configured (or set auto_discover=true)").
			WithContext("forge", forge.Name).
			Build()
	}

	return nil
}

// validateRepositories validates repository-specific configuration.
func (cv *configurationValidator) validateRepositories() error {
	for i := range cv.config.Repositories {
		repo := &cv.config.Repositories[i]
		if repo.Auth != nil {
			if err := cv.validateRepoAuth(*repo); err != nil {
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
		return errors.NewError(errors.CategoryValidation, "unsupported auth type").
			WithContext("repository", repo.Name).
			WithContext("type", string(repo.Auth.Type)).
			Build()
	}

	// Validate basic auth requirements
	if repo.Auth.Type == AuthTypeBasic {
		if repo.Auth.Username == "" || repo.Auth.Password == "" {
			return errors.NewError(errors.CategoryValidation, "basic auth requires username and password").
				WithContext("repository", repo.Name).
				Build()
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
		return errors.NewError(errors.CategoryValidation, "invalid retry_backoff").
			WithContext("actual", string(cv.config.Build.RetryBackoff)).
			WithContext("allowed", "fixed|linear|exponential").
			Build()
	}
	return nil
}

// validateCloneStrategy validates the clone strategy.
func (cv *configurationValidator) validateCloneStrategy() error {
	switch cv.config.Build.CloneStrategy {
	case CloneStrategyFresh, CloneStrategyUpdate, CloneStrategyAuto:
		// Valid clone strategies
	default:
		return errors.NewError(errors.CategoryValidation, "invalid clone_strategy").
			WithContext("actual", string(cv.config.Build.CloneStrategy)).
			WithContext("allowed", "fresh|update|auto").
			Build()
	}
	return nil
}

// validateRetryDelays validates retry delay durations and their relationship.
func (cv *configurationValidator) validateRetryDelays() error {
	// Validate initial delay format
	initDur, err := time.ParseDuration(cv.config.Build.RetryInitialDelay)
	if err != nil {
		return errors.WrapError(err, errors.CategoryValidation, "invalid retry_initial_delay").
			WithContext("value", cv.config.Build.RetryInitialDelay).
			Build()
	}

	// Validate max delay format
	maxDur, err := time.ParseDuration(cv.config.Build.RetryMaxDelay)
	if err != nil {
		return errors.WrapError(err, errors.CategoryValidation, "invalid retry_max_delay").
			WithContext("value", cv.config.Build.RetryMaxDelay).
			Build()
	}

	// Validate relationship between delays
	if maxDur < initDur {
		return errors.NewError(errors.CategoryValidation, "retry_max_delay must be >= retry_initial_delay").
			WithContext("max_delay", cv.config.Build.RetryMaxDelay).
			WithContext("initial_delay", cv.config.Build.RetryInitialDelay).
			Build()
	}

	return nil
}

func (cv *configurationValidator) validateMaxRetries() error {
	if cv.config.Build.MaxRetries < 0 {
		return errors.NewError(errors.CategoryValidation, "max_retries cannot be negative").
			WithContext("value", cv.config.Build.MaxRetries).
			Build()
	}
	return nil
}

// validatePaths ensures output directories are unified across config domains.
// Canonical source is output.directory; daemon.storage.output_dir must match when set.
func (cv *configurationValidator) validatePaths() error {
	out := cv.config.Output.Directory
	if out == "" {
		out = defaultOutputDir // default applied elsewhere, but keep guard for safety
	}
	out = filepath.Clean(out)
	if cv.config.Daemon != nil {
		s := cv.config.Daemon.Storage.OutputDir
		if s != "" {
			s = filepath.Clean(s)
			if s != out {
				return errors.NewError(errors.CategoryValidation, "output directory mismatch").
					WithContext("daemon_output_dir", cv.config.Daemon.Storage.OutputDir).
					WithContext("output_directory", cv.config.Output.Directory).
					Build()
			}
		}
	}
	return nil
}

// validateVersioning validates versioning configuration.
func (cv *configurationValidator) validateVersioning() error {
	// Only validate if versioning is configured
	if cv.config.Versioning == nil {
		return nil
	}

	// If strategy is explicitly provided, validate it regardless of enabled status
	// This catches configuration errors early
	if cv.config.Versioning.Strategy != "" {
		switch cv.config.Versioning.Strategy {
		case StrategyBranchesAndTags, StrategyBranchesOnly, StrategyTagsOnly:
			// Valid versioning strategies
		default:
			return errors.NewError(errors.CategoryValidation, "invalid versioning strategy").
				WithContext("strategy", string(cv.config.Versioning.Strategy)).
				Build()
		}
	}

	// If versioning is explicitly enabled, require a strategy
	if cv.config.Versioning.Enabled && cv.config.Versioning.Strategy == "" {
		return errors.NewError(errors.CategoryValidation, "versioning strategy is required when versioning is enabled").Build()
	}

	return nil
}
