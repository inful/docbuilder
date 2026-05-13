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
		{repoNameAPIDocs, testPatternAPILike, true, "contains match"},
		{"test-org/" + repoNameAPIDocs, testPatternAPILike, true, "contains match"},
		{repoNameAPIDocs, testPatternDocsLike, true, "contains match"},
		{"test-org/" + repoNameAPIDocs, testPatternDocsLike, true, "contains match"},
		{testRepoNameBackendSvc, testPatternAPILike, false, "should not match"},
		{testRepoNameBackendSvc, testPatternDocsLike, false, "should not match"},
		{"docs-website", testPatternDocsLike, true, "contains match"},
		{"legacy-docs", testPatternLegacyLike, true, "contains match"},

		// Test other pattern types
		{repoNameAPIDocs, testTopicAPI + "*", true, "prefix match"},
		{repoNameAPIDocs, "*" + docsToken, true, "suffix match"},
		{repoNameAPIDocs, repoNameAPIDocs, true, "exact match"},
		{repoNameAPIDocs, "*", true, "wildcard match"},
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
		{repoNameAPIDocs, testTopicAPI, true},
		{repoNameAPIDocs, docsToken, true},
		{"test-org/" + repoNameAPIDocs, testTopicAPI, true},
		{"test-org/" + repoNameAPIDocs, docsToken, true},
		{testRepoNameBackendSvc, testTopicAPI, false},
		{testRepoNameBackendSvc, docsToken, false},
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
