package lint

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Linter performs linting operations on documentation files.
type Linter struct {
	cfg   *Config
	rules []Rule
}

// NewLinter creates a new linter with the given configuration.
func NewLinter(cfg *Config) *Linter {
	if cfg == nil {
		cfg = &Config{Format: "text"}
	}

	return &Linter{
		cfg: cfg,
		rules: []Rule{
			&FilenameRule{},
			// Additional rules will be added here in future phases
		},
	}
}

// LintPath lints all documentation files in the given path (file or directory).
func (l *Linter) LintPath(path string) (*Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Issues: []Issue{},
	}

	if info.IsDir() {
		err = l.lintDirectory(path, result)
	} else {
		err = l.lintFile(path, result)
		result.FilesTotal = 1
	}

	return result, err
}

// lintDirectory recursively lints all documentation files in a directory.
func (l *Linter) lintDirectory(dirPath string, result *Result) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if d.Name()[0] == '.' && d.Name() != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip standard ignored files (case-insensitive)
		if isIgnoredFile(d.Name()) {
			return nil
		}

		// Only process documentation and asset files
		if !IsDocFile(path) && !IsAssetFile(path) {
			return nil
		}

		result.FilesTotal++
		return l.lintFile(path, result)
	})
}

// lintFile applies all applicable rules to a single file.
func (l *Linter) lintFile(filePath string, result *Result) error {
	for _, rule := range l.rules {
		if !rule.AppliesTo(filePath) {
			continue
		}

		issues, err := rule.Check(filePath)
		if err != nil {
			return err
		}

		// Filter issues based on configuration
		for _, issue := range issues {
			// Skip info and warnings in quiet mode
			if l.cfg.Quiet && issue.Severity != SeverityError {
				continue
			}
			result.Issues = append(result.Issues, issue)
		}
	}

	return nil
}

// LintFiles lints a specific list of files (useful for Git hooks).
func (l *Linter) LintFiles(files []string) (*Result, error) {
	result := &Result{
		Issues:     []Issue{},
		FilesTotal: 0,
	}

	for _, file := range files {
		// Skip standard ignored files
		if isIgnoredFile(filepath.Base(file)) {
			continue
		}

		// Only process documentation and asset files
		if !IsDocFile(file) && !IsAssetFile(file) {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}

		result.FilesTotal++
		if err := l.lintFile(file, result); err != nil {
			return result, err
		}
	}

	return result, nil
}

// isIgnoredFile returns true if the file should be ignored during linting.
// These are standard repository files that don't follow documentation naming conventions.
func isIgnoredFile(filename string) bool {
	// Convert to uppercase for case-insensitive comparison
	upper := strings.ToUpper(filename)
	ignoredFiles := []string{
		"README.MD",
		"CONTRIBUTING.MD",
		"CHANGELOG.MD",
		"LICENSE.MD",
		"CODE_OF_CONDUCT.MD",
		"SECURITY.MD",
	}

	for _, ignored := range ignoredFiles {
		if upper == ignored {
			return true
		}
	}
	return false
}
