package hugo

import (
	"fmt"
	"reflect"
	"sort"

	"git.home.luguber.info/inful/docbuilder/internal/docs"
	"git.home.luguber.info/inful/docbuilder/internal/hugo/fmcore"
)

// Page is the in-memory representation of a markdown document being transformed.
type Page struct {
	File                docs.DocFile
	Raw                 []byte         // Serialized final bytes (after pipeline)
	Content             string         // Body without original front matter
	HadFrontMatter      bool           // Original file had front matter
	OriginalFrontMatter map[string]any // Parsed original FM (immutable base)
	Patches             []fmcore.FrontMatterPatch
	MergedFrontMatter   map[string]any        // Result after merge step
	Conflicts           []FrontMatterConflict // Recorded conflicts during merge
}

// Re-export enums for backward compatibility (internal only). Prefer fmcore.* directly in new code.
type MergeMode = fmcore.MergeMode

const (
	MergeDeep         = fmcore.MergeDeep
	MergeReplace      = fmcore.MergeReplace
	MergeSetIfMissing = fmcore.MergeSetIfMissing
)

type ArrayMergeStrategy = fmcore.ArrayMergeStrategy

const (
	ArrayReplace = fmcore.ArrayReplace
	ArrayUnion   = fmcore.ArrayUnion
	ArrayAppend  = fmcore.ArrayAppend
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
// FrontMatterPatch preserved for existing references (type alias)
type FrontMatterPatch = fmcore.FrontMatterPatch

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
	// Patches are applied in priority order; OriginalFrontMatter remains immutable baseline.
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
func mergeArrays(oldA, newA []any, patch FrontMatterPatch, _ *Page, key string) []any {
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

// Legacy transformer implementations removed after migration to registry-based pipeline.
