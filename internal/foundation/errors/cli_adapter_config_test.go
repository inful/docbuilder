package errors

import (
	"log/slog"
	"strings"
	"testing"
)

// A mistake in the configuration is the user's to fix, so the message
// says what to fix. "Internal error occurred (use -v for details)"
// leaves the user guessing.
func TestCLIErrorAdapter_ConfigErrorSaysWhatToFix(t *testing.T) {
	adapter := NewCLIErrorAdapter(false, slog.Default())

	// The shape the config loader produces: a config error with a
	// validation error inside it.
	err := NewError(CategoryConfig, "configuration validation failed").
		WithCause(NewError(CategoryValidation, "duplicate repository name").Build()).
		Build()

	got := adapter.FormatError(err)
	if !strings.Contains(got, "duplicate repository name") {
		t.Errorf("FormatError() = %q, want it to name the problem", got)
	}
	if strings.Contains(got, "Internal error occurred") {
		t.Errorf("FormatError() = %q, want the message of the problem", got)
	}
}

// Everything else keeps the internal wording.
func TestCLIErrorAdapter_InternalErrorStaysInternal(t *testing.T) {
	adapter := NewCLIErrorAdapter(false, slog.Default())

	err := NewError(CategoryInternal, "internal issue").
		WithSeverity(SeverityError).
		Build()

	got := adapter.FormatError(err)
	if !strings.Contains(got, "Internal error occurred") {
		t.Errorf("FormatError() = %q, want the internal wording", got)
	}
}
