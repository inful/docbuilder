package normalization

import (
	"fmt"
	"sort"
	"strings"
)

// Normalizer provides type-safe string-to-enum normalization with error handling.
type Normalizer[T comparable] struct {
	validValues  map[string]T
	defaultValue T
	validKeys    []string // Cached for error messages
}

// NewNormalizer creates a normalizer with a map of valid string->value pairs.
// The keys in the values map will be normalized using defaultNormalization.
func NewNormalizer[T comparable](values map[string]T, defaultValue T) *Normalizer[T] {
	// Create a normalized version of the map
	normalized := make(map[string]T, len(values))
	validKeys := make([]string, 0, len(values))

	for k, v := range values {
		normalizedKey := defaultNormalization(k)
		normalized[normalizedKey] = v
		validKeys = append(validKeys, normalizedKey)
	}

	// Sort keys for consistent error messages
	sort.Strings(validKeys)

	return &Normalizer[T]{
		validValues:  normalized,
		defaultValue: defaultValue,
		validKeys:    validKeys,
	}
}

// Normalize attempts to convert a string to the enum type.
// Returns the default value if the string is not recognized.
func (n *Normalizer[T]) Normalize(raw string) T {
	cleaned := defaultNormalization(raw)
	if value, exists := n.validValues[cleaned]; exists {
		return value
	}
	return n.defaultValue
}

// NormalizeWithError attempts to convert a string to the enum type.
// Returns an error if the string is not recognized.
func (n *Normalizer[T]) NormalizeWithError(raw string) (T, error) {
	cleaned := defaultNormalization(raw)
	if value, exists := n.validValues[cleaned]; exists {
		return value, nil
	}

	var zero T
	return zero, fmt.Errorf("invalid value %q, valid options: %v", raw, n.validKeys)
}

// ValidateEnum checks if a value is valid without normalization.
// This is useful for validation after normalization has occurred.
func (n *Normalizer[T]) ValidateEnum(value T) bool {
	for _, v := range n.validValues {
		if v == value {
			return true
		}
	}
	return false
}

// ValidKeys returns all valid normalized keys.
func (n *Normalizer[T]) ValidKeys() []string {
	result := make([]string, len(n.validKeys))
	copy(result, n.validKeys)
	return result
}

// defaultNormalization provides standard string normalization.
// This matches the existing pattern used throughout the config package.
func defaultNormalization(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Func allows custom normalization behavior.
type Func func(string) string

// WithCustomNormalizer creates a normalizer with custom string normalization.
func WithCustomNormalizer[T comparable](values map[string]T, defaultValue T, normalizer Func) *Normalizer[T] {
	normalized := make(map[string]T, len(values))
	validKeys := make([]string, 0, len(values))

	for k, v := range values {
		normalizedKey := normalizer(k)
		normalized[normalizedKey] = v
		validKeys = append(validKeys, normalizedKey)
	}

	sort.Strings(validKeys)

	return &Normalizer[T]{
		validValues:  normalized,
		defaultValue: defaultValue,
		validKeys:    validKeys,
	}
}
