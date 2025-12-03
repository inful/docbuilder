package foundation

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode represents a typed error classification.
type ErrorCode string

const (
	// Core error codes
	ErrorCodeValidation    ErrorCode = "validation"
	ErrorCodeNotFound      ErrorCode = "not_found"
	ErrorCodeAlreadyExists ErrorCode = "already_exists"
	ErrorCodePermission    ErrorCode = "permission"
	ErrorCodeTimeout       ErrorCode = "timeout"
	ErrorCodeCanceled      ErrorCode = "canceled"
	ErrorCodeInternal      ErrorCode = "internal"
	ErrorCodeExternal      ErrorCode = "external"
	ErrorCodeConfiguration ErrorCode = "configuration"
	ErrorCodeNetwork       ErrorCode = "network"
	ErrorCodeAuth          ErrorCode = "auth"
)

// Severity indicates the importance/impact of an error.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
	SeverityFatal    Severity = "fatal"
)

// ClassifiedError provides structured error information with context.
type ClassifiedError struct {
	Code       ErrorCode `json:"code"`
	Severity   Severity  `json:"severity"`
	Message    string    `json:"message"`
	Context    Fields    `json:"context,omitempty"`
	Cause      error     `json:"cause,omitempty"`
	Operation  string    `json:"operation,omitempty"`
	Component  string    `json:"component,omitempty"`
	Retryable  bool      `json:"retryable"`
	UserFacing bool      `json:"user_facing"`
}

// Fields represents structured context data.
type Fields map[string]any

// Error implements the error interface.
func (e *ClassifiedError) Error() string {
	var parts []string

	if e.Component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", e.Component))
	}

	if e.Operation != "" {
		parts = append(parts, fmt.Sprintf("operation=%s", e.Operation))
	}

	parts = append(parts, fmt.Sprintf("code=%s", e.Code), e.Message)

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("cause: %v", e.Cause))
	}

	return strings.Join(parts, " ")
}

// Unwrap returns the underlying cause for error unwrapping.
func (e *ClassifiedError) Unwrap() error {
	return e.Cause
}

// WithContext adds context fields to the error.
func (e *ClassifiedError) WithContext(fields Fields) *ClassifiedError {
	if e.Context == nil {
		e.Context = make(Fields)
	}
	for k, v := range fields {
		e.Context[k] = v
	}
	return e
}

// WithOperation sets the operation context.
func (e *ClassifiedError) WithOperation(operation string) *ClassifiedError {
	e.Operation = operation
	return e
}

// WithComponent sets the component context.
func (e *ClassifiedError) WithComponent(component string) *ClassifiedError {
	e.Component = component
	return e
}

// IsRetryable returns whether the error can be retried.
func (e *ClassifiedError) IsRetryable() bool {
	return e.Retryable
}

// IsUserFacing returns whether the error should be shown to users.
func (e *ClassifiedError) IsUserFacing() bool {
	return e.UserFacing
}

// ErrorBuilder provides a fluent interface for creating classified errors.
type ErrorBuilder struct {
	err *ClassifiedError
}

// NewError creates a new error builder.
func NewError(code ErrorCode, message string) *ErrorBuilder {
	return &ErrorBuilder{
		err: &ClassifiedError{
			Code:       code,
			Severity:   SeverityError,
			Message:    message,
			Context:    make(Fields),
			Retryable:  false,
			UserFacing: false,
		},
	}
}

// WithSeverity sets the error severity.
func (b *ErrorBuilder) WithSeverity(severity Severity) *ErrorBuilder {
	b.err.Severity = severity
	return b
}

// WithCause sets the underlying cause.
func (b *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	b.err.Cause = cause
	return b
}

// WithContext adds context fields.
func (b *ErrorBuilder) WithContext(fields Fields) *ErrorBuilder {
	for k, v := range fields {
		b.err.Context[k] = v
	}
	return b
}

// WithOperation sets the operation context.
func (b *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	b.err.Operation = operation
	return b
}

// WithComponent sets the component context.
func (b *ErrorBuilder) WithComponent(component string) *ErrorBuilder {
	b.err.Component = component
	return b
}

// Retryable marks the error as retryable.
func (b *ErrorBuilder) Retryable() *ErrorBuilder {
	b.err.Retryable = true
	return b
}

// UserFacing marks the error as user-facing.
func (b *ErrorBuilder) UserFacing() *ErrorBuilder {
	b.err.UserFacing = true
	return b
}

// Build returns the constructed error.
func (b *ErrorBuilder) Build() *ClassifiedError {
	return b.err
}

// Predefined error constructors for common cases
func ValidationError(message string) *ErrorBuilder {
	return NewError(ErrorCodeValidation, message).WithSeverity(SeverityWarning).UserFacing()
}

func NotFoundError(resource string) *ErrorBuilder {
	return NewError(ErrorCodeNotFound, fmt.Sprintf("%s not found", resource)).
		WithSeverity(SeverityError).
		WithContext(Fields{"resource": resource}).
		UserFacing()
}

func InternalError(message string) *ErrorBuilder {
	return NewError(ErrorCodeInternal, message).WithSeverity(SeverityCritical)
}

func ConfigurationError(message string) *ErrorBuilder {
	return NewError(ErrorCodeConfiguration, message).
		WithSeverity(SeverityError).
		UserFacing()
}

func NetworkError(message string) *ErrorBuilder {
	return NewError(ErrorCodeNetwork, message).
		WithSeverity(SeverityError).
		Retryable()
}

func AuthError(message string) *ErrorBuilder {
	return NewError(ErrorCodeAuth, message).
		WithSeverity(SeverityError).
		UserFacing()
}

// IsErrorCode checks if an error has a specific error code.
func IsErrorCode(err error, code ErrorCode) bool {
	var classifiedErr *ClassifiedError
	if AsClassified(err, &classifiedErr) {
		return classifiedErr.Code == code
	}
	return false
}

// AsClassified extracts a ClassifiedError from an error chain.
func AsClassified(err error, target **ClassifiedError) bool {
	var classified *ClassifiedError
	if errors.As(err, &classified) {
		*target = classified
		return true
	}
	return false
}
