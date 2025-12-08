package plugin

import (
	"context"
	"testing"
)

// mockPluginForRegistry is a test plugin for registry tests.
type mockPluginForRegistry struct {
	BasePlugin
	metadata PluginMetadata
}

func (m *mockPluginForRegistry) Metadata() PluginMetadata {
	return m.metadata
}

func (m *mockPluginForRegistry) Execute(ctx context.Context, pluginCtx *PluginContext) error {
	return nil
}

func newMockPlugin(name, version string, pluginType PluginType) Plugin {
	return &mockPluginForRegistry{
		metadata: PluginMetadata{
			Name:    name,
			Version: version,
			Type:    pluginType,
		},
	}
}

// TestRegistryRegister tests plugin registration.
func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()

	plugin := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)

	// Register plugin
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Verify it was registered
	if !registry.Has("test-plugin") {
		t.Error("Plugin should be registered")
	}

	// Try to register duplicate
	if err := registry.Register(plugin); err == nil {
		t.Error("Should not allow duplicate registration")
	}
}

// TestRegistryRegisterNil tests registering nil plugin.
func TestRegistryRegisterNil(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(nil); err == nil {
		t.Error("Should not allow registering nil plugin")
	}
}

// TestRegistryRegisterInvalidMetadata tests registering plugin with invalid metadata.
func TestRegistryRegisterInvalidMetadata(t *testing.T) {
	registry := NewRegistry()

	// Plugin with missing name
	plugin := &mockPluginForRegistry{
		metadata: PluginMetadata{
			Version: "v1.0.0",
			Type:    PluginTypeTheme,
		},
	}

	if err := registry.Register(plugin); err == nil {
		t.Error("Should not allow plugin with invalid metadata")
	}
}

// TestRegistryGet tests retrieving plugins.
func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	plugin := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Get existing plugin
	retrieved, err := registry.Get("test-plugin", "v1.0.0")
	if err != nil {
		t.Errorf("Get() failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved plugin should not be nil")
	}

	// Get non-existent plugin
	_, err = registry.Get("non-existent", "v1.0.0")
	if err == nil {
		t.Error("Should return error for non-existent plugin")
	}

	// Get wrong version
	_, err = registry.Get("test-plugin", "v2.0.0")
	if err == nil {
		t.Error("Should return error for wrong version")
	}
}

// TestRegistryGetLatest tests retrieving latest plugin version.
func TestRegistryGetLatest(t *testing.T) {
	registry := NewRegistry()

	plugin1 := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	plugin2 := newMockPlugin("test-plugin", "v2.0.0", PluginTypeTheme)

	if err := registry.Register(plugin1); err != nil {
		t.Fatalf("Register(v1) failed: %v", err)
	}
	if err := registry.Register(plugin2); err != nil {
		t.Fatalf("Register(v2) failed: %v", err)
	}

	// Get latest version
	latest, err := registry.GetLatest("test-plugin")
	if err != nil {
		t.Errorf("GetLatest() failed: %v", err)
	}
	if latest == nil {
		t.Error("Latest plugin should not be nil")
	}

	// Get latest for non-existent plugin
	_, err = registry.GetLatest("non-existent")
	if err == nil {
		t.Error("Should return error for non-existent plugin")
	}
}

// TestRegistryList tests listing all plugins.
func TestRegistryList(t *testing.T) {
	registry := NewRegistry()

	plugin1 := newMockPlugin("plugin1", "v1.0.0", PluginTypeTheme)
	plugin2 := newMockPlugin("plugin2", "v1.0.0", PluginTypeTransform)
	plugin3 := newMockPlugin("plugin1", "v2.0.0", PluginTypeTheme)

	if err := registry.Register(plugin1); err != nil {
		t.Fatalf("Register(plugin1) failed: %v", err)
	}
	if err := registry.Register(plugin2); err != nil {
		t.Fatalf("Register(plugin2) failed: %v", err)
	}
	if err := registry.Register(plugin3); err != nil {
		t.Fatalf("Register(plugin1 v2) failed: %v", err)
	}

	plugins := registry.List()
	if len(plugins) != 3 {
		t.Errorf("List() returned %d plugins, expected 3", len(plugins))
	}
}

// TestRegistryListByType tests listing plugins by type.
func TestRegistryListByType(t *testing.T) {
	registry := NewRegistry()

	theme1 := newMockPlugin("theme1", "v1.0.0", PluginTypeTheme)
	theme2 := newMockPlugin("theme2", "v1.0.0", PluginTypeTheme)
	transform := newMockPlugin("transform1", "v1.0.0", PluginTypeTransform)

	if err := registry.Register(theme1); err != nil {
		t.Fatalf("Register(theme1) failed: %v", err)
	}
	if err := registry.Register(theme2); err != nil {
		t.Fatalf("Register(theme2) failed: %v", err)
	}
	if err := registry.Register(transform); err != nil {
		t.Fatalf("Register(transform) failed: %v", err)
	}

	themes := registry.ListByType(PluginTypeTheme)
	if len(themes) != 2 {
		t.Errorf("ListByType(Theme) returned %d plugins, expected 2", len(themes))
	}

	transforms := registry.ListByType(PluginTypeTransform)
	if len(transforms) != 1 {
		t.Errorf("ListByType(Transform) returned %d plugins, expected 1", len(transforms))
	}
}

