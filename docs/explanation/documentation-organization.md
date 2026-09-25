---
aliases:
  - /_uid/4734c3fa-f748-4571-a7df-81d7c2097b2b/
categories:
  - explanation
fingerprint: 84912e15364399a47a11876b6c1c80643676917db0f0060f6b437fc7d03d2f21
lastmod: "2026-09-25"
tags:
  - documentation
  - diataxis
  - taxonomy
title: Documentation Organization
uid: 4734c3fa-f748-4571-a7df-81d7c2097b2b
---

# Documentation Organization

A snapshot of how the docbuilder documentation site is currently
organized under `docs/`, what's working, what isn't, and a prioritized
list of improvements to consider. This is an "explanation" doc (the
"why") rather than a reference; for the page-shape contract, see
[`doc-page-structure.md`](../reference/doc-page-structure.md).

## What docbuilder does with the docs tree

Docbuilder renders `docs/` into a Hugo site using the Relearn theme.
The shape of the rendered site is determined by:

- One `_index.md` per directory — becomes the directory's landing page
  (Hugo auto-generates a flat listing when absent)
- `categories` frontmatter — drives the sidebar; a doc with
  `categories: [how-to]` appears in the "How-to" sidebar section
- `tags` frontmatter — drives `/tags/<slug>/` index pages
- `weight` frontmatter — orders items within a category (lower = higher)
- Filename kebab-case — enforced by the lint tool
- `params.docbuilder.template.*` — marks a doc as a template (see
  [`doc-template-structure.md`](../reference/doc-template-structure.md))
- `aliases` + canonical `uid` — keeps old URLs working

## Current shape — a snapshot

Counts taken from `docs/` with a small Python audit script:

| Count | Category |
|---|---|
| 23 | `architecture-decisions` (the ADRs) |
| 18 | `how-to` |
| 15 | `explanation` |
| 10 | `reference` |
| 7  | `architecture` |
| 2  | `ci-cd` |
| 2  | `Templates` |
| 1  | `documentation` |
| 1  | `development` |
| 1  | `tutorials` |
| **87** | total docs that have a `categories` field |

