package plugin

import "fmt"

// PluginType identifies the category of plugin.
type PluginType string

const (
	// PluginTypeTheme provides Hugo theme configuration and customization.
	PluginTypeTheme PluginType = "theme"

	// PluginTypeTransform modifies content during the build pipeline.
	PluginTypeTransform PluginType = "transform"

	// PluginTypeForge integrates with Git hosting services (GitHub, GitLab, Gitea).
	PluginTypeForge PluginType = "forge"

	// PluginTypePublisher handles output distribution (S3, GitHub Pages, etc.).
	PluginTypePublisher PluginType = "publisher"
)

// IsValid returns true if the plugin type is recognized.
func (t PluginType) IsValid() bool {
	switch t {
	case PluginTypeTheme, PluginTypeTransform, PluginTypeForge, PluginTypePublisher:
		return true
	default:
		return false
	}
}

// String returns the string representation of the plugin type.
func (t PluginType) String() string {
	return string(t)
}

// ThemePlugin provides Hugo theme-specific functionality.
type ThemePlugin interface {
	Plugin

	// ThemeName returns the Hugo theme name (e.g., "hextra", "docsy").
	ThemeName() string

	// ModulePath returns the Hugo module path if the theme uses modules.
	// Returns empty string for traditional themes.
	ModulePath() string

	// ApplyParams adds theme-specific parameters to Hugo configuration.
	// params is the params section of hugo.yaml.
	ApplyParams(params map[string]interface{}) error

	// CustomizeConfig allows themes to modify the root Hugo configuration.
	// config is the entire hugo.yaml structure.
	CustomizeConfig(config map[string]interface{}) error
}

// TransformPlugin processes content during the build pipeline.
type TransformPlugin interface {
	Plugin

	// Transform modifies a document's content or metadata.
	// Returns the transformed content and updated metadata.
	Transform(content []byte, metadata map[string]interface{}) ([]byte, map[string]interface{}, error)

	// ShouldTransform returns true if this transform should apply to the given file.
	ShouldTransform(filePath string, metadata map[string]interface{}) bool
}

// ForgePlugin integrates with Git hosting services.
type ForgePlugin interface {
	Plugin

	// ForgeName returns the forge identifier (e.g., "github", "gitlab", "gitea").
	ForgeName() string

	// GetEditURL constructs the edit URL for a document in the forge.
	GetEditURL(repoURL, branch, filePath string) string

	// GetIssueURL constructs the issue/PR URL for a repository.
	GetIssueURL(repoURL string, issueNumber int) string
}

// PublisherPlugin handles output distribution.
type PublisherPlugin interface {
	Plugin

	// Publish uploads or deploys the generated site.
	// outputPath is the directory containing the Hugo site.
	Publish(outputPath string, config map[string]interface{}) error

	// GetPublishURL returns the URL where the site will be accessible.
	GetPublishURL(config map[string]interface{}) (string, error)
}

// PluginCapability describes optional features a plugin may provide.
type PluginCapability string

const (
	// CapabilitySearch indicates the plugin provides search functionality.
	CapabilitySearch PluginCapability = "search"

	// CapabilityMath indicates the plugin supports math rendering.
	CapabilityMath PluginCapability = "math"

	// CapabilityMermaid indicates the plugin supports Mermaid diagrams.
	CapabilityMermaid PluginCapability = "mermaid"

	// CapabilityI18n indicates the plugin supports internationalization.
	CapabilityI18n PluginCapability = "i18n"

	// CapabilityCache indicates the plugin supports incremental builds.
	CapabilityCache PluginCapability = "cache"

	// CapabilityWebhook indicates the plugin supports webhook triggers.
	CapabilityWebhook PluginCapability = "webhook"
)

// String returns the string representation of the capability.
func (c PluginCapability) String() string {
	return string(c)
}

// PluginError represents an error that occurred within a plugin.
type PluginError struct {
	// PluginName identifies which plugin failed.
	PluginName string

	// Operation describes what the plugin was doing when it failed.
	Operation string

	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s failed during %s: %v", e.PluginName, e.Operation, e.Err)
}

// Unwrap returns the underlying error for error inspection.
func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a new plugin error.
func NewPluginError(pluginName, operation string, err error) *PluginError {
	return &PluginError{
		PluginName: pluginName,
		Operation:  operation,
		Err:        err,
	}
}
