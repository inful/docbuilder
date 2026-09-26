---
aliases:
  - /_uid/88a81f71-5a61-405e-a1ef-908b9ca6cabe/
categories:
  - explanation
fingerprint: 11164ebfe1d591ed944ccef98329c1bce2eeb9d3e6a1f6ca74b7e111c5283a6e
lastmod: "2026-09-26"
tags:
  - best-practices
  - documentation
  - diataxis
title: DocBuilder Documentation Best Practices
uid: 88a81f71-5a61-405e-a1ef-908b9ca6cabe
---

# DocBuilder Documentation Best Practices

The rules that produce a clean, navigable, lint-clean docbuilder site.
Each rule below is tagged with:

- **MUST** / **SHOULD** / **MAY** — how strongly the rule applies.
- **Enforced** — what currently keeps it honest (linter, build, or
  manual review). "Lint" means `docbuilder lint`; "None" means a
  human has to remember.
- The docbuilder behavior the rule depends on, with the relevant
  package:file where you can verify.

The doc is written so that:

- A human maintainer can read it top-to-bottom as a style guide.
- An LLM authoring-tool skill can lift the MUST/SHOULD rules into
  generation constraints.
- A future linter PR can lift each unenforced SHOULD into a new
  rule.

The per-page schema (frontmatter fields, body rules, filename rules)
is documented separately in
[`docs/reference/doc-page-structure.md`](../reference/doc-page-structure.md).
This doc is the layer above: how to organise many pages.

## 1. Folder organization

### 1.1 Group pages by doc mode, not by topic

**MUST.** Place each page in the directory whose doc mode it
represents:

| Mode | Directory | Purpose |
|---|---|---|
| Tutorial | `tutorials/` | A guided walkthrough the reader follows in order |
| How-to | `how-to/` | A task-oriented recipe for a specific goal |
| Reference | `reference/` | Information-oriented description of a config flag, CLI command, schema, or rule |
| Explanation | `explanation/` | "Why" content: architecture, design trade-offs, history |
| ADR | `adr/` | A discrete architectural decision, time-ordered |
| Security | `security/` | Threat model, security controls, audit notes |
| Examples | `examples/` | Source-of-truth template files (`.template.md`) |

**Why:** The `hugo.sidebar.mode: auto` (default) renders one sidebar
section per category. Mixing modes in one directory means a how-to
recipe sits next to an architectural explanation with no visual cue
that they answer different questions.

**Enforced:** None — but a future linter rule could check that
`docs/<dir>/<file>.md` has a `categories:` value matching the
directory's expected mode (e.g. `how-to/<file>.md` must have
`categories: [how-to]`).

### 1.2 Keep directories shallow

**SHOULD.** Two levels under `docs/` is the practical maximum. Avoid
`docs/how-to/foo/bar/baz.md`.

**Why:** Hugo section depth, breadcrumb length, and the Relearn
sidebar's available vertical space all degrade with depth. Two levels
maps to one sidebar section + one indented subgroup, which the theme
renders cleanly.

**Enforced:** None.

### 1.3 Each directory has at most one mode

