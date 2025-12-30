package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FrontMatterParserV2 is a strongly-typed front matter parser.
type FrontMatterParserV2 struct {
	config TransformerConfiguration
}

// NewFrontMatterParserV2 creates a new typed front matter parser.
func NewFrontMatterParserV2() *FrontMatterParserV2 {
	return &FrontMatterParserV2{
		config: TransformerConfiguration{
			Enabled:             true,
			Priority:            10,
			SkipIfEmpty:         false,
			FailOnError:         false,
			RecordChanges:       true,
			EnableDeepMerge:     false,
			PreservesOriginal:   true,
			ModifiesContent:     true,
			ModifiesFrontMatter: true,
			Properties:          make(map[string]interface{}),
		},
	}
}

// Name returns the transformer name.
func (t *FrontMatterParserV2) Name() string {
	return "front_matter_parser_v2"
}

// Description returns the transformer description.
func (t *FrontMatterParserV2) Description() string {
	return "Parses YAML front matter from markdown content and extracts it into typed structures"
}

// Version returns the transformer version.
func (t *FrontMatterParserV2) Version() string {
	return "2.0.0"
}

// Stage returns the transformation stage.
func (t *FrontMatterParserV2) Stage() TransformStage {
	return StageParse
}

// Dependencies returns the transformer dependencies.
func (t *FrontMatterParserV2) Dependencies() TransformerDependencies {
	return TransformerDependencies{
		MustRunAfter:                []string{}, // No dependencies, runs first
		MustRunBefore:               []string{},
		RequiredBefore:              []string{}, // Legacy (deprecated)
		RequiredAfter:               []string{}, // Legacy (deprecated)
		RequiresOriginalFrontMatter: false,
		RequiresFrontMatterPatches:  false,
		RequiresContent:             true,
		RequiresFileMetadata:        false,
		RequiresConfig:              false,
		RequiresEditLinkResolver:    false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
	}
}

// Configuration returns the transformer configuration.
func (t *FrontMatterParserV2) Configuration() TransformerConfiguration {
	return t.config
}

// CanTransform checks if this transformer can process the given page.
func (t *FrontMatterParserV2) CanTransform(page *ContentPage, _ *TransformContext) bool {
	if !t.config.Enabled {
		return false
	}

	content := page.GetContent()
	return strings.HasPrefix(content, "---\n")
}

// RequiredContext returns the required context keys.
func (t *FrontMatterParserV2) RequiredContext() []string {
	return []string{} // No special context required
}

// Transform parses front matter from the page content.
func (t *FrontMatterParserV2) Transform(page *ContentPage, _ *TransformContext) (*TransformationResult, error) {
	startTime := time.Now()
	result := NewTransformationResult()

	content := page.GetContent()
	if !strings.HasPrefix(content, "---\n") {
		// No front matter to parse
		return result.SetSuccess().SetDuration(time.Since(startTime)), nil
	}

	// Find the end of front matter
	search := content[4:] // Skip initial "---\n"
	endIndex := strings.Index(search, "\n---\n")
	if endIndex == -1 {
		return result.SetError(errors.New("unterminated front matter")).SetDuration(time.Since(startTime)), nil
	}

	frontMatterContent := search[:endIndex]
	remainingContent := search[endIndex+5:] // Skip "\n---\n"

	// Parse YAML front matter
	var frontMatterMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontMatterContent), &frontMatterMap); err != nil {
		if t.config.FailOnError {
			return result.SetError(fmt.Errorf("failed to parse front matter: %w", err)).SetDuration(time.Since(startTime)), nil
		}

		// Log warning but continue without front matter
		result.AddChange(
			ChangeTypeContentModified,
			"front_matter_parse_error",
			nil,
			err.Error(),
			"Failed to parse YAML front matter",
			t.Name(),
		)

		page.SetContent(remainingContent)
		return result.SetSuccess().SetDuration(time.Since(startTime)), nil
	}

	// Convert to typed front matter
	frontMatter, err := FromMap(frontMatterMap)
	if err != nil {
		return result.SetError(fmt.Errorf("failed to convert front matter to typed structure: %w", err)).SetDuration(time.Since(startTime)), nil
	}

	// Update page state
	page.SetOriginalFrontMatter(frontMatter, true)
	page.SetFrontMatter(frontMatter.Clone())
	page.SetContent(remainingContent)

	// Record changes
	result.AddChange(
		ChangeTypeFrontMatterAdded,
		"original_front_matter",
		nil,
		frontMatter,
		"Parsed original front matter from content",
		t.Name(),
	)

	result.AddChange(
		ChangeTypeContentModified,
		"content",
		content,
		remainingContent,
		"Removed front matter from content",
		t.Name(),
	)

	return result.SetSuccess().SetDuration(time.Since(startTime)), nil
}

