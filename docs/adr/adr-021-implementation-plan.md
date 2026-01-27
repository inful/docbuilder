---
aliases:
  - /_uid/eaad3c8b-1c8a-4d4d-a3eb-ff4e7bbebf4c/
categories:
  - architecture-decisions
date: 2026-01-26T00:00:00Z
fingerprint: 5d2d238dd2bb1b39779353b02b5984f5dc177fec67259c71ff7fdd3b985d9cd8
lastmod: "2026-01-27"
tags:
  - daemon
  - events
  - implementation-plan
uid: eaad3c8b-1c8a-4d4d-a3eb-ff4e7bbebf4c
---

# ADR-021 Implementation Plan: Event-driven daemon updates and debounced builds

This plan intentionally evolves the daemon without a big-bang rewrite.

## What “done” looks like

- Webhook storms coalesce into one build (quiet window + max delay).
- Webhook handling never narrows the rendered/published site scope.
- A webhook updates one repo (when possible), but the build renders the full site.
- Triggers publish events; update/build logic lives in workers.

## Documentation updates (cross-cutting)

In addition to code changes, this ADR introduces new operator-facing behavior (debounce timing, update-one/rebuild-all, eventual consistency). We should document these changes explicitly.

Planned doc touchpoints:

- Configuration reference: `docs/reference/configuration.md`
  - Document any new daemon settings for build debouncing (e.g., quiet window and max delay).
  - Clarify semantics:
    - “update one, rebuild all” (targeted update does not narrow site scope)
    - “build uses branch HEAD at build time” (eventual consistency)

- Webhook setup guide: `docs/how-to/configure-webhooks.md`
  - Explain the new flow:
    - webhook publishes `RepoUpdateRequested`
    - repo update detects SHA movement and only then requests a build
    - build requests are debounced/coalesced
  - Add an operator note: a webhook does not necessarily produce an immediate build (quiet window).

- CLI / ops reference (as applicable): `docs/reference/cli.md`
  - If we add debug flags, commands, or event/bus introspection, document them.

- Observability / metrics docs (as applicable)
  - If we add metrics (coalesce count, time-to-build, queue depth), document names and meaning.

Acceptance criteria:

- Operators can answer “why didn’t a webhook build immediately?” from docs.
- New config knobs and semantics are documented in the configuration reference.

## Phase 0: Document invariants (no code)

- Define “coherent-site-first” invariants:
  - a build always renders the full site repo set
  - publishing remains atomic
- Define idempotency expectations:
  - webhook retries must be safe
  - overlapping schedules must coalesce

- Document correctness expectations:
  - eventual consistency is acceptable
  - builds use the HEAD of the configured branch at build time

Acceptance criteria:

- ADR-021 invariants are explicitly documented in the codebase (docs).

## Phase 1: Introduce an in-process event bus (foundation)

- Add `internal/daemon/events` (lightweight in-process pub/sub), integrated with `internal/eventstore` for optional auditing:
  - event interface/type union
  - dispatcher with buffered channels
  - simple `Publish(Event)` + `Subscribe(type)`
- Add unit tests:
  - publish/subscribe delivery
  - backpressure behavior (bounded buffers)

Note: `internal/eventstore` already exists and is primarily used for build telemetry/history. We should avoid turning it into a mandatory dependency for orchestration, but we can record orchestration summaries there if useful.

Acceptance criteria:

- Event bus supports clean shutdown and bounded buffering.
- Tests cover publish/subscribe and backpressure.

## Phase 2: Build debouncer / coalescer

- Implement `BuildDebouncer`:
  - accepts `BuildRequested` events
  - waits for `quietWindow` (e.g. 10s) before emitting `BuildNow`
  - enforces `maxDelay` (e.g. 60s)
  - if a build is already running, coalesce into a single “build again” request
- Add tests:
  - burst coalesces to single build
  - maxDelay forces build
  - build-running scenario queues exactly one follow-up

Acceptance criteria:

- Given N build requests within the quiet window, exactly one build trigger fires.
- Given continuous requests, a build still fires by maxDelay.

## Phase 3: Event wiring (triggers)

This phase was implemented incrementally using a “path of least resistance” approach.

