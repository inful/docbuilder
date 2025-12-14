package foundation

import (
	"fmt"
	"reflect"
	"regexp"
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

// NewValidationResult creates a successful validation result.
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

// Common validators

// Required validates that a value is not the zero value.
func Required[T comparable](field string) Validator[T] {
	return func(value T) ValidationResult {
		var zero T
		if value == zero {
			return Invalid(NewValidationError(field, "required", "field is required"))
		}
		return Valid()
	}
}

// StringNotEmpty validates that a string is not empty.
func StringNotEmpty(field string) Validator[string] {
	return func(value string) ValidationResult {
		if strings.TrimSpace(value) == "" {
			return Invalid(NewValidationError(field, "not_empty", "field cannot be empty"))
		}
		return Valid()
	}
}

// StringMinLength validates minimum string length.
func StringMinLength(field string, minLength int) Validator[string] {
	return func(value string) ValidationResult {
		if len(value) < minLength {
			return Invalid(NewValidationError(
				field,
				"min_length",
				fmt.Sprintf("field must be at least %d characters", minLength),
			))
		}
		return Valid()
	}
}

// StringMaxLength validates maximum string length.
func StringMaxLength(field string, maxLength int) Validator[string] {
	return func(value string) ValidationResult {
		if len(value) > maxLength {
			return Invalid(NewValidationError(
				field,
				"max_length",
				fmt.Sprintf("field must be at most %d characters", maxLength),
			))
		}
		return Valid()
	}
}

// StringPattern validates that a string matches a regex pattern.
func StringPattern(field string, pattern *regexp.Regexp, message string) Validator[string] {
	return func(value string) ValidationResult {
		if !pattern.MatchString(value) {
			return Invalid(NewValidationError(field, "pattern", message))
		}
		return Valid()
	}
}

// SliceNotEmpty validates that a slice is not empty.
func SliceNotEmpty[T any](field string) Validator[[]T] {
	return func(value []T) ValidationResult {
		if len(value) == 0 {
			return Invalid(NewValidationError(field, "not_empty", "field cannot be empty"))
		}
		return Valid()
	}
}

// SliceMinLength validates minimum slice length.
func SliceMinLength[T any](field string, minLength int) Validator[[]T] {
	return func(value []T) ValidationResult {
		if len(value) < minLength {
			return Invalid(NewValidationError(
				field,
				"min_length",
				fmt.Sprintf("field must have at least %d items", minLength),
			))
		}
		return Valid()
	}
}

// SliceMaxLength validates maximum slice length.
func SliceMaxLength[T any](field string, maxLength int) Validator[[]T] {
	return func(value []T) ValidationResult {
		if len(value) > maxLength {
			return Invalid(NewValidationError(
				field,
				"max_length",
				fmt.Sprintf("field must have at most %d items", maxLength),
			))
		}
		return Valid()
	}
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

// Custom creates a custom validator with a predicate function.
func Custom[T any](field, code, message string, predicate func(T) bool) Validator[T] {
	return func(value T) ValidationResult {
		if !predicate(value) {
			return Invalid(NewValidationError(field, code, message))
		}
		return Valid()
	}
}

// StructValidator provides validation for struct fields.
type StructValidator struct {
	validators map[string]func(any) ValidationResult
}

// NewStructValidator creates a new struct validator.
func NewStructValidator() *StructValidator {
	return &StructValidator{
		validators: make(map[string]func(any) ValidationResult),
	}
}

// AddField adds a validator for a specific field.
func (sv *StructValidator) AddField(fieldName string, validator func(any) ValidationResult) *StructValidator {
	sv.validators[fieldName] = validator
	return sv
}

// Validate validates a struct using reflection.
func (sv *StructValidator) Validate(value any) ValidationResult {
	result := Valid()

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return Invalid(NewValidationError("", "invalid_type", "value must be a struct"))
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		if validator, exists := sv.validators[field.Name]; exists {
			fieldResult := validator(fieldValue.Interface())
			result = result.Combine(fieldResult)
		}
	}

	return result
}
