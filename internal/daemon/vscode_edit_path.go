package daemon

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// validateAndResolveEditPath extracts the file path from the URL and validates it.
func (s *HTTPServer) validateAndResolveEditPath(urlPath string) (string, error) {
	// Extract file path from URL
	const editPrefix = "/_edit/"
	if !strings.HasPrefix(urlPath, editPrefix) {
		return "", &editError{
			message:    "Invalid edit URL",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
		}
	}

	relPath := strings.TrimPrefix(urlPath, editPrefix)
	if relPath == "" {
		return "", &editError{
			message:    "No file path specified",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
		}
	}

	// Get docs directory
	docsDir := s.getDocsDirectory()
	if docsDir == "" {
		return "", &editError{
			message:    "Server configuration error",
			statusCode: http.StatusInternalServerError,
			logLevel:   "error",
			logFields:  []any{slog.String("reason", "unable to determine docs directory")},
		}
	}

	// Resolve to absolute path
	absPath := filepath.Join(docsDir, relPath)

	// Security: ensure the resolved path is within the docs directory
	cleanDocs := filepath.Clean(docsDir)
	cleanPath := filepath.Clean(absPath)

	// Ensure proper directory boundary check by adding separator
	if !strings.HasSuffix(cleanDocs, string(filepath.Separator)) {
		cleanDocs += string(filepath.Separator)
	}

	if !strings.HasPrefix(cleanPath, cleanDocs) {
		return "", &editError{
			message:    "Invalid file path",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields: []any{
				slog.String("requested", relPath),
				slog.String("resolved", cleanPath),
				slog.String("docs_dir", cleanDocs),
			},
		}
	}

	// Validate file exists and is a markdown file
	if err := s.validateMarkdownFile(cleanPath); err != nil {
		return "", err
	}

	return cleanPath, nil
}

// validateMarkdownFile checks that the file exists, is regular, and is markdown.
func (s *HTTPServer) validateMarkdownFile(path string) error {
	// Use Lstat to detect symlinks (Stat would follow them)
	fileInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &editError{
				message:    "File not found",
				statusCode: http.StatusNotFound,
				logLevel:   "warn",
				logFields:  []any{slog.String("path", path)},
			}
		}
		return &editError{
			message:    "Failed to access file",
			statusCode: http.StatusInternalServerError,
			logLevel:   "error",
			logFields: []any{
				slog.String("path", path),
				slog.String("error", err.Error()),
			},
		}
	}

	// Security: Reject symlinks to prevent path traversal via symlink attacks
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return &editError{
			message:    "Symlinks are not allowed",
			statusCode: http.StatusForbidden,
			logLevel:   "warn",
			logFields: []any{
				slog.String("path", path),
				slog.String("reason", "symlink detected"),
			},
		}
	}

	if !fileInfo.Mode().IsRegular() {
		return &editError{
			message:    "Not a regular file",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields:  []any{slog.String("path", path)},
		}
	}

	// Verify it's a markdown file
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return &editError{
			message:    "Only markdown files can be edited",
			statusCode: http.StatusBadRequest,
			logLevel:   "warn",
			logFields: []any{
				slog.String("path", path),
				slog.String("extension", ext),
			},
		}
	}

	return nil
}

// getDocsDirectory returns the docs directory for preview mode edit operations.
// VS Code edit links are only supported in preview mode, not daemon mode.
func (s *HTTPServer) getDocsDirectory() string {
	if s.config == nil || len(s.config.Repositories) == 0 {
		return ""
	}

	// In preview mode (single repository), the repository URL is the local docs directory
	docsDir := s.config.Repositories[0].URL

	// Ensure absolute path
	if !filepath.IsAbs(docsDir) {
		if abs, err := filepath.Abs(docsDir); err == nil {
			return abs
		}
	}

	slog.Debug("VS Code edit handler: using repository URL as docs dir",
		slog.String("docs_dir", docsDir))
	return docsDir
}
