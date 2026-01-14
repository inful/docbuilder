---
uid: 327c9967-2b83-47fc-8ebc-996964bb7001
fingerprint: 54d6e6636f3f67c1ef601ab8a88f6ea0ee089a7ebb5cfea1d33a52e84c8fa02d
---

# ADR-009: Push Documents to an External Ingester During Builds

**Status**: Proposed  
**Date**: 2026-01-14  
**Decision Makers**: DocBuilder Core Team  
**Technical Story**: Integrate downstream indexing/ingestion as an explicit build stage

## Context and Problem Statement

DocBuilder already produces outputs that downstream systems commonly depend on:

- A complete Hugo project (content + `hugo.yaml` + generated indexes)
- A build report with deterministic change signals (`doc_files_hash`, `config_hash`, delta fields)

In many deployments, there is an additional required step after a successful build:

- Push the generated documentation (or a structured representation of it) into an external system (“ingester”) such as:
  - a search indexer
  - an enterprise documentation portal
  - a vector store / embedding pipeline
  - a compliance archive

Today, DocBuilder does **not** perform ingestion.

If/when ingestion becomes a requirement, doing it as an ad-hoc downstream step (separate jobs, separate configs) tends to create several problems:

- **Orchestration drift**: “build succeeded” does not imply “docs are searchable/ingested”.
- **Repeated work**: downstream automation must re-implement the same change detection logic.
- **Poor observability**: ingestion success/failure is not represented in DocBuilder’s stage metrics or build report.
- **Ambiguous failure policy**: teams disagree whether ingestion errors should fail the build, warn, or be retried.

## Decision

Introduce an **optional, explicitly modeled ingestion stage** in the build pipeline that can push documents to an external ingester.

### Stage placement

The ingester stage runs after DocBuilder has produced a complete, consistent Hugo project in the staging directory, but before final promotion (or immediately after promotion if the implementation prefers to read from the final output path).

Because ingestion sends **markdown**, it MUST run **before** `run_hugo` (if `run_hugo` is enabled).

In terms of the canonical stage list, it conceptually belongs after:

```
... → CopyContent → Indexes → (Optional) RunHugo
```

And before any post-processing that might mutate generated artifacts.

### What gets ingested

DocBuilder will define a single “ingestion payload” model that can be produced deterministically from the build.

The ingester receives the **full document** (markdown including YAML frontmatter).

Minimum payload requirements:

- `doc_files_hash` and `config_hash` from the build report
- document identity fields that match DocBuilder’s stable addressing:
  - repository
  - optional forge name
  - Hugo content path
- stable document URL derived from the Hugo content path (and the effective `base_url` when available)
   - at minimum: a stable site-relative URL path (e.g. `/repo/section/page/`)
   - optionally: a full absolute URL when `base_url` is set (e.g. `https://docs.example.com/repo/section/page/`)
- per-document stable identifiers from frontmatter:
   - `uid` (stable GUID/UUID, never changes once set)
   - `fingerprint` (content fingerprint reflecting current document contents)
- the full document content as markdown (including YAML frontmatter)
- document metadata suitable for downstream consumers (title, section, edit URL, etc.)

#### Document identity and movement between repositories

DocBuilder MUST treat `uid` as the canonical, globally unique document identity across all repositories.

- Repository name, forge name, and Hugo content path are **location** fields (they can change over time).
- A document that moves between repositories (or sections/paths) MUST keep its original `uid` in frontmatter.
- The ingester SHOULD upsert by `uid` (and consider `fingerprint` as the version indicator), so a move is naturally handled as an update to the same logical document with a new location + URL.

To support moves without accidentally treating them as delete+create, DocBuilder SHOULD build and persist a per-document index keyed by `uid` (e.g., `uid → {fingerprint, repo, hugo_path, url}`) and diff it between builds.

**Hard requirement:** any document pushed to the ingester MUST carry `uid` and `fingerprint` in its YAML frontmatter. If either field is missing, ingestion MUST treat it as an error (failure policy applies).

