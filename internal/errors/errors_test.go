package errors

import (
	stdErrors "errors"
	"fmt"
	"testing"
)

func TestDocBuilderError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DocBuilderError
		expected string
	}{
		{
			name:     "error without cause",
			err:      New(CategoryConfig, SeverityFatal, "configuration invalid"),
			expected: "config (fatal): configuration invalid",
		},
		{
			name:     "error with cause",
			err:      Wrap(fmt.Errorf("file not found"), CategoryConfig, SeverityFatal, "failed to load config"),
			expected: "config (fatal): failed to load config: file not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.err.Error()
			if result != test.expected {
				t.Errorf("Error() = %q, want %q", result, test.expected)
			}
		})
	}
}

func TestDocBuilderError_WithContext(t *testing.T) {
	err := New(CategoryGit, SeverityWarning, "clone failed").
		WithContext("repository", "test-repo").
		WithContext("branch", "main")

	if err.Context == nil {
		t.Fatal("Context should not be nil")
	}

	if err.Context["repository"] != "test-repo" {
		t.Errorf("Context[repository] = %v, want test-repo", err.Context["repository"])
	}

	if err.Context["branch"] != "main" {
		t.Errorf("Context[branch] = %v, want main", err.Context["branch"])
	}
}

func TestIsCategory(t *testing.T) {
	configErr := New(CategoryConfig, SeverityFatal, "config error")
	gitErr := New(CategoryGit, SeverityWarning, "git error")
	standardErr := fmt.Errorf("standard error")

	tests := []struct {
		name     string
		err      error
		category ErrorCategory
		expected bool
	}{
		{"config error matches config category", configErr, CategoryConfig, true},
		{"config error doesn't match git category", configErr, CategoryGit, false},
		{"git error matches git category", gitErr, CategoryGit, true},
		{"standard error doesn't match any category", standardErr, CategoryConfig, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsCategory(test.err, test.category)
			if result != test.expected {
				t.Errorf("IsCategory() = %v, want %v", result, test.expected)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	retryableErr := Retryable(CategoryNetwork, SeverityWarning, "timeout")
	nonRetryableErr := New(CategoryConfig, SeverityFatal, "invalid")
	standardErr := fmt.Errorf("standard error")

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"retryable error", retryableErr, true},
		{"non-retryable error", nonRetryableErr, false},
		{"standard error", standardErr, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsRetryable(test.err)
			if result != test.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, test.expected)
			}
		})
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Test a few convenience functions
	t.Run("ConfigNotFound", func(t *testing.T) {
		err := ConfigNotFound("/path/to/config.yaml")
		if err.Category != CategoryConfig {
			t.Errorf("Category = %v, want %v", err.Category, CategoryConfig)
		}
		if err.Severity != SeverityFatal {
			t.Errorf("Severity = %v, want %v", err.Severity, SeverityFatal)
		}
		if err.Context["path"] != "/path/to/config.yaml" {
			t.Errorf("Context[path] = %v, want /path/to/config.yaml", err.Context["path"])
		}
	})

	t.Run("NetworkTimeout", func(t *testing.T) {
		cause := fmt.Errorf("timeout")
		err := NetworkTimeout("https://example.com", cause)
		if err.Category != CategoryNetwork {
			t.Errorf("Category = %v, want %v", err.Category, CategoryNetwork)
		}
		if !err.Retryable {
			t.Error("NetworkTimeout should be retryable")
		}
		if !stdErrors.Is(err, cause) {
			t.Errorf("Cause should match wrapped cause: %v", cause)
		}
	})

	t.Run("ValidationFailed", func(t *testing.T) {
		err := ValidationFailed("forge.type", "unsupported value")
		if err.Category != CategoryValidation {
			t.Errorf("Category = %v, want %v", err.Category, CategoryValidation)
		}
		if err.Context["field"] != "forge.type" {
			t.Errorf("Context[field] = %v, want forge.type", err.Context["field"])
		}
		if err.Context["reason"] != "unsupported value" {
			t.Errorf("Context[reason] = %v, want unsupported value", err.Context["reason"])
		}
	})
}
