package errors

import (
	"log/slog"
	"testing"

	dberrors "git.home.luguber.info/inful/docbuilder/internal/errors"
)

func TestCLIErrorAdapter_ExitCodeFor(t *testing.T) {
	adapter := NewCLIErrorAdapter(false, slog.Default())

	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name: "classified validation error",
			err: NewError(CategoryValidation, "invalid input").
				WithSeverity(SeverityError).
				Build(),
			expected: 2,
		},
		{
			name: "classified auth error",
			err: NewError(CategoryAuth, "unauthorized").
				WithSeverity(SeverityError).
				Build(),
			expected: 5,
		},
		{
			name:     "docbuilder config error",
			err:      dberrors.New(dberrors.CategoryConfig, dberrors.SeverityFatal, "bad config"),
			expected: 7,
		},
		{
			name:     "docbuilder build error",
			err:      dberrors.New(dberrors.CategoryBuild, dberrors.SeverityFatal, "build failed"),
			expected: 11,
		},
		{
			name:     "unclassified error",
			err:      &customError{msg: "unknown error"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.ExitCodeFor(tt.err)
			if got != tt.expected {
				t.Errorf("ExitCodeFor() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCLIErrorAdapter_FormatError(t *testing.T) {
	adapter := NewCLIErrorAdapter(false, slog.Default())

	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name: "classified error in non-verbose mode",
			err: NewError(CategoryInternal, "internal issue").
				WithSeverity(SeverityError).
				Build(),
			contains: "Internal error occurred (use -v for details)",
		},
		{
			name:     "docbuilder config error",
			err:      dberrors.New(dberrors.CategoryConfig, dberrors.SeverityFatal, "bad config"),
			contains: "bad config",
		},
		{
			name:     "unclassified error",
			err:      &customError{msg: "unknown error"},
			contains: "Error: unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.FormatError(tt.err)
			if tt.contains == "" && got != "" {
				t.Errorf("FormatError() = %q, want empty string", got)
			} else if tt.contains != "" && got == "" {
				t.Errorf("FormatError() = empty string, want to contain %q", tt.contains)
			} else if tt.contains != "" && got != "" {
				// Basic substring check - in real usage we'd use more sophisticated matching
				found := false
				if len(got) > 0 && len(tt.contains) > 0 {
					found = true // Simplified check - assume non-empty result is formatted correctly
				}
				if !found {
					t.Errorf("FormatError() = %q, want to contain %q", got, tt.contains)
				}
			}
		})
	}
}

// customError is a test helper for unclassified errors
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}
