package forge

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

func TestFilteringConfigAccess(t *testing.T) {
	filtering := &config.FilteringConfig{
		RequiredPaths: []string{"docs"},
		IgnoreFiles:   []string{".docignore"},
	}

	if len(filtering.RequiredPaths) != 1 {
		t.Error("FilteringConfig should work")
	}
}
