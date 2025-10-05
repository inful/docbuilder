package errors
import (
	"errors"
	"testing"
)

func TestClassifiedError(t *testing.T) {
	t.Run("Basic error creation", func(t *testing.T) {
		err := NewError(CategoryConfig, "invalid configuration").
			WithSeverity(SeverityFatal).
			WithContext("file", "config.yaml").
			Build()

		if err.Category() != CategoryConfig {
			t.Errorf("expected category %s, got %s", CategoryConfig, err.Category())
		}
		if err.Severity() != SeverityFatal {
			t.Errorf("expected severity %s, got %s", SeverityFatal, err.Severity())
		}
		if err.Message() != "invalid configuration" {
			t.Errorf("expected message 'invalid configuration', got %s", err.Message())
		}

		file, exists := err.Context().GetString("file")
		if !exists || file != "config.yaml" {
			t.Errorf("expected context file=config.yaml, got %v", file)
		}
	})

	t.Run("Error detection", func(t *testing.T) {
		err := ConfigError("test error").Build()

		if !IsClassified(err) {
			t.Error("expected error to be classified")
		}

		if !HasCategory(err, CategoryConfig) {
			t.Error("expected error to have config category")
		}

		if !HasSeverity(err, SeverityFatal) {
			t.Error("expected error to have fatal severity")
		}

		if err.CanRetry() {
			t.Error("expected config error to not be retryable")
		}

		if !err.IsFatal() {
			t.Error("expected config error to be fatal")
		}
	})
}

func TestErrorBuilder(t *testing.T) {
	t.Run("Fluent API", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := WrapError(originalErr, CategoryNetwork, "network failure").
			Warning().
			Retryable().
			WithContext("host", "example.com").
			WithContext("port", 443).
			Build()

		if err.Category() != CategoryNetwork {
			t.Errorf("expected category %s, got %s", CategoryNetwork, err.Category())
		}
		if err.Severity() != SeverityWarning {
			t.Errorf("expected severity %s, got %s", SeverityWarning, err.Severity())
		}
		if err.RetryStrategy() != RetryBackoff {
			t.Errorf("expected retry strategy %s, got %s", RetryBackoff, err.RetryStrategy())
		}
		if !errors.Is(err, originalErr) {
			t.Error("expected error to wrap original error")
		}

		host, _ := err.Context().GetString("host")
		if host != "example.com" {
			t.Errorf("expected host context 'example.com', got %s", host)
		}
	})

	t.Run("Convenience constructors", func(t *testing.T) {
		tests := []struct {
			name     string
			builder  *ErrorBuilder
			category ErrorCategory
			severity ErrorSeverity
			retry    RetryStrategy
		}{
			{"ConfigError", ConfigError("test"), CategoryConfig, SeverityFatal, RetryNever},
			{"ValidationError", ValidationError("test"), CategoryValidation, SeverityFatal, RetryNever},
			{"AuthError", AuthError("test"), CategoryAuth, SeverityError, RetryUserAction},
			{"NetworkError", NetworkError("test"), CategoryNetwork, SeverityError, RetryBackoff},
			{"GitError", GitError("test"), CategoryGit, SeverityError, RetryBackoff},
			{"ForgeError", ForgeError("test"), CategoryForge, SeverityError, RetryBackoff},
			{"BuildError", BuildError("test"), CategoryBuild, SeverityFatal, RetryNever},
			{"HugoError", HugoError("test"), CategoryHugo, SeverityFatal, RetryNever},
			{"FileSystemError", FileSystemError("test"), CategoryFileSystem, SeverityError, RetryBackoff},
			{"RuntimeError", RuntimeError("test"), CategoryRuntime, SeverityFatal, RetryNever},
			{"DaemonError", DaemonError("test"), CategoryDaemon, SeverityFatal, RetryNever},
			{"InternalError", InternalError("test"), CategoryInternal, SeverityFatal, RetryNever},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.builder.Build()
				if err.Category() != tt.category {
					t.Errorf("expected category %s, got %s", tt.category, err.Category())
				}
				if err.Severity() != tt.severity {
					t.Errorf("expected severity %s, got %s", tt.severity, err.Severity())
				}
				if err.RetryStrategy() != tt.retry {
					t.Errorf("expected retry strategy %s, got %s", tt.retry, err.RetryStrategy())
				}
			})
		}
	})
}

func TestErrorContext(t *testing.T) {
	t.Run("Context operations", func(t *testing.T) {
		ctx := make(ErrorContext)
		ctx = ctx.Set("key1", "value1")
		ctx = ctx.Set("key2", 42)

		value1, exists1 := ctx.GetString("key1")
		if !exists1 || value1 != "value1" {
			t.Errorf("expected key1=value1, got %v", value1)
		}

		value2, exists2 := ctx.Get("key2")
		if !exists2 || value2 != 42 {
			t.Errorf("expected key2=42, got %v", value2)
		}

		_, exists3 := ctx.Get("nonexistent")
		if exists3 {
			t.Error("expected nonexistent key to not exist")
		}
	})

	t.Run("Context merge", func(t *testing.T) {
		ctx1 := make(ErrorContext)
		ctx1 = ctx1.Set("key1", "value1")
		ctx1 = ctx1.Set("shared", "original")

		ctx2 := make(ErrorContext)
		ctx2 = ctx2.Set("key2", "value2")
		ctx2 = ctx2.Set("shared", "overridden")

		merged := ctx1.Merge(ctx2)

		value1, _ := merged.GetString("key1")
		value2, _ := merged.GetString("key2")
		shared, _ := merged.GetString("shared")

		if value1 != "value1" {
			t.Errorf("expected key1=value1, got %s", value1)
		}
		if value2 != "value2" {
			t.Errorf("expected key2=value2, got %s", value2)
		}
		if shared != "overridden" {
			t.Errorf("expected shared=overridden, got %s", shared)
		}
	})
}
