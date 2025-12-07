// Package plugin provides a plugin system for extending DocBuilder functionality.
// Plugins can implement themes, transforms, forge integrations, and publishers.
package plugin

import (
	"context"
	"fmt"
)

// Plugin represents a DocBuilder plugin with metadata and lifecycle methods.
type Plugin interface {
	// Metadata returns the plugin's metadata (name, version, type, capabilities).
	Metadata() PluginMetadata

	// Validate checks if the plugin can run with the given configuration.
	// Returns an error if the configuration is invalid or incompatible.
	Validate(config map[string]interface{}) error

	// Execute runs the plugin with the given context.
	// The context provides access to services, configuration, and execution state.
	Execute(ctx context.Context, pluginCtx *PluginContext) error
}

// PluginLifecycle extends Plugin with optional lifecycle hooks.
type PluginLifecycle interface {
	Plugin

	// Init is called once when the plugin is loaded.
	// Use this to initialize resources, validate dependencies, etc.
	Init() error

	// Cleanup is called when the plugin is unloaded.
	// Use this to release resources, close connections, etc.
	Cleanup() error
}

// PluginMetadata describes a plugin's identity and capabilities.
type PluginMetadata struct {
	// Name is the unique plugin identifier (e.g., "hextra", "frontmatter").
	Name string

	// Version is the semantic version (e.g., "v1.0.0", "v0.11.0").
	Version string

	// Type identifies the plugin category (Theme, Transform, Forge, Publisher).
	Type PluginType

	// Description provides a human-readable summary of the plugin's purpose.
	Description string

	// Author is the plugin creator or maintainer.
	Author string

	// Capabilities lists optional features this plugin provides.
	Capabilities []string

	// Dependencies lists other plugins this plugin requires.
	Dependencies []PluginDependency
}

// PluginDependency describes a required or optional plugin dependency.
type PluginDependency struct {
	// Name is the required plugin name.
	Name string

	// Version is the semantic version constraint (e.g., "^1.0.0", ">=2.0.0").
	Version string

	// Optional indicates if this dependency is required.
	Optional bool
}

// String returns a human-readable representation of the plugin metadata.
func (m PluginMetadata) String() string {
	return fmt.Sprintf("%s@%s (%s)", m.Name, m.Version, m.Type)
}

// Validate checks if the plugin metadata is valid.
func (m PluginMetadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("plugin version is required")
	}
	if !m.Type.IsValid() {
		return fmt.Errorf("invalid plugin type: %s", m.Type)
	}
	return nil
}

// BasePlugin provides default implementations for plugin lifecycle methods.
// Plugins can embed this to avoid implementing optional methods.
type BasePlugin struct{}

// Init is a no-op default implementation.
func (b *BasePlugin) Init() error {
	return nil
}

// Cleanup is a no-op default implementation.
func (b *BasePlugin) Cleanup() error {
	return nil
}

// Validate is a no-op default implementation that accepts any configuration.
func (b *BasePlugin) Validate(config map[string]interface{}) error {
	return nil
}
