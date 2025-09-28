package hugo

import (
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"time"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"gopkg.in/yaml.v3"
)

// Page is the in-memory representation of a markdown document being transformed.
type Page struct {
	File                docs.DocFile
	Raw                 []byte         // Serialized final bytes (after pipeline)
	Content             string         // Body without original front matter
	HadFrontMatter      bool           // Original file had front matter
	OriginalFrontMatter map[string]any // Parsed original FM (immutable base)
	Patches             []FrontMatterPatch
	MergedFrontMatter   map[string]any        // Result after merge step
	Conflicts           []FrontMatterConflict // Recorded conflicts during merge
}

// MergeMode defines how a patch applies to existing front matter.
type MergeMode int

const (
	MergeDeep         MergeMode = iota // deep merge maps; arrays follow strategy (initially replace)
	MergeReplace                       // replace entire target keys
	MergeSetIfMissing                  // only set keys absent in base
)

// ArrayMergeStrategy controls how arrays are merged when both old and new are slices under Deep mode.
type ArrayMergeStrategy int

const (
	ArrayReplace ArrayMergeStrategy = iota
	ArrayUnion
	ArrayAppend
)

// effectiveArrayStrategy resolves the actual strategy considering defaults.
// If patch specified ArrayReplace explicitly we still allow taxonomy keys to promote to union
// when original slice exists to preserve existing tags/categories unless explicitly replaced.
func effectiveArrayStrategy(patchStrategy ArrayMergeStrategy, key string, hasExisting bool) ArrayMergeStrategy {
	// Honor explicit choice
	if patchStrategy == ArrayUnion || patchStrategy == ArrayAppend {
		return patchStrategy
	}
	// Heuristic defaults when unspecified (ArrayReplace zero value):
	switch key {
	case "tags", "categories", "keywords":
		if hasExisting {
			return ArrayUnion
		}
	case "outputs":
		return ArrayUnion
	case "resources":
		return ArrayAppend
	}
	return patchStrategy
}

// FrontMatterPatch represents a unit of front matter changes from a transformer.
type FrontMatterPatch struct {
	Source        string
	Mode          MergeMode
	Priority      int                // higher applied later
	Data          map[string]any     // patch data
	ArrayStrategy ArrayMergeStrategy // optional override for all arrays in this patch (0 value = replace)
}

// FrontMatterConflict describes merge decisions for auditing.
type FrontMatterConflict struct {
	Key      string
	Original any
	Attempt  any
	Source   string
	Action   string // kept_original | overwritten | set_if_missing
}

// applyPatches merges all patches into a single map using precedence rules.
// Phase 1 implementation: simple ordered application onto a base copy.
var reservedFrontMatterKeys = map[string]struct{}{
	// Core identifiers / ordering / summaries
	"title": {}, "linkTitle": {}, "description": {}, "summary": {}, "weight": {},
	// URL / routing / structure
	"slug": {}, "url": {}, "aliases": {}, "type": {}, "layout": {},
	// Dates & lifecycle
	"date": {}, "lastmod": {}, "publishDate": {}, "expiryDate": {}, "unpublishdate": {}, "draft": {},
	// Classification / taxonomies / SEO
	"tags": {}, "categories": {}, "keywords": {},
	// Output & rendering specifics
	"resources": {}, "outputs": {}, "markup": {},
	// Configuration / inheritance / metadata containers
	"cascade": {}, "params": {}, "build": {}, "sitemap": {}, "translationKey": {},
	// Menus
	"menu": {}, "menus": {},
	// Internal / custom additions
	"editURL": {}, "repository": {}, "section": {}, "toc": {},
}

// keys that are protected from overwrite (unless MergeReplace) - exclude taxonomy arrays to allow merging
var reservedProtectedKeys = map[string]struct{}{
	// Protect canonical scalar fields; require explicit MergeReplace to override
	"title": {}, "linkTitle": {}, "description": {}, "summary": {}, "weight": {},
	"slug": {}, "url": {}, "aliases": {},
	"date": {}, "lastmod": {}, "publishDate": {}, "expiryDate": {}, "unpublishdate": {}, "draft": {},
	"layout": {}, "type": {}, "markup": {}, "translationKey": {},
	// Internal fixed semantics
	"editURL": {}, "repository": {}, "section": {}, "toc": {},
	// menus / menu and taxonomy & list arrays intentionally unprotected for merging/augmentation
}