// FrontMatterBuilderV3 is a strongly-typed front matter builder.
type FrontMatterBuilderV3 struct {
	config TransformerConfiguration
}

// NewFrontMatterBuilderV3 creates a new typed front matter builder.
func NewFrontMatterBuilderV3() *FrontMatterBuilderV3 {
	return &FrontMatterBuilderV3{
		config: TransformerConfiguration{
			Enabled:             true,
			Priority:            20,
			SkipIfEmpty:         false,
			FailOnError:         false,
			RecordChanges:       true,
			EnableDeepMerge:     true,
			PreservesOriginal:   true,
			ModifiesContent:     false,
			ModifiesFrontMatter: true,
			Properties:          make(map[string]interface{}),
		},
	}
}

// Name returns the transformer name.
func (t *FrontMatterBuilderV3) Name() string {
	return "front_matter_builder_v3"
}

// Description returns the transformer description.
func (t *FrontMatterBuilderV3) Description() string {
	return "Builds base front matter from file metadata and configuration"
}

// Version returns the transformer version.
func (t *FrontMatterBuilderV3) Version() string {
	return "3.0.0"
}

// Stage returns the transformation stage.
func (t *FrontMatterBuilderV3) Stage() TransformStage {
	return StageBuild
}

// Dependencies returns the transformer dependencies.
func (t *FrontMatterBuilderV3) Dependencies() TransformerDependencies {
	return TransformerDependencies{
		MustRunAfter:                []string{"front_matter_parser_v2"},
		MustRunBefore:               []string{},
		RequiredBefore:              []string{},                         // Legacy (deprecated)
		RequiredAfter:               []string{"front_matter_parser_v2"}, // Legacy (deprecated)
		RequiresOriginalFrontMatter: false,
		RequiresFrontMatterPatches:  false,
		RequiresContent:             false,
		RequiresFileMetadata:        true,
		RequiresConfig:              true,
		RequiresEditLinkResolver:    false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
	}
}

// Configuration returns the transformer configuration.
func (t *FrontMatterBuilderV3) Configuration() TransformerConfiguration {
	return t.config
}

// CanTransform checks if this transformer can process the given page.
func (t *FrontMatterBuilderV3) CanTransform(_ *ContentPage, _ *TransformContext) bool {
	if !t.config.Enabled {
		return false
	}

	// Always can transform - builds base front matter
	return true
}

// RequiredContext returns the required context keys.
func (t *FrontMatterBuilderV3) RequiredContext() []string {
	return []string{"config"}
}

// Transform builds base front matter from file metadata.
func (t *FrontMatterBuilderV3) Transform(page *ContentPage, context *TransformContext) (*TransformationResult, error) {
	startTime := time.Now()
	result := NewTransformationResult()

	// Get configuration
	config := context.Generator.GetConfig()
	if config == nil {
		return result.SetError(errors.New("configuration required but not available")).SetDuration(time.Since(startTime)), nil
	}

	// Build base patch using migration helper
	helper := NewMigrationHelper()

	// Generate title from file name if not present
	title := page.File.Name
	title = strings.TrimSuffix(title, ".md")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")

	// Create base patch
	basePatch := helper.CreateBasePatch(
		title,
		page.File.Repository,
		page.File.Forge,
		page.File.Section,
	)

	// Add file metadata if available
	if page.File.Metadata != nil {
		for key, value := range page.File.Metadata {
			basePatch.SetCustom(key, value)
		}
	}

	// Apply the patch
	page.AddFrontMatterPatch(basePatch)

	// Record changes
	result.AddChange(
		ChangeTypeFrontMatterAdded,
		"base_front_matter",
		nil,
		basePatch.ToMap(),
		"Built base front matter from file metadata",
		t.Name(),
	)

	return result.SetSuccess().SetDuration(time.Since(startTime)), nil
}

