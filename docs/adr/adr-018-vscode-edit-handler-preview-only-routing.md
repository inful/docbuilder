---
aliases:
  - /_uid/6b9c3b0c-1f76-45fb-8d3b-7bc8d0d8ab2b/
categories:
  - architecture-decisions
date: 2026-01-23T00:00:00Z
fingerprint: 16664189e38f1c60b592da8f3bbc762ff896ccd24400c8e43ba606240b648639
lastmod: "2026-01-23"
tags:
  - vscode
  - preview
  - http
  - security
  - daemon
uid: 6b9c3b0c-1f76-45fb-8d3b-7bc8d0d8ab2b
---

# ADR-018: Register VS Code edit handler only in local preview

**Status**: Proposed  
**Date**: 2026-01-23  
**Decision Makers**: DocBuilder Core Team

## Context and Problem Statement

DocBuilder supports “edit links” that can open a local Markdown file in VS Code by hitting an HTTP endpoint:

- `GET /_edit/<relative-path>`

Today, the HTTP docs server mux registers the `/_edit/` route unconditionally, and the handler itself enforces the effective policy:

- If `--vscode` is not enabled, return `404`.
- If running in daemon mode, return `501` (“preview mode only”).

This behavior is functionally safe but it is not as strict as intended. The endpoint should be a *preview-only* feature and should not appear at all (even as a blocked endpoint) when DocBuilder is running as a daemon.

### Why this matters

- **Principle of least privilege / smaller attack surface**: if daemon mode should never support opening local files via an HTTP-triggered VS Code action, then it should not expose an edit endpoint at all.
- **Clearer semantics**: a registered-but-blocked endpoint can imply “this exists, but is misconfigured”. For daemon mode, the correct message is “this feature is not part of this product mode”.
- **Operational hygiene**: probes, scanners, or curious users can hit `/_edit/` and generate warning logs and noise.

## Decision

We will make the VS Code edit endpoint *preview-only at the routing level*:

- The `/_edit/` route is registered **only** when running **local preview**.
- The route is registered **only** when the feature flag `--vscode` (or equivalent) is enabled.
- In daemon mode, the docs server will not register `/_edit/` at all, resulting in a normal mux `404`.

We will keep the actual handler implementation in the shared HTTP server package so preview can reuse the same code path, but **route registration becomes conditional** based on runtime mode.

## Definitions

- **Local preview**: the `preview` command (or equivalent preview-mode entrypoint) serving docs from a local filesystem repository and providing developer conveniences.
- **Daemon mode**: the long-running service mode that manages a repository cache, webhooks, and background build/discovery.

(Exact detection/wiring is an implementation detail; the key is that “preview vs daemon” must be explicit at the HTTP mux wiring layer.)

## Decision Drivers

- Strongly enforce “preview-only” scope.
- Avoid relying on handler-side checks as the only barrier.
- Reduce confusion created by a shared `httpserver` package being used by multiple product modes.
- Preserve the existing UX in local preview (edit links work when enabled).

## Consequences

### Pros

- Daemon no longer exposes `/_edit/` even in a blocked form.
- Cleaner logs in daemon mode.
- Makes the security posture easier to explain: “not routed, not reachable”.
- Removes ambiguity about whether daemon “supports” VS Code edit links.

### Cons / Tradeoffs

- Requires preview/daemon mode to be explicitly represented in HTTP server wiring (either via config flags or server options).
- Slightly more wiring complexity: mux construction must know whether it is in preview.

## Implementation Notes (Deferred)

This ADR does not implement the change; it describes the intended direction.

A likely implementation approach:

- Introduce an explicit runtime capability or option passed into the HTTP server wiring, e.g. `Options.EnableVSCodeEditHandler` or `Options.Mode = Preview|Daemon`.
- Register `/_edit/` only when:
  - `mode == Preview`, and
  - `cfg.Build.VSCodeEditLinks == true`.

We should keep handler-side validation as defense-in-depth (path validation, symlink checks, etc.), but the primary enforcement becomes “not registered outside preview”.

## Acceptance Criteria

- In daemon mode, requests to `/_edit/...` return `404` because the route is not registered.
- In local preview with `--vscode` enabled, `/_edit/...` continues to work.
- In local preview without `--vscode`, the route is not registered (preferred) or returns `404` without logging warnings (acceptable as an incremental step).
- Tests cover routing behavior differences between preview and daemon modes.

## Alternatives Considered

1. **Keep unconditional routing; rely on handler-side checks**
   - Rejected: still exposes a discoverable endpoint in daemon mode.

2. **Keep unconditional routing; return `404` in daemon mode instead of `501`**
   - Rejected: improves semantics but does not reduce attack surface or endpoint discoverability.

3. **Move all VS Code edit logic into preview-only packages**
   - Not chosen: we still want a shared implementation for edit behavior, just not shared routing.

## Related Documents

- ADR-017: Split daemon responsibilities (package boundaries)