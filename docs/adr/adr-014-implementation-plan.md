# Plan: Implement ADR-014 (Centralize YAML frontmatter parsing/writing)

- Status: Draft / Tracking
- Date: 2026-01-20
- ADR: adr-014-centralize-frontmatter-parsing-and-writing.md

## Goal

Introduce a single internal component for YAML frontmatter splitting/parsing/writing and migrate all current call sites to it, while keeping behavior stable and diffs minimal where practical.

## Constraints

- **Strict TDD**: for each unit of behavior, add a failing test first, then implement, then refactor.
- **YAML-only** frontmatter, using `---` delimiters.
- Preserve DocBuilder’s existing behavior unless explicitly changed by ADR.
- Prefer small, incremental migrations (one consumer at a time).
- Use `github.com/inful/mdfp v1.2.0` parts-based API (`CalculateFingerprintFromParts(frontmatter, body)`) where fingerprinting is needed.

## Non-goals (for this implementation)

- Supporting TOML (`+++`) or JSON frontmatter.
- Implementing full Markdown parsing (ADR-013 work).
- Building a general-purpose “Hugo frontmatter compatibility layer”.

## Tracking Checklist (TDD-first)

## Commit checkpoints (required)

For any “checkpoint” commit during this ADR implementation:

- [ ] `go test ./... -count=1` passes
- [ ] `golangci-lint run --fix` followed by `golangci-lint run` passes

### 0) Baseline + guardrails

- [x] Capture current behaviors with characterization tests (before refactor)
  - [x] Add tests around current frontmatter edge cases (no frontmatter, empty, malformed, CRLF)
  - [x] Ensure tests cover both lint + pipeline paths that touch frontmatter
- [ ] Ensure module dependency is pinned
  - [x] `github.com/inful/mdfp` is `v1.2.0` in `go.mod`

### 1) Create new package: `internal/frontmatter`

**Target public surface (initial):**

- `Split(content []byte) (frontmatter []byte, body []byte, had bool, style Style, err error)`
- `ParseYAML(frontmatter []byte) (map[string]any, error)`
- `SerializeYAML(fields map[string]any, style Style) ([]byte, error)`
- `Join(frontmatter []byte, body []byte, had bool, style Style) []byte`

**`Style` should minimally capture:**

- newline style: `\n` vs `\r\n`
- whether the input had a frontmatter block (`had` already returns this, but `Style` may still store delimiter/newline normalization choices)
- whether the original file had a trailing newline

#### 1.1 Split / Join

- [x] Write failing unit tests for `Split` in `internal/frontmatter/frontmatter_test.go`
  - [x] No frontmatter: content starts without `---`
  - [x] YAML frontmatter: `---\n<yaml>\n---\n<body>`
  - [x] CRLF variant: `---\r\n...` and ensure round-trip preserves CRLF
  - [x] Empty frontmatter block: `---\n---\n<body>` (define expected `had` and `frontmatter` content)
  - [x] Malformed: starts with `---` but missing closing delimiter (must return an error)
  - [ ] Leading BOM (optional): treat BOM as part of body unless we explicitly decide otherwise
- [x] Implement minimal `Split` until tests pass
- [x] Write failing unit tests for `Join`
  - [x] Round-trip property: `Join(Split(x)) == x` for representative inputs
  - [ ] Preserve trailing newline behavior
- [x] Implement `Join` until tests pass

#### 1.2 Parse / Serialize

- [x] Write failing unit tests for `ParseYAML`
  - [x] Valid YAML maps
  - [x] Empty frontmatter YAML (should return empty map)
  - [x] Invalid YAML (returns error)
- [x] Implement `ParseYAML` using `gopkg.in/yaml.v3`

- [ ] Decide determinism strategy for `SerializeYAML` (TDD via golden assertions)
  - [x] Option A (simpler): deterministic output by sorting keys and encoding via `yaml.Node` (accepts key re-ordering)
  - [ ] Option B (better diffs): keep order using `yaml.Node` and preserve existing order when editing (requires extra work)
