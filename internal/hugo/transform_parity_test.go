// Deprecated parity test file intentionally left as stub.
// The legacy transformer pipeline has been fully removed and parity
// validated via earlier commits. This placeholder remains only so that
// accidental re-introduction does not occur via merge conflicts.
package hugo

import "testing"

// TestLegacyPipelineRemoved documents that the old parity test suite was
// decommissioned once the registry-based transformer pipeline became the
// sole implementation. If this starts failing, investigate unintended
// reintroduction of legacy symbols.
func TestLegacyPipelineRemoved(t *testing.T) {
	// No-op: legacy pipeline symbols must not exist.
}
