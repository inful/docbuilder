package errors
// ErrorBuilder provides a fluent API for creating ClassifiedError instances.
// This makes error creation consistent and discoverable throughout the codebase.
type ErrorBuilder struct {
	category ErrorCategory
	severity ErrorSeverity
	retry    RetryStrategy
	message  string
	cause    error
	context  ErrorContext
}

// NewError creates a new ErrorBuilder with the specified category and message.
func NewError(category ErrorCategory, message string) *ErrorBuilder {
	return &ErrorBuilder{
		category: category,
		severity: SeverityError, // Default severity
		retry:    RetryNever,    // Default to no retry
		message:  message,
		context:  make(ErrorContext),
	}
}

// WrapError creates a new ErrorBuilder that wraps an existing error.
func WrapError(err error, category ErrorCategory, message string) *ErrorBuilder {
	return &ErrorBuilder{
		category: category,
		severity: SeverityError,
		retry:    RetryNever,
		message:  message,
		cause:    err,
		context:  make(ErrorContext),
	}
}

// WithSeverity sets the error severity.
func (b *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder {
	b.severity = severity
	return b
}

// WithRetry sets the retry strategy.
func (b *ErrorBuilder) WithRetry(strategy RetryStrategy) *ErrorBuilder {
	b.retry = strategy
	return b
}

// WithContext adds a context key-value pair.
func (b *ErrorBuilder) WithContext(key string, value any) *ErrorBuilder {
	b.context = b.context.Set(key, value)
	return b
}

// WithContextMap adds multiple context values.
func (b *ErrorBuilder) WithContextMap(ctx ErrorContext) *ErrorBuilder {
	b.context = b.context.Merge(ctx)
	return b
}

// Fatal sets the severity to fatal.
func (b *ErrorBuilder) Fatal() *ErrorBuilder {
	return b.WithSeverity(SeverityFatal)
}

// Warning sets the severity to warning.
func (b *ErrorBuilder) Warning() *ErrorBuilder {
	return b.WithSeverity(SeverityWarning)
}

// Info sets the severity to info.
func (b *ErrorBuilder) Info() *ErrorBuilder {
	return b.WithSeverity(SeverityInfo)
}

// Retryable sets the retry strategy to backoff.
func (b *ErrorBuilder) Retryable() *ErrorBuilder {
	return b.WithRetry(RetryBackoff)
}

// Immediate sets the retry strategy to immediate.
func (b *ErrorBuilder) Immediate() *ErrorBuilder {
	return b.WithRetry(RetryImmediate)
}

// RateLimit sets the retry strategy to rate limit.
func (b *ErrorBuilder) RateLimit() *ErrorBuilder {
	return b.WithRetry(RetryRateLimit)
}

// UserAction sets the retry strategy to require user action.
func (b *ErrorBuilder) UserAction() *ErrorBuilder {
	return b.WithRetry(RetryUserAction)
}

// Build creates the final ClassifiedError.
func (b *ErrorBuilder) Build() *ClassifiedError {
	return &ClassifiedError{
		category: b.category,
		severity: b.severity,
		retry:    b.retry,
		message:  b.message,
		cause:    b.cause,
		context:  b.context,
	}
}

// Convenience constructors for common error patterns

// ConfigError creates a configuration error.
func ConfigError(message string) *ErrorBuilder {
	return NewError(CategoryConfig, message).Fatal()
}

// ValidationError creates a validation error.
func ValidationError(message string) *ErrorBuilder {
	return NewError(CategoryValidation, message).Fatal()
}

// AuthError creates an authentication error.
func AuthError(message string) *ErrorBuilder {
	return NewError(CategoryAuth, message).UserAction()
}

// NetworkError creates a network error (typically retryable).
func NetworkError(message string) *ErrorBuilder {
	return NewError(CategoryNetwork, message).Retryable()
}

// GitError creates a git operation error.
func GitError(message string) *ErrorBuilder {
	return NewError(CategoryGit, message).Retryable()
}

// ForgeError creates a forge integration error.
func ForgeError(message string) *ErrorBuilder {
	return NewError(CategoryForge, message).Retryable()
}

// BuildError creates a build processing error.
func BuildError(message string) *ErrorBuilder {
	return NewError(CategoryBuild, message).Fatal()
}

// HugoError creates a Hugo processing error.
func HugoError(message string) *ErrorBuilder {
	return NewError(CategoryHugo, message).Fatal()
}

// FileSystemError creates a filesystem error.
func FileSystemError(message string) *ErrorBuilder {
	return NewError(CategoryFileSystem, message).Retryable()
}

// RuntimeError creates a runtime error.
func RuntimeError(message string) *ErrorBuilder {
	return NewError(CategoryRuntime, message).Fatal()
}

// DaemonError creates a daemon error.
func DaemonError(message string) *ErrorBuilder {
	return NewError(CategoryDaemon, message).Fatal()
}

// InternalError creates an internal error.
func InternalError(message string) *ErrorBuilder {
	return NewError(CategoryInternal, message).Fatal()
}