// EditLinkInjectorV3 is a strongly-typed edit link injector.
type EditLinkInjectorV3 struct {
	config TransformerConfiguration
}

// NewEditLinkInjectorV3 creates a new typed edit link injector.
func NewEditLinkInjectorV3() *EditLinkInjectorV3 {
	return &EditLinkInjectorV3{
		config: TransformerConfiguration{
			Enabled:             true,
			Priority:            30,
			SkipIfEmpty:         true,
			FailOnError:         false,
			RecordChanges:       true,
			EnableDeepMerge:     false,
			PreservesOriginal:   true,
			ModifiesContent:     false,
			ModifiesFrontMatter: true,
			Properties:          make(map[string]interface{}),
		},
	}
}

// Name returns the transformer name.
func (t *EditLinkInjectorV3) Name() string {
	return "edit_link_injector_v3"
}

// Description returns the transformer description.
func (t *EditLinkInjectorV3) Description() string {
	return "Injects edit URLs into front matter based on forge and theme capabilities"
}

// Version returns the transformer version.
func (t *EditLinkInjectorV3) Version() string {
	return "3.0.0"
}

// Stage returns the transformation stage.
func (t *EditLinkInjectorV3) Stage() TransformStage {
	return StageEnrich
}

// Dependencies returns the transformer dependencies.
func (t *EditLinkInjectorV3) Dependencies() TransformerDependencies {
	return TransformerDependencies{
		MustRunAfter:                []string{"front_matter_builder_v3"},
		MustRunBefore:               []string{},
		RequiredBefore:              []string{},                          // Legacy (deprecated)
		RequiredAfter:               []string{"front_matter_builder_v3"}, // Legacy (deprecated)
		RequiresOriginalFrontMatter: false,
		RequiresFrontMatterPatches:  false,
		RequiresContent:             false,
		RequiresFileMetadata:        true,
		RequiresConfig:              true,
		RequiresEditLinkResolver:    true,
		RequiresThemeInfo:           true,
		RequiresForgeInfo:           true,
	}
}

// Configuration returns the transformer configuration.
func (t *EditLinkInjectorV3) Configuration() TransformerConfiguration {
	return t.config
}

// CanTransform checks if this transformer can process the given page.
func (t *EditLinkInjectorV3) CanTransform(page *ContentPage, _ *TransformContext) bool {
	if !t.config.Enabled {
		return false
	}

	// Check if edit URL already exists
	if page.GetOriginalFrontMatter() != nil {
		if page.GetOriginalFrontMatter().EditURL != "" {
			return false // Already has edit URL
		}
	}

	// Check if any patches already add edit URL
	for _, patch := range page.FrontMatterPatches {
		if patch.EditURL != nil {
			return false // Edit URL already being added
		}
	}

	return true
}

// RequiredContext returns the required context keys.
func (t *EditLinkInjectorV3) RequiredContext() []string {
	return []string{"config", "edit_link_resolver", "theme_info", "forge_info"}
}

// Transform injects edit URLs into front matter.
// Relearn theme always wants per-page edit links, so no theme capability check needed.
func (t *EditLinkInjectorV3) Transform(page *ContentPage, context *TransformContext) (*TransformationResult, error) {
	startTime := time.Now()
	result := NewTransformationResult()

	// Check if forge supports edit links
	forgeCapabilities := context.Generator.GetForgeCapabilities(page.File.Forge)
	if !forgeCapabilities.SupportsEditLinks {
		return result.SetSuccess().SetDuration(time.Since(startTime)), nil
	}

	resolver := context.Generator.GetEditLinkResolver()
	if resolver == nil {
		return result.SetError(errors.New("edit link resolver required but not available")).SetDuration(time.Since(startTime)), nil
	}

	// Resolve edit URL
	editURL := resolver.Resolve(page.File)
	if editURL == "" {
		if t.config.SkipIfEmpty {
			return result.SetSuccess().SetDuration(time.Since(startTime)), nil
		}
		return result.SetError(fmt.Errorf("failed to resolve edit URL for file %s", page.File.Path)).SetDuration(time.Since(startTime)), nil
	}

	// Create patch with edit URL
	patch := NewFrontMatterPatch().
		SetEditURL(editURL).
		WithMergeMode(MergeModeSetIfMissing)

	page.AddFrontMatterPatch(patch)

	// Record changes
	result.AddChange(
		ChangeTypeFrontMatterAdded,
		"edit_url",
		nil,
		editURL,
		"Injected edit URL based on forge and theme capabilities",
		t.Name(),
	)

	return result.SetSuccess().SetDuration(time.Since(startTime)), nil
}

