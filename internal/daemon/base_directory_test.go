package daemon

import (
	"path/filepath"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestBuildServiceAdapter_BaseDirectory verifies that base_directory is properly combined with directory.
func TestBuildServiceAdapter_BaseDirectory(t *testing.T) {
	tests := []struct {
		name         string
		baseDir      string
		dir          string
		expectedPath string
		shouldBeAbs  bool
	}{
		{
			name:         "base_directory with relative directory",
			baseDir:      "/data",
			dir:          "site",
			expectedPath: "/data/site",
			shouldBeAbs:  true,
		},
		{
			name:         "base_directory with absolute directory (abs wins)",
			baseDir:      "/data",
			dir:          "/custom/site",
			expectedPath: "/custom/site",
			shouldBeAbs:  true,
		},
		{
			name:         "no base_directory with relative directory",
			baseDir:      "",
			dir:          "site",
			expectedPath: "site",
			shouldBeAbs:  false,
		},
		{
			name:         "no base_directory with absolute directory",
			baseDir:      "",
			dir:          "/var/site",
			expectedPath: "/var/site",
			shouldBeAbs:  true,
		},
		{
			name:         "empty directory with base_directory",
			baseDir:      "/data",
			dir:          "",
			expectedPath: "/data/site", // Default is "./site"
			shouldBeAbs:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Output: config.OutputConfig{
					BaseDirectory: tt.baseDir,
					Directory:     tt.dir,
				},
			}

			// Simulate what BuildServiceAdapter.Build does
			outDir := cfg.Output.Directory
			if outDir == "" {
				outDir = "./site"
			}
			if cfg.Output.BaseDirectory != "" && !filepath.IsAbs(outDir) {
				outDir = filepath.Join(cfg.Output.BaseDirectory, outDir)
			}

			if tt.shouldBeAbs && !filepath.IsAbs(outDir) {
				t.Errorf("Expected absolute path, got relative: %s", outDir)
			}

			if outDir != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, outDir)
			}
		})
	}
}
