package forge

import (
	"encoding/json"
	"sort"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestGoldenForgeCapabilities snapshots the forge capability matrix to detect accidental changes.
func TestGoldenForgeCapabilities(t *testing.T) {
	type row struct {
		Forge             config.ForgeType `json:"forge"`
		SupportsEditLinks bool             `json:"edit_links"`
		SupportsWebhooks  bool             `json:"webhooks"`
		SupportsTopics    bool             `json:"topics"`
	}
	var rows []row
	for ft, c := range caps {
		rows = append(rows, row{Forge: ft, SupportsEditLinks: c.SupportsEditLinks, SupportsWebhooks: c.SupportsWebhooks, SupportsTopics: c.SupportsTopics})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Forge < rows[j].Forge })
	gotBytes, _ := json.Marshal(rows)
	got := string(gotBytes)
	const expected = `[{"forge":"forgejo","edit_links":true,"webhooks":true,"topics":false},{"forge":"github","edit_links":true,"webhooks":true,"topics":true},{"forge":"gitlab","edit_links":true,"webhooks":true,"topics":true}]`
	if got != expected {
		t.Fatalf("forge capabilities changed\nexpected: %s\n     got: %s", expected, got)
	}
}
