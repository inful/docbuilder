package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestEnsureGoModForModules(t *testing.T) {
	tests := []struct {
		name           string
		baseURL        string
		existingGoMod  string
		wantModuleName string
		wantErr        bool
	}{
		{
			name:           "creates new go.mod with default module name",
			baseURL:        "",
			existingGoMod:  "",
			wantModuleName: "docbuilder-site",
			wantErr:        false,
		},
		{
			name:           "creates go.mod with module name from https URL",
			baseURL:        "https://example.com/docs",
			existingGoMod:  "",
			wantModuleName: "example-com",
			wantErr:        false,
		},
		{
			name:           "creates go.mod with module name from http URL",
			baseURL:        "http://docs.mysite.io",
			existingGoMod:  "",
			wantModuleName: "docs-mysite-io",
			wantErr:        false,
		},
		{
			name:           "creates go.mod with module name from URL with port",
			baseURL:        "http://localhost:1313",
			existingGoMod:  "",
			wantModuleName: "localhost",
			wantErr:        false,
		},
		{
			name:          "preserves valid existing go.mod",
			baseURL:       "https://example.com",
			existingGoMod: "module my-custom-site\n\ngo 1.21\n",
			// Should keep existing valid module name
			wantModuleName: "my-custom-site",
			wantErr:        false,
		},
		{
			name:           "sanitizes invalid go.mod with colon",
			baseURL:        "https://example.com",
			existingGoMod:  "module invalid:module:name\n\ngo 1.21\n",
			wantModuleName: "example-com",
			wantErr:        false,
		},
		{
			name:           "adds go version if missing",
			baseURL:        "",
			existingGoMod:  "module my-site\n",
			wantModuleName: "my-site",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create generator with test config
			cfg := &config.Config{
				Hugo: config.HugoConfig{
					BaseURL: tt.baseURL,
				},
				Output: config.OutputConfig{
					Directory: tmpDir,
				},
			}

			gen := &Generator{
				config:    cfg,
				outputDir: tmpDir,
			}

			// Create existing go.mod if specified
			goModPath := filepath.Join(tmpDir, "go.mod")
			if tt.existingGoMod != "" {
				err := os.WriteFile(goModPath, []byte(tt.existingGoMod), 0o600)
				if err != nil {
					t.Fatalf("Failed to create test go.mod: %v", err)
				}
			}

			// Run the function
			err := gen.ensureGoModForModules()

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureGoModForModules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify go.mod was created
			// #nosec G304 -- test utility reading from test output directory
			content, err := os.ReadFile(goModPath)
			if err != nil {
				t.Fatalf("Failed to read go.mod: %v", err)
			}

			contentStr := string(content)

			// Verify module name
			if !strings.Contains(contentStr, "module "+tt.wantModuleName) {
				t.Errorf("go.mod module name = %q, want %q\nFull content:\n%s",
					extractModuleName(contentStr), tt.wantModuleName, contentStr)
			}

			// Verify go version is present
			if !strings.Contains(contentStr, "go 1.21") {
				t.Errorf("go.mod missing go version directive\nFull content:\n%s", contentStr)
			}
		})
	}
}

func extractModuleName(goModContent string) string {
	lines := strings.SplitSeq(goModContent, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return after
		}
	}
	return ""
}

func TestDeriveModuleName(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{"", "docbuilder-site"},
		{"https://example.com", "example-com"},
		{"http://example.com", "example-com"},
		{"https://docs.example.com", "docs-example-com"},
		{"https://example.com/path", "example-com"},
		{"http://localhost:1313", "localhost"},
		{"https://192.168.1.1:8080", "192-168-1-1"},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &config.Config{
				Hugo: config.HugoConfig{
					BaseURL: tt.baseURL,
				},
				Output: config.OutputConfig{
					Directory: tmpDir,
				},
			}

			gen := &Generator{
				config:    cfg,
				outputDir: tmpDir,
			}

			// Call ensureGoModForModules which will use deriveModuleName internally
			err := gen.ensureGoModForModules()
			if err != nil {
				t.Fatalf("ensureGoModForModules() error = %v", err)
			}

			// Read and verify
			// #nosec G304 -- test utility reading from test output directory
			content, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
			if err != nil {
				t.Fatalf("Failed to read go.mod: %v", err)
			}

			moduleName := extractModuleName(string(content))
			if moduleName != tt.want {
				t.Errorf("module name = %q, want %q", moduleName, tt.want)
			}
		})
	}
}
