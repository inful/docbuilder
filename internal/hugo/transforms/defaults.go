package transforms

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/config"
	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/forge"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
	"gopkg.in/yaml.v3"
)

// The concrete Page type lives in hugo package; we interact via reflection-free field access using type assertions.
// For simplicity we rely on the struct shape remaining stable; Phase 2 can formalize a minimal interface.

// generatorProvider is set by hugo package prior to running registry pipeline (late binding to avoid import cycle).
var generatorProvider func() any

// SetGeneratorProvider allows hugo to inject a closure returning *hugo.Generator without import cycle.
func SetGeneratorProvider(fn func() any) { generatorProvider = fn }

// We duplicate minimal logic from existing transformers here; once stable we can delete originals.

type FrontMatterParser struct{}

func (t FrontMatterParser) Name() string { return "front_matter_parser" }

func (t FrontMatterParser) Stage() TransformStage {
	return StageParse
}

func (t FrontMatterParser) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: false,
		ModifiesContent:             true,
		ModifiesFrontMatter:         true,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t FrontMatterParser) Transform(p PageAdapter) error {
	// Accept either PageShim or any object exposing required facade subset.
	var (
		pg *PageShim
		ok bool
	)
	if pg, ok = p.(*PageShim); !ok {
		// Attempt reflective duck-typing: minimal path (GetContent/SetContent, SetOriginalFrontMatter)
		// If not satisfied, exit silently.
		return nil
	}
	body := pg.GetContent()
	if strings.HasPrefix(body, "---\n") {
		search := body[4:]
		if idx := strings.Index(search, "\n---\n"); idx >= 0 {
			fmContent := search[:idx]
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
				slog.Warn("Failed to parse existing front matter (registry)", "file", pg.FilePath, "error", err)
			} else {
				pg.SetOriginalFrontMatter(fm, true)
				if pg.SyncOriginal != nil {
					pg.SyncOriginal(fm, true)
				}
			}
			pg.SetContent(search[idx+5:])
		}
	}
	return nil
}

// Legacy front_matter_builder & edit_link_injector removed (greenfield policy: no backward compat layering).

type MergeFrontMatter struct{}

func (t MergeFrontMatter) Name() string { return "front_matter_merge" }

func (t MergeFrontMatter) Stage() TransformStage {
	return StageMerge
}

func (t MergeFrontMatter) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"edit_link_injector_v2"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: true,
		ModifiesContent:             false,
		ModifiesFrontMatter:         true,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t MergeFrontMatter) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		shim.ApplyPatchesFacade()
		return nil
	}
	return nil
}

type RelativeLinkRewriter struct{}

func (t RelativeLinkRewriter) Name() string { return "relative_link_rewriter" }

func (t RelativeLinkRewriter) Stage() TransformStage {
	return StageTransform
}

func (t RelativeLinkRewriter) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"front_matter_merge"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: false,
		ModifiesContent:             true,
		ModifiesFrontMatter:         false,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           true,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        true,
	}
}

func (t RelativeLinkRewriter) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		if shim.RewriteLinks != nil {
			shim.SetContent(shim.RewriteLinks(shim.GetContent()))
		}
		return nil
	}
	return nil
}

type Serializer struct{}

func (t Serializer) Name() string { return "front_matter_serialize" }

func (t Serializer) Stage() TransformStage {
	return StageSerialize
}

func (t Serializer) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"front_matter_merge"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: true,
		ModifiesContent:             false,
		ModifiesFrontMatter:         false,
		RequiresConfig:              false,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           false,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        false,
	}
}

func (t Serializer) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		return shim.Serialize()
	}
	// If a future direct PageFacade implementation is passed, expect it to implement Serialize.
	if f, ok := p.(interface{ Serialize() error }); ok {
		return f.Serialize()
	}
	return nil
}

// PageShim mirrors the subset of hugo.Page needed for registry-based transformers; constructed in hugo package.
type PageShim struct {
	FilePath            string
	Doc                 docs.DocFile
	Content             string
	OriginalFrontMatter map[string]any
	HadFrontMatter      bool
	Patches             []fmcore.FrontMatterPatch
	ApplyPatches        func()
	RewriteLinks        func(string) string
	SerializeFn         func() error
	SyncOriginal        func(fm map[string]any, had bool)
	BackingAddPatch     func(fmcore.FrontMatterPatch) // optional: forwards patch to underlying concrete Page for final merge
}

// Facade-style minimal methods (progressive migration toward PageFacade usage in registry)
func (p *PageShim) GetContent() string                     { return p.Content }
func (p *PageShim) SetContent(s string)                    { p.Content = s }
func (p *PageShim) GetOriginalFrontMatter() map[string]any { return p.OriginalFrontMatter }
func (p *PageShim) SetOriginalFrontMatter(fm map[string]any, had bool) {
	p.OriginalFrontMatter = fm
	p.HadFrontMatter = had
}

// Additional facade-aligned helpers (mirroring methods on real PageFacade implementation)
func (p *PageShim) AddPatch(fp fmcore.FrontMatterPatch) {
	p.Patches = append(p.Patches, fp)
	if p.BackingAddPatch != nil {
		p.BackingAddPatch(fp)
	}
}
func (p *PageShim) ApplyPatchesFacade() {
	if p.ApplyPatches != nil {
		p.ApplyPatches()
	}
}
func (p *PageShim) HadOriginalFrontMatter() bool { return p.HadFrontMatter }
func (p *PageShim) Serialize() error {
	if p.SerializeFn != nil {
		return p.SerializeFn()
	}
	return nil
}