- Webhook handler publishes `RepoUpdateRequested` (implemented):
  - `RepoUpdateRequested{Immediate:true, RepoURL, Branch}`
  - `RepoUpdater` detects remote HEAD movement and only then requests a build.
  - Consumers still perform a full-site build (scope is never narrowed).
  - The `Immediate:true` flag bypasses the quiet window but still respects “build running → emit one follow-up”.

- Scheduled tick publishes `BuildRequested` (explicit repo mode):
  - `BuildRequested{Reason:"scheduled build"}`

- Discovery completion publishes `BuildRequested` (forge mode):
  - `BuildRequested{Reason:"discovery"}`

Note: the intended longer-term flow is now in place:
`RepoUpdateRequested` → (RepoUpdater checks/updates that repo) → `RepoUpdated(changed=true)` → `BuildRequested`.

- Ensure discovery diffs publish removal events:
  - `RepoRemoved` (or equivalent)

Acceptance criteria:

- Webhook handlers only parse/validate and publish orchestration events.
- Removal is represented as a first-class event.

## Phase 4: Repository update worker

- Implement `RepoUpdater`:
  - Full update: refresh known clones or check remote heads; emit `RepoUpdated` per repo
  - Single update: refresh/check one repo; emit `RepoUpdated`
  - Determine “changed” primarily via commit SHA movement (eventual consistency; HEAD-of-branch)
  - Optionally determine `docsChanged` using cheap signals (quick hash), and treat it as an optimization hint
- Wire `RepoUpdated(changed=true)` → `BuildRequested`

This phase must explicitly support: webhook → single repo update → rebuild if changed.

Acceptance criteria:

- A webhook-triggered repo update publishes `RepoUpdated(changed=true)` only when SHA moves.
- A build request is emitted only after change detection.

## Phase 5: Build execution remains canonical

- When debouncer emits `BuildNow`, enqueue a normal build job using the full repo set.
- Keep existing serialization to prevent concurrent staging/output clobbering.

Decision: even when only one repository was updated, the build still renders the full site (“update one, rebuild all”).

Acceptance criteria:

- Builds triggered from webhooks render/publish the full repo set.
- Site output remains coherent (search/index/taxonomies consistent).

### Job IDs under coalescing (operational semantics)

When requests are coalesced, multiple triggers may map to a single build job. To keep IDs stable and non-misleading:

- Triggers should reuse the debouncer’s planned job ID when one is already pending.
- Webhook endpoints return the planned job ID (so bursts return a stable ID that corresponds to the actual build).
- Scheduled/discovery triggers also reuse the planned job ID to avoid logging “phantom” job IDs that won’t be enqueued.

## Phase 6: Optional correctness upgrade (snapshot builds)

- Represent a “snapshot” as `{repoURL: commitSHA}` produced by repo update stage.
- Teach build to optionally:
  - checkout exact SHAs
  - skip `fetch` if already at desired SHA
- This enables strict “build corresponds to event state” semantics.

Note: snapshot builds are optional because Phase 0 explicitly accepts eventual consistency.

Acceptance criteria:

- Snapshot builds (if implemented) can pin repo → SHA for strict “what was built”.

## Rollout strategy

- Start with the debounced build path only for webhooks (biggest storm source).
- Keep scheduled builds unchanged initially.
- Add metrics:
  - debouncer coalesce count
  - time-to-build after first trigger
  - repos updated per cycle

## Migration / compatibility

- Preserve existing config fields and HTTP endpoints.
- Keep the build pipeline untouched initially; only rewire triggers into events.

## Cleanup / simplification tasks (planned removals)

ADR-021 is expected to simplify the daemon over time. We should treat these as explicit tasks, not “maybe later”.

Planned simplifications:

- Make triggers thin
  - Webhook/schedule/admin endpoints should only validate inputs and publish events.
  - Remove trigger code that decides build scope or repo set.

- Ensure a single build gate
  - Only `BuildDebouncer` (or a single gate component) should emit `BuildNow`.
  - Remove scattered coalescing/backoff logic elsewhere.

- Converge on one canonical build entry point
  - Route all builds through the same build runner/queue path so semantics stay consistent.

- Centralize shutdown behavior
  - Avoid bespoke goroutine lifecycles per trigger; use dispatcher/worker shutdown semantics.

Acceptance criteria:

- No trigger path calls update/build logic directly.
- No trigger path computes the daemon’s site repo set.
- There is exactly one component that decides when to start builds.