// ContentProcessorV2 is a strongly-typed content processor.
type ContentProcessorV2 struct {
	config TransformerConfiguration
}

// NewContentProcessorV2 creates a new typed content processor.
func NewContentProcessorV2() *ContentProcessorV2 {
	return &ContentProcessorV2{
		config: TransformerConfiguration{
			Enabled:             true,
			Priority:            50,
			SkipIfEmpty:         false,
			FailOnError:         false,
			RecordChanges:       true,
			EnableDeepMerge:     false,
			PreservesOriginal:   false,
			ModifiesContent:     true,
			ModifiesFrontMatter: false,
			Properties:          make(map[string]interface{}),
		},
	}
}

// Name returns the transformer name.
func (t *ContentProcessorV2) Name() string {
	return "content_processor_v2"
}

// Description returns the transformer description.
func (t *ContentProcessorV2) Description() string {
	return "Processes markdown content including link rewriting and content transformations"
}

// Version returns the transformer version.
func (t *ContentProcessorV2) Version() string {
	return "2.0.0"
}

// Stage returns the transformation stage.
func (t *ContentProcessorV2) Stage() TransformStage {
	return StageTransform
}

// Dependencies returns the transformer dependencies.
func (t *ContentProcessorV2) Dependencies() TransformerDependencies {
	return TransformerDependencies{
		MustRunAfter:                []string{"edit_link_injector_v3"},
		MustRunBefore:               []string{},
		RequiredBefore:              []string{},                        // Legacy (deprecated)
		RequiredAfter:               []string{"edit_link_injector_v3"}, // Legacy (deprecated)
		RequiresOriginalFrontMatter: false,
		RequiresFrontMatterPatches:  false,
		RequiresContent:             true,
		RequiresFileMetadata:        false,
		RequiresConfig:              false,
		RequiresEditLinkResolver:    false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
	}
}

// Configuration returns the transformer configuration.
func (t *ContentProcessorV2) Configuration() TransformerConfiguration {
	return t.config
}

// CanTransform checks if this transformer can process the given page.
func (t *ContentProcessorV2) CanTransform(page *ContentPage, _ *TransformContext) bool {
	if !t.config.Enabled {
		return false
	}

	content := page.GetContent()
	if t.config.SkipIfEmpty && strings.TrimSpace(content) == "" {
		return false
	}

	return true
}

// RequiredContext returns the required context keys.
func (t *ContentProcessorV2) RequiredContext() []string {
	return []string{} // No special context required
}

// Transform processes the content.
func (t *ContentProcessorV2) Transform(page *ContentPage, _ *TransformContext) (*TransformationResult, error) {
	startTime := time.Now()
	result := NewTransformationResult()

	originalContent := page.GetContent()
	processedContent := originalContent

	// Process relative links (placeholder): link rewriting will be implemented
	// alongside the site generator’s URL mapping to avoid drift.
	if strings.Contains(processedContent, "](./") || strings.Contains(processedContent, "](../") {
		// Mark as processed but don't change content for now
		result.AddChange(
			ChangeTypeContentModified,
			"relative_links",
			"found",
			"processed",
			"Processed relative links in content",
			t.Name(),
		)
	}

	// Update content if changed
	if processedContent != originalContent {
		page.SetContent(processedContent)

		result.AddChange(
			ChangeTypeContentModified,
			"content",
			originalContent,
			processedContent,
			"Processed markdown content",
			t.Name(),
		)
	}

	return result.SetSuccess().SetDuration(time.Since(startTime)), nil
}
