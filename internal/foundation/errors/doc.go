// Package errors provides foundational, type-safe error primitives used across DocBuilder.
//
// This package contains classified error types and helpers for robust error handling,
// including a fluent builder API for constructing ClassifiedError values with context.
//
// Key features:
//   - ErrorCategory: Broad error classification (config, network, git, build, etc.)
//   - ErrorSeverity: Impact level (error, warning, info)
//   - RetryStrategy: Retry behavior (should-retry, no-retry, backoff)
//   - ClassifiedError: Structured error with category, severity, and context
//   - ErrorBuilder: Fluent API for creating classified errors
//   - HTTP and CLI adapters for error presentation
//
// Example usage:
//
//	err := errors.NewError(errors.CategoryGit, "clone failed").
//		WithSeverity(errors.SeverityError).
//		WithRetry(errors.RetryWithBackoff).
//		WithContext("url", repoURL).
//		WithCause(originalErr).
//		Build()
package errors
