# Architecture Overview

Test

DocBuilder implements a staged pipeline to turn multiple Git repositories into a unified Hugo documentation site.

## Pipeline Flow

```
Config → Clone → Discover → Generate Hugo Config → Copy Content → Index Pages → (Optional) Run Hugo → Post Process
```

Each stage records duration, outcome, and issues for observability.

## Key Components

| Component | Responsibility |
|-----------|----------------|
| Config Loader | Parse YAML, expand `${ENV}` variables, apply defaults. |
| Git Client | Clone/update repositories with auth strategies (token, ssh, basic). |
| Discovery | Walk configured doc paths, filter markdown, build `DocFile` list. |
| Hugo Generator | Emit `hugo.yaml`, content tree, index pages, theme params. |
| Front Matter Builder | Merge computed metadata (repository, section, forge, editURL). |
| Report | Aggregate metrics & fingerprints for external tooling. |

## Namespacing Logic

Forge namespacing (conditional `content/<forge>/<repo>/...`) prevents collisions and yields scalable URL design. Auto mode activates only when more than one forge type exists.

## Idempotence & Change Detection

- Repository update strategy (`clone_strategy`) avoids unnecessary reclones.
- Combined check: unchanged repo heads + identical doc file set ⇒ logged and optionally triggers early exit (when output already valid).
- `doc_files_hash` offers external determinism for CI/CD.

## Error & Retry Model

Stage errors classified as:

- fatal (abort pipeline)
- warning (continue; surfaced in issues)
- canceled (context termination)

Transient classification guides retry policy (clone/update network issues; certain Hugo invocations).

## Content Generation Details

- Supported themes use Hugo Modules (no local theme directory needed).
- Index template override search order ensures safe customization without forking defaults.
- Front matter includes forge, repository, section, editURL for downstream theme logic.

## Pruning Strategy

Optional top-level pruning removes non-doc directories to shrink workspace footprint—controlled with allow/deny precedence rules to avoid accidental removal of required assets.

## Design Rationale Highlights

| Concern | Approach |
|---------|----------|
| Cross-repo collisions | Conditional forge prefix + repository segmentation. |
| Performance | Incremental fetch + pruning + shallow clones. |
| Theming | Module-based imports; param injection per theme. |
| Observability | Structured build report + issue taxonomy + stage timing. |
| Reproducibility | Environment expansion + explicit config + stable hashing. |

## Extensibility Points

- Add new theme: extend generator theme parameter injection.
- Additional issue codes: augment taxonomy without breaking consumers.
- Future caching: leverage `doc_files_hash` for selective downstream regeneration.

## Non-Goals

- Rendering arbitrary SSGs other than Hugo.
- Full-text search indexing logic (delegated to Hugo theme or external indexing). 
