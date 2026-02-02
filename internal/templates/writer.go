package templates

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteGeneratedFile writes content to a path under docsDir and returns full path.
func WriteGeneratedFile(docsDir, relativePath, content string) (string, error) {
	if docsDir == "" {
		return "", errors.New("docs directory is required")
	}
	if relativePath == "" {
		return "", errors.New("output path is required")
	}

	cleanRel := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "..") {
		return "", errors.New("output path must be relative to docs")
	}

	fullPath := filepath.Join(docsDir, cleanRel)
	rel, err := filepath.Rel(docsDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("output path escapes docs directory")
	}

	if err = os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// #nosec G304 -- fullPath is validated to stay under docsDir.
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("write output file: %w", err)
	}

	return fullPath, nil
}
