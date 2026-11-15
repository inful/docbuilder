package models

import "testing"

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
			expected: "", // no version in the output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHugoVersion(tt.output)
			if result != tt.expected {
				t.Errorf("ParseHugoVersion(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

func TestDetectHugoVersion(t *testing.T) {
	// The version depends on the environment: empty when hugo is not
	// installed, a version string otherwise. It is detected once per
	// process and cached.
	version := DetectHugoVersion(t.Context())

	if version == "" {
		t.Log("Hugo not detected (not available in PATH)")
		return
	}
	t.Logf("Hugo detected: version %s", version)

	// A second call must return the same cached value.
	if again := DetectHugoVersion(t.Context()); again != version {
		t.Errorf("DetectHugoVersion() = %q on the second call, want %q", again, version)
	}
}
