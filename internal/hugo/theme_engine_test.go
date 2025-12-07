package hugo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
)

// TestThemeEngineCachesFeatures verifies that generateHugoConfig caches theme features
// and that deriveThemeFeatures returns the same features afterward.
func TestThemeEngineCachesFeatures(t *testing.T) {
	cfg := &config.Config{}
	// Select a known theme registered via side-effect imports in generator.go
	cfg.Hugo.Theme = "hextra"

	g := NewGenerator(cfg, t.TempDir())

	// Prepare staging so generateHugoConfig can write to build root
	if err := g.beginStaging(); err != nil {
		t.Fatalf("beginStaging: %v", err)
	}
	defer g.abortStaging()

	// Run config generation (calls theme engine and caches features)
	if err := g.generateHugoConfig(); err != nil {
		t.Fatalf("generateHugoConfig: %v", err)
	}

	if g.cachedThemeFeatures == nil {
		t.Fatalf("expected cachedThemeFeatures to be set")
	}

	cached := *g.cachedThemeFeatures
	derived := g.deriveThemeFeatures()

	if cached.Name != derived.Name {
		t.Fatalf("feature name mismatch: cached=%q derived=%q", cached.Name, derived.Name)
	}

	// Basic sanity: engine should reflect the selected theme type
	if derived.Name != cfg.Hugo.ThemeType() {
		t.Fatalf("expected features.Name=%q, got %q", cfg.Hugo.ThemeType(), derived.Name)
	}

	// Assert module import block exists when theme uses modules
	cfgPath := filepath.Join(g.buildRoot(), "hugo.yaml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read hugo.yaml: %v", err)
	}
	content := string(b)
	if strings.Contains(content, "module:") {
		if !strings.Contains(content, cached.ModulePath) {
			t.Fatalf("expected module import with path %q", cached.ModulePath)
		}
	} else if cached.UsesModules {
		t.Fatalf("expected module imports for a modules-enabled theme")
	}

	// Assert JSON output enabled for offline search when feature is set
	if cached.EnableOfflineSearchJSON {
		if !strings.Contains(content, "JSON") {
			t.Fatalf("expected outputs to include JSON when offline search is enabled")
		}
	}
}
