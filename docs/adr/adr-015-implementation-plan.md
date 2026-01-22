---
goal: "Implement ADR-015: central parsed document model (frontmatter + Markdown body)"
adr: "docs/adr/adr-015-centralize-parsed-doc-model.md"
version: "1.0"
date_created: "2026-01-21"
last_updated: "2026-01-22"
owner: "DocBuilder Core Team"
status: "Done"
tags: ["adr", "tdd", "refactor", "markdown", "frontmatter", "lint", "performance"]
---

# ADR-015 Implementation Plan: Centralize parsed document model

## Guardrails (must hold after every step)

- Strict TDD: write a failing test first (RED), then implement (GREEN), then refactor.
- After completing *each* step:
  - `go test ./...` passes
  - `golangci-lint run --fix` then `golangci-lint run` passes
  - This plan file is updated to mark the step completed (with date + commit hash)
  - A commit is created **before** moving on to the next step
- Commit messages must follow Conventional Commits.

## Acceptance Criteria (global)

- Consumers that need the same metadata (links/line mapping) get identical results for identical inputs.
- Existing link update behavior remains minimal-diff (byte-range edits); no Markdown re-rendering.
- No new parsing libraries are added (Goldmark stays the Markdown engine).
- All tests pass.
- All golangci-lint issues are fixed.
- At least one new test covers a multi-consumer scenario to prevent workflow drift.

## Status Legend

- [ ] Not started
- [x] Done (must include date + commit hash)

---

## Phase 0 — Baseline & discovery

### Step 0.1 — Verify baseline (tests + lint)

- [x] Run `go test ./...` and `golangci-lint run` on branch `central-doc`.
- [x] If baseline fails due to *unrelated* issues, stop and decide whether to:
  - fix them first (with a dedicated commit), or
  - defer and adjust the branch strategy.

**Completion**: _date:_ 2026-01-21  _commit:_ `78ee4e7`

**Commit message**: `chore: verify baseline for ADR-015 work` (or omit if no repo changes)

### Step 0.2 — Identify duplication hotspots (parsing workflow)

- [x] Locate current call sites doing: read → `frontmatter.Split` → `markdown.ExtractLinks` → line mapping/skip rules.
  - Primary hotspots to consolidate first:
    - `internal/lint/fixer_link_detection.go` (split + ExtractLinks + skippable lines + lineOffset)
    - `internal/lint/fixer_broken_links.go` (split + ExtractLinks + line attribution)
  - Other consumers (later phases):
    - `internal/linkverify/service.go` (frontmatter awareness)
    - `internal/hugo/indexes.go`, `internal/hugo/models/typed_transformers.go`, `internal/hugo/pipeline/transform_frontmatter.go` (split/join workflows)
- [x] Confirm initial migration target(s): start with `internal/lint` (fixer + broken-links) to freeze line-number behavior early.

**Completion**: _date:_ 2026-01-21  _commit:_ `82195fa`

**Commit message**: `docs(plan): note ADR-015 migration targets`

---

## Phase 1 — New package: `internal/docmodel`

### Step 1.1 — RED: doc model parse + split/join contract tests

Write failing unit tests for a new package `internal/docmodel`:

- [x] `Parse([]byte, Options)` returns a `ParsedDoc` with:
  - original bytes
  - frontmatter raw bytes (no delimiters)
  - body bytes
  - hadFrontmatter + `frontmatter.Style`
- [x] Frontmatter cases:
  - no frontmatter
  - empty frontmatter block (`---\n---\n`)
  - missing closing delimiter error matches `frontmatter.ErrMissingClosingDelimiter`
- [x] Round-trip join: no edits → output equals original bytes.

**Completion**: _date:_ 2026-01-21  _commit:_ `4a35836`

**Commit message**: `test(docmodel): add parse and split/join contract tests`

### Step 1.2 — GREEN: implement minimal `internal/docmodel` parsing

Implement `internal/docmodel` using existing primitives:

