package normalization

import "fmt"

// EnumNormalizer provides a higher-level interface for enum normalization
// that integrates with the existing config validation patterns.
type EnumNormalizer[T comparable] struct {
	normalizer *Normalizer[T]
	enumName   string // For better error messages
}

// NewEnumNormalizer creates an enum normalizer with descriptive error messages.
func NewEnumNormalizer[T comparable](enumName string, values map[string]T, defaultValue T) *EnumNormalizer[T] {
	return &EnumNormalizer[T]{
		normalizer: NewNormalizer(values, defaultValue),
		enumName:   enumName,
	}
}

// Normalize converts raw string to enum value, returning default on invalid input.
func (e *EnumNormalizer[T]) Normalize(raw string) T {
	return e.normalizer.Normalize(raw)
}

// NormalizeWithValidation converts raw string to enum value with validation error.
// This method is useful during config validation phases.
func (e *EnumNormalizer[T]) NormalizeWithValidation(raw string) (T, error) {
	result, err := e.normalizer.NormalizeWithError(raw)
	if err != nil {
		return result, fmt.Errorf("invalid %s: %w", e.enumName, err)
	}
	return result, nil
}

// IsValid checks if the normalized value would be valid.
func (e *EnumNormalizer[T]) IsValid(raw string) bool {
	normalized := e.normalizer.Normalize(raw)
	return e.normalizer.ValidateEnum(normalized)
}

// ValidValues returns all valid enum values for documentation/help.
func (e *EnumNormalizer[T]) ValidValues() []string {
	return e.normalizer.ValidKeys()
}

// NormalizationResult represents the outcome of a normalization operation
// with optional warnings about value changes.
type NormalizationResult[T comparable] struct {
	Value   T
	Changed bool
	Warning string
}

// NormalizeWithWarning performs normalization and tracks if the value was changed.
// This is useful for generating user-friendly warnings about config changes.
func (e *EnumNormalizer[T]) NormalizeWithWarning(fieldName, raw string) NormalizationResult[T] {
	cleaned := defaultNormalization(raw)
	normalized := e.normalizer.Normalize(raw)

	changed := cleaned != raw
	var warning string
	if changed {
		warning = fmt.Sprintf("normalized %s from '%s' to '%s'", fieldName, raw, cleaned)
	}

	return NormalizationResult[T]{
		Value:   normalized,
		Changed: changed,
		Warning: warning,
	}
}
