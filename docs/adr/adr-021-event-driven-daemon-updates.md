---
aliases:
  - /_uid/6dbdbcb0-6ed4-4b8f-8f1c-4cd14a89de14/
categories:
  - architecture-decisions
date: 2026-01-26T00:00:00Z
fingerprint: 3fdf49c4ca4e864f46520fc5c444683eeb451a731bb46949a8a16725a39203c8
lastmod: "2026-01-27"
tags:
  - daemon
  - events
  - webhooks
  - discovery
  - build
  - git
uid: 6dbdbcb0-6ed4-4b8f-8f1c-4cd14a89de14
---

# ADR-021: Event-driven daemon updates and debounced builds

**Status**: Proposed
**Date**: 2026-01-26
**Decision Makers**: DocBuilder Core Team

## Decision summary

DocBuilder daemon will become event-driven internally.

- Introduce a small, typed, in-process orchestration event bus (single daemon; no external broker).
- Separate responsibilities explicitly: discovery, repo update, and build are distinct workflows.
- Webhooks/schedules/admin endpoints only publish events; they do not run update/build logic directly.
- Debounce builds (quiet window + max delay) to coalesce webhook storms.
- Correctness model is eventual consistency: builds render the current branch HEAD at build time.
- Coherent-site-first output: update one repository if needed, but rebuild and publish the full site.

## Terminology

- **Orchestration events**: in-process control flow (what should happen next).
- **Telemetry events**: build history and observability stored in `internal/eventstore`.
- **Repo update**: refresh local clone(s) and compute repo state (e.g., commit SHA change).
- **Build**: generate and atomically publish a coherent Hugo site.
- **Repo set**: the set of repositories that define the site (configured or last discovery result).

## Context and problem statement

DocBuilder already has an event mechanism in `internal/eventstore` used for **build history** (BuildStarted/BuildCompleted and stage-level events).

That system is keyed by `build_id` and is optimized for telemetry/audit: it records what happened during a build. It is not designed to be the daemon’s orchestration mechanism.

Today, daemon mode effectively treats “build” as a single end-to-end transaction:

```
Update repos → Discover docs → Transform → Generate Hugo site → Atomically publish
```

This is correct, but it couples concerns and makes some operational behaviors hard:

- A webhook cannot cleanly express “repo X changed” without implicitly invoking the entire pipeline.
- Under webhook storms, rebuilding on every webhook is wasteful.
- If a webhook trigger narrows the repo list, it can accidentally narrow what the site renders.
- Operators want predictable behavior: fast enough freshness without rebuilding dozens of times.

## Goals

- Make daemon workflows explicit and composable: discovery, repo update, and build.
- Preserve coherent-site-first semantics and atomic publishing.
- Provide debounced build behavior that coalesces bursts.
- Keep the daemon single-instance and operationally simple.

## Non-goals

- Multi-replica / HA daemon support (leader election, distributed locks).
- Requiring a durable broker (Kafka/NATS/Redis Streams).
- Rewriting the build pipeline stages (ADR-008 remains the foundation).

## Decision

We will introduce an **in-process orchestration event bus** and model daemon work as event-driven workflows.

### Invariants

- **Coherent-site-first**: a build renders and publishes the full site for the daemon’s repo set.
- **Atomic publishing** remains unchanged (stage → promote).
- **Update one, rebuild all**: targeted triggers only reduce the update work, not the rendered/published scope.
- **Serialized builds**: builds remain single-flight to avoid output clobbering.

### Correctness model

We adopt an **eventually consistent** model:

- Triggers indicate that work should happen, not a strict promise of “build commit X”.
- When a build runs, it builds the **current HEAD of each repository’s configured branch** at build time.

Follow-up work may add “snapshot builds” (repo → commit SHA mapping) for stricter semantics, but it is not required for the main goals.

### Event sources

Webhooks are not special from an architecture perspective; they are just one event source. The daemon should route all triggers through the same event bus:

- webhook HTTP handlers
- scheduled ticks
- manual/admin endpoints

Webhook handlers validate/parse the payload and publish events. They do not directly run update/build logic.

