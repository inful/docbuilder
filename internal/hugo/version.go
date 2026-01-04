package hugo

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

// DetectHugoVersion attempts to detect the version of the hugo binary on PATH.
// Returns the version string (e.g., "0.152.2") or empty string if detection fails.
// This is best-effort and will not error if hugo is unavailable.
func DetectHugoVersion(ctx context.Context) string {
	// Check if hugo is available
	hugoPath, err := exec.LookPath("hugo")
	if err != nil {
		return ""
	}

	// Run hugo version command
	// #nosec G204 -- hugoPath is from exec.LookPath, not user-controlled
	cmd := exec.CommandContext(ctx, hugoPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse version from output
	// Expected format examples:
	//   hugo v0.152.2+extended linux/amd64 BuildDate=2024-12-20T08:00:00Z
	//   Hugo Static Site Generator v0.152.2-extended
	//   v0.152.2
	return parseHugoVersion(string(output))
}

// parseHugoVersion extracts the semantic version from hugo version output.
// Returns empty string if parsing fails.
func parseHugoVersion(output string) string {
	// Match version pattern: v0.152.2, v0.152.2-extended, v0.152.2+extended
	// We want to extract just the numeric version: 0.152.2
	versionRegex := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}

	// Fallback: try to extract from simpler patterns
	// Look for first occurrence of X.Y.Z pattern
	simpleRegex := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches = simpleRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}

	return strings.TrimSpace(output) // Last resort: return trimmed output
}
