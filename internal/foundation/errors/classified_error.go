package errors
import (
	"fmt"
)

// ClassifiedError represents a structured error with category, severity, and context.
// This provides a foundation for building type-safe error handling throughout DocBuilder.
type ClassifiedError struct {
	category ErrorCategory
	severity ErrorSeverity
	retry    RetryStrategy
	message  string
	cause    error
	context  ErrorContext
}

// Error implements the standard error interface.
func (e *ClassifiedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.category, e.severity, e.message, e.cause)
	}
	return fmt.Sprintf("[%s:%s] %s", e.category, e.severity, e.message)
}

// Unwrap implements Go 1.13+ error unwrapping.
func (e *ClassifiedError) Unwrap() error {
	return e.cause
}

// Category returns the error category.
func (e *ClassifiedError) Category() ErrorCategory {
	return e.category
}

// Severity returns the error severity.
func (e *ClassifiedError) Severity() ErrorSeverity {
	return e.severity
}

// RetryStrategy returns the recommended retry strategy.
func (e *ClassifiedError) RetryStrategy() RetryStrategy {
	return e.retry
}

// Message returns the error message.
func (e *ClassifiedError) Message() string {
	return e.message
}

// Cause returns the underlying error.
func (e *ClassifiedError) Cause() error {
	return e.cause
}

// Context returns the error context.
func (e *ClassifiedError) Context() ErrorContext {
	return e.context
}

// WithContext adds context to the error and returns a new error.
func (e *ClassifiedError) WithContext(key string, value any) *ClassifiedError {
	newContext := e.context.Set(key, value)
	return &ClassifiedError{
		category: e.category,
		severity: e.severity,
		retry:    e.retry,
		message:  e.message,
		cause:    e.cause,
		context:  newContext,
	}
}

// WithContextMap adds multiple context values and returns a new error.
func (e *ClassifiedError) WithContextMap(ctx ErrorContext) *ClassifiedError {
	newContext := e.context.Merge(ctx)
	return &ClassifiedError{
		category: e.category,
		severity: e.severity,
		retry:    e.retry,
		message:  e.message,
		cause:    e.cause,
		context:  newContext,
	}
}

// Is implements error comparison for Go 1.13+ error handling.
func (e *ClassifiedError) Is(target error) bool {
	if other, ok := target.(*ClassifiedError); ok {
		return e.category == other.category && e.message == other.message
	}
	return false
}

// IsCategory checks if the error belongs to a specific category.
func (e *ClassifiedError) IsCategory(category ErrorCategory) bool {
	return e.category == category
}

// IsSeverity checks if the error has a specific severity.
func (e *ClassifiedError) IsSeverity(severity ErrorSeverity) bool {
	return e.severity == severity
}

// CanRetry checks if the error allows retry operations.
func (e *ClassifiedError) CanRetry() bool {
	return e.retry != RetryNever && e.retry != RetryUserAction
}

// IsFatal checks if the error is fatal (should stop execution).
func (e *ClassifiedError) IsFatal() bool {
	return e.severity == SeverityFatal
}

// IsTransient checks if the error represents a transient condition.
func (e *ClassifiedError) IsTransient() bool {
	return e.retry == RetryImmediate || e.retry == RetryBackoff || e.retry == RetryRateLimit
}

// Helper functions for error detection and extraction

// IsClassified checks if an error is a ClassifiedError.
func IsClassified(err error) bool {
	_, ok := err.(*ClassifiedError)
	return ok
}

// AsClassified attempts to convert an error to a ClassifiedError.
func AsClassified(err error) (*ClassifiedError, bool) {
	if classified, ok := err.(*ClassifiedError); ok {
		return classified, true
	}
	return nil, false
}

// HasCategory checks if any error in the chain belongs to a category.
func HasCategory(err error, category ErrorCategory) bool {
	if classified, ok := AsClassified(err); ok {
		return classified.IsCategory(category)
	}
	return false
}

// HasSeverity checks if any error in the chain has a specific severity.
func HasSeverity(err error, severity ErrorSeverity) bool {
	if classified, ok := AsClassified(err); ok {
		return classified.IsSeverity(severity)
	}
	return false
}

// GetCategory extracts the category from an error, or returns CategoryInternal.
func GetCategory(err error) ErrorCategory {
	if classified, ok := AsClassified(err); ok {
		return classified.Category()
	}
	return CategoryInternal
}

// GetSeverity extracts the severity from an error, or returns SeverityError.
func GetSeverity(err error) ErrorSeverity {
	if classified, ok := AsClassified(err); ok {
		return classified.Severity()
	}
	return SeverityError
}

// GetRetryStrategy extracts the retry strategy from an error, or returns RetryNever.
func GetRetryStrategy(err error) RetryStrategy {
	if classified, ok := AsClassified(err); ok {
		return classified.RetryStrategy()
	}
	return RetryNever
}