**Enforcement mechanism:** DocBuilder already has lint rules for frontmatter `uid` and `fingerprint`. The ingestion stage MUST only run after the content pipeline has finalized frontmatter, and SHOULD run a targeted validation pass equivalent to `docbuilder lint`’s frontmatter checks before sending payloads.

**URL injection requirement:** during posting, DocBuilder MUST inject the stable URL into the document frontmatter as a YAML list:

```yaml
url:
   - <url>
```

This injection is part of the ingestion step (the generated on-disk markdown does not need to be modified).

If a document with the same `uid` is detected at a new location, DocBuilder MAY include multiple URLs in the injected `url:` list (e.g., previous URL(s) + current URL) to support downstream redirect/alias behavior. The exact alias/redirect behavior is owned by the ingester.

### Configuration

Ingestion is disabled by default.

When enabled, configuration specifies:

- endpoint / transport details
- authentication (token/header/etc.)
- timeouts and retry policy
- failure policy (`fail`, `warn`, `ignore`)
- send mode:
   - `all` (default): post the full document set every build; the ingester performs smart updates using `uid` + `fingerprint`
   - `changed`: post only documents DocBuilder considers changed (best-effort optimization using existing delta/change detection)

### Observability

Ingestion becomes a first-class stage with:

- stage duration metrics
- stage result counters (`success`, `warning`, `fatal`, `canceled`)
- build report issue entries when ingestion partially or fully fails

## Rationale

- DocBuilder already has stable change signals (hashes, delta reasons) that are directly useful for ingestion.
- A first-class ingestion stage makes build outcomes align with operational reality ("done" includes indexing).
- Explicit stage modeling improves reliability and reduces duplicated scripting.

## Consequences

### Benefits

- Unified build+ingest workflow with consistent error handling and metrics.
- Better integration with daemon mode and incremental builds.
- Consumers can rely on DocBuilder to drive ingestion using the same hashes it computes.

### Trade-offs

- Adds configuration surface area and operational concerns (auth, retries, rate limiting).
- Adds a new failure mode to builds (ingester downtime, partial acceptance).
- Requires careful design to avoid leaking sensitive content or credentials.

## Failure and Retry Model

Ingestion failures will be classified and handled consistently with DocBuilder’s error foundation:

- Retry only on transient categories (network timeouts, rate limits) with bounded backoff.
- Respect context cancellation (daemon shutdown, user interrupt).
- Apply configured failure policy:
  - `fail`: ingestion failure marks build failed
  - `warn`: ingestion failure marks build warning
  - `ignore`: ingestion failure recorded but build outcome remains success

## Alternatives Considered

1. **Do nothing; keep ingestion in CI scripts**
   - Rejected: repeats change detection and hides ingestion failures from DocBuilder’s observability.

2. **Emit only a manifest and let another service ingest**
   - Partially acceptable; still requires a standard payload model, but keeps DocBuilder simpler.

3. **Daemon-only ingestion**
   - Rejected: CI/CD builds also need deterministic ingestion behavior.

## Open Questions (for iteration)

1. **Ingestion content format**: resolved as **full markdown document** (including frontmatter).

2. **Stage placement**: resolved as **before `run_hugo`** (markdown-based).

3. **Change granularity**:
   - default to ingest always-full sets (ingester dedupes using `uid` + `fingerprint`)
   - optionally ingest only changed docs using DocBuilder’s existing change detection (optimization, not correctness)

4. **Idempotency key**: do we want a stable per-document version key (e.g., `$doc_files_hash` + content path, or a content hash per file)?

5. **Delete handling**: how should removals be communicated (tombstones, full re-sync, periodic reconciliation)?

6. **Moves/renames across repositories**: if a document changes repository and/or path but preserves `uid`, should ingestion treat this as a pure upsert (recommended), and should the `url:` list include previous URL(s) as aliases?

7. **Authentication**: what auth types must be supported initially (bearer token, basic, mTLS)?

8. **Operational limits**: batch sizes, concurrency, and rate limiting needs.

## Related Documents

- docs/explanation/architecture.md
- docs/reference/report.md
- ADR-008: Staged Pipeline Architecture
- ADR-003: Fixed Transform Pipeline
