package hugo

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "strings"

    "git.home.luguber.info/inful/docbuilder/internal/config"
    "git.home.luguber.info/inful/docbuilder/internal/logfields"
)

// ensureGoModForModules creates a minimal go.mod to allow Hugo Modules to work
func (g *Generator) ensureGoModForModules() error {
	goModPath := filepath.Join(g.buildRoot(), "go.mod")
	deriveModuleName := func() string {
		moduleName := "docbuilder-site"
		if g.config.Hugo.BaseURL != "" {
			s := strings.TrimPrefix(strings.TrimPrefix(g.config.Hugo.BaseURL, "https://"), "http://")
			host := s
			if idx := strings.IndexByte(s, '/'); idx >= 0 {
				host = s[:idx]
			}
			if p := strings.IndexByte(host, ':'); p >= 0 {
				host = host[:p]
			}
			if host != "" {
				moduleName = strings.ReplaceAll(host, ".", "-")
			}
		}
		return moduleName
	}
	if _, err := os.Stat(goModPath); err == nil { // exists
		b, readErr := os.ReadFile(goModPath)
		if readErr == nil {
			lines := strings.SplitN(string(b), "\n", 2)
			if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
				existing := strings.TrimSpace(strings.TrimPrefix(lines[0], "module "))
				if strings.Contains(existing, ":") {
					sanitized := deriveModuleName()
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
					if writeErr := os.WriteFile(goModPath, []byte(newContent), 0644); writeErr != nil {
						slog.Warn("Failed to rewrite invalid go.mod module line", "error", writeErr)
					} else {
						slog.Debug("Rewrote go.mod with sanitized module name", logfields.Path(goModPath), "module", sanitized)
					}
				}
			}
		}
		return g.ensureThemeVersionRequires(goModPath)
	}
	moduleName := deriveModuleName()
	content := fmt.Sprintf("module %s\n\ngo 1.21\n", moduleName)
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}
	slog.Debug("Created go.mod for Hugo Modules", logfields.Path(goModPath))
	return g.ensureThemeVersionRequires(goModPath)
}

// ensureThemeVersionRequires appends require directives for known themes to pin versions
func (g *Generator) ensureThemeVersionRequires(goModPath string) error {
	b, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}
	s := string(b)
	if g.config.Hugo.Theme == config.ThemeHextra { // pin version
		const hextraModule = "github.com/imfing/hextra"
		const hextraVersion = "v0.11.0"
		if !strings.Contains(s, hextraModule) {
			s += fmt.Sprintf("\nrequire %s %s\n", hextraModule, hextraVersion)
		}
	}
	return os.WriteFile(goModPath, []byte(s), 0644)
}