A separate 14 docs have **no** `categories` field at all (see
[Open issues](#open-issues)).

## Directory layout

```
docs/
├── README.md            ← repo-root docs index; auto-generated content
├── ci-cd-setup.md
├── adr/                 ← 28 ADRs (sequence-numbered adr-NNN-slug.md)
├── explanation/         ← 9 docs (architecture deep-dives)
├── how-to/              ← 22 docs (task guides)
├── reference/           ← 14 docs (CLI / config / lint specs)
├── security/            ← 1 doc (vscode-edit-handler.md)
├── tutorials/           ← 1 doc (getting-started.md)
└── examples/            ← 2 template files (the source templates)
```

The directory layout is roughly Diataxis-aligned:

| Mode | Directory | Purpose |
|---|---|---|
| Tutorial | `tutorials/` | Learning-oriented walkthroughs |
| How-to | `how-to/` | Task-oriented recipes for specific problems |
| Reference | `reference/` | Information-oriented machinery specs |
| Explanation | `explanation/` | Understanding-oriented "why" |
| (extra) | `adr/` | Architectural decisions, time-ordered |
| (extra) | `security/` | Security model and threat analysis |
| (extra) | `examples/` | Template source files (the source-of-truth templates) |

## What's working

- **Diataxis split is in place.** The four doc modes live in their
  own directories, and the sidebar renders them as sections.
- **ADRs follow a consistent sequence-numbering convention**
  (`adr-NNN-slug.md`). The lint tool doesn't enforce the numbering,
  but every existing ADR is named correctly.
- **Filenames are uniformly kebab-case.** The lint tool rejects
  anything else.
- **Each doc has a real UUID `uid`** and a canonical `/_uid/<uid>/`
  alias. The lint tool requires the format and the alias; the audit
  test catches drift.

## Open issues

A snapshot of what would benefit from attention. Ordered by impact per
effort.

### High value, low risk

1. **Two parallel "architecture" categories**: `architecture` (7) and
   `explanation` (15) overlap and confuse readers. The sidebar ends up
   with two near-identical sections. Fix: rename the 7 `architecture`
   entries to `explanation`.

2. **14 docs missing a `categories` field**. Most visible:
   - `docs/security/vscode-edit-handler.md` (would belong in `security`)
   - `docs/how-to/pr-comment-integration.md` (would belong in `how-to`)
   - `docs/how-to/vscode-edit-links.md` (would belong in `how-to`)
   - `docs/reference/index-files.md` (would belong in `reference`)
   - `docs/reference/lint-json-schema.md` (would belong in `reference`)
   - 7 ADRs (should consistently use `architecture-decisions`, like
     the 23 correctly-categorized ADRs)

3. **Inconsistent category naming**: `Templates` (capitalized) is the
   only category not in kebab-case. Two files use it
   (`docs/examples/adr.template.md`, `docs/examples/guide.template.md`).
   Fix: rename to `examples`.

4. **`security/` is missing from the sidebar** even though it has a
   doc. Either:
   - Tag `vscode-edit-handler.md` with `categories: [security]` so it
     surfaces, or
   - Add one or two more security docs so the section earns its
     keep.

### High value, moderate effort

5. **No per-category landing pages** (`_index.md`). Hugo
   auto-generates a flat listing of children when there's no
   `_index.md`. That works but produces a wall-of-links first
   impression. Curated landing pages — one paragraph "what lives here"
   plus the most important docs hand-picked — give newcomers a better
   entry point.

6. **No explicit `weight` values**. The sidebar order is whatever
   Hugo defaults to (likely alphabetical by title). A new visitor's
   first impression is a jumbled menu. Recommended sidebar weight
   bands:

   | Category | Weight band |
   |---|---|
   | tutorials | 10–29 |
   | how-to | 30–59 |
   | reference | 60–89 |
   | explanation | 90–119 |
   | security | 120–139 |
   | architecture-decisions (ADRs) | 200+ (the sequence already orders them) |

7. **No "See also" cross-links** between related how-to, reference,
   and explanation docs. Hard to navigate laterally. Hugo's built-in
   `next` / `prev` frontmatter does the linear neighbours; we still
   need cross-category pointers written by hand.

### High value, higher effort

8. **MCP server has no explanation doc.** The largest architectural
   addition in this session (`cmd/mcp-server/`) is invisible in `docs/`
   beyond a brief mention in the README. An explanation doc at
   `docs/explanation/mcp-server.md` should cover:
   - stdio JSON-RPC transport lifecycle
   - tools / resources / prompts taxonomy
   - the safety model (secret redaction, path containment, destructive
     hints)
   - the instructions + structure resources the server exposes to
     LLMs

9. **No top-level `docs/index.md`**. Newcomers currently land on
   Hugo's auto-generated docs index. A one-page entry point — "What is
   docbuilder?" plus a one-paragraph description of each doc mode
   with one link each — would significantly improve first-impression.

10. **ADRs are not bidirectionally linked**. Each ADR lists what it
    cites (Hugo's `{{</* ref */>}}` links) but nothing lists what cites
    it. A small CI step that scrapes ADR-NNN references across the
    repo and rewrites an "Affects" or "Referenced by" section would
    make the ADRs navigable from either direction.

## Where I would not invest

- **Growing `docs/tutorials/`**. The directory has one doc. Either
  delete it or commit to writing 3–5 more tutorials in the next
  quarter. A single-doc section looks unfinished.
- **Splitting `reference/` further**. The 14 docs there are all
  information-oriented and they cross-reference each other. Sub-
  dividing would make them harder to find, not easier.
- **Re-doing the categories sidebar config**. Hugo + Relearn already
  render categories sensibly. The fixes above are all in
  frontmatter; no theme changes needed.

## Verification

The existing `TestAudit_LintDocs` in `cmd/mcp-server/lint_docs_audit_test.go`
spawns the docbuilder-mcp binary and runs `lint_docs` followed by
`lint_fix` over the whole `docs/` tree. It currently asserts that after
one round the error count is zero, which means any doc-builder schema
issue (missing uid, missing canonical alias, stale fingerprint) is
caught at test time. Any re-organization that adds new docs should
keep that test green.

For structural sync between docs and the MCP resources, see
`cmd/mcp-server/structure_resource_test.go` and
`cmd/mcp-server/structure_template_resource_test.go` — both assert that
the `structure://*` MCP resources and the human-readable reference docs
agree on the same field set.

## See also

- [Doc Page Structure](../reference/doc-page-structure.md) — the
  per-page schema.
- [Doc Template Structure](../reference/doc-template-structure.md) —
  the per-template schema.
- [Architecture](architecture.md) — the build pipeline architecture
  this documentation site is itself a product of.
