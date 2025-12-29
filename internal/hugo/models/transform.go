package models

import (
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TransformContext provides strongly-typed context for transformations.
// This replaces the interface{} approach with compile-time type safety.
type TransformContext struct {
	// Generator access for configuration and utilities
	Generator GeneratorProvider

	// Timing information
	StartTime time.Time

	// Transformation metadata
	Source     string                 // Name of the transformer
	Priority   int                    // Execution priority
	Properties map[string]interface{} // Custom transformer properties
}

// GeneratorProvider provides access to generator functionality without import cycles.
type GeneratorProvider interface {
	// Configuration access
	GetConfig() ConfigProvider

	// Resolver access
	GetEditLinkResolver() EditLinkResolver

	// Forge information (theme capabilities removed - Relearn always wants per-page edit links)
	GetForgeCapabilities(forgeType string) ForgeCapabilities
}

// ConfigProvider provides type-safe access to configuration.
type ConfigProvider interface {
	GetHugoConfig() HugoConfig
	GetForgeConfig() ForgeConfig
	GetTransformConfig() TransformConfig
}

// HugoConfig represents Hugo-specific configuration.
type HugoConfig struct {
	ThemeType      string
	BaseURL        string
	Title          string
	EnableMarkdown bool
}

// ForgeConfig represents forge-specific configuration.
type ForgeConfig struct {
	DefaultForge string
	EditLinks    bool
}

// TransformConfig represents transformation pipeline configuration.
type TransformConfig struct {
	EnabledTransforms  []string
	DisabledTransforms []string
	Properties         map[string]interface{}
}

// EditLinkResolver provides type-safe edit link resolution.
type EditLinkResolver interface {
	Resolve(file docs.DocFile) string
	SupportsFile(file docs.DocFile) bool
}

// ForgeCapabilities represents forge-specific capabilities.
type ForgeCapabilities struct {
	SupportsEditLinks bool
	SupportsWebhooks  bool
	APIAvailable      bool
}

// ContentTransformation represents a single transformation operation.
type ContentTransformation struct {
	// Identification
	Name        string
	Description string
	Version     string

	// Execution metadata
	Priority int
	Enabled  bool

	// Dependencies
	RequiredBefore []string
	RequiredAfter  []string

	// Configuration
	Config map[string]interface{}
}

// TransformationResult represents the result of a transformation.
type TransformationResult struct {
	// Success status
	Success bool
	Error   error
	Source  string // Name of the transformer that produced this result

	// Changes made
	ContentModified     bool
	FrontMatterModified bool
	MetadataModified    bool

	// Performance metrics
	Duration time.Duration

	// Detailed changes
	Changes []ChangeRecord
}

// ChangeRecord documents a specific change made during transformation.
type ChangeRecord struct {
	Type      ChangeType
	Field     string
	OldValue  interface{}
	NewValue  interface{}
	Reason    string
	Source    string
	Timestamp time.Time
}

// ChangeType enumerates the types of changes that can be made.
type ChangeType int

const (
	ChangeTypeContentModified ChangeType = iota
	ChangeTypeFrontMatterAdded
	ChangeTypeFrontMatterModified
	ChangeTypeFrontMatterRemoved
	ChangeTypeMetadataAdded
	ChangeTypeMetadataModified
	ChangeTypeMetadataRemoved
	ChangeTypeStructureModified
)

// String returns a string representation of the ChangeType.
func (ct ChangeType) String() string {
	switch ct {
	case ChangeTypeContentModified:
		return "content_modified"
	case ChangeTypeFrontMatterAdded:
		return "front_matter_added"
	case ChangeTypeFrontMatterModified:
		return "front_matter_modified"
	case ChangeTypeFrontMatterRemoved:
		return "front_matter_removed"
	case ChangeTypeMetadataAdded:
		return "metadata_added"
	case ChangeTypeMetadataModified:
		return "metadata_modified"
	case ChangeTypeMetadataRemoved:
		return "metadata_removed"
	case ChangeTypeStructureModified:
		return "structure_modified"
	default:
		return "unknown"
	}
}

// TransformationPipeline represents a complete transformation pipeline.
type TransformationPipeline struct {
	// Identification
	Name        string
	Description string
	Version     string

	// Configuration
	Transformations []ContentTransformation
	Context         *TransformContext

	// Execution state
	Started   bool
	Completed bool
	Failed    bool

	// Results
	Results []TransformationResult

	// Performance
	TotalDuration time.Duration
	StartTime     time.Time
	EndTime       time.Time
}

// NewTransformContext creates a new transform context with the given provider.
func NewTransformContext(provider GeneratorProvider) *TransformContext {
	return &TransformContext{
		Generator:  provider,
		StartTime:  time.Now(),
		Properties: make(map[string]interface{}),
	}
}

// WithSource sets the source transformer name.
func (tc *TransformContext) WithSource(source string) *TransformContext {
	tc.Source = source
	return tc
}

// WithPriority sets the execution priority.
func (tc *TransformContext) WithPriority(priority int) *TransformContext {
	tc.Priority = priority
	return tc
}

// WithProperty sets a custom property.
func (tc *TransformContext) WithProperty(key string, value interface{}) *TransformContext {
	if tc.Properties == nil {
		tc.Properties = make(map[string]interface{})
	}
	tc.Properties[key] = value
	return tc
}

// GetProperty retrieves a custom property.
func (tc *TransformContext) GetProperty(key string) (interface{}, bool) {
	if tc.Properties == nil {
		return nil, false
	}
	value, exists := tc.Properties[key]
	return value, exists
}

// GetPropertyString retrieves a custom property as a string.
func (tc *TransformContext) GetPropertyString(key string) (string, bool) {
	value, exists := tc.GetProperty(key)
	if !exists {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

// GetPropertyInt retrieves a custom property as an integer.
func (tc *TransformContext) GetPropertyInt(key string) (int, bool) {
	value, exists := tc.GetProperty(key)
	if !exists {
		return 0, false
	}
	if i, ok := value.(int); ok {
		return i, true
	}
	return 0, false
}

// GetPropertyBool retrieves a custom property as a boolean.
func (tc *TransformContext) GetPropertyBool(key string) (bool, bool) {
	value, exists := tc.GetProperty(key)
	if !exists {
		return false, false
	}
	if b, ok := value.(bool); ok {
		return b, true
	}
	return false, false
}

// NewTransformationResult creates a new transformation result.
func NewTransformationResult() *TransformationResult {
	return &TransformationResult{
		Changes: make([]ChangeRecord, 0),
	}
}

// SetSuccess marks the transformation as successful.
func (tr *TransformationResult) SetSuccess() *TransformationResult {
	tr.Success = true
	tr.Error = nil
	return tr
}

// SetError marks the transformation as failed with the given error.
func (tr *TransformationResult) SetError(err error) *TransformationResult {
	tr.Success = false
	tr.Error = err
	return tr
}

// SetSource sets the source transformer name for this result.
func (tr *TransformationResult) SetSource(source string) *TransformationResult {
	tr.Source = source
	return tr
}

// AddChange records a change made during transformation.
func (tr *TransformationResult) AddChange(changeType ChangeType, field string, oldValue, newValue interface{}, reason, source string) *TransformationResult {
	change := ChangeRecord{
		Type:      changeType,
		Field:     field,
		OldValue:  oldValue,
		NewValue:  newValue,
		Reason:    reason,
		Source:    source,
		Timestamp: time.Now(),
	}
	tr.Changes = append(tr.Changes, change)

	// Update modification flags
	switch changeType {
	case ChangeTypeContentModified:
		tr.ContentModified = true
	case ChangeTypeFrontMatterAdded, ChangeTypeFrontMatterModified, ChangeTypeFrontMatterRemoved:
		tr.FrontMatterModified = true
	case ChangeTypeMetadataAdded, ChangeTypeMetadataModified, ChangeTypeMetadataRemoved:
		tr.MetadataModified = true
	}

	return tr
}

// SetDuration sets the transformation duration.
func (tr *TransformationResult) SetDuration(duration time.Duration) *TransformationResult {
	tr.Duration = duration
	return tr
}

// HasChanges returns true if any changes were recorded.
func (tr *TransformationResult) HasChanges() bool {
	return len(tr.Changes) > 0
}

// GetChangesByType returns changes of a specific type.
func (tr *TransformationResult) GetChangesByType(changeType ChangeType) []ChangeRecord {
	var result []ChangeRecord
	for _, change := range tr.Changes {
		if change.Type == changeType {
			result = append(result, change)
		}
	}
	return result
}

// GetChangesByField returns changes for a specific field.
func (tr *TransformationResult) GetChangesByField(field string) []ChangeRecord {
	var result []ChangeRecord
	for _, change := range tr.Changes {
		if change.Field == field {
			result = append(result, change)
		}
	}
	return result
}

// NewTransformationPipeline creates a new transformation pipeline.
func NewTransformationPipeline(name, description, version string) *TransformationPipeline {
	return &TransformationPipeline{
		Name:            name,
		Description:     description,
		Version:         version,
		Transformations: make([]ContentTransformation, 0),
		Results:         make([]TransformationResult, 0),
	}
}

// AddTransformation adds a transformation to the pipeline.
func (tp *TransformationPipeline) AddTransformation(transformation ContentTransformation) *TransformationPipeline {
	tp.Transformations = append(tp.Transformations, transformation)
	return tp
}

// SetContext sets the transform context for the pipeline.
func (tp *TransformationPipeline) SetContext(context *TransformContext) *TransformationPipeline {
	tp.Context = context
	return tp
}

// Start marks the pipeline as started.
func (tp *TransformationPipeline) Start() *TransformationPipeline {
	tp.Started = true
	tp.StartTime = time.Now()
	return tp
}

// Complete marks the pipeline as completed.
func (tp *TransformationPipeline) Complete() *TransformationPipeline {
	tp.Completed = true
	tp.EndTime = time.Now()
	tp.TotalDuration = tp.EndTime.Sub(tp.StartTime)
	return tp
}

// Fail marks the pipeline as failed.
func (tp *TransformationPipeline) Fail() *TransformationPipeline {
	tp.Failed = true
	tp.EndTime = time.Now()
	tp.TotalDuration = tp.EndTime.Sub(tp.StartTime)
	return tp
}

// AddResult adds a transformation result to the pipeline.
func (tp *TransformationPipeline) AddResult(result TransformationResult) *TransformationPipeline {
	tp.Results = append(tp.Results, result)
	return tp
}

// IsRunning returns true if the pipeline is currently running.
func (tp *TransformationPipeline) IsRunning() bool {
	return tp.Started && !tp.Completed && !tp.Failed
}

// IsComplete returns true if the pipeline completed successfully.
func (tp *TransformationPipeline) IsComplete() bool {
	return tp.Completed && !tp.Failed
}

// IsFailed returns true if the pipeline failed.
func (tp *TransformationPipeline) IsFailed() bool {
	return tp.Failed
}

// GetSuccessfulResults returns only successful transformation results.
func (tp *TransformationPipeline) GetSuccessfulResults() []TransformationResult {
	var results []TransformationResult
	for _, result := range tp.Results {
		if result.Success {
			results = append(results, result)
		}
	}
	return results
}

// GetFailedResults returns only failed transformation results.
func (tp *TransformationPipeline) GetFailedResults() []TransformationResult {
	var results []TransformationResult
	for _, result := range tp.Results {
		if !result.Success {
			results = append(results, result)
		}
	}
	return results
}

// GetTotalChanges returns the total number of changes across all results.
func (tp *TransformationPipeline) GetTotalChanges() int {
	total := 0
	for _, result := range tp.Results {
		total += len(result.Changes)
	}
	return total
}
