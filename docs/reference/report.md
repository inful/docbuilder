---
uid: 48b52695-0104-48d5-a91c-4698b031113e
title: "Build Reports Reference"
date: 2025-12-15
categories:
  - reference
tags:
  - reports
  - builds
  - output
fingerprint: 42591efb30008410395aec489e2daf73f957e888398e9c6792662ab49d7c5b6c
---

# Build Report Reference

DocBuilder writes a machine-readable `build-report.json` and a summary `build-report.txt` after each build.

## Lifecycle

1. Report initialized at pipeline start.
2. Stages record durations, outcomes, issue codes.
3. On early exit (no changes) outcome and timestamps are still finalized.
4. Report persisted atomically (temp file then rename).

## JSON Fields Reference

### Core Metadata

| Field | Type | Description |
|-------|------|-------------|
| schema_version | int | Schema contract version (currently 1). |
| doc_builder_version | string | DocBuilder version that generated the report. |
| hugo_version | string | Hugo version detected during build. |
| start | time | Build start timestamp (UTC). |
| end | time | Build completion timestamp (UTC). |
| outcome | string | Final build status: `success`, `warning`, `failed`, or `canceled`. |

### Repository Statistics

| Field | Type | Description |
|-------|------|-------------|
| repositories | int | Count of repositories with at least one doc file. |
| files | int | Count of discovered documentation files. |
| cloned_repositories | int | Successful clone/update count. |
| failed_repositories | int | Failed clone attempts. |
| skipped_repositories | int | Repositories filtered out pre-clone. |
| clone_stage_skipped | bool | Whether clone stage was skipped (incremental builds). |

### Build Results

| Field | Type | Description |
|-------|------|-------------|
| rendered_pages | int | Markdown pages written to content directory. |
| static_rendered | bool | Hugo build executed successfully. |
| effective_render_mode | string | Actual render mode used: `always`, `auto`, or `never`. |

### Stage Information

| Field | Type | Description |
|-------|------|-------------|
| stage_durations | object | Map of stage name → duration (nanoseconds, human-readable when pretty printed). |
| stage_error_kinds | object | Map of stage name → error kind (`fatal`, `warning`, `canceled`). |
| stage_counts | object | Detailed per-stage counts (success, skipped, failed). |

### Error Tracking

| Field | Type | Description |
|-------|------|-------------|
| errors | []string | Fatal error messages that caused build abortion. |
| warnings | []string | Non-fatal warning messages. |
| issues | []Issue | Structured issue list (see Issues Array section). |
| retries | int | Aggregate retry attempts across all stages. |
| retries_exhausted | bool | True if any stage exhausted its retry budget. |

### Incremental Build Fields

| Field | Type | Description |
|-------|------|-------------|
| doc_files_hash | string | Stable SHA‑256 hex of sorted Hugo content paths. |
| config_hash | string | Configuration hash for change detection. |
| delta_decision | string | Decision made for incremental build: `full_rebuild`, `incremental`, `skip`. |
| delta_changed_repos | []string | List of repositories that changed (incremental builds). |
| delta_repo_reasons | object | Map of repository name → change reason (`quick_hash_diff`, `assumed_changed`, `unknown`). |
| skip_reason | string | Reason for early exit (e.g., `no_changes`, `no_repositories`). |

### Template Information

| Field | Type | Description |
|-------|------|-------------|
| index_templates | object | Map of template kind → template info (source, exists, custom). |
| pipeline_version | int | Content processing pipeline version number. |

## Issues Array

Each issue item:

```json
{
  "code": "CLONE_FAILURE",
  "stage": "clone_repos",
  "severity": "error",
  "message": "fatal stage clone_repos: ...",
  "transient": false
}
```

Codes include (non-exhaustive):

- CLONE_FAILURE
- PARTIAL_CLONE
- ALL_CLONES_FAILED
- DISCOVERY_FAILURE
- NO_REPOSITORIES
- HUGO_EXECUTION
- BUILD_CANCELED
- AUTH_FAILURE
- REPO_NOT_FOUND
- UNSUPPORTED_PROTOCOL
- REMOTE_DIVERGED
- GENERIC_STAGE_ERROR

## Hash Usage

### doc_files_hash

`doc_files_hash` lets downstream tasks (search indexing, publishing) short-circuit when the doc set is identical between builds. Computed as SHA-256 of sorted Hugo content paths.

### config_hash

`config_hash` enables daemon mode to detect configuration changes and trigger full rebuilds. Changes in repository URLs, paths, authentication, or Hugo settings invalidate incremental builds.

## Incremental Build Support

When running in daemon/incremental mode, the report includes delta-specific fields:

- **delta_decision**: Strategy chosen (`full_rebuild`, `incremental`, `skip`)
- **delta_changed_repos**: Repositories with changes detected
- **delta_repo_reasons**: Per-repository change detection method
  - `quick_hash_diff`: Git commit hash changed
  - `assumed_changed`: Unable to verify, assumed changed for safety
  - `unknown`: Change detection failed or unavailable

These fields enable observability into incremental build decisions and help diagnose unexpected full rebuilds.

## Example Report

```json
{
  "schema_version": 1,
  "doc_builder_version": "2.1.0",
  "hugo_version": "0.139.3",
  "repositories": 2,
  "files": 15,
  "start": "2025-12-29T14:00:00Z",
  "end": "2025-12-29T14:00:05Z",
  "errors": [],
  "warnings": [],
  "stage_durations": {
    "prepare_output": 5000000,
    "clone_repos": 1500000000,
    "discover_docs": 100000000,
    "generate_config": 10000000,
    "copy_content": 200000000,
    "generate_indexes": 50000000
  },
  "stage_error_kinds": {},
  "cloned_repositories": 2,
  "failed_repositories": 0,
  "skipped_repositories": 0,
  "rendered_pages": 15,
  "static_rendered": false,
  "retries": 0,
  "retries_exhausted": false,
  "outcome": "success",
  "doc_files_hash": "abc123def456...",
  "config_hash": "789ghi012jkl...",
  "issues": [],
  "skip_reason": "",
  "pipeline_version": 1,
  "effective_render_mode": "never"
}
```

## Stability Notes

- New fields may be added; existing fields will not be repurposed before v1.0.0.
- Treat unknown fields as optional to remain forward compatible.
- Fields marked `omitempty` may be absent if not applicable to the build type.