**SHOULD.** Don't mix `categories: [how-to]` and `categories:
[reference]` pages in the same directory. If a topic legitimately
spans modes, split the pages across directories.

**Why:** Mixed-mode directories produce a sidebar section that
contains both a recipe and a spec, which reads as confused intent.

**Enforced:** None.

### 1.4 Templates live in `examples/`, not `reference/`

**MUST.** Source-of-truth template files (the `.template.md` files
that `docbuilder template new` consumes) belong in `docs/examples/`.

**Why:** The template-discovery code (`internal/templates/discovery.go`)
matches HTML anchors whose `href` contains `.template/`. The URL
shape is determined by the file's directory + filename, so templates
must be at a predictable path. Putting them in `reference/` makes
discovery fragile and confuses readers about what's consumable.

**Enforced:** None, but the template discovery will silently skip a
template that doesn't end in `.template.md` and isn't tagged
`categories: [Templates]`.

## 2. Per-directory landing pages

### 2.1 Use `_index.md` as the canonical landing-page filename

**SHOULD.** A directory's landing page is `_index.md`, not
`README.md` or `index.md`.

**Why:** Hugo treats `_index.md` as the section page. Docbuilder's
`internal/hugo/indexes.go::normalizeIndexFiles` does normalize
`README.md` and `index.md` to `_index.md` per-repo, but this is a
repo-root convenience — it doesn't apply to nested directories. A
nested `README.md` will render as a regular page, not a section
landing page. Use `_index.md` consistently.

**Enforced:** The Hugo build. No lint rule.

### 2.2 Every directory has a landing page

**SHOULD.** Every directory under `docs/` (other than `adr/` and
`examples/` which are flat collections) should have an `_index.md`.
Don't rely on Hugo's auto-generated flat listing.

**Why:** An auto-generated listing is a wall of links. A curated
landing page is a 1-paragraph "what lives here" intro plus the 3-7
most important pages hand-picked, which dramatically shortens
onboarding for newcomers.

**Enforced:** None — but a future linter rule could detect a
directory with ≥3 child docs and no `_index.md`.

### 2.3 A landing page has the same frontmatter as a regular page

**SHOULD.** A `_index.md` is a regular doc as far as the linter is
concerned: it needs `uid`, `title`, `date`, `lastmod`, `fingerprint`,
`categories`, `tags`, `aliases`. The only difference is that Hugo
treats it as a section rather than a leaf.

**Why:** Inconsistency between landing pages and regular pages creates
two classes of doc that the linter handles differently, which
inevitably leads to one of them drifting.

**Enforced:** Lint (`frontmatter-uid`, `frontmatter-fingerprint`).

## 3. File naming

### 3.1 Use kebab-case filenames

**MUST.** `how-to/use-templates.md`, not `how-to/UseTemplates.md` or
`how-to/use_templates.md`.

**Why:** URLs inherit from filenames; kebab-case produces
`/how-to/use-templates/` which reads cleanly. The lint tool rejects
anything else.

**Enforced:** Lint (`filename-conventions` rule).

### 3.2 Filename extension is `.md` or `.markdown`

**MUST.**

**Enforced:** Lint.

### 3.3 Sequence-numbered pages use zero-padded prefix

**SHOULD.** ADRs and any other sequence-numbered pages follow
`<prefix>-NNN-slug.md` with a fixed width. ADRs use 3-digit
zero-padded: `adr-001-initial-architecture.md`. Numbering past 999
requires revisiting the format.

**Why:** Stable sorting by filename gives chronological reading order
in listings. A 3-digit prefix leaves room for hundreds of entries
without renumbering; bump to 4 digits explicitly when needed.

**Enforced:** None (convention only). A future linter rule could
verify the prefix matches `^\w+-\d{3}-.+\.md$` for files in known
sequence directories.

### 3.4 Template files end in `.template.md`

**MUST.** A source-of-truth template that `docbuilder template new`
should discover ends in `.template.md` and lives under
`docs/examples/`.

**Why:** The discovery code (`internal/templates/discovery.go`) parses
the `/categories/templates/` taxonomy page and matches anchors with
`href` containing `.template/`. Without the `.template.md` suffix the
generated URL won't contain `.template/`, and the template is invisible
to discovery.

**Enforced:** None (convention + category check).

### 3.5 Don't put the page title in the filename

**SHOULD.** `how-to/configure-webhooks.md`, not
`how-to/how-to-configure-webhooks-in-docbuilder.md` or
`how-to/configuring-webhooks.md`. Filenames are slug-y; titles are
prose-y.

**Why:** Filenames become URL slugs. Stripping the imperative voice
out of the filename matches the title's inflection and avoids
double-naming ("How to: How to configure webhooks").

**Enforced:** None.

## 4. Required frontmatter on every page

### 4.1 Every page has a UUID `uid`

**MUST.** A valid RFC 4122 v4 UUID. Generate with `uuidgen` or
`uuidgen | tr '[:upper:]' '[:lower:]'`.

**Why:** `uid` is the canonical identifier that survives renames,
moves, and repo consolidations. Old URLs redirect via the
`aliases: [/_uid/<uid>/]` pattern.

**Enforced:** Lint (`frontmatter-uid` rule).

### 4.2 Every page has a canonical alias

**MUST.** `aliases:` must include `/_uid/<uid>/`. The lint fixer
auto-adds it if missing.

**Why:** Without this alias, the canonical URL becomes ambiguous
when a page moves. With it, every page has one permanent URL.

**Enforced:** Lint fixer (auto-adds). Lint doesn't currently *fail*
on missing canonical alias — it gets fixed silently.

### 4.3 Every page has a `fingerprint` that matches its content

**MUST.** `fingerprint` is a SHA-256 hex of the content, excluding
`fingerprint`, `lastmod`, `uid`, and `aliases`. Computed by
`docbuilder lint --fix`; never hand-edited.

**Why:** The fingerprint detects content drift — when a page changes,
the fingerprint changes too, and downstream systems (search index,
embed cache, ragabast ingest) can invalidate their cached version.
Hand-edited fingerprints drift silently.

**Enforced:** Lint (`frontmatter-fingerprint` rule).

### 4.4 Every page has `title`, `date`, `lastmod`, `categories`,
`tags`

**MUST.**

- `title` — string, displayed as the page H1 and in nav/search
- `date` — creation timestamp in RFC 3339 / ISO 8601
- `lastmod` — `YYYY-MM-DD`, auto-managed by `lint_fix`
- `categories` — non-empty list; drives the sidebar section
- `tags` — list; drives `/tags/<slug>/` index pages

**Why:** Each is consumed by either Hugo's rendering or the
discovery pipeline. Missing any of them produces either an empty
sidebar slot or an unindexed page.

**Enforced:** Lint enforces `uid` and `fingerprint`; the others are
not currently lint-enforced. A future rule could enforce them.

### 4.5 Don't add unrelated frontmatter

**SHOULD.** Stick to the canonical fields (`title`, `date`,
`lastmod`, `fingerprint`, `uid`, `categories`, `tags`, `aliases`,
`weight`, `draft`, `description`, `keywords`, plus the
DocBuilder-injected `repository`, `forge`, `section`, `edit_url`).
Custom keys clutter the frontmatter and confuse the linter.

**Why:** Custom keys ride into the rendered page as HTML data
attributes and clutter the YAML. If a key doesn't drive any visible
behavior, drop it.

**Enforced:** None.

## 5. Content structure

### 5.1 No H1 in the body

**MUST.** The page's H1 is the `title` frontmatter field. The body
should start at H2 (`##`).