func init() {
	// Register all transforms with dependency-based registry
	Register(FrontMatterParser{})
	Register(FrontMatterBuilderV2{})
	Register(EditLinkInjectorV2{})
	Register(MergeFrontMatter{})
	Register(RelativeLinkRewriter{})
	Register(Serializer{})

	_ = fmt.Sprintf
}

// FrontMatterBuilderV2 builds base front matter (without editURL) and adds a patch.
type FrontMatterBuilderV2 struct{}

func (t FrontMatterBuilderV2) Name() string { return "front_matter_builder_v2" }

func (t FrontMatterBuilderV2) Stage() TransformStage {
	return StageBuild
}

func (t FrontMatterBuilderV2) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"front_matter_parser"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: true,
		ModifiesContent:             false,
		ModifiesFrontMatter:         true,
		RequiresConfig:              true,
		RequiresThemeInfo:           false,
		RequiresForgeInfo:           true,
		RequiresEditLinkResolver:    false,
		RequiresFileMetadata:        true,
	}
}

func (t FrontMatterBuilderV2) Transform(p PageAdapter) error {
	shim, ok := p.(*PageShim)
	if !ok {
		return nil
	}
	if shim == nil {
		return nil
	}
	// Acquire generator for config & current time
	var cfg *config.Config
	if generatorProvider != nil {
		if g, ok2 := generatorProvider().(interface{ Config() *config.Config }); ok2 {
			cfg = g.Config()
		}
	}
	existing := shim.OriginalFrontMatter
	if existing == nil {
		existing = map[string]any{}
	}
	// Convert metadata map[string]string -> map[string]any
	var mdAny map[string]any
	if shim.Doc.Metadata != nil {
		mdAny = make(map[string]any, len(shim.Doc.Metadata))
		for k, v := range shim.Doc.Metadata {
			mdAny[k] = v
		}
	} else {
		mdAny = map[string]any{}
	}
	// Build typed front matter then convert to map for patch application
	builtTyped := fmcore.ComputeBaseFrontMatterTyped(shim.Doc.Name, shim.Doc.Repository, shim.Doc.Forge, shim.Doc.Section, mdAny, existing, cfg, time.Now())
	built := builtTyped.ToMap()
	// Only add patch if we actually mutated compared to existing map (len compare insufficient, just always patch for simplicity)
	shim.AddPatch(fmcore.FrontMatterPatch{Source: "builder_v2", Mode: fmcore.MergeDeep, Priority: 50, Data: built})
	return nil
}

// EditLinkInjectorV2 adds editURL if missing using resolver logic, separate from base builder.
type EditLinkInjectorV2 struct{}

func (t EditLinkInjectorV2) Name() string { return "edit_link_injector_v2" }

func (t EditLinkInjectorV2) Stage() TransformStage {
	return StageEnrich
}

func (t EditLinkInjectorV2) Dependencies() TransformDependencies {
	return TransformDependencies{
		MustRunAfter:                []string{"front_matter_builder_v2"},
		MustRunBefore:               []string{},
		RequiresOriginalFrontMatter: false,
		ModifiesContent:             false,
		ModifiesFrontMatter:         true,
		RequiresConfig:              true,
		RequiresThemeInfo:           true,
		RequiresForgeInfo:           true,
		RequiresEditLinkResolver:    true,
		RequiresFileMetadata:        true,
	}
}

func (t EditLinkInjectorV2) Transform(p PageAdapter) error {
	shim, ok := p.(*PageShim)
	if !ok || shim == nil {
		return nil
	}
	// Skip if already present in original
	if shim.OriginalFrontMatter != nil {
		if _, exists := shim.OriginalFrontMatter["editURL"]; exists {
			return nil
		}
	}
	// Skip if any prior patch already added editURL
	for _, patch := range shim.Patches {
		if patch.Data != nil {
			if _, exists := patch.Data["editURL"]; exists {
				return nil
			}
		}
	}
	// Need config + resolver
	var (
		cfg      *config.Config
		resolver interface {
			Resolve(file docs.DocFile) string
		}
	)
	if generatorProvider != nil {
		if g, ok2 := generatorProvider().(interface{ Config() *config.Config }); ok2 {
			cfg = g.Config()
		}
		// Access full generator to reach centralized resolver if available
		if gFull, ok3 := generatorProvider().(interface {
			Config() *config.Config
			EditLinkResolver() interface{ Resolve(docs.DocFile) string }
		}); ok3 {
			resolver = gFull.EditLinkResolver()
		}
	}
	if cfg == nil {
		return nil
	}
	// Relearn theme supports per-page edit links
	// Forge capability check: ensure repository's forge (if tagged) supports edit links
	if shim.Doc.Forge != "" {
		if !forge.GetCapabilities(config.ForgeType(shim.Doc.Forge)).SupportsEditLinks {
			return nil
		}
	}
	var val string
	if resolver != nil {
		val = resolver.Resolve(shim.Doc)
	}
	if val == "" { // fallback path (should be rare once resolver always set)
		// Do nothing; we intentionally removed inline fmcore.ResolveEditLink.
		return nil
	}
	shim.AddPatch(fmcore.FrontMatterPatch{Source: "edit_link_v2", Mode: fmcore.MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}})
	return nil
}
