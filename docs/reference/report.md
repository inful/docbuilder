# Build Report Reference

DocBuilder writes a machine-readable `build-report.json` and a summary `build-report.txt` after each build.

## Lifecycle

1. Report initialized at pipeline start.
2. Stages record durations, outcomes, issue codes.
3. On early exit (no changes) outcome and timestamps are still finalized.
4. Report persisted atomically (temp file then rename).

## Selected JSON Fields

| Field | Type | Description |
|-------|------|-------------|
| schema_version | int | Schema contract version (currently 1). |
| repositories | int | Count of repositories with at least one doc file. |
| files | int | Count of discovered documentation files. |
| start / end | time | Timestamps (UTC). |
| stage_durations | object | Map stage→duration nanoseconds (human-friendly when pretty printed). |
| stage_error_kinds | object | Map of stage name to error kind (`fatal`, `warning`, `canceled`). |
| cloned_repositories | int | Successful clone/update count. |
| failed_repositories | int | Failed clone attempts. |
| skipped_repositories | int | Repositories filtered out pre-clone. |
| rendered_pages | int | Markdown pages written (pre-render). |
| static_rendered | bool | Hugo build executed successfully. |
| retries | int | Aggregate retry attempts across stages. |
| retries_exhausted | bool | True if any stage exhausted its retry budget. |
| outcome | string | Final build status: `success`, `warning`, `failed`, or `canceled`. |
| doc_files_hash | string | Stable SHA‑256 hex of sorted Hugo paths. |
| issues | []Issue | Structured issue list. |
| skip_reason | string | Non-empty when early exit (e.g. no_changes). |

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

`doc_files_hash` lets downstream tasks (search indexing, publishing) short-circuit when the doc set is identical between builds.

## Stability Notes

- New fields may be added; existing fields will not be repurposed before v1.0.0.
- Treat unknown fields as optional to remain forward compatible.
