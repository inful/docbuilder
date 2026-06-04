# Plan: `group_by` Conformance Lint (deferred)

**Status:** Deferred — to be implemented *after* N-level `group_by` max-depth nesting lands. See `plan/group-by-max-nesting.md` (to be created) for the upstream work.

**Owner:** TBD
**Date parked:** 2026-06-03

## Context

DocBuilder's sidebar `group_by` config (`hugo.sidebar.group_by`) is a *runtime contract* that varies per deployment. The renderer is permissive (docs with missing fields fall back to Repository-based grouping), so non-conformance is invisible at build time. We want doc authors to be told their docs are missing the front-matter fields the deployment expects, without depending on the build's own output.

## Mental model

```
docbuilder (build server)            docbuilder lint (separate machine)
        │                                       │
        │  resolves group_by from               │  resolves BASE_URL via
        │  local config + emits                 │  flag / env / config
        │  sidebar (already does)               │
        │                                       │
        │  publishes contract at                 │  fetches contract from
        │  <base_url>/sidebar/group_by.yaml     │  <base_url>/sidebar/group_by.yaml
        │                                       │
        ▼                                       ▼
   deployed site ──────────────────► lint runner checks docs
```

Not circular: the build server writes the contract once per deploy; the lint runner reads it once per invocation. Decoupled in time and machine. Reuses the same BASE_URL resolution as `docbuilder template list`.

## Locked decisions

| # | Question | Decision |
|---|---|---|
| — | Unreachable contract | Skip the linter step; emit single `SeverityInfo` entry under rule `sidebar-group-by-conformance` with message "contract unreachable; skipping conformance check". Lint run completes; exit code reflects only real warnings/errors. |
| — | Info-message shape | `SeverityInfo` `Issue` (option a). Hidden in `--quiet` mode (matches existing `internal/lint/linter.go:128-133` filter). |
| 1 | URL path on deployed site | `/sidebar/group_by.yaml` |
| 2 | First-deploy bootstrap | **Dropped.** No special first-time handling. The build publishes the contract as part of the regular output; the lint runner's no-contract case is already handled. |
| 4 | Local vs remote precedence | **Remote wins** when reachable and parseable. Local `config.SidebarConfig.GroupBy` is the offline fallback (used only when the remote fetch fails or the contract is malformed). |

## Defaults to ship against (speak up if any are wrong)

| # | Question | Default |
|---|---|---|
| 3 | Wire format | YAML mirroring in-config shape: `map[lower-case-category][]field` |
| 6 | Caching | Yes, local cache under `.docbuilder/contract-cache.yaml` with TTL; `--refresh-contract` flag to bypass |
| 7 | Multi-category issue shape | Pack category + missing field into `Issue.Message` (no new `Issue` field) |
| 8 | Severity for conformance issues | `Warning` (does not block builds) |
| 9 | Private docs (`public: true` filter) | Gate the rule on the same condition as the categories-menu stage |
| 10 | Contract drift | Separate rule `sidebar-group-by-drift`, also `Warning`, when local `group_by` is set and disagrees with the remote |
| 11 | Rule scope | All docs that declare a `categories` value (after the public-only gate) |
| 12 | Test surface | Golden (`test/integration/lint_golden_test.go`), unit (`internal/lint/`), and integration (local HTTP server returning a fixture contract) |

## Three parts

### Part A — Publish the contract from the build server

Extend `internal/hugo/categories_menu.go` (or its stage wrapper `categories_menu_stage.go`) to write the resolved `group_by` to the Hugo output as `sidebar/group_by.yaml`. The stage already consumes `config.SidebarConfig.GroupBy` and runs as part of the normal build.

**Sub-question deferred to implementation:** exact path under the Hugo output dir (Hugo `static/` convention vs `output/`). Pick the obvious one, document the choice in the PR.

### Part B — Fetch the contract on the lint runner

New function in `internal/templates/` (next to `http_fetch.go`) that:
- Resolves the BASE_URL with the same precedence as `template list`: `--base-url` flag → `DOCBUILDER_TEMPLATE_BASE_URL` env → `hugo.base_url` from config. Reuses `cmd/docbuilder/commands/template_common.go:10-23`.
- GETs `<base-url>/sidebar/group_by.yaml`.
- Parses YAML → `map[string][]string`; lowercases keys and fields (mirrors `internal/config/hugo.go:150-164`).
- Caches the result under `.docbuilder/contract-cache.yaml` with a TTL.
- Returns three distinguishable outcomes: `OK(contract)`, `Unreachable(err)`, `Malformed(err)`. The lint command maps these to: use the contract / emit info + skip / emit info + skip.

Wired into the `docbuilder lint` command so the contract is loaded before the linter is constructed.

### Part C — The conformance lint rule(s)

New `Rule`s in `internal/lint/`:

- **`sidebar-group-by-conformance`** — for each doc, parse front matter (`internal/frontmatterops.Read`), take `categories:`, and for each declared category, check that the front matter has a non-empty value for every field in the contract. Emit one `Issue` per `(file, category, missing-field)` with `SeverityWarning`.
- **`sidebar-group-by-drift`** — emitted once per lint run when local and remote `group_by` disagree. `SeverityWarning`. `Message` describes the diff.

`Linter` struct gains a `contract` field; `NewLinter` gains an optional contract argument. Both rules added to the `rules` slice in `NewLinter` (`internal/lint/linter.go:27-32`). Auto-fix: none for either rule.

## Success criteria

- [ ] `docbuilder lint` (no args) loads the contract from `<base-url>/sidebar/group_by.yaml` using the same URL resolution as `template list`.
- [ ] A doc with a declared category whose required field is missing in front matter produces a `sidebar-group-by-conformance` `Warning` naming the category and the field.
- [ ] A doc whose category has no contract entry produces no issue.
- [ ] A doc with no `categories:` produces no issue.
- [ ] A private doc (`public: true` not set) produces no issue.
- [ ] Unreachable contract emits a single `SeverityInfo` entry under rule `sidebar-group-by-conformance` with message "contract unreachable; skipping conformance check". Lint run completes; exit code reflects only real warnings/errors. Hidden in `--quiet`.
- [ ] Malformed contract behaves the same as unreachable.
- [ ] Local and remote `group_by` disagreeing emits a single `sidebar-group-by-drift` `Warning` describing the diff.
- [ ] Build server writes the resolved contract to the Hugo output at `sidebar/group_by.yaml` as part of the normal build.
- [ ] Golden tests pass; unit tests pass; integration test (local HTTP server) passes.
- [ ] No new config keys introduced; no new HTTP code; reuses `internal/templates/http_fetch.go`'s URL validation.

## What this plan does *not* do

- Does not change the `group_by` config shape or add N-level nesting.
- Does not change the rendering rule (A1 stays).
- Does not auto-fix missing fields.
- Does not introduce a new config key for the contract URL — reuses the existing BASE_URL resolution.

## What blocks re-engagement

1. **N-level `group_by` max-depth nesting** must land first, including:
   - Decision on max depth value (number of levels).
   - Decision on the empty-level fallback rule for the renderer (A1 / B1 / C / other).
   - The contract format must accommodate the new shape (likely a list of field-chains per category, not a flat list).
2. The linter rule above implicitly assumes the contract shape; once nesting lands, the contract format and the rule's parsing need to be re-confirmed in a small follow-up planning pass.
