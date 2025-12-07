package plugin

import (
	"fmt"
	"sync"
)

// Registry manages plugin registration and discovery.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]map[string]Plugin // map[name]map[version]Plugin
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
// Returns an error if a plugin with the same name and version already exists.
func (r *Registry) Register(plugin Plugin) error {
	if plugin == nil {
		return fmt.Errorf("cannot register nil plugin")
	}

	metadata := plugin.Metadata()
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("invalid plugin metadata: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize version map if needed
	if r.plugins[metadata.Name] == nil {
		r.plugins[metadata.Name] = make(map[string]Plugin)
	}

	// Check for duplicate
	if _, exists := r.plugins[metadata.Name][metadata.Version]; exists {
		return fmt.Errorf("plugin %s@%s already registered", metadata.Name, metadata.Version)
	}

	r.plugins[metadata.Name][metadata.Version] = plugin
	return nil
}

// Get retrieves a specific plugin by name and version.
// Returns an error if the plugin is not found.
func (r *Registry) Get(name, version string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	plugin, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("plugin %s@%s not found", name, version)
	}

	return plugin, nil
}

// GetLatest retrieves the latest registered version of a plugin by name.
// Returns an error if no versions are registered.
func (r *Registry) GetLatest(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("plugin %s has no registered versions", name)
	}

	// For now, return any version (in practice, we'd need semantic versioning)
	// This is a simple implementation that returns the first version found
	for _, plugin := range versions {
		return plugin, nil
	}

	return nil, fmt.Errorf("plugin %s has no registered versions", name)
}

// List returns all registered plugins.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, versions := range r.plugins {
		for _, plugin := range versions {
			result = append(result, plugin)
		}
	}

	return result
}

// ListByType returns all plugins of a specific type.
func (r *Registry) ListByType(pluginType PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, versions := range r.plugins {
		for _, plugin := range versions {
			if plugin.Metadata().Type == pluginType {
				result = append(result, plugin)
			}
		}
	}

	return result
}

// ListVersions returns all registered versions of a plugin.
func (r *Registry) ListVersions(name string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.plugins[name]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}

	return result
}

// Has checks if a plugin with the given name exists (any version).
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.plugins[name]
	return ok
}

// HasVersion checks if a specific plugin version exists.
func (r *Registry) HasVersion(name, version string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.plugins[name]
	if !ok {
		return false
	}

	_, ok = versions[version]
	return ok
}

// Unregister removes a plugin from the registry.
func (r *Registry) Unregister(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	versions, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}

	if _, ok := versions[version]; !ok {
		return fmt.Errorf("plugin %s@%s not found", name, version)
	}

	delete(versions, version)

	// Clean up empty version map
	if len(versions) == 0 {
		delete(r.plugins, name)
	}

	return nil
}

// Clear removes all plugins from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]map[string]Plugin)
}

// Count returns the total number of registered plugins (all versions).
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, versions := range r.plugins {
		count += len(versions)
	}

	return count
}

// globalRegistry is the default plugin registry used throughout the application.
var globalRegistry = NewRegistry()

// DefaultRegistry returns the global plugin registry.
func DefaultRegistry() *Registry {
	return globalRegistry
}

// Register adds a plugin to the global registry.
func Register(plugin Plugin) error {
	return globalRegistry.Register(plugin)
}

// Get retrieves a plugin from the global registry.
func Get(name, version string) (Plugin, error) {
	return globalRegistry.Get(name, version)
}

// GetLatest retrieves the latest version from the global registry.
func GetLatest(name string) (Plugin, error) {
	return globalRegistry.GetLatest(name)
}

// List returns all plugins from the global registry.
func List() []Plugin {
	return globalRegistry.List()
}

// ListByType returns plugins of a specific type from the global registry.
func ListByType(pluginType PluginType) []Plugin {
	return globalRegistry.ListByType(pluginType)
}
