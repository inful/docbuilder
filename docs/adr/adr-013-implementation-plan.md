# Plan: Implement ADR-013 (Use Goldmark for internal Markdown parsing)

- Status: Draft / Tracking
- Date: 2026-01-21
- ADR: adr-013-goldmark-for-internal-markdown-parsing.md

## Goal

Replace ad-hoc Markdown scanning in DocBuilder’s lint/fix workflows with a centralized Goldmark-based parsing layer for **analysis** (link discovery, broken-link detection, code-block skipping), while preserving **minimal surprise** behavior:

- Parse **Markdown body only** (frontmatter handled by `internal/frontmatter` per ADR-014)
- For any link rewrites, prefer **minimal-diff edits** (targeted byte-range patches), not Markdown re-rendering
- Keep behavior aligned with DocBuilder-generated Hugo `markup.goldmark` config

## Hard Requirements (Process)

This plan is a tracking tool and must be updated as work progresses.

For **every** step that changes code:

1. Write a failing test first (TDD)
2. Implement the smallest change to make the test pass
3. Run:
   - `go test ./... -count=1`
   - `golangci-lint run --fix` then `golangci-lint run`
4. Update this plan (mark the step as completed, add brief notes)
5. Commit changes **before** starting the next step
  - Commit messages must follow the **Conventional Commits** format (e.g., `feat(markdown): ...`, `fix(lint): ...`, `test(markdown): ...`).

## Acceptance Criteria (must stay green at the end)

- All tests pass: `go test ./... -count=1`
- No golangci-lint issues remain: `golangci-lint run`

## Non-goals (for this ADR implementation)

- Rendering Markdown to HTML (Hugo does that)
- Reformatting/normalizing Markdown output
- Broad refactors unrelated to Markdown parsing

## Design Constraints / Decisions

- Frontmatter is split/parsed via `internal/frontmatter`; Goldmark operates only on the body.
- Goldmark extensions/settings should mirror the generated Hugo config (`markup.goldmark`) insofar as they affect parsing.
- Link rewriting (when introduced) must avoid “format churn” and preserve user formatting.

---

## Tracking Checklist (Strict TDD)

### 0) Baseline + guardrails (no behavior change)

- [x] Add characterization tests for current link detection behavior (including known edge cases)
  - [x] Links inside fenced code blocks are ignored
  - [x] Links inside inline code spans are ignored
  - [x] Reference-style links and definitions are handled consistently
  - [x] Nested parentheses in URLs don’t produce false positives (documented current limitation)
  - [x] Escaped brackets/parentheses don’t break detection (documented current limitation)
- [x] Document current “known limitations” in test names (so improvements are explicit)

Notes (Step 0):
- Added `internal/lint/link_scanner_characterization_test.go` to capture current broken-link scanner behavior, including known limitations (tilde code fences, nested parentheses, escaped link text).

**Commit checkpoint:** `test(markdown): add characterization coverage for existing scanners`

### 1) Introduce `internal/markdown` package (parsing + visitors)

**Target initial API (minimal, internal):**

- `ParseBody(body []byte, opts Options) (ast.Node, error)` or equivalent
- `ExtractLinks(root ast.Node, source []byte) ([]Link, error)`

Where `Link` minimally captures:

- kind: inline link, image link, autolink, reference link usage, reference definition
- destination: raw URL/path as it appears
- source range for destination/definition (byte offsets) for future minimal-diff patches

TDD steps:

- [ ] Write failing unit tests for parsing a Markdown body and extracting:
- [x] Write failing unit tests for parsing a Markdown body and extracting:
  - [x] Inline links: `[text](dest)`
  - [x] Images: `![alt](dest)`
  - [x] Autolinks: `<https://...>`
  - [x] Reference link usages: `[text][ref]` and `[ref]` (resolved to Link nodes with destinations)
  - [x] Reference definitions: `[ref]: dest "title"` (extracted from Goldmark parser context)
  - [x] Ensure links in code blocks / inline code are excluded