// TestRegistryListVersions tests listing plugin versions.
func TestRegistryListVersions(t *testing.T) {
	registry := NewRegistry()

	plugin1 := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	plugin2 := newMockPlugin("test-plugin", "v2.0.0", PluginTypeTheme)
	plugin3 := newMockPlugin("test-plugin", "v3.0.0", PluginTypeTheme)

	if err := registry.Register(plugin1); err != nil {
		t.Fatalf("Register(v1) failed: %v", err)
	}
	if err := registry.Register(plugin2); err != nil {
		t.Fatalf("Register(v2) failed: %v", err)
	}
	if err := registry.Register(plugin3); err != nil {
		t.Fatalf("Register(v3) failed: %v", err)
	}

	versions := registry.ListVersions("test-plugin")
	if len(versions) != 3 {
		t.Errorf("ListVersions() returned %d versions, expected 3", len(versions))
	}

	// Check for non-existent plugin
	versions = registry.ListVersions("non-existent")
	if versions != nil {
		t.Error("ListVersions() should return nil for non-existent plugin")
	}
}

// TestRegistryHas tests checking plugin existence.
func TestRegistryHas(t *testing.T) {
	registry := NewRegistry()

	plugin := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	if !registry.Has("test-plugin") {
		t.Error("Has() should return true for registered plugin")
	}

	if registry.Has("non-existent") {
		t.Error("Has() should return false for non-existent plugin")
	}
}

// TestRegistryHasVersion tests checking specific plugin version.
func TestRegistryHasVersion(t *testing.T) {
	registry := NewRegistry()

	plugin := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	if !registry.HasVersion("test-plugin", "v1.0.0") {
		t.Error("HasVersion() should return true for registered version")
	}

	if registry.HasVersion("test-plugin", "v2.0.0") {
		t.Error("HasVersion() should return false for non-existent version")
	}

	if registry.HasVersion("non-existent", "v1.0.0") {
		t.Error("HasVersion() should return false for non-existent plugin")
	}
}

// TestRegistryUnregister tests removing plugins.
func TestRegistryUnregister(t *testing.T) {
	registry := NewRegistry()

	plugin := newMockPlugin("test-plugin", "v1.0.0", PluginTypeTheme)
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Unregister existing plugin
	if err := registry.Unregister("test-plugin", "v1.0.0"); err != nil {
		t.Errorf("Unregister() failed: %v", err)
	}

	if registry.Has("test-plugin") {
		t.Error("Plugin should be removed after Unregister()")
	}

	// Try to unregister non-existent plugin
	if err := registry.Unregister("non-existent", "v1.0.0"); err == nil {
		t.Error("Unregister() should return error for non-existent plugin")
	}
}

// TestRegistryClear tests clearing all plugins.
func TestRegistryClear(t *testing.T) {
	registry := NewRegistry()

	plugin1 := newMockPlugin("plugin1", "v1.0.0", PluginTypeTheme)
	plugin2 := newMockPlugin("plugin2", "v1.0.0", PluginTypeTransform)

	if err := registry.Register(plugin1); err != nil {
		t.Fatalf("Register(plugin1) failed: %v", err)
	}
	if err := registry.Register(plugin2); err != nil {
		t.Fatalf("Register(plugin2) failed: %v", err)
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Count() = %d after Clear(), expected 0", registry.Count())
	}
}

// TestRegistryCount tests counting plugins.
func TestRegistryCount(t *testing.T) {
	registry := NewRegistry()

	if registry.Count() != 0 {
		t.Error("New registry should have count 0")
	}

	plugin1 := newMockPlugin("plugin1", "v1.0.0", PluginTypeTheme)
	plugin2 := newMockPlugin("plugin1", "v2.0.0", PluginTypeTheme)
	plugin3 := newMockPlugin("plugin2", "v1.0.0", PluginTypeTransform)

	if err := registry.Register(plugin1); err != nil {
		t.Fatalf("Register(plugin1 v1) failed: %v", err)
	}
	if err := registry.Register(plugin2); err != nil {
		t.Fatalf("Register(plugin1 v2) failed: %v", err)
	}
	if err := registry.Register(plugin3); err != nil {
		t.Fatalf("Register(plugin2) failed: %v", err)
	}

	if registry.Count() != 3 {
		t.Errorf("Count() = %d, expected 3", registry.Count())
	}
}

// TestRegistryConcurrency tests concurrent access to registry.
func TestRegistryConcurrency(t *testing.T) {
	registry := NewRegistry()

	// Register plugins concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			plugin := newMockPlugin("concurrent-plugin", "v1.0."+string(rune('0'+id)), PluginTypeTheme)
			_ = registry.Register(plugin)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify plugins were registered
	if registry.Count() < 1 {
		t.Error("Concurrent registration should register at least one plugin")
	}
}

// TestGlobalRegistry tests the global registry functions.
func TestGlobalRegistry(t *testing.T) {
	// Clear global registry first
	DefaultRegistry().Clear()

	plugin := newMockPlugin("global-plugin", "v1.0.0", PluginTypeTheme)

	// Register to global registry
	if err := Register(plugin); err != nil {
		t.Fatalf("Register() to global registry failed: %v", err)
	}

	// Get from global registry
	retrieved, err := Get("global-plugin", "v1.0.0")
	if err != nil {
		t.Errorf("Get() from global registry failed: %v", err)
	}
	if retrieved == nil {
		t.Error("Retrieved plugin from global registry should not be nil")
	}

	// List from global registry
	plugins := List()
	if len(plugins) == 0 {
		t.Error("List() from global registry should return plugins")
	}

	// GetLatest from global registry
	latest, err := GetLatest("global-plugin")
	if err != nil {
		t.Errorf("GetLatest() from global registry failed: %v", err)
	}
	if latest == nil {
		t.Error("GetLatest() should return plugin")
	}

	// ListByType from global registry
	themes := ListByType(PluginTypeTheme)
	if len(themes) == 0 {
		t.Error("ListByType() from global registry should return themes")
	}

	// Clean up
	DefaultRegistry().Clear()
}