### Event taxonomy (conceptual)

Orchestration events (new):

- `DiscoveryRequested`
- `DiscoveryCompleted(repos)`
  - derived: `RepoAdded`, `RepoRemoved`, `RepoMetadataChanged`
- `RepoUpdateRequested(repoURL, branch)`
- `RepoUpdated(repoURL, oldSHA, newSHA, changed, docsChanged)`
- `BuildRequested(reason, repoURL?)`
- `BuildNow` (emitted by the debouncer)

Telemetry events (existing):

- `BuildStarted` / `BuildCompleted` (+ existing stage telemetry)

### Relationship to `internal/eventstore`

To avoid two unrelated “event” concepts:

- Orchestration events are in-process control flow.
- `internal/eventstore` remains the system of record for build history/telemetry.

We may optionally append orchestration summaries into `internal/eventstore`, but durable orchestration is not required for this ADR.

## Design outline (high level)

New daemon components:

- **Event bus**: typed events, buffered channels, clean shutdown.
- **RepoUpdater**: full update or single-repo update; emits `RepoUpdated`.
- **BuildDebouncer**: coalesces `BuildRequested` and emits `BuildNow`.
- **Build runner**: on `BuildNow`, enqueue the canonical build using the full repo set.

## Example flow: webhook → update one repo → rebuild full site

This ADR explicitly supports the following scenario:

1. The daemon receives a webhook for a specific repository.
2. The webhook handler validates/parses the payload and publishes `RepoUpdateRequested(repoURL, branch)`.
3. `RepoUpdater` refreshes only that repository (fetch/fast-forward to branch HEAD).
4. If the updater detects a change (`oldSHA != newSHA`), it publishes `RepoUpdated(...changed=true...)` and then `BuildRequested(reason=webhook, repoURL=...)`.
5. `BuildDebouncer` coalesces bursts and emits a single `BuildNow` once quiet (or max delay is reached).
6. The build renders the **full site** for the daemon’s repo set and publishes atomically.

## Rationale

- Clear operator semantics: a webhook should mean “something changed”, not redefine the site.
- Performance: rebuild storms coalesce into a single build.
- Extensibility: update-only, discovery-only, or other workflows can exist without coupling.
- Right-sized complexity: in-process events capture the architecture without requiring a broker.

## Consequences

Benefits:

- Explicit separation of discovery/update/build.
- Predictable storm behavior.
- Safer webhook handling (no accidental repo set narrowing).

Trade-offs:

- More moving parts (routing, buffering, ordering, backpressure).
- Must design idempotency/deduplication (webhook retries, overlapping schedules).

## Alternatives considered

1. Keep current pipeline-only approach
   - Simple and correct, but cannot separate update/build operationally.

2. External durable broker
   - Too heavy for the single-daemon target.

3. Trigger a build for every webhook
   - Easy to implement but inefficient under bursts.

## Libraries considered

Primary requirements are correctness, backpressure control, and clean shutdown (not cross-process delivery):

1. Custom in-process bus (Go channels + dispatcher)
   - Pros: typed events, explicit buffering/backpressure, deterministic shutdown, minimal dependencies.
   - Cons: some implementation work.

2. `github.com/ThreeDotsLabs/watermill`
   - Pros: mature router patterns, easy to swap transports later.
   - Cons: heavier than needed for in-process control flow; still need domain components.

3. `github.com/cskr/pubsub`
   - Pros: tiny.
   - Cons: untyped; would need wrappers for contracts/backpressure.

4. `github.com/asaskevich/EventBus`
   - Pros: simple API.
   - Cons: reflection-based/untyped.

Decision: start with a small custom in-process subsystem with a narrow API.

## Open questions

- Do we ever evolve webhook triggers to be commit-specific, or remain branch-latest?
- Do we need per-repo update concurrency limits distinct from build concurrency?
- How should discovery-driven removals be handled?
  - This ADR requires `RepoRemoved` (or equivalent) as a first-class event.
  - Later work may integrate removal via forge-specific webhooks where supported.
