---
goal: "Implement ADR-016: centralize frontmatter mutations (map-based ops)"
adr: "docs/adr/adr-016-centralize-frontmatter-mutations.md"
version: "1.0"
date_created: "2026-01-22"
last_updated: "2026-01-22"
owner: "DocBuilder Core Team"
status: "Draft"
tags: ["adr", "tdd", "refactor", "frontmatter", "hugo", "lint", "fingerprint", "uid"]
uid: "6df43140-ba90-4590-b923-0847aabee743"
---

# ADR-016 Implementation Plan: Centralize frontmatter mutations (map-based ops)

Related ADR: [adr-016-centralize-frontmatter-mutations.md](adr-016-centralize-frontmatter-mutations.md)

## Guardrails (must hold after every step)

- Strict TDD: write a failing test first (RED), then implement (GREEN), then refactor.
- After completing *each* step:
  - `go test ./...` passes
  - `golangci-lint run --fix` then `golangci-lint run` passes
  - This plan file is updated to mark the step completed (with date + commit hash)
  - A commit is created **before** moving on to the next step
- Keep behavior stable: avoid output changes unless the ADR explicitly intends them.

## Acceptance Criteria (global)

- Build pipeline and lint/fix compute the same fingerprint for the same (frontmatter, body) input, using one shared helper.
- UID/alias behavior is identical in all code paths that can write it.
- Key naming drift is reduced:
  - `editURL` remains the canonical map-based output key.
  - Readers accept both `editURL` and `edit_url` and normalize.
- No regression in golden/integration tests.

## Status Legend

- [ ] Not started
- [x] Done (must include date + commit hash)

---

## Phase 0 — Baseline & scope confirmation

### Step 0.1 — Verify baseline (tests + lint)

- [x] Run `go test ./... -count=1`.
- [x] Run `golangci-lint run`.
- [ ] If baseline fails due to unrelated issues, stop and decide whether to:
  - fix them first (with a dedicated commit), or
  - defer and adjust branch strategy.

**Completion**: _date:_ 2026-01-22  _commit:_ `n/a` (baseline verification only)

---

## Phase 1 — New package: `internal/frontmatterops`

### Step 1.1 — RED: contract tests for read/write convenience

Add failing unit tests (new package) covering:

- Read behavior for:
  - no frontmatter
  - empty frontmatter block
  - valid YAML frontmatter
  - malformed frontmatter (unterminated)
- Write behavior:
  - `had=false` returns body as-is
  - `had=true` emits deterministic YAML + joins with correct newlines

**Completion**: _date:_ 2026-01-22  _commit:_ `152dc12`

### Step 1.2 — GREEN: implement `Read`/`Write`

Implement `internal/frontmatterops` with:

- `Read(content []byte) (fields map[string]any, body []byte, had bool, style frontmatter.Style, err error)`
- `Write(fields map[string]any, body []byte, had bool, style frontmatter.Style) ([]byte, error)`

Constraints:

- Delegate splitting/parsing/serializing/joining to `internal/frontmatter`.
- Prefer minimal behavior differences vs existing call sites.

**Completion**: _date:_ 2026-01-22  _commit:_ `152dc12`

---

## Phase 2 — Canonical mutators (policy helpers)

### Step 2.1 — RED: UID + aliases helpers

Add failing tests for:

- `EnsureUID(fields)` generates a new UID only when missing.
- `EnsureUIDAlias(fields, uid)` ensures `aliases` contains `/_uid/<uid>/` with stable behavior across:
  - `aliases: []string`
  - `aliases: []any`
  - `aliases: string`
  - `aliases: null` / missing

**Completion**: _date:_ 2026-01-22  _commit:_ `152dc12`

### Step 2.2 — GREEN: implement UID helpers

Implement:

- `EnsureUID(fields map[string]any) (uid string, changed bool, err error)`
- `EnsureUIDAlias(fields map[string]any, uid string) (changed bool, err error)`

**Completion**: _date:_ 2026-01-22  _commit:_ `152dc12`

### Step 2.3 — RED: required Hugo base fields helpers

Add tests for:

- `EnsureTypeDocs(fields)` sets `type: docs` only when missing.
- `EnsureTitle(fields, fallback)` sets title only when missing/empty.
- `EnsureDate(fields, commitDate, now)` sets date only when missing; preserves existing string/time shapes.

