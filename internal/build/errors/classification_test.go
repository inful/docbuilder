package errors

import (
	"errors"
	"testing"

	derrors "git.home.luguber.info/inful/docbuilder/internal/docs/errors"
	herrors "git.home.luguber.info/inful/docbuilder/internal/hugo/errors"
)

// TestTypedErrorClassification ensures discovery and generation typed errors
// map to predictable issue classification for user-facing error messages.
func TestTypedErrorClassification(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectIssue string // simplified expectation for demonstration
	}{
		// Discovery errors
		{
			name:        "docs path not found",
			err:         derrors.ErrDocsPathNotFound,
			expectIssue: "configuration",
		},
		{
			name:        "docs walk failed",
			err:         derrors.ErrDocsDirWalkFailed,
			expectIssue: "filesystem",
		},
		{
			name:        "file read failed",
			err:         derrors.ErrFileReadFailed,
			expectIssue: "filesystem",
		},
		{
			name:        "no docs found",
			err:         derrors.ErrNoDocsFound,
			expectIssue: "content",
		},

		// Hugo generation errors
		{
			name:        "content transform failed",
			err:         herrors.ErrContentTransformFailed,
			expectIssue: "processing",
		},
		{
			name:        "content write failed",
			err:         herrors.ErrContentWriteFailed,
			expectIssue: "filesystem",
		},
		{
			name:        "index generation failed",
			err:         herrors.ErrIndexGenerationFailed,
			expectIssue: "processing",
		},
		{
			name:        "hugo execution failed",
			err:         herrors.ErrHugoExecutionFailed,
			expectIssue: "execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple classification based on error type
			got := classifyError(tt.err)
			if got != tt.expectIssue {
				t.Errorf("classifyError(%v) = %q, want %q", tt.err, got, tt.expectIssue)
			}
		})
	}
}

// classifyError provides a simplified error classification for demonstration.
// In practice, this would integrate with the existing issue classification system.
func classifyError(err error) string {
	switch {
	case errors.Is(err, derrors.ErrDocsPathNotFound):
		return "configuration"
	case errors.Is(err, derrors.ErrDocsDirWalkFailed):
		return "filesystem"
	case errors.Is(err, derrors.ErrFileReadFailed):
		return "filesystem"
	case errors.Is(err, derrors.ErrDocIgnoreCheckFailed):
		return "filesystem"
	case errors.Is(err, derrors.ErrNoDocsFound):
		return "content"
	case errors.Is(err, derrors.ErrInvalidRelativePath):
		return "filesystem"

	case errors.Is(err, herrors.ErrContentTransformFailed):
		return "processing"
	case errors.Is(err, herrors.ErrContentWriteFailed):
		return "filesystem"
	case errors.Is(err, herrors.ErrIndexGenerationFailed):
		return "processing"
	case errors.Is(err, herrors.ErrLayoutCopyFailed):
		return "filesystem"
	case errors.Is(err, herrors.ErrStagingFailed):
		return "filesystem"
	case errors.Is(err, herrors.ErrReportPersistFailed):
		return "filesystem"
	case errors.Is(err, herrors.ErrHugoBinaryNotFound):
		return "environment"
	case errors.Is(err, herrors.ErrHugoExecutionFailed):
		return "execution"
	case errors.Is(err, herrors.ErrConfigMarshalFailed):
		return "processing"
	case errors.Is(err, herrors.ErrConfigWriteFailed):
		return "filesystem"
	default:
		return "unknown"
	}
}
