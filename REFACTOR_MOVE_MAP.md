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

- [x] Decide on canonical error package: **internal/errors** will be the single source of error types and helpers. All error creation, adapters, and context will be unified here. `internal/foundation/errors` and its adapters will be removed.

- [ ] Inventory all usages of `internal/foundation/errors` and its adapters (HTTP, CLI, etc.)
- [ ] Update all error creation and handling to use `internal/errors` exclusively
- [ ] Migrate HTTP and CLI adapters to use `internal/errors` types
- [ ] Remove `internal/foundation/errors` and related adapters
- [ ] Update all tests and documentation to reference the unified error system

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
