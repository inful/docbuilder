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
	pg, ok := p.(*PageShim)
	if !ok {
		return nil
	}
	body := pg.Content
	if strings.HasPrefix(body, "---\n") {
		search := body[4:]
		if idx := strings.Index(search, "\n---\n"); idx >= 0 {
			fmContent := search[:idx]
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
				slog.Warn("Failed to parse existing front matter (registry)", "file", pg.FilePath, "error", err)
			} else {
				pg.OriginalFrontMatter = fm
				pg.HadFrontMatter = true
				if pg.SyncOriginal != nil {
					pg.SyncOriginal(fm, true)
				}
			}
			pg.Content = search[idx+5:]
		}
	}
	return nil
}

type FrontMatterBuilder struct{}

func (t FrontMatterBuilder) Name() string  { return "front_matter_builder" }
func (t FrontMatterBuilder) Priority() int { return prFrontMatterBuild }
func (t FrontMatterBuilder) Transform(p PageAdapter) error {
	// Defer to existing hugo implementation via shim method if available; fallback no-op.
	if shim, ok := p.(*PageShim); ok {
		if shim.BuildFrontMatter != nil {
			shim.BuildFrontMatter(time.Now())
		}
	}
	return nil
}

type EditLinkInjector struct{}

func (t EditLinkInjector) Name() string  { return "edit_link_injector" }
func (t EditLinkInjector) Priority() int { return prEditLink }
func (t EditLinkInjector) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		if shim.InjectEditLink != nil {
			shim.InjectEditLink()
		}
	}
	return nil
}

type MergeFrontMatter struct{}

func (t MergeFrontMatter) Name() string  { return "front_matter_merge" }
func (t MergeFrontMatter) Priority() int { return prFrontMatterMerge }
func (t MergeFrontMatter) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		if shim.ApplyPatches != nil {
			shim.ApplyPatches()
		}
	}
	return nil
}

type RelativeLinkRewriter struct{}

func (t RelativeLinkRewriter) Name() string  { return "relative_link_rewriter" }
func (t RelativeLinkRewriter) Priority() int { return prRelLink }
func (t RelativeLinkRewriter) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		if shim.RewriteLinks != nil {
			shim.Content = shim.RewriteLinks(shim.Content)
		}
	}
	return nil
}

type Serializer struct{}

func (t Serializer) Name() string  { return "front_matter_serialize" }
func (t Serializer) Priority() int { return prSerialize }
func (t Serializer) Transform(p PageAdapter) error {
	if shim, ok := p.(*PageShim); ok {
		if shim.Serialize != nil {
			return shim.Serialize()
		}
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
	Serialize        func() error
	SyncOriginal     func(fm map[string]any, had bool) // allows parser to propagate parsed FM back to real Page
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
