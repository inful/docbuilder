# DocBuilder Refactor: Cohesive Structure & Error Unification

## Overview
This document tracks the planned moves and refactors to achieve a more cohesive, maintainable repository structure and unify error handling. Each phase is a checklist; check off as you complete each step.

---

## Phase 1: Planning & Preparation
- [x] Create dedicated branch for refactor
- [x] Draft move map and checklist (this file)
- [ ] Communicate plan to team (PR, issue, etc.)

---

## Phase 2: Error System Unification
- [ ] Decide on canonical error package (internal/errors vs internal/foundation/errors)
- [ ] Migrate all error creation and adapters to canonical system
- [ ] Remove legacy/duplicate error types and adapters
- [ ] Update all tests and docs

---

## Phase 3: Daemon/Server Extraction
- [ ] Create `internal/server/` (or `internal/daemon/` → `internal/server/`)
- [ ] Move HTTP API, handlers, middleware, and httpx helpers into cohesive subpackages:
    - `api/` (route registration, OpenAPI, etc.)
    - `handlers/` (endpoint logic)
    - `middleware/` (auth, logging, recovery, etc.)
    - `httpx/` (helpers, adapters)
- [ ] Update imports and references
- [ ] Move examples/fixtures to `examples/` and `testdata/`
- [ ] Update docs and tests

---

## Phase 4: Type Tightening & Cleanup
- [ ] Replace remaining `interface{}` and `map[string]any` in daemon state/config/theme params with typed structs/aliases
- [ ] Remove or refactor weakly-typed surfaces in transforms, registry, etc.
- [ ] Update tests and docs

---

## Phase 5: CI & Guardrails
- [ ] Add CI checks to forbid `http.Error`, legacy symbol reintroduction, and weak typing in new code
- [ ] Document new conventions

---

## Progress Tracking
- Use this checklist to track each phase. Mark items as complete in PRs or issues as you go.
- Keep tests green after each major move.

---

## Notes
- Each phase should be a separate PR if possible.
- Communicate breaking changes early.
- Use `make build` and `go test ./...` after each major move.
