package transforms

// TransformStage represents a major phase in the transformation pipeline.
// Stages execute in the order defined by StageOrder.
type TransformStage string

const (
	// StageParse extracts and parses source content (e.g., YAML front matter extraction)
	StageParse TransformStage = "parse"

	// StageBuild generates base metadata (e.g., title, date, repository info)
	StageBuild TransformStage = "build"

	// StageEnrich adds computed fields and enrichments (e.g., edit links)
	StageEnrich TransformStage = "enrich"

	// StageMerge combines and merges data (e.g., front matter patches)
	StageMerge TransformStage = "merge"

	// StageTransform modifies content (e.g., link rewriting, heading manipulation)
	StageTransform TransformStage = "transform"

	// StageFinalize performs post-processing (e.g., shortcode escaping, theme-specific tweaks)
	StageFinalize TransformStage = "finalize"

	// StageSerialize generates output (e.g., YAML + content serialization)
	StageSerialize TransformStage = "serialize"
)

// StageOrder defines the execution order of transformation stages.
// Transforms are grouped by stage and executed in this order.
var StageOrder = []TransformStage{
	StageParse,
	StageBuild,
	StageEnrich,
	StageMerge,
	StageTransform,
	StageFinalize,
	StageSerialize,
}

// Transformer is the dependency-based interface for content transformations.
// Transforms implementing this interface declare explicit dependencies and capabilities.
type Transformer interface {
	// Name returns the unique identifier for this transformer (lowercase snake_case)
	Name() string

	// Stage returns the pipeline stage where this transform executes
	Stage() TransformStage

	// Dependencies declares ordering constraints and capability requirements
	Dependencies() TransformDependencies

	// Transform performs the actual transformation on a page
	Transform(p PageAdapter) error
}

// TransformDependencies declares explicit ordering constraints and capabilities.
// This replaces the fragile priority-based system with self-documenting dependencies.
type TransformDependencies struct {
	// MustRunAfter lists transform names that must complete before this one.
	// Use this to express direct dependencies (e.g., "I need the parser to run first").
	MustRunAfter []string

	// MustRunBefore lists transform names that must run after this one.
	// Use this when a transform must prepare data for specific consumers.
	MustRunBefore []string

	// --- Capability Flags (for documentation and validation) ---

	// RequiresOriginalFrontMatter indicates this transform needs parsed front matter
	RequiresOriginalFrontMatter bool

	// ModifiesContent indicates this transform changes the markdown content body
	ModifiesContent bool

	// ModifiesFrontMatter indicates this transform changes front matter data
	ModifiesFrontMatter bool

	// RequiresConfig indicates this transform needs access to configuration
	RequiresConfig bool

	// RequiresThemeInfo indicates this transform needs theme information
	RequiresThemeInfo bool

	// RequiresForgeInfo indicates this transform needs forge/repository information
	RequiresForgeInfo bool

	// RequiresEditLinkResolver indicates this transform needs edit link resolution capability
	RequiresEditLinkResolver bool

	// RequiresFileMetadata indicates this transform needs file metadata (path, name, etc.)
	RequiresFileMetadata bool
}

// StageIndex returns the numeric index of a stage in StageOrder.
// Returns -1 if the stage is not found.
func StageIndex(stage TransformStage) int {
	for i, s := range StageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// IsValidStage returns true if the stage is defined in StageOrder.
func IsValidStage(stage TransformStage) bool {
	return StageIndex(stage) >= 0
}
