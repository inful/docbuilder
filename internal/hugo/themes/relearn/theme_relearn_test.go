package relearn

import (
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	th "git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

func TestRelearnThemeRegistration(t *testing.T) {
	theme := th.Get(config.ThemeRelearn)
	if theme == nil {
		t.Fatal("Relearn theme not registered")
	}

	if theme.Name() != config.ThemeRelearn {
		t.Errorf("theme.Name() = %v, want %v", theme.Name(), config.ThemeRelearn)
	}
}

func TestRelearnThemeFeatures(t *testing.T) {
	theme := Theme{}
	features := theme.Features()

	if features.Name != config.ThemeRelearn {
		t.Errorf("Features.Name = %v, want %v", features.Name, config.ThemeRelearn)
	}

	if !features.UsesModules {
		t.Error("Relearn should use Hugo modules")
	}

	if features.ModulePath != "github.com/McShelby/hugo-theme-relearn" {
		t.Errorf("ModulePath = %v, want github.com/McShelby/hugo-theme-relearn", features.ModulePath)
	}

	if !features.EnableMathPassthrough {
		t.Error("Relearn should support math passthrough")
	}

	if !features.ProvidesMermaidSupport {
		t.Error("Relearn should support Mermaid diagrams")
	}

	if !features.SupportsPerPageEditLinks {
		t.Error("Relearn should support per-page edit links")
	}
}

func TestRelearnApplyParams(t *testing.T) {
	theme := Theme{}
	params := make(map[string]any)

	theme.ApplyParams(nil, params)

	// editURL should not be set by default (requires valid URL pattern from user)
	if _, exists := params["editURL"]; exists {
		t.Error("editURL should not be set by default")
	}

	// disableSearch should not be set by default (v8+ uses search.disable)
	if _, exists := params["disableSearch"]; exists {
		t.Error("disableSearch should not be set by default (deprecated in v8+)")
	}

	// Check theme variant default
	if params["themeVariant"] != "auto" {
		t.Errorf("themeVariant = %v, want auto", params["themeVariant"])
	}

	// Check Mermaid support
	if mermaid, ok := params["mermaid"].(map[string]any); ok {
		if mermaid["enable"] != true {
			t.Errorf("mermaid.enable = %v, want true", mermaid["enable"])
		}
	} else {
		t.Error("mermaid should be a map with enable=true")
	}

	// Check Math support
	if math, ok := params["math"].(map[string]any); ok {
		if math["enable"] != true {
			t.Errorf("math.enable = %v, want true", math["enable"])
		}
	} else {
		t.Error("math should be a map with enable=true")
	}
}

func TestRelearnApplyParams_PreservesExisting(t *testing.T) {
	theme := Theme{}
	params := map[string]any{
		"themeVariant": "relearn-dark",
	}

	theme.ApplyParams(nil, params)

	// Should not override existing themeVariant
	if params["themeVariant"] != "relearn-dark" {
		t.Errorf("themeVariant = %v, should preserve existing value relearn-dark", params["themeVariant"])
	}
}
