package foundation

import (
	"fmt"
	"strings"
)

// defaultNormalizer provides standard string normalization.
func defaultNormalizer(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Normalizer provides common string normalization functions.
type Normalizer[T comparable] struct {
	validValues  map[string]T
	defaultValue T
}

// NewNormalizer creates a normalizer with a map of valid string->value pairs.
func NewNormalizer[T comparable](values map[string]T, defaultValue T) *Normalizer[T] {
	// Create a normalized version of the map
	normalized := make(map[string]T, len(values))
	for k, v := range values {
		normalized[defaultNormalizer(k)] = v
	}

	return &Normalizer[T]{
		validValues:  normalized,
		defaultValue: defaultValue,
	}
}

// Normalize attempts to convert a string to the enum type.
// Returns the default value if the string is not recognized.
func (n *Normalizer[T]) Normalize(raw string) T {
	cleaned := defaultNormalizer(raw)
	if value, exists := n.validValues[cleaned]; exists {
		return value
	}
	return n.defaultValue
}

// NormalizeWithError attempts to convert a string to the enum type.
// Returns an error if the string is not recognized.
func (n *Normalizer[T]) NormalizeWithError(raw string) (T, error) {
	cleaned := defaultNormalizer(raw)
	if value, exists := n.validValues[cleaned]; exists {
		return value, nil
	}

	var zero T
	return zero, fmt.Errorf("invalid value: %s", raw)
}
