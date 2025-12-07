package plugin

import (
	"context"
	"testing"
)

// TestPluginMetadataValidation tests plugin metadata validation.
func TestPluginMetadataValidation(t *testing.T) {
	tests := []struct {
		name      string
		metadata  PluginMetadata
		expectErr bool
	}{
		{
			name: "valid metadata",
			metadata: PluginMetadata{
				Name:        "test-plugin",
				Version:     "v1.0.0",
				Type:        PluginTypeTheme,
				Description: "Test plugin",
			},
			expectErr: false,
		},
		{
			name: "missing name",
			metadata: PluginMetadata{
				Version: "v1.0.0",
				Type:    PluginTypeTheme,
			},
			expectErr: true,
		},
		{
			name: "missing version",
			metadata: PluginMetadata{
				Name: "test-plugin",
				Type: PluginTypeTheme,
			},
			expectErr: true,
		},
		{
			name: "invalid type",
			metadata: PluginMetadata{
				Name:    "test-plugin",
				Version: "v1.0.0",
				Type:    PluginType("invalid"),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metadata.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestPluginTypeValidation tests plugin type validation.
func TestPluginTypeValidation(t *testing.T) {
	tests := []struct {
		name       string
		pluginType PluginType
		expected   bool
	}{
		{"theme is valid", PluginTypeTheme, true},
		{"transform is valid", PluginTypeTransform, true},
		{"forge is valid", PluginTypeForge, true},
		{"publisher is valid", PluginTypePublisher, true},
		{"invalid type", PluginType("invalid"), false},
		{"empty type", PluginType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pluginType.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestPluginMetadataString tests metadata string representation.
func TestPluginMetadataString(t *testing.T) {
	metadata := PluginMetadata{
		Name:    "test-plugin",
		Version: "v1.0.0",
		Type:    PluginTypeTheme,
	}

	expected := "test-plugin@v1.0.0 (theme)"
	result := metadata.String()

	if result != expected {
		t.Errorf("String() = %q, expected %q", result, expected)
	}
}

// TestPluginError tests plugin error creation and unwrapping.
func TestPluginError(t *testing.T) {
	baseErr := context.Canceled
	pluginErr := NewPluginError("test-plugin", "execute", baseErr)

	// Test error message format
	expected := "plugin test-plugin failed during execute: context canceled"
	if pluginErr.Error() != expected {
		t.Errorf("Error() = %q, expected %q", pluginErr.Error(), expected)
	}

	// Test error unwrapping
	if pluginErr.Unwrap() != baseErr {
		t.Errorf("Unwrap() = %v, expected %v", pluginErr.Unwrap(), baseErr)
	}
}

// mockPlugin is a test implementation of the Plugin interface.
type mockPlugin struct {
	BasePlugin
	metadata PluginMetadata
}

func (m *mockPlugin) Metadata() PluginMetadata {
	return m.metadata
}

func (m *mockPlugin) Execute(ctx context.Context, pluginCtx *PluginContext) error {
	return nil
}

// TestBasePluginDefaults tests the default implementations in BasePlugin.
func TestBasePluginDefaults(t *testing.T) {
	plugin := &mockPlugin{
		metadata: PluginMetadata{
			Name:    "mock",
			Version: "v1.0.0",
			Type:    PluginTypeTheme,
		},
	}

	// Test default Init
	if err := plugin.Init(); err != nil {
		t.Errorf("Init() returned error: %v", err)
	}

	// Test default Cleanup
	if err := plugin.Cleanup(); err != nil {
		t.Errorf("Cleanup() returned error: %v", err)
	}

	// Test default Validate
	if err := plugin.Validate(nil); err != nil {
		t.Errorf("Validate() returned error: %v", err)
	}
}

// TestPluginContextValueStorage tests the PluginContext data storage.
func TestPluginContextValueStorage(t *testing.T) {
	ctx := &PluginContext{
		Context: context.Background(),
		Data:    make(map[string]interface{}),
	}

	// Test storing and retrieving values
	ctx = ctx.WithValue("key1", "value1")
	ctx = ctx.WithValue("key2", 42)
	ctx = ctx.WithValue("key3", true)

	// Test GetValue
	if val := ctx.GetValue("key1"); val != "value1" {
		t.Errorf("GetValue(key1) = %v, expected value1", val)
	}

	// Test GetString
	if val := ctx.GetString("key1"); val != "value1" {
		t.Errorf("GetString(key1) = %v, expected value1", val)
	}

	// Test GetInt
	if val := ctx.GetInt("key2"); val != 42 {
		t.Errorf("GetInt(key2) = %v, expected 42", val)
	}

	// Test GetBool
	if val := ctx.GetBool("key3"); val != true {
		t.Errorf("GetBool(key3) = %v, expected true", val)
	}

	// Test non-existent key
	if val := ctx.GetValue("nonexistent"); val != nil {
		t.Errorf("GetValue(nonexistent) = %v, expected nil", val)
	}

	// Test type mismatch
	if val := ctx.GetString("key2"); val != "" {
		t.Errorf("GetString(key2) = %v, expected empty string", val)
	}
}

// TestPluginContextImmutability tests that WithValue creates a new context.
func TestPluginContextImmutability(t *testing.T) {
	ctx1 := &PluginContext{
		Context: context.Background(),
		Data:    make(map[string]interface{}),
	}

	ctx2 := ctx1.WithValue("key1", "value1")
	ctx3 := ctx2.WithValue("key2", "value2")

	// ctx1 should not have key1
	if ctx1.GetValue("key1") != nil {
		t.Error("ctx1 should not have key1 after WithValue")
	}

	// ctx2 should have key1 but not key2
	if ctx2.GetValue("key1") == nil {
		t.Error("ctx2 should have key1")
	}
	if ctx2.GetValue("key2") != nil {
		t.Error("ctx2 should not have key2")
	}

	// ctx3 should have both keys
	if ctx3.GetValue("key1") == nil {
		t.Error("ctx3 should have key1")
	}
	if ctx3.GetValue("key2") == nil {
		t.Error("ctx3 should have key2")
	}
}
