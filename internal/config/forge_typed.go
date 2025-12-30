package config

import (
	"fmt"
	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"

	"git.home.luguber.info/inful/docbuilder/internal/foundation"
)

// ForgeTyped represents a type-safe forge type using the foundation enum system.
type ForgeTyped struct {
	value string
}

// Predefined forge types using the new pattern.
var (
	ForgeTypedGitHub  = ForgeTyped{"github"}
	ForgeTypedGitLab  = ForgeTyped{"gitlab"}
	ForgeTypedForgejo = ForgeTyped{"forgejo"}

	// Registry for validation and parsing.
	forgeTypeNormalizer = foundation.NewNormalizer(map[string]ForgeTyped{
		"github":  ForgeTypedGitHub,
		"gitlab":  ForgeTypedGitLab,
		"forgejo": ForgeTypedForgejo,
	}, ForgeTypedGitHub) // default to GitHub

	// Validation for forge type fields.
	forgeTypeValidator = foundation.OneOf("forge_type", []ForgeTyped{
		ForgeTypedGitHub, ForgeTypedGitLab, ForgeTypedForgejo,
	})
)

// String returns the string representation of the forge type.
func (ft ForgeTyped) String() string {
	return ft.value
}

// Valid checks if the forge type is one of the known types.
func (ft ForgeTyped) Valid() bool {
	return forgeTypeValidator(ft).Valid
}

// ParseForgeTyped parses a string into a ForgeTyped with error handling.
func ParseForgeTyped(s string) foundation.Result[ForgeTyped, error] {
	forgeType, err := forgeTypeNormalizer.NormalizeWithError(s)
	if err != nil {
		return foundation.Err[ForgeTyped, error](
			errors.ValidationError(fmt.Sprintf("invalid forge type: %s", s)).
				WithContext("input", s).
				WithContext("valid_values", []string{"github", "gitlab", "forgejo"}).
				Build(),
		)
	}
	return foundation.Ok[ForgeTyped, error](forgeType)
}

// NormalizeForgeTyped normalizes a string to a ForgeTyped, returning the default if invalid.
func NormalizeForgeTyped(s string) ForgeTyped {
	return forgeTypeNormalizer.Normalize(s)
}

// TypedForgeConfig demonstrates how to use strong typing instead of map[string]any.
type TypedForgeConfig struct {
	Type     ForgeTyped                `json:"type" yaml:"type"`
	BaseURL  foundation.Option[string] `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	Token    foundation.Option[string] `json:"token,omitempty" yaml:"token,omitempty"`
	Username foundation.Option[string] `json:"username,omitempty" yaml:"username,omitempty"`
	Password foundation.Option[string] `json:"password,omitempty" yaml:"password,omitempty"`
	Settings map[string]any            `json:"settings,omitempty" yaml:"settings,omitempty"` // For truly dynamic fields
}

// Validate performs comprehensive validation of the forge configuration.
func (fc *TypedForgeConfig) Validate() foundation.ValidationResult {
	chain := foundation.NewValidatorChain(
		// Validate forge type
		func(config TypedForgeConfig) foundation.ValidationResult {
			return forgeTypeValidator(config.Type)
		},

		// Validate that we have authentication if token is provided
		func(config TypedForgeConfig) foundation.ValidationResult {
			if config.Token.IsSome() && config.Token.Unwrap() == "" {
				return foundation.Invalid(
					foundation.NewValidationError("token", "not_empty", "token cannot be empty if provided"),
				)
			}
			return foundation.Valid()
		},

		// Validate base URL format if provided
		func(config TypedForgeConfig) foundation.ValidationResult {
			if config.BaseURL.IsSome() {
				url := config.BaseURL.Unwrap()
				if url == "" {
					return foundation.Invalid(
						foundation.NewValidationError("base_url", "not_empty", "base_url cannot be empty if provided"),
					)
				}
				// Could add URL format validation here
			}
			return foundation.Valid()
		},
	)

	return chain.Validate(*fc)
}

// Legacy conversion functions removed - use ForgeTyped directly instead.
