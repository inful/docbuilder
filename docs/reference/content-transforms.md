---
title: "Content Transforms Reference"
date: 2025-12-15
categories:
  - reference
tags:
  - transforms
  - content-processing
---

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

## Transform Coverage

All transforms registered in the pipeline apply to **all markdown files** including:
- Regular documentation pages
- README.md files
- README.md files promoted to _index.md (repository/section indexes)
- Custom index pages

The index generation stage uses already-transformed content via `DocFile.TransformedBytes`, ensuring transforms are never bypassed. Previously, the index stage would re-read source files and bypass the transform pipeline, but this has been fixed as of ADR-002 implementation.

## Capability System

**Forge Capabilities** (`internal/forge/capabilities.go`): Each forge type declares feature support flags:

- `SupportsEditLinks` - whether the forge can generate web edit URLs
- `SupportsWebhooks` - whether web hook integration is possible
- `SupportsTopics` - whether repository topics/tags are supported

**Theme Capabilities** (`internal/hugo/theme/capabilities.go`): Each theme declares UI integration flags:

- `WantsPerPageEditLinks` - whether theme UI expects `editURL` front matter
- `SupportsSearchJSON` - whether theme can consume search index JSON

These capability maps are snapshotted by golden tests to detect unintentional changes. Adding a new forge or theme requires updating the corresponding capability map and golden test expectations.

## Registry & Ordering

Transforms implement:

```go
type Transformer interface {
  Name() string                        // stable identifier, lowercase snake_case
  Stage() TransformStage               // execution stage (parse, build, enrich, etc.)
  Dependencies() TransformDependencies // explicit ordering constraints
  Transform(PageAdapter) error
}
```

Registration occurs in `init()` via `transforms.Register(t)`. The registry produces an ordered slice by:
1. **Stage order** (parse → build → enrich → merge → transform → finalize → serialize)
2. **Dependency resolution** within each stage using topological sort

### Current Transform Stages

| Stage | Transforms | Purpose |
|-------|------------|---------|  
| parse | front_matter_parser | Extract existing front matter & strip it |
| build | front_matter_builder_v2 | Generate baseline fields (title/date/repository/forge/section/metadata) (no editURL) |
| enrich | edit_link_injector_v2 | Adds `editURL` with set-if-missing semantics using **centralized resolver** & **capability-gated** theme detection |
| merge | front_matter_merge | Apply ordered patches into merged map |
| transform | relative_link_rewriter | Rewrite intra-repo markdown links to Hugo-friendly paths (strip .md) |
| finalize | strip_first_heading, shortcode_escaper, hextra_type_enforcer | Post-process content |
| serialize | front_matter_serialize | Serialize merged front matter + body |

Why split builder and edit link? Decoupling eliminates implicit coupling between title/metadata generation and theme-specific edit link logic, enabling future themes to provide alternative edit URL policies or disable them entirely via transform filters.**Edit Link Generation (Post-Consolidation)**: Edit links are now generated exclusively through `hugo.EditLinkResolver` which centralizes path normalization, forge type detection, and theme capability checking. The previous `fmcore.ResolveEditLink` function has been removed. This ensures consistent behavior and eliminates the risk of `docs/docs/` path duplication. Theme capability flags (`ThemeCapabilities.WantsPerPageEditLinks`) and forge capability flags (`ForgeCapabilities.SupportsEditLinks`) gate edit link injection at the transform level.

Removed legacy transforms (`front_matter_builder`, `edit_link_injector`) under the greenfield policy (no backward compatibility shims maintained). If you had explicit allowlists containing them, replace with their V2 counterparts.

## PageShim & Hooks

`PageShim` (in `internal/hugo/transforms/defaults.go`) exposes only required fields + function hooks. The transform layer now operates via facade-style getters/setters and an adapter that allows future direct `PageFacade` implementations. Custom transformers MUST avoid reaching into unlisted struct fields; rely on the facade methods below. A golden test (`pipeline_golden_test.go`) locks baseline behavior.

As of 2025-09-30 the shim gained a `BackingAddPatch` hook so facade patches registered by transforms propagate to the underlying concrete `Page` early (supporting conflict recording that previously happened only during final merge).

### PageFacade Methods (Current Stable Set)

| Method | Purpose |
|--------|---------|
| `GetContent()` | Retrieve mutable markdown body (without serialized front matter). |
| `SetContent(string)` | Replace markdown body in-place. |
| `GetOriginalFrontMatter()` | Access parsed original front matter (immutable baseline). |
| `SetOriginalFrontMatter(map[string]any, bool)` | Set baseline front matter & had-front-matter flag (used by parser). |
| `AddPatch(FrontMatterPatch)` | Append a pending front matter patch (also forwarded via `BackingAddPatch`). |
| `ApplyPatches()` | Merge pending patches into `MergedFrontMatter`. |
| `HadOriginalFrontMatter()` | Whether the source file originally contained front matter. |
| `Serialize()` | Serialize merged front matter + body (facade method; serializer transform delegates here). |

