# Content Transform Pipeline

This document explains DocBuilder's markdown content transform architecture: how the registry works, built‑in transforms, how to add new ones, and the merge semantics for front matter.

## Goals

- Deterministic, ordered transformation of markdown files before writing to the Hugo `content/` tree.
- Extensibility: adding a new transform should not require editing existing ones.
- Behavioral parity with the legacy inline pipeline (now validated via parity + conflict tests).
- Safe front matter augmentation with explicit conflict reporting.

## High-Level Flow

1. Source markdown (optionally with YAML front matter) discovered.
2. Registry pipeline executes transformers in ascending priority order.
3. Each transformer mutates a `PageShim` (bridging to the internal `Page`).
4. Front matter is parsed, patches are accumulated, merged, then content and merged front matter are serialized back to bytes.

## Registry & Ordering

Transforms implement:

```go
type Transformer interface {
  Name() string      // stable identifier, lowercase snake_case
  Priority() int     // lower runs earlier; gaps reserved for future insertion
  Transform(PageAdapter) error
}
```go

Registration occurs in `init()` via `transforms.Register(t)`. The registry produces a sorted slice by `(priority, name)`.

### Current Priority Bands

| Priority | Transform                      | Purpose |
|----------|--------------------------------|---------|
| 10       | front_matter_parser            | Extract existing front matter & strip it |
| 20       | front_matter_builder           | Generate baseline fields & patches |
| 30       | edit_link_injector             | Add per-page `editURL` (if theme expects it) |
| 40       | front_matter_merge             | Apply ordered patches into merged map |
| 50       | relative_link_rewriter         | Rewrite intra-repo relative links to Hugo-friendly paths |
| 90       | front_matter_serialize         | Serialize merged front matter + body |

Keep `90` high to leave space for future pre-serialization transforms (e.g., code fence augmentation, heading slug injection) at 60–80.

## PageShim & Hooks

`PageShim` (in `internal/hugo/transforms/defaults.go`) exposes only required fields + function hooks. The transform layer now operates via facade-style getters/setters and an adapter that allows future direct `PageFacade` implementations. Custom transformers MUST avoid reaching into unlisted struct fields; rely on the facade methods below. A golden test (`pipeline_golden_test.go`) locks baseline behavior.

### PageFacade Methods (Current Stable Set)

| Method | Purpose |
|--------|---------|
| `GetContent()` | Retrieve mutable markdown body (without serialized front matter). |
| `SetContent(string)` | Replace markdown body in-place. |
| `GetOriginalFrontMatter()` | Access parsed original front matter (immutable baseline). |
| `SetOriginalFrontMatter(map[string]any, bool)` | Set baseline front matter & had-front-matter flag (used by parser). |
| `AddPatch(FrontMatterPatch)` | Append a pending front matter patch. |
| `ApplyPatches()` | Merge pending patches into `MergedFrontMatter`. |
| `HadOriginalFrontMatter()` | Whether the source file originally contained front matter. |
| `Serialize()` | Serialize merged front matter + body (now part of the stable facade). |

The stability test (`page_facade_stability_test.go`) enforces that this method set does not change without deliberate update. Treat additions as a versioned change—prefer helper functions if possible. `Serialize()` was promoted into the facade (2025-09-30) eliminating the special serializer closure path.

- `BuildFrontMatter(now time.Time)` – constructs builder patch using injected timestamp.
- `InjectEditLink()` – conditional edit link insertion.
- `ApplyPatches()` – performs merge (`Page.applyPatches`).
- `RewriteLinks(string) string` – markdown link rewriting.
- `Serialize()` – final YAML + content emission.
- `SyncOriginal(fm, had)` – lets parser propagate parsed original front matter into the real `Page` before builder runs (critical for protecting existing titles etc.).

## Front Matter Patching Semantics

Patches (`FrontMatterPatch`) are applied in ascending `Priority`; later patches win unless keys are protected.

Merge modes:

- `MergeDeep` – recursively merges maps, with array strategy heuristics.
- `MergeReplace` – overwrites targeted keys entirely.
- `MergeSetIfMissing` – sets keys only if absent (records `kept_original` conflicts otherwise).

Array strategies (effective via `effectiveArrayStrategy`):

- Default heuristics:
  - Taxonomies (`tags`, `categories`, `keywords`) → union when existing.
  - `outputs` → union.
  - `resources` → append.
  - Otherwise replace.
- Explicit overrides via patch `ArrayStrategy` take precedence (`ArrayUnion`, `ArrayAppend`, `ArrayReplace`).

### Conflict Recording

`FrontMatterConflict{Key, Original, Attempt, Source, Action}` actions:

- `kept_original` – protected or set-if-missing existing key retained
- `overwritten` – value replaced by later patch
- `set_if_missing` – value added by set-if-missing patch

Taxonomy unions and new-key additions without replacement do not generate conflicts.

See `transform_conflicts_test.go` for locked expectations.

## Adding a New Transform

1. Select a priority band (avoid collisions; use gaps 55, 60, 65... if between rewrite and serialize).
2. Implement a struct with `Name()`, `Priority()`, `Transform(PageAdapter) error`.
3. Register in `init()` inside a new file under `internal/hugo/transforms/` (keep single concern per file when possible).
4. Access `PageShim` via type assertion (`shim, ok := p.(*PageShim)`); guard if absent.
5. Mutate only `shim.Content` or attach front matter patches through hooks if you need to prepend/append modifications; avoid directly mutating internal maps outside merge stage unless you know the ordering implications.
6. Add unit tests:
   - Ordering: confirm your transform appears at expected index relative to neighbors.
   - Behavior: given sample content, assert expected modifications.
7. If transform alters front matter, add/update conflict tests if behavior touches protected keys.

### Example Skeleton

```go
type CodeBlockCounter struct{}

func (t CodeBlockCounter) Name() string  { return "code_block_counter" }
func (t CodeBlockCounter) Priority() int { return 55 } // after link rewrite before serialize
func (t CodeBlockCounter) Transform(p transforms.PageAdapter) error {
  shim, ok := p.(*transforms.PageShim)
  if !ok { return nil }
  count := strings.Count(shim.Content, "```") / 2
  // Add patch with set-if-missing semantics
  // (would require exposing a hook or extending shim to push patches; current approach is to add a dedicated patch injector transform)
  return nil
}

func init() { transforms.Register(CodeBlockCounter{}) }
```

## Testing Strategy

Current safety nets:

- Parity hash tests (extended scenarios) ensure registry mirrors legacy semantics.
- Conflict logging test locks merge outcome & conflict classification.
- Front matter builder tests ensure time injection & edit link logic stable.
- Ordering test ensures deterministic registry order.

Recommended for new transforms:

- Unit test its isolated mutation.
- A golden test if serialization changes (normalize volatile fields like timestamps).

## Future Enhancements

- Config-driven enable/disable list (`config.Hugo.Transforms.Enabled/Disabled`).
- External plugin injection (dynamic registration before build start).
- Parallelizable transform segments (content-only transforms batched after merge, before serialize).
- Shared metrics (duration, error counts) per transform name.

## Migration Notes

The legacy inline transformer pipeline will be removed after a stabilization window; at that point parity tests may be converted to golden fixtures against the registry output.

---
Questions or additions? Extend this doc as the pipeline evolves.