- [x] Implement parser + visitor(s) to satisfy tests
- [x] Ensure parsing options mirror DocBuilder’s Hugo Goldmark config as needed (note: link parsing relies on CommonMark semantics; Hugo-specific renderer settings are not relevant at this step)

Notes (Step 1):
- Added `internal/markdown` with `ParseBody` and `ExtractLinks`.
- Goldmark does not represent reference definitions as AST nodes; they are retrieved from the parse context (`parser.Context.References()`).

**Commit checkpoint:** `feat(markdown): add goldmark-based body parser and link extraction`

### 2) Wire broken-link detection to Goldmark extraction (read-only behavior change)

Scope: switch broken-link detection to use `internal/markdown` link extraction rather than ad-hoc scanning.

- [x] Add failing tests that reproduce at least one known scanner bug (edge case) and verify Goldmark-based detection fixes it
- [x] Update broken-link detection codepaths to use `internal/frontmatter.Split` → parse body via `internal/markdown`
- [x] Keep output stable (same error format, same file/line reporting where applicable)

Notes (Step 2):
- `detectBrokenLinksInFile` now uses `internal/frontmatter.Split` and `internal/markdown.ExtractLinks`.
- Added coverage that ensures links inside `~~~` fenced code blocks are ignored.
- Updated the former “known limitation” characterization tests (tilde fences, nested parentheses, escaped link text) to the new intended behavior.

**Commit checkpoint:** `fix(lint): use goldmark for broken-link detection`

### 3) Wire fixer link discovery to Goldmark extraction (still read-only)

Scope: adopt AST-driven link discovery for fixer operations (e.g., ADR-012 link healing) without rewriting yet.

- [x] Add failing tests covering discovery parity with current fixer (mixed link types)
- [x] Switch fixer link discovery to use `internal/markdown` extracted links

Notes (Step 3):
- Uses Goldmark extraction as the primary source of links (robustly skips both ``` and ~~~ fenced code blocks).
- Supplements with a body-only legacy scan to preserve existing “minimal surprise” behavior where tests rely on permissive parsing (notably destinations containing spaces).
- Applies a frontmatter line offset so discovered link line numbers match original file positions for edit operations.

**Commit checkpoint:** `refactor(lint): use goldmark for fixer link discovery`

### 4) Implement minimal-diff link rewriting (byte-range patches)

Scope: when the fixer needs to change a link destination/definition, apply targeted byte-range edits to the original body.

Key correctness requirements:

- stable diffs: change only destination text, preserve formatting
- safe multi-edit: multiple links per file without corrupting offsets
- frontmatter untouched

TDD steps:

- [ ] Add failing unit tests for `ApplyEdits(source []byte, edits []Edit) ([]byte, error)`
  - [ ] Single inline link destination replacement
  - [ ] Multiple replacements (ensure reverse-order patching or offset adjustment)
  - [ ] Reference definition destination replacement
  - [ ] Preserve fragments `#...` and relative path prefixes `./` `../`
  - [ ] CRLF input preserved when joining with frontmatter (via `internal/frontmatter.Style`)
- [ ] Implement edit application logic
- [ ] Integrate into link healing/update path(s)
- [ ] Add integration-ish tests around `docbuilder lint --fix` link healing behavior if applicable in current test structure

**Commit checkpoint:** `feat(lint): minimal-diff link rewriting using goldmark source ranges`

### 5) Remove duplicated scanners once parity is achieved

- [ ] Identify obsolete ad-hoc scanners (internal/lint/*link* and any shared helpers)
- [ ] Delete or deprecate them, keeping public behavior the same
- [ ] Ensure all tests still pass and coverage remains strong

**Commit checkpoint:** `refactor(markdown): remove legacy link scanners after goldmark parity`

### 6) Final verification gate (must be clean)

- [ ] `go test ./... -count=1`
- [ ] `golangci-lint run --fix`
- [ ] `golangci-lint run`

**Commit checkpoint:** `chore: verify adr-013 implementation is green`

---

## Notes / Decisions Log

Update this section during implementation to record any decisions that affect behavior (e.g., which Goldmark extensions are enabled internally and why, how offsets are computed, how Windows paths are normalized).