**Why:** Adding an H1 in the body duplicates the rendered title and
breaks Hugo's section-page layout, which expects the section header
to come from frontmatter.

**Enforced:** None — but a future rule could flag `^# ` in the body.

### 5.2 Use a single H1 in frontmatter, not in the body

**MUST.** Same as 5.1.

### 5.3 Internal links use relative `.md` paths

**MUST.** `[CLI Reference](cli.md)` not `[CLI
Reference](../reference/cli.md)`. Hugo resolves them to the same
target, but the lint tool's broken-link detector operates on the
`.md` form.

**Why:** Docbuilder's broken-link detection
(`internal/lint/linter.go::detectBrokenLinks`) checks that relative
`.md` paths resolve to existing files. Absolute and site-rooted links
are not checked.

**Enforced:** Lint (broken-link detection in `LintPath`).

### 5.4 Code blocks are fenced with a language tag

**SHOULD.** Use `\`\`\`yaml`, `\`\`\`go`, `\`\`\`bash`, etc. Bare
`\`\`\`` fences render as plain text without syntax highlighting.

**Enforced:** None.

### 5.5 Don't repeat the title in the first paragraph

**SHOULD.** The rendered page already shows the title as H1. Opening
with "In this guide we will explain X" wastes the first 200px of
viewport. Open with the substantive content.

