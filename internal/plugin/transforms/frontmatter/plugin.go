package frontmatter

import (
	"context"
	"strings"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
	"git.home.luguber.info/inful/docbuilder/internal/plugin/transforms"
)

// FrontmatterTransform is an example transform that processes frontmatter/metadata.
type FrontmatterTransform struct {
	transforms.BaseTransformPlugin
}

// NewFrontmatterTransform creates a new frontmatter transform plugin.
func NewFrontmatterTransform() *FrontmatterTransform {
	return &FrontmatterTransform{}
}

// Metadata returns the plugin metadata for the frontmatter transform.
func (p *FrontmatterTransform) Metadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:        "frontmatter",
		Version:     "v1.0.0",
		Type:        plugin.PluginTypeTransform,
		Description: "Processes and normalizes frontmatter/metadata in markdown files",
		Author:      "docbuilder",
		Capabilities: []string{
			string(plugin.CapabilityCache),
		},
	}
}

// Validate checks if the transform can run with the given configuration.
func (p *FrontmatterTransform) Validate(config map[string]interface{}) error {
	// Frontmatter transform accepts any configuration
	return nil
}

// Execute runs the frontmatter transform.
func (p *FrontmatterTransform) Execute(ctx context.Context, pluginCtx *plugin.PluginContext) error {
	// Execution logic handled in Apply()
	return nil
}

// Stage returns when this transform should apply (during frontmatter processing).
func (p *FrontmatterTransform) Stage() transforms.TransformStage {
	return transforms.StageFrontmatter
}

// Dependencies returns the dependency constraints for this transform.
// FrontmatterTransform has no dependencies as it runs early in the pipeline.
func (p *FrontmatterTransform) Dependencies() transforms.TransformDependencies {
	return transforms.TransformDependencies{
		MustRunAfter:  []string{}, // No dependencies
		MustRunBefore: []string{}, // No specific ordering required
	}
}

// ShouldApply returns true for markdown files with frontmatter.
func (p *FrontmatterTransform) ShouldApply(input *transforms.TransformInput) bool {
	// Apply to files with .md or .markdown extension
	return strings.HasSuffix(input.FilePath, ".md") || strings.HasSuffix(input.FilePath, ".markdown")
}

// Apply processes frontmatter in the content.
func (p *FrontmatterTransform) Apply(input *transforms.TransformInput) *transforms.TransformResult {
	result := &transforms.TransformResult{
		Content:  input.Content,
		Metadata: input.Metadata,
	}

	// Normalize frontmatter structure
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}

	// Ensure required fields exist
	ensureField(result.Metadata, "title", "Untitled")
	ensureField(result.Metadata, "date", "")
	ensureField(result.Metadata, "draft", false)

	// Add computed fields
	result.Metadata["processed"] = true

	// Add source file path
	sourcePathField := "source_file"
	if field, ok := input.Config["source_path_field"]; ok {
		if fieldStr, ok := field.(string); ok {
			sourcePathField = fieldStr
		}
	}
	result.Metadata[sourcePathField] = input.FilePath

	return result
}

// ensureField sets a field in metadata if it doesn't exist.
func ensureField(metadata map[string]interface{}, key string, defaultValue interface{}) {
	if _, exists := metadata[key]; !exists {
		metadata[key] = defaultValue
	}
}

func init() {
	// Register the plugin in the global plugin registry
	if err := plugin.Register(NewFrontmatterTransform()); err != nil {
		// Log error but don't panic during init
		_ = err
	}
}
