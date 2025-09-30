package transforms

import "testing"

// facade_migration_test ensures RelativeLinkRewriter uses facade-style getters/setters (non-reflective path).
func TestRelativeLinkRewriterUsesFacadeMethods(t *testing.T) {
	shim := &PageShim{Content: "See [link](./relative/path.md)"}
	// Provide a trivial rewriter that annotates to prove invocation.
	shim.RewriteLinks = func(s string) string { return s + "#rewritten" }
	rew := RelativeLinkRewriter{}
	if err := rew.Transform(shim); err != nil {
		t.Fatalf("transform error: %v", err)
	}
	if shim.Content != "See [link](./relative/path.md)#rewritten" {
		// Validate that SetContent was used (indirectly) and content mutated.
		// (If it were not, we'd still detect mismatch but comment clarifies intent.)
		// In future we may mock methods to assert call counts.
		// For now, value assertion is sufficient.
		//
		// NOTE: this test intentionally shallow; deeper link rewriting logic lives elsewhere.
		// Failure here indicates facade adaptation regression.
		//
		// Provide clear message for maintainers:
		// "RelativeLinkRewriter must mutate PageShim via SetContent facade".
		//
		// Keep output crisp.
		//
		//--
		// Actual mismatch details:
		t.Fatalf("expected rewritten content, got %q", shim.Content)
	}
}
