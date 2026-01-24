---
aliases:
  - /_uid/a4b1f7ac-95c0-441b-827a-4c94aa7ed82b/
categories:
  - architecture-decisions
date: 2026-01-23T00:00:00Z
fingerprint: 55e09a572b17d638436f929e41c0347a939a3fc34e8baca0e888ecfb7a409b9d
lastmod: "2026-01-23"
tags:
  - daemon
  - security
  - content
  - frontmatter
  - hugo
uid: a4b1f7ac-95c0-441b-827a-4c94aa7ed82b
---

# ADR-019: Daemon mode public-only rendering via frontmatter

**Status**: Proposed  
**Date**: 2026-01-23  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder’s daemon mode is designed to run continuously, ingest docs from multiple repositories, and serve the resulting site over HTTP.

When daemon mode is exposed on a network, the operational risk is not that Hugo “leaks” data, but that DocBuilder can *accidentally* aggregate and publish documentation that was never intended to be public (e.g., internal runbooks, design notes, customer-specific docs).

We want a simple, repo-author-driven mechanism to explicitly opt pages into being published by the daemon.

## Goals

- Provide an explicit, per-page “publish” opt-in.
- Make the default safe: a page is not published unless it is explicitly marked.
- Keep the mechanism repo-agnostic and forge-agnostic.
- Keep behavior limited to daemon mode (direct/local builds remain unchanged unless explicitly enabled).

## Non-Goals

- Implement authentication/authorization for the docs HTTP server.
- Provide fine-grained per-user access control.
- Implement an asset dependency graph (copy only assets referenced by public pages).

## Decision

Introduce an optional “public-only” mode for daemon builds:

- When enabled, DocBuilder will only include Markdown pages that contain `public: true` in their YAML frontmatter.
- Any Markdown page without `public: true` will not be written into the generated Hugo content tree and therefore will not be rendered/served.
- Generated index pages are created only for scopes that contain at least one public page, and those generated indexes include `public: true`.
- If zero pages are public, DocBuilder will publish an empty site (no warning or failure).

### Definition: What counts as public?

A page is considered **public** if and only if:

- It has YAML frontmatter, and
- The parsed YAML frontmatter contains the key `public` with boolean value `true`.

Public status is evaluated **per page only**. It does not inherit from parent sections or `_index.md` files (no `cascade` support for this policy).

All other cases are treated as **not public**:

- No frontmatter present
- Frontmatter present but missing `public`
- `public: false`
- Invalid YAML frontmatter (treated as no frontmatter)

This aligns with the “explicit opt-in” safety goal.

## Proposed Configuration Surface

Add a daemon-only setting that enables this behavior, conceptually:

```yaml
daemon:
  content:
    public_only: true
```

Notes:

- The exact field name/shape is an implementation detail, but it should live under `daemon` because this policy is daemon-specific.
- Default should be `false` for backwards compatibility.

## Behavior Details

### Pipeline location

Filtering should happen after discovery and before writing Hugo content files.

Practical implementation options:

- Filter at the Hugo pipeline entrypoint (when converting discovered docs to pipeline `Document`s).
- Or, filter inside the pipeline processor after a minimal frontmatter parse step.

Either way, filtering must inspect the page’s *original* frontmatter (not default-injected fields like `title`, `type`, etc.).

Filtering must be strict and local to the page being evaluated. It must not apply Hugo frontmatter inheritance semantics (e.g., `cascade`).

### Generated index pages

DocBuilder currently generates index pages (`content/_index.md`, per-repo `_index.md`, per-section `_index.md`) when they don’t exist.

In public-only mode, we must choose between:

1. **Strict mode**: generated index pages are excluded unless they also include `public: true`.
2. **Usability mode**: generated index pages are created only for “public scopes” (site/repo/section that contains at least one public page) and generated with `public: true`.

This ADR proposes **usability mode** as the default behavior when public-only is enabled:

- Site root index is generated with `public: true`.
- Repository index is generated with `public: true` *only if* the repository contains at least one public page.
- Section indexes are generated with `public: true` *only if* the section contains at least one public page.

This preserves navigation while maintaining “only pages with `public: true` are rendered”.

### Static assets

This policy applies to Markdown pages. Static assets (images, PDFs, etc.) are not pages and should continue to be copied as today.

Rationale:

- Public pages commonly reference nearby images; a strict asset filter is complex and error-prone.
- This feature is about preventing *accidental publication of Markdown content*.

If needed later, we can add an optional “public assets only” rule based on referenced links.

### Reporting and observability

When public-only is enabled, we should surface:

- Count of discovered pages vs rendered pages
- Count of excluded pages
- (Optional) debug log entries identifying excluded files

This makes “why is my page missing?” diagnosable.

If no public pages are present, the build should succeed and produce an empty site. This is intentional: it makes the policy safe to turn on without risking daemon instability.

## Acceptance Criteria

- With public-only enabled, a Markdown page is rendered only when its own frontmatter contains `public: true`.
- A parent `_index.md` with Hugo `cascade` does not make child pages public.
- Generated index pages are created only for scopes with at least one public page and include `public: true`.
- If zero pages are public, the build succeeds and publishes an empty site.

## Security Considerations

- **Broken links are expected**: public pages may link to non-public pages. In public-only mode those targets will not be rendered, and resulting links may 404. This is an acceptable tradeoff for strict opt-in publishing.
- **Assets are still copied**: this policy filters Markdown pages only. Static assets are copied as today to keep public pages functional and to avoid implementing a fragile “only referenced assets” graph. This means non-public images/PDFs may still be present in the output directory if they exist under discovered asset paths.

## Consequences

### Pros

- Prevents accidental publication by requiring explicit per-page opt-in.
- Repo maintainers can control publication without changing repo layout.
- Works across forges and repository sources.

### Cons / Tradeoffs

- Easy to misconfigure: forgetting `public: true` makes pages disappear.
- Public pages may link to private pages; those links will become broken in the published site.
- Generated navigation must be carefully scoped to avoid empty sections or confusing UX.

## Alternatives Considered

1. **Directory-based opt-in** (e.g., only include `docs/public/**`)
   - Rejected: requires repo restructuring and doesn’t work well with multi-path discovery.

2. **Repository-level opt-in** (publish or ignore entire repositories)
   - Rejected: too coarse; many repos contain mixed public/private docs.

3. **Access control at the HTTP layer** (auth)
   - Not a replacement: still risks accidental publication into the generated site artifacts.
   - Could be complementary in the future.

4. **Hugo `draft: true` / `private: true` conventions**
   - Not chosen: we want a DocBuilder-specific, explicit opt-in for daemon publishing.

## Implementation Notes (Deferred)

This ADR describes the intended direction; it does not implement the change.

Suggested implementation approach:

- Add a daemon-only config flag (see Proposed Configuration Surface).
- Implement a small filter function that:
  - Parses frontmatter using the existing `docmodel` frontmatter splitter/parsing.
  - Selects only pages with `public: true`.
- Update index generators to:
  - Detect whether a scope (repo/section) contains any public pages.
  - Generate indexes only for public scopes and include `public: true` in generated index frontmatter.

Suggested test strategy:

- Unit tests for the filtering logic (frontmatter variations).
- Golden integration test covering:
  - Mixed public/private pages
  - Expected content tree only includes public pages
  - Generated indexes appear only for public scopes

## Open Questions

- Should `public: true` be enforced only in daemon mode, or should there be a general `build.content.public_only` toggle for non-daemon builds as well?

## Related Documents

- ADR-005: Documentation linting
- ADR-008: Staged pipeline architecture
- ADR-017: Split daemon responsibilities