- [x] Write failing tests for `SerializeYAML` covering:
  - [x] stable output across runs for same input
  - [x] newline style matches `Style`
  - [x] ends with newline (or preserves prior behavior)
- [x] Implement `SerializeYAML` until tests pass

### 2) Migrate one consumer first: `internal/linkverify`

Goal: reduce risk by migrating a read-only consumer first.

- [x] Add a failing test in `internal/linkverify` that exercises frontmatter extraction behavior currently used
- [x] Refactor `internal/linkverify/service.go` to use `internal/frontmatter.Split` + `ParseYAML`
- [x] Ensure tests pass

### 3) Migrate build pipeline frontmatter transform

Target files:

- `internal/hugo/pipeline/transform_frontmatter.go`

- [ ] Add failing tests for the transform (prefer existing test patterns in `internal/hugo/pipeline`)
  - [ ] Ensure frontmatter is preserved/normalized as expected
  - [ ] Ensure behavior is unchanged for “no frontmatter” files
- [x] Refactor transform to use `internal/frontmatter` package
- [x] Ensure tests pass

**Note (process deviation):** For Part 3 we proceeded without adding *new* transform-specific tests.
We treated the existing characterization coverage in `internal/hugo/pipeline/pipeline_test.go` (which asserts `parseFrontMatter` behavior across frontmatter edge cases) as satisfying the intent of the “add failing tests” checkbox, and continued to avoid duplicating coverage.

### 4) Migrate fingerprint transform to parts-based API (mdfp v1.2.0)

Target file:

- `internal/hugo/pipeline/transform_fingerprint.go`

Goals:

- Stop using `mdfp.ProcessContent(...)` where we can (avoid full-document rewrite)
- Compute fingerprint via `mdfp.CalculateFingerprintFromParts(frontmatter, body)`
- Update **only** the YAML `fingerprint` field (and `lastmod` via ADR-011 policy where applicable)

- [x] Add failing tests covering:
  - [x] adding fingerprint to docs with no fingerprint
  - [x] updating fingerprint when body changes
  - [x] ensuring non-fingerprint YAML fields remain unchanged
  - [x] ensuring body is unchanged
- [x] Implement by:
  - [x] `Split` → `ParseYAML` → compute fingerprint via `CalculateFingerprintFromParts` → set `fingerprint` → `SerializeYAML` → `Join`
- [x] Ensure tests pass

### 5) Migrate lint/fixer frontmatter helpers

Target areas (expected):

- `internal/lint/rule_frontmatter_fingerprint.go`
- `internal/lint/*frontmatter*` rules and any UID/lastmod utilities
- `internal/lint/fixer.go` (any frontmatter writes)

Approach:

- migrate rule-by-rule, keeping behavior stable

- [ ] Fingerprint rule first
  - [x] Add failing tests for lint rule behavior (verify + fix)
  - [x] Refactor to use `internal/frontmatter` + `mdfp.CalculateFingerprintFromParts`
- [ ] UID rule(s)
  - [ ] Add failing tests ensuring UID insertion/preservation stays stable
  - [ ] Refactor to use `internal/frontmatter`
- [ ] lastmod rule(s)
  - [ ] Add failing tests per ADR-011 interaction
  - [ ] Refactor to use `internal/frontmatter`

### 6) Delete duplicated implementations

- [ ] Identify and remove old frontmatter helpers (only after all migrations are complete)
- [ ] Ensure no other packages parse frontmatter ad-hoc

### 7) Verification checklist (must stay green)

- [ ] `gofmt ./...`
- [ ] `go test ./... -count=1`
- [ ] `go test ./test/integration -v` (golden tests)
- [ ] `golangci-lint run --fix`
- [ ] `golangci-lint run`

## Notes / Decisions to record during implementation

- Decide and document how `Split` treats:
  - empty frontmatter blocks
  - leading BOM
  - malformed frontmatter (error vs treat-as-body)
- Decide determinism rules for YAML serialization (and whether preserving key order is required for “minimal diffs”).
