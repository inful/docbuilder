// Package errors provides a lightweight structured error type (DocBuilderError)
// for category-based classification and retry semantics in HTTP adapters and CLI.
package errors

import (
	"fmt"
)

// ...existing code...

// ErrorCategory represents the category of a DocBuilder error for classification
type ErrorCategory string

const (
	// User-facing configuration and input errors
	CategoryConfig     ErrorCategory = "config"
	CategoryValidation ErrorCategory = "validation"
	CategoryAuth       ErrorCategory = "auth"

	// External system integration errors
	CategoryNetwork ErrorCategory = "network"
	CategoryGit     ErrorCategory = "git"
	CategoryForge   ErrorCategory = "forge"

	// Build and processing errors
	CategoryBuild      ErrorCategory = "build"
	CategoryHugo       ErrorCategory = "hugo"
	CategoryFileSystem ErrorCategory = "filesystem"

	// Runtime and infrastructure errors
	CategoryRuntime  ErrorCategory = "runtime"
	CategoryDaemon   ErrorCategory = "daemon"
	CategoryInternal ErrorCategory = "internal"
)

// ErrorSeverity indicates how critical an error is
type ErrorSeverity string

const (
	SeverityFatal   ErrorSeverity = "fatal"   // Stops execution
	SeverityError   ErrorSeverity = "error"   // Error, but not fatal
	SeverityWarning ErrorSeverity = "warning" // Continues with degraded functionality
	SeverityInfo    ErrorSeverity = "info"    // Informational, no impact
)

// DocBuilderError is a structured error with category, retryability, and context
type DocBuilderError struct {
	Category  ErrorCategory `json:"category"`
	Severity  ErrorSeverity `json:"severity"`
	Message   string        `json:"message"`
	Cause     error         `json:"cause,omitempty"`
	Retryable bool          `json:"retryable"`
	Context   ContextFields `json:"context,omitempty"`
}

// Build returns the error itself for compatibility with legacy error adapter usage.
func (e *DocBuilderError) Build() *DocBuilderError {
	return e
}

// ContextFields carries structured context for DocBuilderError
type ContextFields map[string]any

// Error implements the error interface
func (e *DocBuilderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (%s): %s: %v", e.Category, e.Severity, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s (%s): %s", e.Category, e.Severity, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+ error handling
func (e *DocBuilderError) Unwrap() error {
	return e.Cause
}

// WithContext adds context information to the error
func (e *DocBuilderError) WithContext(key string, value any) *DocBuilderError {
	if e.Context == nil {
		e.Context = make(ContextFields)
	}
	e.Context[key] = value
	return e
}

// New creates a new DocBuilderError
func New(category ErrorCategory, severity ErrorSeverity, message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Retryable: false,
	}
}

// Wrap creates a new DocBuilderError that wraps an existing error
func Wrap(err error, category ErrorCategory, severity ErrorSeverity, message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Cause:     err,
		Retryable: false,
	}
}

// Retryable creates a new retryable DocBuilderError
func Retryable(category ErrorCategory, severity ErrorSeverity, message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Retryable: true,
	}
}

// WrapRetryable creates a new retryable DocBuilderError that wraps an existing error
func WrapRetryable(err error, category ErrorCategory, severity ErrorSeverity, message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Cause:     err,
		Retryable: true,
	}
}

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category ErrorCategory) bool {
	if dbe, ok := err.(*DocBuilderError); ok {
		return dbe.Category == category
	}
	return false
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if dbe, ok := err.(*DocBuilderError); ok {
		return dbe.Retryable
	}
	return false
}

// GetCategory extracts the category from an error, or returns CategoryInternal if not a DocBuilderError
func GetCategory(err error) ErrorCategory {
	if dbe, ok := err.(*DocBuilderError); ok {
		return dbe.Category
	}
	return CategoryInternal
}

// ValidationError creates a new validation error (400 Bad Request)
func ValidationError(message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  CategoryValidation,
		Severity:  SeverityWarning,
		Message:   message,
		Retryable: false,
	}
}

// DaemonError creates a new daemon error (service unavailable)
func DaemonError(message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  CategoryDaemon,
		Severity:  SeverityError,
		Message:   message,
		Retryable: false,
	}
}

// WrapError wraps an existing error with a new DocBuilderError
func WrapError(err error, category ErrorCategory, message string) *DocBuilderError {
	return &DocBuilderError{
		Category:  category,
		Severity:  SeverityError,
		Message:   message,
		Cause:     err,
		Retryable: false,
	}
}
