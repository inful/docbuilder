package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// deriveModuleName derives a Go module name from the base URL.
// Returns "docbuilder-site" if no base URL is configured.
func deriveModuleName(baseURL string) string {
	if baseURL == "" {
		return "docbuilder-site"
	}

	// Remove protocol prefix
	s := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")

	// Extract host (before first slash)
	host := s
	if before, _, ok := strings.Cut(s, "/"); ok {
		host = before
	}

	// Remove port if present
	if p := strings.IndexByte(host, ':'); p >= 0 {
		host = host[:p]
	}

	// Replace dots with hyphens or return default
	if host != "" {
		return strings.ReplaceAll(host, ".", "-")
	}
	return "docbuilder-site"
}

// ensureGoModForModules creates a minimal go.mod to allow Hugo Modules to work.
func (g *Generator) ensureGoModForModules() error {
	goModPath := filepath.Join(g.buildRoot(), "go.mod")

	if _, err := os.Stat(goModPath); err == nil { // exists
		return g.handleExistingGoMod(goModPath)
	}

	return g.createNewGoMod(goModPath)
}

// handleExistingGoMod processes an existing go.mod file.
func (g *Generator) handleExistingGoMod(goModPath string) error {
	// #nosec G304 - goModPath is internal, controlled by application
	b, readErr := os.ReadFile(goModPath)
	if readErr != nil {
		return g.ensureThemeVersionRequires(goModPath)
	}

	lines := strings.SplitN(string(b), "\n", 2)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "module ") {
		return g.ensureThemeVersionRequires(goModPath)
	}

	existing := strings.TrimSpace(strings.TrimPrefix(lines[0], "module "))

	// Check if module name contains colon - needs sanitization
	if strings.Contains(existing, ":") {
		return g.sanitizeGoMod(goModPath, lines)
	}

	// Check if go version directive is missing
	content := string(b)
	if !strings.Contains(content, "go ") {
		return g.addGoVersionToExisting(goModPath, content)
	}

	return g.ensureThemeVersionRequires(goModPath)
}

// addGoVersionToExisting adds a go version directive to an existing go.mod without one.
func (g *Generator) addGoVersionToExisting(goModPath, existingContent string) error {
	lines := strings.Split(existingContent, "\n")
	newLines := make([]string, 0, len(lines)+2)

	// Add module line and go version after it
	for i, line := range lines {
		newLines = append(newLines, line)
		if i == 0 && strings.HasPrefix(line, "module ") {
			newLines = append(newLines, "", "go 1.21")
		}
	}

	newContent := strings.Join(newLines, "\n")
	// #nosec G306 -- go.mod is a module configuration file
	if err := os.WriteFile(goModPath, []byte(newContent), 0o644); err != nil {
		slog.Warn("Failed to add go version to go.mod", "error", err)
		return g.ensureThemeVersionRequires(goModPath)
	}

	slog.Debug("Added go version directive to existing go.mod", logfields.Path(goModPath))
	return g.ensureThemeVersionRequires(goModPath)
}

// sanitizeGoMod rewrites go.mod with a sanitized module name.
func (g *Generator) sanitizeGoMod(goModPath string, lines []string) error {
	sanitized := deriveModuleName(g.config.Hugo.BaseURL)
	rest := ""
	if len(lines) > 1 {
		rest = lines[1]
	}

	newContent := fmt.Sprintf("module %s\n", sanitized)
	if !strings.Contains(rest, "go ") {
		newContent += "\ngo 1.21\n"
	} else {
		newContent += rest
	}

	// #nosec G306 -- go.mod is a module configuration file
	if writeErr := os.WriteFile(goModPath, []byte(newContent), 0o644); writeErr != nil {
		slog.Warn("Failed to rewrite invalid go.mod module line", "error", writeErr)
		return g.ensureThemeVersionRequires(goModPath)
	}

	slog.Debug("Rewrote go.mod with sanitized module name", logfields.Path(goModPath), slog.String("module", sanitized))
	return g.ensureThemeVersionRequires(goModPath)
}

// createNewGoMod creates a new go.mod file.
func (g *Generator) createNewGoMod(goModPath string) error {
	moduleName := deriveModuleName(g.config.Hugo.BaseURL)
	content := fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)

	// #nosec G306 -- go.mod is a module configuration file
	if err := os.WriteFile(goModPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	slog.Debug("Created go.mod for Hugo Modules", logfields.Path(goModPath))
	return g.ensureThemeVersionRequires(goModPath)
}

// ensureThemeVersionRequires is no longer needed since we hardcode Relearn module
// Hugo will fetch the latest compatible version automatically.
func (g *Generator) ensureThemeVersionRequires(goModPath string) error {
	// No-op: Hugo Modules will automatically resolve and download Relearn
	// The module path in hugo.yaml is sufficient
	return nil
}
