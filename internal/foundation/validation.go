package foundation

import (
	"fmt"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/foundation/errors"
)

// Validator represents a validation function.
type Validator[T any] func(T) ValidationResult

// ValidationResult contains the result of a validation operation.
type ValidationResult struct {
	Valid  bool
	Errors []FieldError
}

// FieldError represents a single validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// Error implements the error interface.
func (fe FieldError) Error() string {
	if fe.Field != "" {
		return fmt.Sprintf("field '%s': %s", fe.Field, fe.Message)
	}
	return fe.Message
}

// Valid creates a successful validation result.
func Valid() ValidationResult {
	return ValidationResult{Valid: true}
}

// Invalid creates a failed validation result with errors.
func Invalid(errors ...FieldError) ValidationResult {
	return ValidationResult{
		Valid:  false,
		Errors: errors,
	}
}

// NewValidationError creates a validation error.
func NewValidationError(field, code, message string) FieldError {
	return FieldError{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

// Combine merges multiple validation results.
func (vr ValidationResult) Combine(other ValidationResult) ValidationResult {
	if vr.Valid && other.Valid {
		return Valid()
	}

	var allErrors []FieldError
	allErrors = append(allErrors, vr.Errors...)
	allErrors = append(allErrors, other.Errors...)

	return Invalid(allErrors...)
}

// ToError converts a validation result to an error if invalid.
func (vr ValidationResult) ToError() error {
	if vr.Valid {
		return nil
	}

	messages := make([]string, 0, len(vr.Errors))
	for _, err := range vr.Errors {
		messages = append(messages, err.Error())
	}

	return errors.ValidationError(strings.Join(messages, "; ")).Build()
}

// ValidatorChain allows chaining multiple validators.
type ValidatorChain[T any] struct {
	validators []Validator[T]
}

// NewValidatorChain creates a new validator chain.
func NewValidatorChain[T any](validators ...Validator[T]) *ValidatorChain[T] {
	return &ValidatorChain[T]{validators: validators}
}

// Add appends a validator to the chain.
func (vc *ValidatorChain[T]) Add(validator Validator[T]) *ValidatorChain[T] {
	vc.validators = append(vc.validators, validator)
	return vc
}

// Validate runs all validators in the chain.
func (vc *ValidatorChain[T]) Validate(value T) ValidationResult {
	result := Valid()

	for _, validator := range vc.validators {
		result = result.Combine(validator(value))
	}

	return result
}

// OneOf validates that a value is in a set of allowed values.
func OneOf[T comparable](field string, allowed []T) Validator[T] {
	allowedSet := make(map[T]bool, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = true
	}

	return func(value T) ValidationResult {
		if !allowedSet[value] {
			return Invalid(NewValidationError(
				field,
				"one_of",
				fmt.Sprintf("field must be one of: %v", allowed),
			))
		}
		return Valid()
	}
}
