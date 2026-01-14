---
uid: d374b432-e8a1-4f9a-903b-553d63964670
fingerprint: ab15b45151c5d9b974e7fba0688781eabf07dc822b830769da3430ebcae0ec41
---

# ADR-008: Staged Pipeline Architecture

**Status**: Accepted  
**Date**: 2026-01-14  
**Implementation Date**: Already implemented (see architecture docs)  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Provide a predictable, observable build pipeline for multi-repo documentation

## Context and Problem Statement

DocBuilder aggregates documentation from one or more Git repositories into a single Hugo site.

The system needs to:

- Support multiple repositories (and optionally multiple forges) without content path collisions.
- Produce a Hugo-ready content tree with stable, predictable paths.
- Transform markdown into a consistent, theme-friendly shape (front matter, link rewriting, edit links, etc.).
- Enable reliable incremental/daemon builds by detecting changes deterministically.
- Provide clear observability: stage timings, issue taxonomy, build reports.

A monolithic “do everything at once” implementation makes it hard to reason about failures, test stages independently, and introduce incremental behavior.

## Decision

We adopt a **staged pipeline architecture** where each stage has clear inputs/outputs and is responsible for a single part of the build.

Pipeline:

```
Config → Clone/Update → Discover → Generate Hugo Config → Transform Content → Index Pages → (Optional) Run Hugo
```

Key properties:

- **Explicit ordering**: stages are executed in a defined sequence.
- **Stable content addressing**: content is mapped into Hugo sections using repository (and optional forge) segmentation.
- **Fixed transform pipeline**: markdown files are processed using a deterministic, fixed-order transform pipeline.
- **Observability-first**: each stage records duration, outcome, and issues for reporting.

## Rationale

This structure optimizes for:

- **Predictability**: explicit stages are easier to understand and debug.
- **Testability**: discovery, transforms, and config generation can be verified independently.
- **Incremental builds**: change detection can short-circuit or narrow work at specific stages.
- **Operability**: daemon mode benefits from clear stage boundaries and stable fingerprints/hashes.

This ADR intentionally builds on existing decisions rather than redefining them:

- Fixed transform pipeline: see ADR-003.
- Golden testing strategy: see ADR-001.

## Consequences

### Benefits

- Clear separation of concerns across config, Git, discovery, generation, and transforms.
- Easier failure isolation (errors have a natural “stage” boundary).
- Enables consistent reporting (durations, issues, fingerprints).
- Supports daemon mode decisions (`skip`, `incremental`, `full_rebuild`) using stage-level signals.

### Trade-offs

- More plumbing between stages (explicit data models passed between components).
- Some duplication of “context” (e.g., repo/forge metadata) across stages, which must remain consistent.

## Alternatives Considered

1. **Monolithic generator** (clone/discover/transform/render all in one flow)
   - Rejected: difficult to test and to introduce reliable incremental behavior.

2. **Dynamic plugin pipeline** (registry + dependency resolution for stages)
   - Rejected: adds complexity without a clear user-facing extensibility requirement.

3. **Render-only focus** (treat Hugo rendering as the primary deliverable)
   - Rejected: DocBuilder’s primary responsibility is producing a correct Hugo project; rendering is optional and environment-dependent.

## Related Documents

- docs/explanation/architecture.md
- ADR-003: Fixed Transform Pipeline
- ADR-001: Golden Testing Strategy
