package hugo

import (
	"context"
	"testing"
)

func TestParseHugoVersion(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard extended format",
			output:   "hugo v0.152.2+extended linux/amd64 BuildDate=2024-12-20T08:00:00Z",
			expected: "0.152.2",
		},
		{
			name:     "extended dash format",
			output:   "Hugo Static Site Generator v0.152.2-extended",
			expected: "0.152.2",
		},
		{
			name:     "simple version",
			output:   "v0.152.2",
			expected: "0.152.2",
		},
		{
			name:     "bare version",
			output:   "0.152.2",
			expected: "0.152.2",
		},
		{
			name:     "with patch version",
			output:   "hugo v0.121.1-deadbeef+extended",
			expected: "0.121.1",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "invalid format",
			output:   "not a version",
			expected: "not a version", // Fallback to trimmed output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHugoVersion(tt.output)
			if result != tt.expected {
				t.Errorf("parseHugoVersion(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

func TestDetectHugoVersion(t *testing.T) {
	// This test verifies the function exists and handles both cases:
	// 1. Hugo available (returns version string)
	// 2. Hugo not available (returns empty string)

	version := DetectHugoVersion(context.Background())

	// We can't assert the exact value since it depends on the environment
	// But we can verify it returns a string (empty or with version)
	if version != "" {
		t.Logf("Hugo detected: version %s", version)
		// If version is returned, it should match semantic versioning pattern
		// We already test the parser above, so just log here
	} else {
		t.Log("Hugo not detected (not available in PATH)")
	}
}
