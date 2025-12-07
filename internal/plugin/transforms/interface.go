package transforms

import (
	"fmt"

	"git.home.luguber.info/inful/docbuilder/internal/plugin"
)

// TransformStage represents when a transform should be applied.
type TransformStage string

const (
	// StagePreProcess: Applied before markdown parsing
	StagePreProcess TransformStage = "preprocess"

	// StagePostProcess: Applied after markdown parsing
	StagePostProcess TransformStage = "postprocess"

	// StageFrontmatter: Applied to frontmatter/metadata only
	StageFrontmatter TransformStage = "frontmatter"

	// StageContent: Applied to content only
	StageContent TransformStage = "content"
)

// TransformResult represents the result of a transform operation.
type TransformResult struct {
	// Content is the transformed content
	Content []byte

	// Metadata is any metadata to update/add
	Metadata map[string]interface{}

	// Skipped indicates if the transform was skipped
	Skipped bool

	// Error is any error that occurred
	Error error
}

// TransformInput represents input to a transform.
type TransformInput struct {
	// FilePath is the source file path
	FilePath string

	// Content is the file content
	Content []byte

	// Metadata is the parsed frontmatter/metadata
	Metadata map[string]interface{}

	// Config is the transform-specific configuration
	Config map[string]interface{}
}

// TransformPlugin extends the base Plugin interface with transform-specific methods.
type TransformPlugin interface {
	plugin.Plugin

	// Stage returns when this transform should be applied.
	Stage() TransformStage

	// ShouldApply returns true if the transform should apply to the given input.
	ShouldApply(input *TransformInput) bool

	// Apply executes the transform on the input.
	// Returns a TransformResult with the transformed content and any metadata updates.
	Apply(input *TransformInput) *TransformResult

	// Order returns the execution order (lower values execute first).
	// Default is 0. Use negative for pre-transforms, positive for post-transforms.
	Order() int
}

// BaseTransformPlugin provides default implementations for transform lifecycle methods.
type BaseTransformPlugin struct {
	plugin.BasePlugin
}

// Order returns default execution order (0).
func (b *BaseTransformPlugin) Order() int {
	return 0
}

// ShouldApply returns true by default (applies to all files).
func (b *BaseTransformPlugin) ShouldApply(input *TransformInput) bool {
	return true
}

// TransformRegistry manages transform plugin registration and execution.
type TransformRegistry struct {
	transforms []TransformPlugin
}

// NewTransformRegistry creates a new transform registry.
func NewTransformRegistry() *TransformRegistry {
	return &TransformRegistry{
		transforms: make([]TransformPlugin, 0),
	}
}

// Register adds a transform plugin to the registry.
func (r *TransformRegistry) Register(transform TransformPlugin) error {
	if transform == nil {
		return fmt.Errorf("cannot register nil transform")
	}

	metadata := transform.Metadata()
	if err := metadata.Validate(); err != nil {
		return fmt.Errorf("invalid transform metadata: %w", err)
	}

	// Verify it's a transform type
	if metadata.Type != plugin.PluginTypeTransform {
		return fmt.Errorf("plugin %s has type %s, expected %s", metadata.Name, metadata.Type, plugin.PluginTypeTransform)
	}

	r.transforms = append(r.transforms, transform)
	return nil
}

// ApplyToContent applies all registered transforms to content.
// Transforms are applied in order according to their Order() value.
func (r *TransformRegistry) ApplyToContent(input *TransformInput) (*TransformResult, error) {
	result := &TransformResult{
		Content:  input.Content,
		Metadata: input.Metadata,
	}

	// Sort transforms by order (not implemented here, would need sort.Slice)
	for _, transform := range r.transforms {
		if !transform.ShouldApply(input) {
			continue
		}

		input.Content = result.Content
		input.Metadata = result.Metadata

		trResult := transform.Apply(input)
		if trResult.Error != nil {
			return nil, fmt.Errorf("transform %s failed: %w", transform.Metadata().Name, trResult.Error)
		}

		if !trResult.Skipped {
			result.Content = trResult.Content
			if trResult.Metadata != nil {
				for k, v := range trResult.Metadata {
					result.Metadata[k] = v
				}
			}
		}
	}

	return result, nil
}

// List returns all registered transforms.
func (r *TransformRegistry) List() []TransformPlugin {
	return r.transforms
}

// Count returns the number of registered transforms.
func (r *TransformRegistry) Count() int {
	return len(r.transforms)
}

// Clear removes all transforms from the registry.
func (r *TransformRegistry) Clear() {
	r.transforms = make([]TransformPlugin, 0)
}
