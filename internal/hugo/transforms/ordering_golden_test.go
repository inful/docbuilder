package transforms

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGoldenOrdering emits a stable JSON array of transformer names in execution order.
// If this test fails after intentional ordering changes, update expectedJSON accordingly.
func TestGoldenOrdering(t *testing.T) {
	ts, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	names := make([]string, 0, len(ts))
	for _, tr := range ts {
		names = append(names, tr.Name())
	}
	gotBytes, _ := json.Marshal(names)
	got := string(gotBytes)
	// Keep lexicographically simple expected value (manually updated when pipeline changes).
	// Current expected order is determined by dependency-based topological sorting.
	const expectedJSON = `["front_matter_parser","front_matter_builder_v2","extract_index_title","rewrite_image_paths","edit_link_injector_v2","front_matter_merge","relative_link_rewriter","strip_first_heading","shortcode_escaper","hextra_type_enforcer","front_matter_serialize"]`
	if got != expectedJSON {
		t.Fatalf("transform ordering changed\nexpected: %s\n     got: %s\nDiff hint: split and compare. Names: %s", expectedJSON, got, strings.Join(names, ", "))
	}
}
