package editlink

import (
	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
)

// Resolver provides edit link resolution with a simplified, testable interface.
// It replaces the complex monolithic EditLinkResolver.Resolve method with a
// chain of responsibility pattern for better maintainability.
type Resolver struct {
	detectorChain *DetectorChain
	urlBuilder    EditURLBuilder
}

// NewResolver creates a new edit link resolver with the standard detector chain.
func NewResolver() *Resolver {
	chain := NewDetectorChain().
		Add(NewConfiguredDetector()).  // Try repository tags first
		Add(NewForgeConfigDetector()). // Then try forge configuration
		Add(NewHeuristicDetector())    // Finally try hostname heuristics

	return &Resolver{
		detectorChain: chain,
		urlBuilder:    NewStandardEditURLBuilder(),
	}
}

// NewResolverWithChain creates a resolver with a custom detector chain (for testing).
func NewResolverWithChain(chain *DetectorChain, builder EditURLBuilder) *Resolver {
	return &Resolver{
		detectorChain: chain,
		urlBuilder:    builder,
	}
}

// Resolve determines the edit URL for a DocFile.
// Returns empty string if edit links should not be generated.
func (r *Resolver) Resolve(file docs.DocFile, cfg *config.Config) string {
	// Pre-flight checks
	if !r.shouldGenerateEditLink(cfg) {
		return ""
	}

	if r.isSiteLevelSuppressed(cfg) {
		return ""
	}

	// Prepare detection context
	ctx, ok := PrepareDetectionContext(file, cfg)
	if !ok {
		return ""
	}

	// Run detection chain
	result := r.detectorChain.Detect(ctx)
	if !result.Found {
		return ""
	}

	// Build the final URL
	return r.urlBuilder.BuildURL(
		result.ForgeType,
		result.BaseURL,
		result.FullName,
		ctx.Branch,
		ctx.RepoRel,
	)
}

// shouldGenerateEditLink checks if edit links should be generated at all.
func (r *Resolver) shouldGenerateEditLink(cfg *config.Config) bool {
	// Edit links now work for all themes
	return cfg != nil
}

// isSiteLevelSuppressed checks if edit links are suppressed at the site level.
func (r *Resolver) isSiteLevelSuppressed(cfg *config.Config) bool {
	if cfg.Hugo.Params == nil {
		return false
	}

	editURL, exists := cfg.Hugo.Params["editURL"]
	if !exists {
		return false
	}

	editURLMap, ok := editURL.(map[string]any)
	if !ok {
		return false
	}

	base, exists := editURLMap["base"].(string)
	return exists && base != ""
}
