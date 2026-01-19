package errors

import "maps"

// ErrorCategory represents the broad category of an error for classification and routing.
type ErrorCategory string

const (
	// CategoryConfig represents user-facing configuration and input errors.
	CategoryConfig        ErrorCategory = "config"
	CategoryValidation    ErrorCategory = "validation"
	CategoryAuth          ErrorCategory = "auth"
	CategoryNotFound      ErrorCategory = "not_found"
	CategoryAlreadyExists ErrorCategory = "already_exists"

	// CategoryNetwork represents external system integration errors.
	CategoryNetwork ErrorCategory = "network"
	CategoryGit     ErrorCategory = "git"
	CategoryForge   ErrorCategory = "forge"

	// CategoryBuild represents build and processing errors.
	CategoryBuild      ErrorCategory = "build"
	CategoryHugo       ErrorCategory = "hugo"
	CategoryFileSystem ErrorCategory = "filesystem"
	CategoryDocs       ErrorCategory = "docs"
	CategoryEventStore ErrorCategory = "eventstore"

	// CategoryRuntime represents runtime and infrastructure errors.
	CategoryRuntime  ErrorCategory = "runtime"
	CategoryDaemon   ErrorCategory = "daemon"
	CategoryInternal ErrorCategory = "internal"
)

// ErrorSeverity indicates the impact level of an error.
type ErrorSeverity string

const (
	SeverityFatal   ErrorSeverity = "fatal"   // Stops execution completely
	SeverityError   ErrorSeverity = "error"   // Fails the current operation
	SeverityWarning ErrorSeverity = "warning" // Continues with degraded functionality
	SeverityInfo    ErrorSeverity = "info"    // Informational, no impact
)

// RetryStrategy indicates how an error should be handled in retry scenarios.
type RetryStrategy string

const (
	RetryNever      RetryStrategy = "never"      // Permanent failure, don't retry
	RetryImmediate  RetryStrategy = "immediate"  // Retry immediately
	RetryBackoff    RetryStrategy = "backoff"    // Retry with exponential backoff
	RetryRateLimit  RetryStrategy = "rate_limit" // Retry after rate limit window
	RetryUserAction RetryStrategy = "user"       // Requires user intervention
)

// ErrorContext provides structured context for errors.
type ErrorContext map[string]any

// Set adds or updates a context value.
func (c ErrorContext) Set(key string, value any) ErrorContext {
	if c == nil {
		c = make(ErrorContext)
	}
	c[key] = value
	return c
}

// Get retrieves a context value.
func (c ErrorContext) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	value, exists := c[key]
	return value, exists
}

// GetString retrieves a string context value.
func (c ErrorContext) GetString(key string) (string, bool) {
	if value, exists := c.Get(key); exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// Merge combines two contexts, with other taking precedence.
func (c ErrorContext) Merge(other ErrorContext) ErrorContext {
	if c == nil {
		return other
	}
	if other == nil {
		return c
	}
	result := make(ErrorContext)
	maps.Copy(result, c)
	maps.Copy(result, other)
	return result
}
