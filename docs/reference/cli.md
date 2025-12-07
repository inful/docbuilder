# CLI Reference

Primary commands (Kong-based CLI):

| Command | Description |
|---------|-------------|
| `build` | Run full pipeline (clone → discover → generate config → copy content → indexes → (optional) Hugo run). |
| `init` | Create an example configuration file. |
| `discover` | (If present) Run discovery only (debug content paths). |
| `daemon` | Run long-lived service (if implemented in current codebase). |

## Global Flags (Common)

| Flag | Purpose |
|------|---------|
| `-c, --config` | Path to YAML config file. |
| `-v` | Verbose logging. |
| `--version` | Print version/build info. |

## Environment Variables (Behavior Modifiers)

| Variable | Effect |
|----------|--------|
| `--render-mode always` | Force running Hugo after scaffolding. |
| `--render-mode never` | Force skipping Hugo even when enabled in config. |

## Build Report Outputs

Generated in output directory:

- `build-report.json` — machine-readable summary (contains `doc_files_hash`).
- `build-report.txt` — human summary line.

Key JSON fields:

| Field | Meaning |
|-------|---------|
| `repositories` | Number of repositories that produced at least one doc file. |
| `files` | Number of discovered documentation files. |
| `outcome` | Final build result. One of: `success`, `warning`, `failed`, `canceled`. |
| `cloned_repositories` | Successfully cloned or updated repos. |
| `failed_repositories` | Repos that failed clone/auth. |
| `rendered_pages` | Markdown files written to content directory. |
| `static_rendered` | True if Hugo was run and succeeded. |
| `doc_files_hash` | Stable fingerprint of doc file set. |
| `issues[]` | Structured issue entries (code, stage, severity, message, transient). |

## Exit Codes

| Condition | Exit Code |
|-----------|-----------|
| Success | 0 |
| Fatal error | Non-zero (varies by underlying error path) |
| Canceled (context) | Non-zero |

## Logging Highlights

- Clone stage logs per-repo successes/failures and update method (fast-forward vs already up-to-date).
- Discovery stage logs unchanged doc set when identical to prior run (and repository heads unchanged).
- Early exit path logs when entire pipeline is skipped due to no changes and valid prior output.
