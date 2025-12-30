package forge

import (
	"testing"
)

// TestPatternMatchingDebug tests the pattern matching logic directly.
func TestPatternMatchingDebug(t *testing.T) {
	// Test cases that should match
	testCases := []struct {
		str     string
		pattern string
		want    bool
		note    string
	}{
		{"api-docs", "*api*", true, "contains match"},
		{"test-org/api-docs", "*api*", true, "contains match"},
		{"api-docs", "*docs*", true, "contains match"},
		{"test-org/api-docs", "*docs*", true, "contains match"},
		{"backend-service", "*api*", false, "should not match"},
		{"backend-service", "*docs*", false, "should not match"},
		{"docs-website", "*docs*", true, "contains match"},
		{"legacy-docs", "*legacy*", true, "contains match"},

		// Test other pattern types
		{"api-docs", "api*", true, "prefix match"},
		{"api-docs", "*docs", true, "suffix match"},
		{"api-docs", "api-docs", true, "exact match"},
		{"api-docs", "*", true, "wildcard match"},
	}

	for _, tc := range testCases {
		result := matchesPattern(tc.str, tc.pattern)
		if result != tc.want {
			t.Errorf("matchesPattern(%q, %q) = %v, want %v (%s)", tc.str, tc.pattern, result, tc.want, tc.note)
		} else {
			t.Logf("✓ matchesPattern(%q, %q) = %v (%s)", tc.str, tc.pattern, result, tc.note)
		}
	}
}

// TestContainsFunction tests the contains helper function.
func TestContainsFunction(t *testing.T) {
	testCases := []struct {
		str    string
		substr string
		want   bool
	}{
		{"api-docs", "api", true},
		{"api-docs", "docs", true},
		{"test-org/api-docs", "api", true},
		{"test-org/api-docs", "docs", true},
		{"backend-service", "api", false},
		{"backend-service", "docs", false},
	}

	for _, tc := range testCases {
		result := contains(tc.str, tc.substr)
		if result != tc.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tc.str, tc.substr, result, tc.want)
		} else {
			t.Logf("✓ contains(%q, %q) = %v", tc.str, tc.substr, result)
		}
	}
}