func (p *Page) applyPatches() {
	if p.OriginalFrontMatter == nil {
		p.OriginalFrontMatter = map[string]any{}
	}
	base := make(map[string]any, len(p.OriginalFrontMatter))
	for k, v := range p.OriginalFrontMatter {
		base[k] = v
	}
	// legacy FrontMatter injection removed; patches must be explicitly added by transformers
	sort.SliceStable(p.Patches, func(i, j int) bool { return p.Patches[i].Priority < p.Patches[j].Priority })
	for _, patch := range p.Patches {
		if patch.Data == nil {
			continue
		}
		for k, v := range patch.Data {
			origVal, existed := base[k]
			if patch.Mode == MergeSetIfMissing {
				if existed {
					p.Conflicts = append(p.Conflicts, FrontMatterConflict{Key: k, Original: origVal, Attempt: v, Source: patch.Source, Action: "kept_original"})
					continue
				}
				base[k] = v
				p.Conflicts = append(p.Conflicts, FrontMatterConflict{Key: k, Original: nil, Attempt: v, Source: patch.Source, Action: "set_if_missing"})
				continue
			}
			if _, isProtected := reservedProtectedKeys[k]; isProtected && existed && patch.Mode != MergeReplace {
				if notDeepEqual(origVal, v) {
					p.Conflicts = append(p.Conflicts, FrontMatterConflict{Key: k, Original: origVal, Attempt: v, Source: patch.Source, Action: "kept_original"})
				}
				continue
			}
			// Deep merge maps when requested.
			if patch.Mode == MergeDeep && existed {
				if bm, okb := origVal.(map[string]any); okb {
					if nm, okn := v.(map[string]any); okn {
						merged := deepMergeMaps(bm, nm, patch, p)
						base[k] = merged
						continue
					}
				}
				// arrays under deep mode with strategy
				if oa, oka := origVal.([]any); oka {
					if na, okn := toAnySlice(v); okn {
						base[k] = mergeArrays(oa, na, patch, p, k)
						continue
					}
				}
			}
			// If merging arrays but original absent or mode not deep treat by strategy against empty.
			if patch.Mode == MergeDeep {
				if na, okn := toAnySlice(v); okn {
					var oa []any
					if existed {
						if prevArr, okpa := origVal.([]any); okpa {
							oa = prevArr
						}
					}
					base[k] = mergeArrays(oa, na, patch, p, k)
					if existed {
						p.Conflicts = append(p.Conflicts, FrontMatterConflict{Key: k, Original: origVal, Attempt: v, Source: patch.Source, Action: "overwritten"})
					}
					continue
				}
			}
			base[k] = v
			if existed {
				p.Conflicts = append(p.Conflicts, FrontMatterConflict{Key: k, Original: origVal, Attempt: v, Source: patch.Source, Action: "overwritten"})
			}
		}
	}
	p.MergedFrontMatter = base
}

// deepMergeMaps recursively merges src into dst (modifying a copy) honoring patch array strategy for nested arrays.
func deepMergeMaps(dst map[string]any, src map[string]any, patch FrontMatterPatch, page *Page) map[string]any {
	out := make(map[string]any, len(dst))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if existing, ok := out[k]; ok {
			// recurse maps
			if em, okm := existing.(map[string]any); okm {
				if nm, okn := v.(map[string]any); okn {
					out[k] = deepMergeMaps(em, nm, patch, page)
					continue
				}
			}
			// arrays
			if ea, okl := toAnySlice(existing); okl {
				if na, okn := toAnySlice(v); okn {
					out[k] = mergeArrays(ea, na, patch, page, k)
					continue
				}
			}
			// scalar or type mismatch: replace
			if notDeepEqual(existing, v) {
				page.Conflicts = append(page.Conflicts, FrontMatterConflict{Key: k, Original: existing, Attempt: v, Source: patch.Source, Action: "overwritten"})
			}
			out[k] = v
			continue
		}
		out[k] = v
	}
	return out
}

// toAnySlice attempts to convert a value to []any when it's already a slice of concrete types.
func toAnySlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []string:
		res := make([]any, len(s))
		for i, e := range s {
			res[i] = e
		}
		return res, true
	case []int:
		res := make([]any, len(s))
		for i, e := range s {
			res[i] = e
		}
		return res, true
	default:
		return nil, false
	}
}

// mergeArrays applies strategy; default strategy: union for tags/categories, else replace, unless patch.ArrayStrategy overrides.
func mergeArrays(oldA, newA []any, patch FrontMatterPatch, page *Page, key string) []any {
	strategy := effectiveArrayStrategy(patch.ArrayStrategy, key, len(oldA) > 0)
	if strategy == ArrayReplace {
		return cloneSlice(newA)
	}
	if strategy == ArrayAppend {
		return append(cloneSlice(oldA), newA...)
	}
	// Union: use stringification to avoid panic on unhashable types; fallback to append if element not string/number.
	seen := map[string]struct{}{}
	res := make([]any, 0, len(oldA)+len(newA))
	add := func(e any) {
		keyStr := stringifyArrayElem(e)
		if keyStr == "" { // unhashable or empty repr—accept duplicates conservatively
			res = append(res, e)
			return
		}
		if _, ok := seen[keyStr]; ok {
			return
		}
		seen[keyStr] = struct{}{}
		res = append(res, e)
	}
	for _, e := range oldA {
		add(e)
	}
	for _, e := range newA {
		add(e)
	}
	return res
}

func stringifyArrayElem(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case fmt.Stringer:
		return s.String()
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", s)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", s)
	case float32, float64:
		return fmt.Sprintf("%g", s)
	case bool:
		if s {
			return "true"
		}
		return "false"
	default:
		return "" // unhashable or complex type
	}
}

func cloneSlice(in []any) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	copy(out, in)
	return out
}

