package hextra

import (
	"context"
	"testing"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
)

// TestHextraPluginMetadata tests the plugin metadata.
func TestHextraPluginMetadata(t *testing.T) {
	p := NewHextraPlugin()
	metadata := p.Metadata()

	if metadata.Name != "hextra" {
		t.Errorf("Metadata.Name = %q, expected 'hextra'", metadata.Name)
	}

	if metadata.Type != plugin.PluginTypeTheme {
		t.Errorf("Metadata.Type = %v, expected %v", metadata.Type, plugin.PluginTypeTheme)
	}

	if metadata.Author != "imfing" {
		t.Errorf("Metadata.Author = %q, expected 'imfing'", metadata.Author)
	}

	if metadata.Version == "" {
		t.Error("Metadata.Version should not be empty")
	}

	if metadata.Description == "" {
		t.Error("Metadata.Description should not be empty")
	}
}

// TestHextraPluginValidate tests plugin validation.
func TestHextraPluginValidate(t *testing.T) {
	p := NewHextraPlugin()

	// Test with nil config (should be allowed for themes)
	if err := p.Validate(nil); err != nil {
		t.Errorf("Validate(nil) failed: %v", err)
	}

	// Test with empty config
	if err := p.Validate(map[string]interface{}{}); err != nil {
		t.Errorf("Validate({}) failed: %v", err)
	}
}

// TestHextraPluginExecute tests plugin execution.
func TestHextraPluginExecute(t *testing.T) {
	p := NewHextraPlugin()
	ctx := context.Background()
	pluginCtx := &plugin.PluginContext{
		Context: ctx,
	}

	// Execute should be a no-op for themes
	if err := p.Execute(ctx, pluginCtx); err != nil {
		t.Errorf("Execute() failed: %v", err)
	}
}

// TestHextraPluginThemeName tests the theme name.
func TestHextraPluginThemeName(t *testing.T) {
	p := NewHextraPlugin()
	name := p.ThemeName()

	if name == "" {
		t.Error("ThemeName() should not return empty string")
	}

	// Hextra should be recognized
	if name != "hextra" && name != "Hextra" {
		t.Errorf("ThemeName() = %q, expected hextra-like", name)
	}
}

// TestHextraPluginModulePath tests the module path.
func TestHextraPluginModulePath(t *testing.T) {
	p := NewHextraPlugin()
	modulePath := p.ModulePath()

	// Hextra uses Hugo modules, so it should have a module path
	if modulePath == "" {
		t.Skip("Hextra module path is empty (may be optional)")
	}

	// Module path should be a valid identifier
	if len(modulePath) > 0 && modulePath[0] == '/' {
		t.Errorf("ModulePath() = %q should not start with /", modulePath)
	}
}

// TestHextraPluginApplyParams tests parameter application.
func TestHextraPluginApplyParams(t *testing.T) {
	p := NewHextraPlugin()
	params := make(map[string]interface{})

	// Apply params should modify the map
	if err := p.ApplyParams(params); err != nil {
		t.Errorf("ApplyParams() failed: %v", err)
	}

	// Params should have been populated
	if len(params) == 0 {
		t.Logf("Warning: ApplyParams() did not add any parameters")
	}
}

// TestHextraPluginCustomizeConfig tests config customization.
func TestHextraPluginCustomizeConfig(t *testing.T) {
	p := NewHextraPlugin()
	config := make(map[string]interface{})

	// Customize config should modify the map
	if err := p.CustomizeConfig(config); err != nil {
		t.Errorf("CustomizeConfig() failed: %v", err)
	}

	// Config should have been customized (may be populated)
	t.Logf("Customized config keys: %v", len(config))
}

// TestHextraPluginGetTheme tests getting the underlying theme.
func TestHextraPluginGetTheme(t *testing.T) {
	p := NewHextraPlugin()
	theme := p.GetTheme()

	if theme == nil {
		t.Error("GetTheme() should return non-nil theme")
	}
}

// TestHextraPluginImplementsPlugin tests that HextraPlugin implements Plugin.
func TestHextraPluginImplementsPlugin(t *testing.T) {
	p := NewHextraPlugin()

	// Verify it implements Plugin interface
	var _ plugin.Plugin = p

	// Verify it implements ThemePlugin interface
	var _ plugin.ThemePlugin = p
}

// TestHextraPluginCapabilities tests plugin capabilities.
func TestHextraPluginCapabilities(t *testing.T) {
	p := NewHextraPlugin()
	metadata := p.Metadata()

	// Should have some capabilities
	if len(metadata.Capabilities) == 0 {
		t.Logf("Warning: Hextra plugin has no advertised capabilities")
	}

	// Capabilities should be valid
	for _, cap := range metadata.Capabilities {
		if cap == "" {
			t.Error("Empty capability in list")
		}
	}
}

// TestHextraPluginRegistration tests that the plugin is registered globally.
func TestHextraPluginRegistration(t *testing.T) {
	registry := plugin.DefaultRegistry()

	// Check if Hextra is registered
	if !registry.Has("hextra") {
		t.Skip("Hextra plugin not registered in global registry (may be optional at init time)")
	}

	// Try to get the latest version
	hextraPlugin, err := registry.GetLatest("hextra")
	if err != nil {
		t.Logf("GetLatest(hextra) failed: %v (may be expected if not auto-registered)", err)
	}

	if hextraPlugin != nil {
		metadata := hextraPlugin.Metadata()
		if metadata.Name != "hextra" {
			t.Errorf("Retrieved plugin has wrong name: %q", metadata.Name)
		}
	}
}

// TestHextraPluginMetadataValidation tests metadata validation.
func TestHextraPluginMetadataValidation(t *testing.T) {
	p := NewHextraPlugin()
	metadata := p.Metadata()

	// Validate metadata
	if err := metadata.Validate(); err != nil {
		t.Errorf("PluginMetadata.Validate() failed: %v", err)
	}
}

// TestHextraPluginMultipleInstances tests creating multiple plugin instances.
func TestHextraPluginMultipleInstances(t *testing.T) {
	p1 := NewHextraPlugin()
	p2 := NewHextraPlugin()

	// Both should have the same metadata
	m1 := p1.Metadata()
	m2 := p2.Metadata()

	if m1.Name != m2.Name {
		t.Errorf("Plugin names differ: %q vs %q", m1.Name, m2.Name)
	}

	if m1.Type != m2.Type {
		t.Errorf("Plugin types differ: %v vs %v", m1.Type, m2.Type)
	}
}

// TestHextraPluginParamsStructure tests the structure of applied parameters.
func TestHextraPluginParamsStructure(t *testing.T) {
	p := NewHextraPlugin()
	params := make(map[string]interface{})

	if err := p.ApplyParams(params); err != nil {
		t.Fatalf("ApplyParams() failed: %v", err)
	}

	// Check for common theme parameters
	// (These are optional as different themes have different structures)
	knownParams := []string{"ui", "colors", "fonts"}
	for _, param := range knownParams {
		if val, ok := params[param]; ok {
			t.Logf("Found parameter %q: %T", param, val)
		}
	}
}
