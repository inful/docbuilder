package models

import (
	"errors"
	"fmt"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// TransformStage represents a major phase in the typed transformation pipeline.
// This aligns with the core transforms stage model.
type TransformStage string

const (
	StageParse     TransformStage = "parse"     // Extract/parse source content
	StageBuild     TransformStage = "build"     // Generate base metadata
	StageEnrich    TransformStage = "enrich"    // Add computed fields
	StageMerge     TransformStage = "merge"     // Combine/merge data
	StageTransform TransformStage = "transform" // Modify content
	StageFinalize  TransformStage = "finalize"  // Post-process
	StageSerialize TransformStage = "serialize" // Output generation
)

// ContentPage represents a strongly-typed page being transformed.
// This replaces the interface{} approach with compile-time type safety.
type ContentPage struct {
	// File information
	File docs.DocFile

	// Content
	Content  string
	RawBytes []byte

	// Front matter
	FrontMatter         *FrontMatter
	OriginalFrontMatter *FrontMatter
	FrontMatterPatches  []*FrontMatterPatch

	// State tracking
	HadOriginalFrontMatter bool
	ContentModified        bool
	FrontMatterModified    bool

	// Transformation tracking
	TransformationHistory []TransformationRecord
	Conflicts             []FrontMatterConflict

	// Metadata
	ProcessingStartTime time.Time
	LastModified        time.Time
}

// TransformationRecord tracks individual transformation operations.
type TransformationRecord struct {
	Transformer string
	Priority    int
	Timestamp   time.Time
	Duration    time.Duration
	Success     bool
	Error       error
	Changes     []ChangeRecord
}

// FrontMatterConflict describes merge decisions for auditing.
type FrontMatterConflict struct {
	Key      string
	Original interface{}
	Attempt  interface{}
	Source   string
	Action   string // kept_original | overwritten | set_if_missing
}

// TypedTransformer defines a strongly-typed content transformation interface.
// This replaces the reflection-based PageAdapter approach.
type TypedTransformer interface {
	// Identity
	Name() string
	Description() string
	Version() string

	// Execution
	Transform(page *ContentPage, context *TransformContext) (*TransformationResult, error)

	// Metadata
	Priority() int // DEPRECATED: Use Dependencies().MustRunAfter/MustRunBefore instead
	Stage() TransformStage
	Dependencies() TransformerDependencies
	Configuration() TransformerConfiguration

	// Capabilities
	CanTransform(page *ContentPage, context *TransformContext) bool
	RequiredContext() []string
}

// TransformerDependencies defines what a transformer needs to run properly.
type TransformerDependencies struct {
	// Order dependencies (aligned with core transforms pattern)
	MustRunAfter  []string // Must run after these transformers
	MustRunBefore []string // Must run before these transformers

	// Legacy order dependencies (DEPRECATED)
	RequiredBefore []string // DEPRECATED: Use MustRunBefore instead
	RequiredAfter  []string // DEPRECATED: Use MustRunAfter instead

	// Feature dependencies
	RequiresOriginalFrontMatter bool
	RequiresFrontMatterPatches  bool
	RequiresContent             bool
	RequiresFileMetadata        bool

	// Context dependencies
	RequiresConfig           bool
	RequiresEditLinkResolver bool
	RequiresThemeInfo        bool
	RequiresForgeInfo        bool
}

// TransformerConfiguration defines transformer-specific configuration.
type TransformerConfiguration struct {
	// Basic settings
	Enabled  bool
	Priority int

	// Behavior configuration
	SkipIfEmpty   bool
	FailOnError   bool
	RecordChanges bool

	// Feature flags
	EnableDeepMerge     bool
	PreservesOriginal   bool
	ModifiesContent     bool
	ModifiesFrontMatter bool

	// Custom configuration
	Properties map[string]interface{}
}

// TypedTransformerRegistry provides a strongly-typed transformer registry.
type TypedTransformerRegistry struct {
	transformers map[string]TypedTransformer
	order        []string
}

// NewTypedTransformerRegistry creates a new typed transformer registry.
func NewTypedTransformerRegistry() *TypedTransformerRegistry {
	return &TypedTransformerRegistry{
		transformers: make(map[string]TypedTransformer),
		order:        make([]string, 0),
	}
}

// Register adds a transformer to the registry.
func (r *TypedTransformerRegistry) Register(transformer TypedTransformer) error {
	if transformer == nil {
		return errors.New("transformer cannot be nil")
	}

	name := transformer.Name()
	if name == "" {
		return errors.New("transformer name cannot be empty")
	}

	// Check for conflicts
	if _, exists := r.transformers[name]; exists {
		return fmt.Errorf("transformer with name %q already registered", name)
	}

	r.transformers[name] = transformer
	r.order = append(r.order, name)

	return nil
}

// Get retrieves a transformer by name.
func (r *TypedTransformerRegistry) Get(name string) (TypedTransformer, bool) {
	transformer, exists := r.transformers[name]
	return transformer, exists
}

// List returns all registered transformers in registration order.
func (r *TypedTransformerRegistry) List() []TypedTransformer {
	result := make([]TypedTransformer, 0, len(r.order))
	for _, name := range r.order {
		if transformer, exists := r.transformers[name]; exists {
			result = append(result, transformer)
		}
	}
	return result
}

// ListByPriority returns transformers sorted by priority.
// DEPRECATED: Use ListByDependencies() for dependency-based ordering.
func (r *TypedTransformerRegistry) ListByPriority() []TypedTransformer {
	transformers := r.List()

	// Sort by priority (lower runs first), then by name for stability
	for i := range len(transformers) - 1 {
		for j := i + 1; j < len(transformers); j++ {
			iPriority := transformers[i].Priority()
			jPriority := transformers[j].Priority()

			if iPriority > jPriority ||
				(iPriority == jPriority && transformers[i].Name() > transformers[j].Name()) {
				transformers[i], transformers[j] = transformers[j], transformers[i]
			}
		}
	}

	return transformers
}

// ListByDependencies returns transformers sorted by stage and dependencies.
func (r *TypedTransformerRegistry) ListByDependencies() ([]TypedTransformer, error) {
	transformers := r.List()
	return buildTypedPipeline(transformers)
}

// buildTypedPipeline constructs execution order using stages and dependencies.
func buildTypedPipeline(transformers []TypedTransformer) ([]TypedTransformer, error) {
	// Stage order for typed transformers (matches core transforms)
	stageOrder := []TransformStage{
		StageParse,
		StageBuild,
		StageEnrich,
		StageMerge,
		StageTransform,
		StageFinalize,
		StageSerialize,
	}

	// Group by stage
	byStage := make(map[TransformStage][]TypedTransformer)
	for _, t := range transformers {
		stage := t.Stage()
		byStage[stage] = append(byStage[stage], t)
	}

	// Sort each stage by dependencies using topological sort
	var result []TypedTransformer
	for _, stage := range stageOrder {
		stageTransforms, exists := byStage[stage]
		if !exists {
			continue
		}

		sorted, err := topologicalSortTyped(stageTransforms)
		if err != nil {
			return nil, fmt.Errorf("stage %s: %w", stage, err)
		}

		result = append(result, sorted...)
	}

	return result, nil
}

// topologicalSortTyped performs dependency resolution for typed transformers.
func topologicalSortTyped(transformers []TypedTransformer) ([]TypedTransformer, error) {
	if len(transformers) == 0 {
		return transformers, nil
	}

	// Build name -> transform map
	byName := make(map[string]TypedTransformer)
	for _, t := range transformers {
		byName[t.Name()] = t
	}

	// Build adjacency list (dependencies graph)
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, t := range transformers {
		name := t.Name()
		deps := t.Dependencies()

		if _, exists := graph[name]; !exists {
			graph[name] = []string{}
		}

		// Handle new MustRunAfter dependencies
		for _, dep := range deps.MustRunAfter {
			if _, exists := byName[dep]; exists {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
			// Skip if dependency not in this stage
		}

		// Handle new MustRunBefore dependencies
		for _, after := range deps.MustRunBefore {
			if _, exists := byName[after]; exists {
				graph[name] = append(graph[name], after)
				inDegree[after]++
			}
			// Skip if dependency not in this stage
		}

		// Handle legacy RequiredAfter (maps to MustRunAfter)
		for _, dep := range deps.RequiredAfter {
			if _, exists := byName[dep]; exists {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
		}

		// Handle legacy RequiredBefore (maps to MustRunBefore)
		for _, after := range deps.RequiredBefore {
			if _, exists := byName[after]; exists {
				graph[name] = append(graph[name], after)
				inDegree[after]++
			}
		}
	}

	// Kahn's algorithm for topological sort
	var queue []string
	for _, t := range transformers {
		name := t.Name()
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	// Keep deterministic ordering
	sortStrings(queue)

	var result []TypedTransformer
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]

		result = append(result, byName[current])

		// Process neighbors
		neighbors := graph[current]
		sortStrings(neighbors)

		for _, neighbor := range neighbors {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				sortStrings(queue)
			}
		}
	}

	// Check for cycles
	if len(result) != len(transformers) {
		return nil, errors.New("circular dependency detected in typed transformers")
	}

	return result, nil
}

// sortStrings sorts a string slice in-place for deterministic ordering.
func sortStrings(s []string) {
	for i := range len(s) - 1 {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// BuildExecutionPlan creates an execution plan with dependency resolution.
func (r *TypedTransformerRegistry) BuildExecutionPlan(filter []string) ([]TypedTransformer, error) {
	// Use dependency-based ordering (V2)
	available, err := r.ListByDependencies()
	if err != nil {
		return nil, fmt.Errorf("failed to build execution plan: %w", err)
	}

	// Apply filter if provided
	if len(filter) > 0 {
		filterSet := make(map[string]bool)
		for _, name := range filter {
			filterSet[name] = true
		}

		var filtered []TypedTransformer
		for _, transformer := range available {
			if filterSet[transformer.Name()] {
				filtered = append(filtered, transformer)
			}
		}
		available = filtered
	}

	return available, nil
}

// ContentPage methods

// NewContentPage creates a new content page from a doc file.
func NewContentPage(file docs.DocFile) *ContentPage {
	return &ContentPage{
		File:                  file,
		FrontMatter:           NewFrontMatter(),
		TransformationHistory: make([]TransformationRecord, 0),
		Conflicts:             make([]FrontMatterConflict, 0),
		ProcessingStartTime:   time.Now(),
		LastModified:          time.Now(),
	}
}

// SetContent updates the page content and marks it as modified.
func (p *ContentPage) SetContent(content string) {
	if p.Content != content {
		p.Content = content
		p.ContentModified = true
		p.LastModified = time.Now()
	}
}

// GetContent returns the current page content.
func (p *ContentPage) GetContent() string {
	return p.Content
}

// SetFrontMatter updates the page front matter and marks it as modified.
func (p *ContentPage) SetFrontMatter(fm *FrontMatter) {
	if fm != nil && !p.frontMatterEqual(p.FrontMatter, fm) {
		p.FrontMatter = fm
		p.FrontMatterModified = true
		p.LastModified = time.Now()
	}
}

// GetFrontMatter returns the current front matter.
func (p *ContentPage) GetFrontMatter() *FrontMatter {
	return p.FrontMatter
}

// SetOriginalFrontMatter sets the original front matter (immutable baseline).
func (p *ContentPage) SetOriginalFrontMatter(fm *FrontMatter, had bool) {
	p.OriginalFrontMatter = fm
	p.HadOriginalFrontMatter = had
}

// GetOriginalFrontMatter returns the original front matter.
func (p *ContentPage) GetOriginalFrontMatter() *FrontMatter {
	return p.OriginalFrontMatter
}

// AddFrontMatterPatch adds a front matter patch.
func (p *ContentPage) AddFrontMatterPatch(patch *FrontMatterPatch) {
	if patch != nil {
		p.FrontMatterPatches = append(p.FrontMatterPatches, patch)
		p.FrontMatterModified = true
		p.LastModified = time.Now()
	}
}

// ApplyFrontMatterPatches applies all patches to create the final front matter.
func (p *ContentPage) ApplyFrontMatterPatches() error {
	if p.OriginalFrontMatter == nil {
		p.OriginalFrontMatter = NewFrontMatter()
	}

	result := p.OriginalFrontMatter.Clone()

	for i, patch := range p.FrontMatterPatches {
		applied, err := patch.Apply(result)
		if err != nil {
			return fmt.Errorf("failed to apply patch %d: %w", i, err)
		}
		result = applied
	}

	p.SetFrontMatter(result)
	return nil
}

// AddTransformationRecord records a transformation operation.
func (p *ContentPage) AddTransformationRecord(record TransformationRecord) {
	p.TransformationHistory = append(p.TransformationHistory, record)
}

// GetTransformationHistory returns the transformation history.
func (p *ContentPage) GetTransformationHistory() []TransformationRecord {
	return p.TransformationHistory
}

// HasBeenTransformed returns true if any transformations have been applied.
func (p *ContentPage) HasBeenTransformed() bool {
	return len(p.TransformationHistory) > 0
}

// IsModified returns true if the page has been modified.
func (p *ContentPage) IsModified() bool {
	return p.ContentModified || p.FrontMatterModified
}

// Serialize converts the page to its final byte representation.
func (p *ContentPage) Serialize() ([]byte, error) {
	if p.FrontMatter == nil {
		// Content only, no front matter
		return []byte(p.Content), nil
	}

	// Serialize front matter to YAML
	frontMatterMap := p.FrontMatter.ToMap()
	if len(frontMatterMap) == 0 {
		// No front matter to serialize
		return []byte(p.Content), nil
	}

	// Note: YAML serialization of front matter is intentionally deferred.
	// Current behavior: content-only output; front matter is preserved in
	// typed structures for downstream generators.
	return []byte(p.Content), nil
}

// Clone creates a deep copy of the content page.
func (p *ContentPage) Clone() *ContentPage {
	clone := &ContentPage{
		File:                   p.File,
		Content:                p.Content,
		HadOriginalFrontMatter: p.HadOriginalFrontMatter,
		ContentModified:        p.ContentModified,
		FrontMatterModified:    p.FrontMatterModified,
		ProcessingStartTime:    p.ProcessingStartTime,
		LastModified:           p.LastModified,
	}

	// Deep copy byte slice
	if p.RawBytes != nil {
		clone.RawBytes = make([]byte, len(p.RawBytes))
		copy(clone.RawBytes, p.RawBytes)
	}

	// Clone front matter
	if p.FrontMatter != nil {
		clone.FrontMatter = p.FrontMatter.Clone()
	}
	if p.OriginalFrontMatter != nil {
		clone.OriginalFrontMatter = p.OriginalFrontMatter.Clone()
	}

	// Clone patches
	if p.FrontMatterPatches != nil {
		clone.FrontMatterPatches = make([]*FrontMatterPatch, len(p.FrontMatterPatches))
		for i, patch := range p.FrontMatterPatches {
			if patch != nil {
				clonedPatch := *patch // Shallow copy for now
				clone.FrontMatterPatches[i] = &clonedPatch
			}
		}
	}

	// Clone transformation history
	if p.TransformationHistory != nil {
		clone.TransformationHistory = make([]TransformationRecord, len(p.TransformationHistory))
		copy(clone.TransformationHistory, p.TransformationHistory)
	}

	// Clone conflicts
	if p.Conflicts != nil {
		clone.Conflicts = make([]FrontMatterConflict, len(p.Conflicts))
		copy(clone.Conflicts, p.Conflicts)
	}

	return clone
}

// Validate performs basic validation of the content page.
func (p *ContentPage) Validate() error {
	if p.File.Path == "" {
		return errors.New("file path is required")
	}

	if p.FrontMatter != nil {
		if err := p.FrontMatter.Validate(); err != nil {
			return fmt.Errorf("front matter validation failed: %w", err)
		}
	}

	return nil
}

// Helper methods

// frontMatterEqual compares two front matter objects for equality.
func (p *ContentPage) frontMatterEqual(a, b *FrontMatter) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare key fields (simplified for now)
	return a.Title == b.Title &&
		a.Repository == b.Repository &&
		a.Section == b.Section
}
