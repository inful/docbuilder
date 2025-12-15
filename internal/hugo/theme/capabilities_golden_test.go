package theme

import (
	"encoding/json"
	"sort"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGoldenThemeCapabilities snapshots theme capability declarations.
func TestGoldenThemeCapabilities(t *testing.T) {
	type row struct {
		Theme      config.Theme `json:"theme"`
		EditLinks  bool         `json:"edit_links"`
		SearchJSON bool         `json:"search_json"`
	}
	var rows []row
	for tt, c := range themeCaps {
		rows = append(rows, row{Theme: tt, EditLinks: c.WantsPerPageEditLinks, SearchJSON: c.SupportsSearchJSON})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Theme < rows[j].Theme })
	b, _ := json.Marshal(rows)
	got := string(b)
	const expected = `[{"theme":"docsy","edit_links":true,"search_json":true},{"theme":"hextra","edit_links":true,"search_json":true},{"theme":"relearn","edit_links":true,"search_json":true}]`
	if got != expected {
		t.Fatalf("theme capabilities changed\nexpected: %s\n     got: %s", expected, got)
	}
}