- [x] Use `internal/frontmatter.Split` and `internal/frontmatter.Join`.
- [x] Provide `Parse` and `ParseFile`.
- [x] Keep the API minimal and internal-only.

**Completion**: _date:_ 2026-01-21  _commit:_ `4a35836`

**Commit message**: `feat(docmodel): add ParsedDoc with frontmatter split/join`

### Step 1.3 — REFACTOR: tighten API + error contracts

- [x] Ensure `ParsedDoc` does not expose mutable slices directly (document immutability policy).
- [x] Ensure errors include context (path when using `ParseFile`).
- [x] Keep dependencies one-way: `docmodel` may depend on `frontmatter` + `markdown`; not the reverse.

**Completion**: _date:_ 2026-01-21  _commit:_ `9be21f3`

**Commit message**: `refactor(docmodel): harden API and error contracts`

---

## Phase 2 — Derived metadata: frontmatter fields, links, and line mapping

### Step 2.1 — RED: lazy frontmatter fields parsing tests

- [x] Add tests for `FrontmatterFields()` (or equivalent) that:
  - returns empty map when no frontmatter / empty frontmatter
  - returns parsed fields for valid YAML
  - returns error for invalid YAML

**Completion**: _date:_ 2026-01-21  _commit:_ `8000c4e`

**Commit message**: `test(docmodel): add frontmatter fields parsing tests`

### Step 2.2 — GREEN: implement frontmatter fields parsing

- [x] Implement using `frontmatter.ParseYAML`.
- [x] Prefer lazy evaluation (parse only when fields are requested), with optional eager mode via `Options` if needed.

**Completion**: _date:_ 2026-01-21  _commit:_ `8000c4e`

**Commit message**: `feat(docmodel): add frontmatter fields parsing`

### Step 2.3 — RED: shared line mapping + skippable rules tests

Goal: make line-number attribution consistent across consumers.

