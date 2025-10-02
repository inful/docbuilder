package foundation

import (
	"fmt"
	"strings"
)

// Enum represents a type-safe enumeration with validation capabilities.
type Enum[T comparable] interface {
	String() string
	Valid() bool
}

// EnumRegistry provides parsing and validation for enum types.
type EnumRegistry[T comparable] struct {
	values       []T
	stringValues map[string]T
	normalizer   func(string) string
}

// NewEnumRegistry creates a new registry for enum values.
func NewEnumRegistry[T comparable](values []T, toString func(T) string) *EnumRegistry[T] {
	registry := &EnumRegistry[T]{
		values:       make([]T, len(values)),
		stringValues: make(map[string]T, len(values)),
		normalizer:   defaultNormalizer,
	}

	copy(registry.values, values)

	for _, value := range values {
		key := registry.normalizer(toString(value))
		registry.stringValues[key] = value
	}

	return registry
}

// WithNormalizer sets a custom string normalization function.
// Note: This requires rebuilding the registry with the original toString function.
func (r *EnumRegistry[T]) WithNormalizer(normalizer func(string) string) *EnumRegistry[T] {
	r.normalizer = normalizer
	return r
}

// Parse attempts to parse a string into the enum type.
func (r *EnumRegistry[T]) Parse(s string) (T, error) {
	normalized := r.normalizer(s)
	if value, exists := r.stringValues[normalized]; exists {
		return value, nil
	}

	var zero T
	return zero, fmt.Errorf("invalid enum value: %s", s)
}

// Valid checks if a value is in the registry.
func (r *EnumRegistry[T]) Valid(value T) bool {
	for _, v := range r.values {
		if v == value {
			return true
		}
	}
	return false
}

// Values returns all valid enum values.
func (r *EnumRegistry[T]) Values() []T {
	result := make([]T, len(r.values))
	copy(result, r.values)
	return result
}

// defaultNormalizer provides standard string normalization.
func defaultNormalizer(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// StringEnum provides a concrete implementation for string-based enums.
type StringEnum struct {
	value    string
	registry *EnumRegistry[StringEnum]
}

// NewStringEnum creates a new string-based enum value.
func NewStringEnum(value string, registry *EnumRegistry[StringEnum]) StringEnum {
	return StringEnum{
		value:    value,
		registry: registry,
	}
}

// String returns the string representation of the enum.
func (e StringEnum) String() string {
	return e.value
}

// Valid checks if the enum value is valid according to its registry.
func (e StringEnum) Valid() bool {
	return e.registry.Valid(e)
}

// ParseStringEnum parses a string into a StringEnum using the given registry.
func ParseStringEnum(s string, registry *EnumRegistry[StringEnum]) (StringEnum, error) {
	return registry.Parse(s)
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