**Why:** Reader attention is highest in the first paragraph. Use it.

**Enforced:** None.

### 5.6 A "See also" section at the bottom of every doc

**SHOULD.** Two to four cross-links: one how-to (if it has
user-facing consequences), one reference (if it has API surface),
one explanation (if there's a "why" doc), plus the most relevant
template (if one applies).

**Why:** Lateral navigation is the difference between a docs site
that helps users complete tasks and one that orphans every page.
Hugo's built-in `next` / `prev` frontmatter gives linear navigation;
"See also" gives topic navigation.

**Enforced:** None.

### 5.7 Length budget per page

**SHOULD.** Reference pages can be long (CLI flag tables,
configuration reference) but should be scannable. Use H2 sections
every 100–200 lines. How-to pages should fit on one screen where
possible — split into multiple how-tos if not.

**Why:** Page length correlates inversely with completion rate.

**Enforced:** None.

## 6. Taxonomy: categories vs tags

### 6.1 Use `categories` for the doc mode

**MUST.** `categories` is one of the four Diataxis modes (or
`security` / `architecture-decisions`). Pages have exactly one
primary mode.

**Why:** Categories drive the sidebar. Putting a how-to in the
`reference` sidebar section is structurally wrong.

**Enforced:** None.

### 6.2 Use `tags` for cross-cutting attributes

**SHOULD.** Tags are free-form and many. Use them for: technology
(`lint`, `hugo`, `mcp`, `git`), audience (`maintainer`, `template-author`),
or topic (`performance`, `security`, `migration`).

**Why:** Tags create orthogonal cross-sections. Categories are
mutually exclusive; tags are not.

**Enforced:** None.

### 6.3 Don't over-tag

**SHOULD.** Three to seven tags per page. More than ten is noise.

**Enforced:** None.

### 6.4 Don't put a tag in `categories` (or vice versa)

**MUST.** The two fields are semantically different. Tags are
descriptive (`lint`, `mcp`, `security`); categories are positional
(`how-to`, `reference`, `explanation`).

**Why:** Mixing them produces a sidebar section named "lint" that
mixes reference and how-to pages — confusing.

**Enforced:** None.

## 7. Templates

### 7.1 Templates are categorized `Templates` and end in `.template.md`

**MUST.** A source-of-truth template that should be discoverable via
`docbuilder template list` must satisfy both. The discovery code
(`internal/templates/discovery.go`) parses the
`/categories/templates/` taxonomy page and looks for anchors whose
`href` contains `.template/`.

**Why:** If either fails, the template is silently invisible.

**Enforced:** Discovery will skip it (silent failure, not an error).

### 7.2 Template body is exactly one fenced markdown block

**MUST.** The parser (`internal/templates/template_page.go`) errors
on zero or multiple code blocks.

**Enforced:** Discovery fails at parse time.

### 7.3 Template output paths are relative to `docs/`

**MUST.** `output_path: guides/{{ .Slug }}.md` not
`output_path: /Users/me/project/docs/guides/{{ .Slug }}.md`.

**Why:** `WriteGeneratedFile` (`internal/templates/writer.go`)
rejects absolute paths and `..` traversal.

**Enforced:** Build (WriteGeneratedFile rejects).

### 7.4 Template field names match their use sites

**SHOULD.** A field declared in `schema.fields[]` as `key: "Title"`
should appear in the body as `{{ .Title }}`. Unused fields waste
prompt turns.

**Enforced:** None.

### 7.5 Templates carry one sequence when they emit sequence-numbered files

**SHOULD.** A single `sequence:` block, or none. Multiple sequences
are not supported.

**Enforced:** None.

## 8. Cross-cutting

### 8.1 Every directory has a `categories:` value on its pages, even landing pages

**MUST.** Both `_index.md` and leaf pages in the same directory
must share the directory's category. Otherwise the sidebar renders
them under different sections.

**Enforced:** None.

### 8.2 Sidebar ordering uses explicit `weight`

**SHOULD.** Don't rely on alphabetical-by-title or
Hugo's "first appearance" heuristic. Set `weight: <int>` on every
page. Recommended bands:

| Category | Weight band |
|---|---|
| `tutorials` | 10–29 |
| `how-to` | 30–59 |
| `reference` | 60–89 |
| `explanation` | 90–119 |
| `security` | 120–139 |
| `architecture-decisions` | 200+ (sequence already orders them) |

**Why:** Sidebar order is the user's mental model of the docs.
Alphabetical is rarely what you want.

**Enforced:** None.

### 8.3 Don't use `next` / `prev` for cross-mode links

**SHOULD.** Hugo's `next` / `prev` frontmatter walks the *directory*
siblings. Use it for tutorial pagination. For cross-mode links use
plain markdown links in a "See also" section.

**Why:** A how-to's `next`/`prev` should stay in `how-to/`. Cross-
mode navigation is what "See also" is for.

**Enforced:** None.

## 9. Anti-patterns

Things to avoid. Each one has been seen in the wild and produced
real friction.

### 9.1 Don't put build steps in a doc's frontmatter description

**AVOID.** The `description` field becomes a meta tag in the rendered
HTML. Build instructions belong in the body or in `params` under
`hugo`.

### 9.2 Don't use `description:` for content body

**AVOID.** Some doc systems use `description:` for an abstract.
Docbuilder uses it for SEO meta description. If you want an abstract,
use a normal `## Overview` section.

### 9.3 Don't bypass the linter with hand-rolled fingerprints

**AVOID.** `lint_fix` is the canonical way to populate
`fingerprint`. Hand-computed values use a different algorithm than the
linter expects (`mdfp` only excludes `fingerprint`; the linter also
excludes `lastmod`, `uid`, `aliases`).

### 9.4 Don't write `.docbuilder-cache/` or workspace state to disk

**AVOID.** Those are runtime artifacts. Don't commit them; don't add
docs that describe them as user-facing state.

### 9.5 Don't author templates at the docbuilder source-root

**AVOID.** Templates belong in `docs/examples/`. Authoring them at
the repo root breaks discovery (which scans the rendered site, not
the repo) and produces confusing URL paths.

### 9.6 Don't use `Templates` as a category name (use `Templates` only on template files)

**AVOID.** `Templates` is a category that exists to make template
files discoverable. Using it for how-to articles about templates
(e.g. `docs/how-to/author-templates.md`) produces a sidebar section
that mixes authoring guides with the actual templates — confusing
for both humans and discovery.

### 9.7 Don't create `_index.md` inside `adr/` or `examples/`

**AVOID.** These directories are flat collections of similarly-shaped
items. Hugo's auto-generated listing is the right answer here.

### 9.8 Don't put shell commands inside list items without fenced blocks

**AVOID.** Markdown loses indentation context inside list items.
Prefer fenced code blocks even for one-liners.

### 9.9 Don't reference URLs that aren't `/`-rooted or `.md`-relative

**AVOID.** Mixed link styles break broken-link detection. Pick one:
`[other](other.md)` for in-tree, `[name](https://...)` for
external.

### 9.10 Don't write multi-document content as one long file

**AVOID.** A single 2,000-line reference page is a maintenance
burden. Split into per-flag / per-section pages and link them.

## 10. What the linter enforces today

A snapshot of what `docbuilder lint` (and `lint_fix`) actually do.
This is what you can rely on without re-reading code:

| Rule | Behaviour | Source |
|---|---|---|
| `filename-conventions` | Lowercase + ASCII alphanumerics/hyphens/underscores/dots only | `internal/lint/rule_filename_conventions.go` (FilenameRule) |
| `frontmatter-uid` | ERROR if `uid` is missing or not a valid UUID | `internal/lint/rule_frontmatter_uid.go` |
| `frontmatter-fingerprint` | ERROR if `fingerprint` is missing or doesn't match content hash | `internal/lint/rule_frontmatter_fingerprint.go` |
| `lint_fix` auto-adds | canonical `/_uid/<uid>/` alias; populates `fingerprint`; updates `lastmod` when fingerprint changes | `internal/lint/fixer_*.go` |
| Broken-link detection | ERROR if a relative `.md` link target doesn't exist | `internal/lint/linter.go::LintPath` |

That's the entire enforced surface. Everything else in this doc is
**convention, not policy**.

## 11. Linter-feature candidates

Each unenforced SHOULD above is a candidate for a new linter rule.
Roughly ordered by ROI. Each candidate is tracked as a GitHub issue
so the work can be picked up incrementally:

1. **Body H1 detection** — flag `^# ` in the body (rule 5.1). —
   [#68](https://github.com/inful/docbuilder/issues/68)
2. **Directory / category consistency** — `docs/how-to/*.md` must
   have `categories: [how-to]` (rule 1.1 / 8.1). —
   [#69](https://github.com/inful/docbuilder/issues/69)
3. **`_index.md` per directory** — warn on directories with ≥3
   children and no `_index.md` (rule 2.2). —
   [#70](https://github.com/inful/docbuilder/issues/70)
4. **Category naming** — warn on `Templates` (capitalized) or other
   inconsistent casings (rule 1.1 / 6.4). —
   [#71](https://github.com/inful/docbuilder/issues/71)
5. **Required frontmatter completeness** — check `title`, `date`,
   `lastmod`, `categories`, `tags` are present (rule 4.4). —
   [#72](https://github.com/inful/docbuilder/issues/72)
6. **Sequence-prefix filenames** — `adr-NNN-slug.md` for known
   sequence directories (rule 3.3). —
   [#73](https://github.com/inful/docbuilder/issues/73)
7. **Cross-mode category check** — flag if a doc has both
   `categories: [how-to]` and `categories: [reference]` (rule 6.4). —
   [#74](https://github.com/inful/docbuilder/issues/74)
8. **Tag count** — warn when `len(tags) > 10` (rule 6.3). —
   [#75](https://github.com/inful/docbuilder/issues/75)
9. **Internal-link style** — flag `../foo` or site-rooted links in
   body text (rule 5.3). —
   [#76](https://github.com/inful/docbuilder/issues/76)

Each is a 30-100 line PR. Together they'd close most of the
"convention, not policy" gap in §10.

## 12. Verification

Use `docbuilder lint_docs` over `docs/` to confirm every rule in
this doc passes. The MCP server (`cmd/mcp-server/`) exposes this as a
tool:

```text
lint_docs { path: "docs/" }
```

The audit test in `cmd/mcp-server/lint_docs_audit_test.go` runs this
in CI. A follow-up `lint_fix` should bring the error count to zero
without manual intervention for everything except true content bugs
(invalid UID placeholders, missing categories).

## See also

- [Doc Page Structure](../reference/doc-page-structure.md) — the
  per-page schema (fields, body, filename).
- [Doc Template Structure](../reference/doc-template-structure.md) —
  the per-template schema.
- [Documentation Organization](documentation-organization.md) — a
  point-in-time audit of the docs/ tree this project.
- [Hugo sidebar configuration](../reference/configuration.md#hugo-sidebar)
  — the `hugo.sidebar.mode` / `group_by` knobs.
- [Index files reference](../reference/index-files.md) — how
  `_index.md` / `index.md` / `README.md` interact.