- [x] Add tests for docmodel line mapping that cover:
  - correct line offset when YAML frontmatter is present (opening + closing delimiter + fmRaw lines)
  - skipping fenced code blocks (``` and ~~~) and indented code blocks
  - skipping inline-code spans when searching for a destination
  - stable behavior when the same destination appears multiple times

(These tests should be based on current behavior in `internal/lint` to avoid breaking workflows.)

**Completion**: _date:_ 2026-01-21  _commit:_ `aa72624`

**Commit message**: `test(docmodel): add line mapping and skippable rules tests`

### Step 2.4 — GREEN: implement line mapping helpers in docmodel

- [x] Implement a small, reusable line index API, e.g.:
  - `LineOffset()` (from original file start to body start)
  - `FindNextLineContaining(target string, startLine int) int` (skips code blocks + inline code)
- [x] Ensure functions operate on the **body** but return line numbers in either:
  - body coordinates, plus a helper to convert to file coordinates, or
  - file coordinates directly (preferred for consumers).

**Completion**: _date:_ 2026-01-21  _commit:_ `aa72624`

**Commit message**: `feat(docmodel): add shared line mapping helpers`

### Step 2.5 — RED: links extraction parity tests (body-only)

- [x] Add tests that `ParsedDoc.Links()` matches `markdown.ExtractLinks(doc.Body(), markdown.Options{})` for:
  - inline links, images, autolinks, reference defs
  - permissive destinations with spaces
  - links inside inline code / fenced code are not returned

**Completion**: _date:_ 2026-01-21  _commit:_ `992918b`

**Commit message**: `test(docmodel): add links extraction parity tests`

### Step 2.6 — GREEN: implement `Links()` and `LinkRefs()` (links + line numbers)

- [x] Implement `Links()` as a thin wrapper around `markdown.ExtractLinks(body)`.
- [x] Add `LinkRefs()` (or similar) that enriches extracted links with line numbers via docmodel line mapping.
- [x] Preserve existing lint fixer behavior:
  - only include kinds that are updateable/searchable (inline, image, reference_definition)
  - ignore external URLs, fragment-only links, and empty destinations

**Completion**: _date:_ 2026-01-21  _commit:_ `992918b`

**Commit message**: `feat(docmodel): add links with line attribution`

---

## Phase 3 — Minimal-diff edits (no Markdown re-rendering)

### Step 3.1 — RED: apply edits round-trip and boundary tests

- [x] Add tests that applying edits:
  - only changes specified byte ranges in the body
  - preserves frontmatter bytes exactly
  - produces identical output when edits are empty

**Completion**: _date:_ 2026-01-21  _commit:_ `d6f68dd`

**Commit message**: `test(docmodel): add apply-edits tests`

### Step 3.2 — GREEN: implement `ApplyBodyEdits`/`ApplyEdits`

- [x] Use `markdown.ApplyEdits` on body bytes.
- [x] Re-join with `frontmatter.Join`.

**Completion**: _date:_ 2026-01-21  _commit:_ `d6f68dd`

**Commit message**: `feat(docmodel): support minimal-diff body edits`

---

## Phase 4 — Migrate first consumer: `internal/lint`

### Step 4.1 — RED: lock current lint behavior with regression tests

- [x] Add/extend tests so that link detection + broken link detection behavior is frozen before refactor.
- [x] Include frontmatter + repeated links + code-block edge cases.

**Completion**: _date:_ 2026-01-21  _commit:_ `1501ef5`

**Commit message**: `test(lint): add regression coverage before docmodel migration`

### Step 4.2 — GREEN: migrate lint broken-link detection to docmodel

Target: `detectBrokenLinksInFile`.

- [x] Replace ad-hoc split + extract with `docmodel.ParseFile` and `doc.LinkRefs()` / `doc.Links()` as appropriate.
- [x] Ensure reported line numbers are unchanged.

**Completion**: _date:_ 2026-01-21  _commit:_ `1501ef5`

**Commit message**: `refactor(lint): use docmodel for broken link detection`

### Step 4.3 — GREEN: migrate lint fixer link detection to docmodel

Target: `Fixer.findLinksInFile` / `findLinksInBodyWithGoldmark`.

- [x] Replace ad-hoc split/extract/lineOffset/skip logic with `docmodel`.
- [x] Ensure edit workflows still use line numbers compatible with `applyLinkUpdates`.

**Completion**: _date:_ 2026-01-21  _commit:_ `1501ef5`

**Commit message**: `refactor(lint): use docmodel for link detection and attribution`

### Step 4.4 — Drift-prevention test (multi-consumer scenario)

- [x] Add a test that exercises **two consumers** on the same input and asserts they agree on:
  - destinations found
  - line numbers (file coordinates)

Example: run broken-link detection and link-detection (for updates) over the same file and ensure shared line mapping rules are applied consistently.

**Completion**: _date:_ 2026-01-21  _commit:_ `2e5059f`

**Commit message**: `test: add multi-consumer docmodel parity regression`

---

## Phase 5 — Migrate remaining consumers (follow-up)

### Step 5.1 — Identify next consumer(s) and add RED tests

Likely targets (confirm during Step 0.2):

- `internal/linkverify` markdown frontmatter awareness (`ParseFrontMatter`)
- `internal/hugo` frontmatter transforms (pipeline stage)

**Completion**: _date:_ 2026-01-22  _commit:_ `81bffc2`

**Commit message**: `docs(plan): select next ADR-015 migration target`

### Step 5.2 — Migrate and remove duplication

- [x] Migrate chosen consumer(s) to `docmodel`.
- [x] Delete duplicated helper code where safe.

**Completion**: _date:_ 2026-01-22  _commit:_ `55fc33b`, `d517e3f`

**Commit message**: `refactor: migrate <consumer> to docmodel`

---

## Phase 6 — Final hardening

### Step 6.1 — Run full suite, lint, and tidy up

- [x] `go test ./... -count=1`
- [x] `golangci-lint run --fix` then `golangci-lint run`
- [x] Ensure new package has clear, minimal API and no import cycles.

**Completion**: _date:_ 2026-01-22  _commit:_ `aa88ac6`

**Commit message**: `chore: final polish for ADR-015`
