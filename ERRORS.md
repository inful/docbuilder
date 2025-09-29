# Error & Issue Taxonomy

This document describes DocBuilder's structured error model, issue codes, and transient classification semantics. It is intended for operators, integrators, and future contributors extending the pipeline.

## Layers of Error Representation

1. Underlying Errors (raw causes): Errors returned by subsystems (git library, filesystem, Hugo process, network).
2. Typed Domain Errors: Lightweight wrappers providing semantic classification (e.g. `AuthError`, `RateLimitError`). Located under `internal/git/typed_errors.go` and analogous future locations for non‑git domains.
3. StageError: Pipeline wrapper carrying `Stage`, `Kind` (`fatal|warning|canceled`) and the underlying error. Lives in `internal/hugo/classification.go`.
4. BuildReport Issues: Serialized, machine‑parseable entries (`ReportIssue`) with stable `Code` values for automation and metrics.

Flow: underlying error → (optional typed wrapper) → StageError → BuildReport.AddIssue() → metrics emission.

## StageError

A `StageError` has three key fields:

- `Stage`: Which pipeline stage failed (e.g. `clone_repos`, `discover_docs`).
- `Kind`: Severity category controlling pipeline flow:
  - `fatal`: Abort the build immediately.
  - `warning`: Record, continue to next stage.
  - `canceled`: Context canceled (timeout/shutdown/manual stop).
- `Err`: Underlying Go error (can be wrapped stacks; kept for logging only).

### Transient Classification

`StageError.Transient()` returns true when the error is considered retry-able / low-confidence permanent. Current logic:

- Canceled errors are never transient.
- For clone/discovery/render stages, certain sentinel errors (`build.ErrClone`, `build.ErrDiscovery`, `build.ErrHugo`) mark warnings as transient.
- Typed transient git errors (`RateLimitError`, `NetworkTimeoutError`) are always transient when surfaced as warning stage errors.
- Fatal kinds are never transient even if the underlying cause would otherwise qualify (conservative stance prevents loops on systemic failures).

The daemon’s retry logic wraps the build and searches the resulting report for at least one transient `StageError` to decide whether to enqueue a retry (bounded by `build.max_retries`).

## Issue Codes

Issue codes are appended only (no reuse). They provide a stable contract for dashboards, alerts, and CI consumers parsing `build-report.json`.

| Code | Meaning | Typical Stage | Severity | Transient | Notes |
|------|---------|---------------|----------|-----------|-------|
| CLONE_FAILURE | Generic clone stage warning/failure (mixed causes) | clone_repos | error | false | Deprecated path; replaced progressively by granular codes. |
| PARTIAL_CLONE | Some repositories failed, at least one succeeded | clone_repos | error | false | Usually emitted alongside more granular issues per repo. |
| DISCOVERY_FAILURE | Unable to discover docs (empty repositories or IO failure) | discover_docs | error | false | Distinct from having zero repos cloned. |
| NO_REPOSITORIES | Config/discovery produced zero repositories | prepare/clone | error | false | Early exit condition. |
| HUGO_EXECUTION | Hugo process failed or returned non‑zero | run_hugo | error | maybe | Currently treated as non‑transient unless wrapped as warning with retry context. |
| BUILD_CANCELED | Build context canceled (timeout/shutdown) | any | error | false | Pipeline aborted intentionally. |
| ALL_CLONES_FAILED | Every repository failed to clone | clone_repos | error | false | Distinguishes systemic outage or credential issue. |
| GENERIC_STAGE_ERROR | Fallback when no specific classification matched | any | error | false | Indicates missing taxonomy coverage. |
| AUTH_FAILURE | Authentication failure (git) | clone_repos | error | false | Permanent: do not retry. |
| REPO_NOT_FOUND | Repository does not exist or no access | clone_repos | error | false | Permanent. |
| UNSUPPORTED_PROTOCOL | Unsupported git transport/protocol | clone_repos | error | false | Permanent (configuration). |
| REMOTE_DIVERGED | Local branch diverged, hard reset disabled | clone_repos | error | false | User action required or enable `hard_reset_on_diverge`. |
| RATE_LIMIT | Remote API or git host rate limit | clone_repos | error | true | Adaptive retry applies (longer backoff). |
| NETWORK_TIMEOUT | Network timeout / IO timeout during git operation | clone_repos | error | true | Retry with standard backoff. |

Severity column reflects `IssueSeverity` enum; transient column indicates default retry suitability (final decision considers StageError.Kind).

## Typed Git Errors

Implemented types (wrapping underlying causes):

- `AuthError`
- `NotFoundError`
- `UnsupportedProtocolError`
- `RemoteDivergedError`
- `RateLimitError` (transient)
- `NetworkTimeoutError` (transient)

Classification precedence always favors typed errors over string heuristics. Heuristic fallback remains for legacy paths or non‑wrapped errors.

## Metrics Mapping

When metrics are enabled:

- Each issue increments `docbuilder_issues_total{code,stage,severity,transient}`.
- Render mode is exported via `docbuilder_effective_render_mode` gauge (static=0, noop=1, unknown=2).
- Retry attempts for transient StageErrors increment `docbuilder_build_retries_total{stage}`; exhaustion increments `docbuilder_build_retry_exhausted_total{stage}`.

Planned additions:

- Per‑stage transient vs permanent counters (explicit separation instead of reading `transient` label).
- Content transform failure counter: `docbuilder_content_transform_failures_total`.

## Adaptive Retry Behavior

The git layer applies adaptive multipliers based on classified transient type:

- `rate_limit`: multiplier 3× base delay.
- `network_timeout`: multiplier 1× (no scaling beyond policy).

Backoff modes (`fixed|linear|exponential`) are applied first, then multiplier, then clamped to `retry_max_delay`.

## Adding New Issue Codes

1. Define new constant in `report.go` (append only).
2. Add typed error (if applicable) and wrapping at origin site.
3. Extend `classifyGitFailure` (or other domain classifier) to map typed error → code.
4. Add unit test entry to matrix (ensuring stability).
5. Update this document & any dashboard queries.

## Debugging Tips

- If you see `GENERIC_STAGE_ERROR`, search code for opportunities to introduce a new typed error.
- Multiple rate limit issues likely indicate missing server‑side tokens or excessively high concurrency.
- Divergence errors with `hard_reset_on_diverge=false` are intentional safety checks; enable the flag if destructive resets are acceptable.

## Roadmap (Error System)

- Extend typed taxonomy beyond git (Hugo execution categories, filesystem space errors, configuration resolution errors).
- Surface transient classification and retry budget decisions on the JSON status endpoint.
- Add histogram for retry delay distribution to tune backoff policies.

---
Maintainers: keep taxonomy minimal—prefer broad permanent/transient categories unless a new code enables a distinct operator response.
