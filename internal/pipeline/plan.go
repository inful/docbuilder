package pipeline

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/theme"
)

// BuildPlan is an immutable execution plan derived from config.
// It captures normalized inputs and knobs for the pipeline stages.
type BuildPlan struct {
	Config       *config.Config
	OutputDir    string
	WorkspaceDir string
	Incremental  bool

	// Resolved execution context for deterministic behavior
	ThemeFeatures  theme.Features
	EnabledFilters ResolvedFilters
	TransformNames []string // ordered transform names to apply
}

// ResolvedFilters contains normalized repository filtering criteria.
type ResolvedFilters struct {
	RequiredPaths   []string
	IgnoreFiles     []string
	IncludePatterns []string
	ExcludePatterns []string
}

// BuildPlanBuilder constructs a BuildPlan with resolved features and filters.
type BuildPlanBuilder struct {
	plan BuildPlan
}

// NewBuildPlanBuilder creates a builder with base config.
func NewBuildPlanBuilder(cfg *config.Config) *BuildPlanBuilder {
	return &BuildPlanBuilder{plan: BuildPlan{Config: cfg}}
}

// WithOutput sets output and workspace directories.
func (b *BuildPlanBuilder) WithOutput(outputDir, workspaceDir string) *BuildPlanBuilder {
	b.plan.OutputDir = outputDir
	b.plan.WorkspaceDir = workspaceDir
	return b
}

// WithIncremental enables incremental build mode.
func (b *BuildPlanBuilder) WithIncremental(inc bool) *BuildPlanBuilder {
	b.plan.Incremental = inc
	return b
}

// WithThemeFeatures resolves and caches theme features for the plan.
func (b *BuildPlanBuilder) WithThemeFeatures(feats theme.Features) *BuildPlanBuilder {
	b.plan.ThemeFeatures = feats
	return b
}

// ResolveThemeFeatures derives theme features from config and caches them.
func (b *BuildPlanBuilder) ResolveThemeFeatures() *BuildPlanBuilder {
	themeType := b.plan.Config.Hugo.ThemeType()
	if t := theme.Get(themeType); t != nil {
		b.plan.ThemeFeatures = t.Features()
	} else {
		b.plan.ThemeFeatures = theme.Features{Name: themeType}
	}
	return b
}

// WithFilters resolves filtering configuration into normalized criteria.
func (b *BuildPlanBuilder) WithFilters(filters ResolvedFilters) *BuildPlanBuilder {
	b.plan.EnabledFilters = filters
	return b
}

// ResolveFilters extracts and normalizes filters from config.
func (b *BuildPlanBuilder) ResolveFilters() *BuildPlanBuilder {
	if b.plan.Config.Filtering != nil {
		b.plan.EnabledFilters = ResolvedFilters{
			RequiredPaths:   b.plan.Config.Filtering.RequiredPaths,
			IgnoreFiles:     b.plan.Config.Filtering.IgnoreFiles,
			IncludePatterns: b.plan.Config.Filtering.IncludePatterns,
			ExcludePatterns: b.plan.Config.Filtering.ExcludePatterns,
		}
	}
	return b
}

// WithTransforms sets the ordered list of transform names to apply.
func (b *BuildPlanBuilder) WithTransforms(names []string) *BuildPlanBuilder {
	b.plan.TransformNames = names
	return b
}

// ResolveTransforms determines which transforms to enable based on config.
func (b *BuildPlanBuilder) ResolveTransforms() *BuildPlanBuilder {
	if b.plan.Config.Hugo.Transforms != nil {
		// Use explicit enable list if provided
		if len(b.plan.Config.Hugo.Transforms.Enable) > 0 {
			b.plan.TransformNames = b.plan.Config.Hugo.Transforms.Enable
		} else {
			// Default: all transforms except those explicitly disabled
			// In production, query transform registry for available transforms
			defaultTransforms := []string{"frontmatter", "links", "metadata"}
			disabled := make(map[string]bool)
			for _, name := range b.plan.Config.Hugo.Transforms.Disable {
				disabled[name] = true
			}
			for _, name := range defaultTransforms {
				if !disabled[name] {
					b.plan.TransformNames = append(b.plan.TransformNames, name)
				}
			}
		}
	}
	return b
}

// Build returns the constructed BuildPlan.
func (b *BuildPlanBuilder) Build() *BuildPlan {
	return &b.plan
}
