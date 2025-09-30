package transforms

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The concrete Page type lives in hugo package; we interact via reflection-free field access using type assertions.
// For simplicity we rely on the struct shape remaining stable; Phase 2 can formalize a minimal interface.

// priority constants (gaps allow future insertion)
const (
	prFrontMatterParse = 10
	prFrontMatterBuild = 20
	prEditLink         = 30
	prFrontMatterMerge = 40
	prRelLink          = 50
	prSerialize        = 90
)

// generatorProvider is set by hugo package prior to running registry pipeline (late binding to avoid import cycle).
var generatorProvider func() any

// SetGeneratorProvider allows hugo to inject a closure returning *hugo.Generator without import cycle.
func SetGeneratorProvider(fn func() any) { generatorProvider = fn }

// Helper accessors to avoid repeating assertions.
type pageLike interface {
	GetContent() string
}

// We duplicate minimal logic from existing transformers here; once stable we can delete originals.

type FrontMatterParser struct{}

func (t FrontMatterParser) Name() string  { return "front_matter_parser" }
func (t FrontMatterParser) Priority() int { return prFrontMatterParse }
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

type FrontMatterBuilder struct{}

func (t FrontMatterBuilder) Name() string  { return "front_matter_builder" }
func (t FrontMatterBuilder) Priority() int { return prFrontMatterBuild }
func (t FrontMatterBuilder) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok && shim.BuildFrontMatter != nil {
		shim.BuildFrontMatter(time.Now())
		return nil
	}
	// If another facade implementation is provided directly, we currently have
	// no generic build hook; future extension could introduce a BuildFrontMatter
	// method onto PageFacade once stabilized.
	return nil
}

type EditLinkInjector struct{}

func (t EditLinkInjector) Name() string  { return "edit_link_injector" }
func (t EditLinkInjector) Priority() int { return prEditLink }
func (t EditLinkInjector) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok && shim.InjectEditLink != nil {
		shim.InjectEditLink()
	}
	return nil
}

type MergeFrontMatter struct{}

func (t MergeFrontMatter) Name() string  { return "front_matter_merge" }
func (t MergeFrontMatter) Priority() int { return prFrontMatterMerge }
func (t MergeFrontMatter) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		shim.ApplyPatchesFacade()
		return nil
	}
	return nil
}

type RelativeLinkRewriter struct{}

func (t RelativeLinkRewriter) Name() string  { return "relative_link_rewriter" }
func (t RelativeLinkRewriter) Priority() int { return prRelLink }
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

func (t Serializer) Name() string  { return "front_matter_serialize" }
func (t Serializer) Priority() int { return prSerialize }
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
	Content             string
	OriginalFrontMatter map[string]any
	HadFrontMatter      bool
	Patches             []any // placeholder; full type lives in hugo
	// Function hooks to avoid duplicating logic or importing hugo internals
	BuildFrontMatter func(now time.Time)
	InjectEditLink   func()
	ApplyPatches     func()
	RewriteLinks     func(string) string
	SerializeFn      func() error
	SyncOriginal     func(fm map[string]any, had bool) // allows parser to propagate parsed FM back to real Page
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
func (p *PageShim) AddPatch(_ any) { /* patches added via BuildFrontMatter / InjectEditLink closures */
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
	Register(FrontMatterParser{})
	Register(FrontMatterBuilder{})
	Register(EditLinkInjector{})
	Register(MergeFrontMatter{})
	Register(RelativeLinkRewriter{})
	Register(Serializer{})
	_ = fmt.Sprintf // silence unused imports if stripped by future edits
}