// notDeepEqual returns true if values differ using reflect.DeepEqual; safe for maps/slices.
func notDeepEqual(a, b any) bool { return !reflect.DeepEqual(a, b) }

// ContentTransformer applies a transformation to a Page.
type ContentTransformer interface {
	Name() string
	Transform(p *Page) error
}

// TransformerPipeline runs a sequence of transformers.
type TransformerPipeline struct{ stages []ContentTransformer }

func NewTransformerPipeline(stages ...ContentTransformer) *TransformerPipeline {
	return &TransformerPipeline{stages: stages}
}

func (tp *TransformerPipeline) Run(p *Page) error {
	for _, st := range tp.stages {
		if err := st.Transform(p); err != nil {
			return fmt.Errorf("transform %s failed: %w", st.Name(), err)
		}
	}
	return nil
}

// RelativeLinkRewriter transformer
type RelativeLinkRewriter struct{}

func (r *RelativeLinkRewriter) Name() string { return "relative_link_rewriter" }
func (r *RelativeLinkRewriter) Transform(p *Page) error {
	p.Content = RewriteRelativeMarkdownLinks(p.Content)
	return nil
}

// FrontMatterParser extracts existing front matter (if any) and strips it from content.
type FrontMatterParser struct{}

func (f *FrontMatterParser) Name() string { return "front_matter_parser" }
func (f *FrontMatterParser) Transform(p *Page) error {
	body := p.Content
	if strings.HasPrefix(body, "---\n") {
		search := body[4:]
		if idx := strings.Index(search, "\n---\n"); idx >= 0 {
			fmContent := search[:idx]
			fm := map[string]any{}
			if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
				slog.Warn("Failed to parse existing front matter", "file", p.File.RelativePath, "error", err)
			} else {
				p.OriginalFrontMatter = fm
				p.HadFrontMatter = true
			}
			p.Content = search[idx+5:]
		}
	}
	return nil
}

// FrontMatterBuilder populates or augments front matter using existing map.
type FrontMatterBuilder struct{ ConfigProvider func() *Generator }

func (f *FrontMatterBuilder) Name() string { return "front_matter_builder" }
func (f *FrontMatterBuilder) Transform(p *Page) error {
	gen := f.ConfigProvider()
	built := BuildFrontMatter(FrontMatterInput{File: p.File, Existing: p.OriginalFrontMatter, Config: gen.config, Now: time.Now()})
	p.Patches = append(p.Patches, FrontMatterPatch{Source: "builder", Mode: MergeDeep, Priority: 50, Data: built})
	return nil
}

// FinalFrontMatterSerializer serializes front matter + content back to bytes in Page.Raw for writing.
type FinalFrontMatterSerializer struct{}

func (s *FinalFrontMatterSerializer) Name() string { return "front_matter_serialize" }
func (s *FinalFrontMatterSerializer) Transform(p *Page) error {
	// Phase 1: if merged map not built yet, fall back to current FrontMatter.
	if p.MergedFrontMatter == nil {
		p.applyPatches()
	}
	fm := p.MergedFrontMatter
	if fm == nil {
		fm = map[string]any{}
	}
	fmData, err := yaml.Marshal(fm)
	if err != nil {
		return err
	}
	combined := fmt.Sprintf("---\n%s---\n%s", string(fmData), p.Content)
	p.Raw = []byte(combined)
	return nil
}

// MergeFrontMatterTransformer performs an explicit merge earlier in the pipeline for future stages that might need merged view.
type MergeFrontMatterTransformer struct{}

func (m *MergeFrontMatterTransformer) Name() string { return "front_matter_merge" }
func (m *MergeFrontMatterTransformer) Transform(p *Page) error {
	p.applyPatches()
	// legacy FrontMatter field removed; consumers should use MergedFrontMatter
	return nil
}

// EditLinkInjector ensures an editURL exists (when theme expects it) after front matter build but before serialization.
// It mirrors the logic in BuildFrontMatter but only runs if editURL is still absent; this allows it to be inserted
// flexibly in future pipelines without duplicating site-level param logic.
type EditLinkInjector struct{ ConfigProvider func() *Generator }

func (e *EditLinkInjector) Name() string { return "edit_link_injector" }
func (e *EditLinkInjector) Transform(p *Page) error {
	if p.OriginalFrontMatter != nil {
		if _, ok := p.OriginalFrontMatter["editURL"]; ok {
			return nil
		}
	}
	for _, patch := range p.Patches {
		if patch.Data != nil {
			if _, ok := patch.Data["editURL"]; ok {
				return nil
			}
		}
	}
	gen := e.ConfigProvider()
	if gen == nil {
		return nil
	}
	val := ""
	if gen.editLinkResolver != nil {
		val = gen.editLinkResolver.Resolve(p.File)
	} else {
		val = generatePerPageEditURL(gen.config, p.File)
	}
	if val == "" {
		return nil
	}
	p.Patches = append(p.Patches, FrontMatterPatch{Source: "edit_link", Mode: MergeSetIfMissing, Priority: 60, Data: map[string]any{"editURL": val}})
	return nil
}