The stability test (`page_facade_stability_test.go`) enforces that this method set does not change without deliberate update. Treat additions as a versioned change—prefer helper functions if possible. `Serialize()` was promoted into the facade (2025-09-30) eliminating the special serializer closure path.

### Supporting Functions

The transform system uses these standalone functions to process pages:

- `BuildFrontMatter(FrontMatterInput)` – Generates front matter from file metadata, config, and existing values (in `internal/hugo/frontmatter.go`). Used by `front_matter_builder_v2` transform.
- `ApplyPatches()` – Merges pending patches into final front matter (facade method on `Page.applyPatches`).

## Front Matter Patching Semantics

Patches (`FrontMatterPatch`) are applied in transform execution order (stage + dependencies); later patches win unless keys are protected.

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

1. Select a stage (`StageParse`, `StageBuild`, `StageEnrich`, `StageMerge`, `StageTransform`, `StageFinalize`, or `StageSerialize`).
2. Declare dependencies on other transforms using `MustRunAfter` and/or `MustRunBefore`.
3. Implement a struct with `Name()`, `Stage()`, `Dependencies()`, `Transform(PageAdapter) error`.
4. Register in `init()` inside a new file under `internal/hugo/transforms/` (keep single concern per file when possible).
5. Access `PageShim` via type assertion (`shim, ok := p.(*PageShim)`); guard if absent.
6. Mutate only `shim.Content` or attach front matter patches through hooks if you need to prepend/append modifications; avoid directly mutating internal maps outside merge stage unless you know the ordering implications.
7. Add unit tests:
   - Ordering: confirm your transform appears at expected position relative to neighbors.
   - Behavior: given sample content, assert expected modifications.
8. If transform alters front matter, add/update conflict tests if behavior touches protected keys.

### Example Skeleton

```go
type CodeBlockCounter struct{}

func (t CodeBlockCounter) Name() string { return "code_block_counter" }

func (t CodeBlockCounter) Stage() TransformStage {
  return StageTransform // Runs during content transformation stage
}

func (t CodeBlockCounter) Dependencies() TransformDependencies {
  return TransformDependencies{
    MustRunAfter: []string{"relative_link_rewriter"},
  }
}

func (t CodeBlockCounter) Transform(p transforms.PageAdapter) error {
  shim, ok := p.(*transforms.PageShim)
  if !ok { return nil }
  count := strings.Count(shim.Content, "```") / 2
  // Add patch with set-if-missing semantics
  shim.AddPatch(fmcore.FrontMatterPatch{
    Key:   "code_block_count",
    Value: count,
    Mode:  fmcore.MergeSetIfMissing,
  })
  return nil
}

func init() { transforms.Register(CodeBlockCounter{}) }
```

## Testing Strategy

Current safety nets:

- Parity hash tests (extended scenarios) ensure registry mirrors legacy semantics.
- Conflict logging test locks merge outcome & conflict classification.
- Front matter builder tests ensure time injection & edit link logic stable.
- Ordering golden test ensures deterministic registry order.
- Capability golden tests snapshot forge and theme feature matrices.
- No-reflection guard test prevents accidental reflection usage in transforms.
- Path normalization tests ensure edit links handle multi-level docs bases correctly.

Recommended for new transforms:

- Unit test its isolated mutation.
- A golden test if serialization changes (normalize volatile fields like timestamps).

## Future Enhancements

The project is intentionally and permanently self-contained. We will NOT introduce a dynamic external plugin / extension / runtime injection mechanism. All transforms and behavioral changes must land via normal code changes and review.

Planned/possible incremental internal improvements (still in-repo only):

- Per-transform param injection sourced from config (extending existing enable/disable filtering).
- Evaluating parallelization for pure content transforms after merge but before serialization.
- Additional shared metrics dimensions (e.g., bytes processed, patch counts) leveraging existing duration/error counters.
- Continued pruning or consolidation of transforms if responsibilities shift (greenfield policy).

## Migration Notes

Legacy inline transformer pipeline fully removed; registry is authoritative. The project charter forbids third‑party or dynamically loaded transformer plugins—this will not change. All runtime transformer code lives in-repo. Greenfield policy: we remove obsolete paths aggressively and accept minor backward incompatibilities during pre‑1.0 to keep surface area minimal.

---
Questions or additions? Extend this doc as the pipeline evolves.