**Completion**: _date:_ 2026-01-22  _commit:_ `14db03c`

### Step 2.4 — GREEN: implement base fields helpers

Implement:

- `EnsureTypeDocs(fields map[string]any) (changed bool)`
- `EnsureTitle(fields map[string]any, fallback string) (changed bool)`
- `EnsureDate(fields map[string]any, commitDate time.Time, now time.Time) (changed bool)`

**Completion**: _date:_ 2026-01-22  _commit:_ `14db03c`

---

## Phase 3 — Canonical fingerprinting (shared build + lint)

### Step 3.1 — RED: fingerprint canonicalization tests

Add tests that lock the canonical hashing form:

- `ComputeFingerprint` excludes at least: `fingerprint`, `lastmod`, `uid`, `aliases`.
- Hashing uses LF serialization and trims a single trailing newline.
- Fingerprint is stable across key ordering differences.

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

### Step 3.2 — GREEN: implement `ComputeFingerprint`

Implement:

- `ComputeFingerprint(fields map[string]any, body []byte) (string, error)`

Notes:

- Use `internal/frontmatter.SerializeYAML` for canonical serialization.
- Use `mdfp.CalculateFingerprintFromParts(frontmatter, body)` for hashing.

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

### Step 3.3 — RED: fingerprint upsert + ADR-011 lastmod tests

Add tests for:

- `UpsertFingerprintAndMaybeLastmod` updates `fingerprint` when changed.
- When fingerprint changes, sets/updates `lastmod` to today’s UTC date (`YYYY-MM-DD`) per ADR-011.
- When fingerprint does not change, `lastmod` remains unchanged.

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

### Step 3.4 — GREEN: implement fingerprint upsert

Implement:

- `UpsertFingerprintAndMaybeLastmod(fields map[string]any, body []byte, now time.Time) (changed bool, err error)`

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

---

## Phase 4 — Migrate consumers incrementally

### Step 4.1 — Migrate Hugo pipeline fingerprinting

Targets:

- `internal/hugo/pipeline/transform_fingerprint.go`

Plan:

- [ ] Add a failing test demonstrating current behavior that must remain stable.
- [ ] Refactor to call `frontmatterops.ComputeFingerprint` / `UpsertFingerprintAndMaybeLastmod`.
- [ ] Ensure no output differences vs current tests.

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

### Step 4.2 — Migrate lint fingerprint rule + fixer

Targets:

- `internal/lint/rule_frontmatter_fingerprint.go`
- `internal/lint/fixer.go` (fingerprint update path)

Plan:

- [ ] Add failing tests if coverage is missing.
- [ ] Refactor to use the shared ops helpers.

**Completion**: _date:_ 2026-01-22  _commit:_ `a802ce5`

### Step 4.3 — Migrate lint UID insertion + alias preservation

Targets:

- `internal/lint/fixer_uid.go`

Plan:

- [ ] Add failing tests if coverage is missing.
- [ ] Refactor to route all UID/alias mutation through ops.

**Completion**: _date:_ 2026-01-22  _commit:_ `3a60f1e`

### Step 4.4 — Migrate index generation helpers

Targets:

- `internal/hugo/indexes.go` (`ensureRequiredIndexFields`, `reconstructContentWithFrontMatter`, template parsing paths)

Plan:

- [ ] Add a characterization test if needed.
- [ ] Replace ad-hoc field setting + serialize/join with ops helpers.

**Completion**: _date:_ ____  _commit:_ `____`

**Completion**: _date:_ 2026-01-22  _commit:_ `149a660`

---

## Phase 5 — Cleanup

### Step 5.1 — Remove duplicated helpers

- [ ] Identify any remaining ad-hoc split/parse/mutate/serialize/join loops for the fields covered by ops.
- [ ] Remove or refactor them to use `internal/frontmatterops`.

**Completion**: _date:_ ____  _commit:_ `____`

**Completion**: _date:_ 2026-01-22  _commit:_ `TBD`

### Step 5.2 — Final verification

- [ ] `go test ./... -count=1`
- [ ] `go test ./test/integration -v`
- [ ] `golangci-lint run --fix` then `golangci-lint run`

**Completion**: _date:_ ____  _commit:_ `____`
