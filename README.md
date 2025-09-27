# docbuilder

A Go utility for creating documentation sites from multiple Git repositories using Hugo.

## Features

- Clone documentation from multiple Git repositories
- Support for various authentication methods (SSH, tokens, basic auth)
## Recommended Filesystem Layouts

- Generate Hugo-compatible static sites
- Optional automatic rendering (invoke the Hugo binary) to produce a ready-to-serve `public/` folder
- Environment variable support with `.env` files
- Incremental builds for faster updates
- Auto-discover repositories (v2 config) across all organizations accessible to the token (Forgejo)
- Theme-aware configuration (Hextra & Docsy) using Hugo Modules

## Quick Start

1. **Build the application**:
   ```bash
   make build
   ```
2. **Initialize configuration**:
   ```bash
   ./bin/docbuilder init
   ```
3. **Set up environment variables** (optional):
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```
4. **Build documentation site**:
   ```bash
   ./bin/docbuilder build -v
   ```

## Configuration (Unified v2)

### Environment Variables

The application automatically loads environment variables from `.env` and `.env.local` files. Example:

```bash
# .env
GIT_ACCESS_TOKEN=your_git_access_token_here
```

### Configuration File

Example minimal `config.yaml` (direct build mode):

```yaml
repositories:
  - url: https://git.example.com/owner/repo.git
    name: my-docs
    branch: main
    paths:
      - docs
    auth:
      type: token
      token: "${GIT_ACCESS_TOKEN}"

hugo:
  title: "My Documentation Site"
  description: "Aggregated documentation"
  base_url: "https://docs.example.com"

output:
  directory: "./site"
  clean: true
```

### Daemon (v2) Configuration & Discovery

When running the daemon (`docbuilder daemon`) you use the v2 config format (version: "2.0").
Organizations / groups for a forge are OPTIONAL. If you omit both `organizations:` and `groups:` the daemon enters
an auto-discovery mode: it enumerates all organizations/groups your token can access and then lists their repositories.

Forge example with scoped discovery:

```yaml
version: "2.0"
daemon:
  http:
    docs_port: 8080
    admin_port: 8081
    webhook_port: 8082
  sync:
    schedule: "0 */4 * * *"
    queue_size: 100
    concurrent_builds: 3
forges:
  - name: "forgejo"
    type: "forgejo"
    api_url: "https://git.example.com/api/v1"
    base_url: "https://git.example.com"
    # organizations: []   # (optional) if omitted -> auto-discovery
    auth:
      type: token
      token: "${FORGEJO_TOKEN}"
filtering:
  required_paths: ["docs"]
output:
  directory: "./site"
  clean: true
```

Key notes:

- Provide explicit `organizations` / `groups` to limit scope and speed up discovery.
- Per-forge discovery errors are exposed at `/status?format=json` under `discovery_errors`.
- To include repositories even if they have no matching documentation paths, set `required_paths: []`.
- The `build` section (see below) controls performance knobs like clone concurrency.

### Static Site Rendering (Running Hugo Automatically)

By default DocBuilder only scaffolds a Hugo project (content + `hugo.yaml`). To also produce a pre-built static site (`site/public/`) you can opt in via environment variables:

```bash
DOCBUILDER_RUN_HUGO=1 docbuilder build -c config.yaml
```

You can skip execution explicitly (useful in CI matrix) with:

```bash
DOCBUILDER_SKIP_HUGO=1 docbuilder build -c config.yaml
```

Precedence:
 
1. If `DOCBUILDER_SKIP_HUGO=1` -> never run.
2. Else if `DOCBUILDER_RUN_HUGO=1` -> run.
3. Else -> scaffold only.

Hugo must be installed and on `PATH`. If the build fails, the scaffolded site remains available (warning is logged) so you can run `hugo` manually inside the output directory.

### Theme Support (Hextra & Docsy)

When `hugo.theme` is set to `hextra` or `docsy`, DocBuilder configures Hugo Modules automatically:

- Creates / maintains a minimal `go.mod` (sanitizes module name from `base_url` host; ports are stripped).
- Adds theme-specific params (search, edit links, UI defaults, offline search for the Docsy theme, FlexSearch config for Hextra).
- Avoids using the legacy `theme` filesystem lookup; relies on modules for reproducible builds.

Pinning:

- Hextra is pinned to a stable version in `go.mod` automatically.
- The Docsy theme currently floats (you can pin manually by editing `go.mod`).

### Edit Links

Per-page edit links are enabled by default for supported themes when repository metadata allows deriving an edit URL. The first repository (or the one containing a file) is used to construct the link; future enhancements may allow per-file overrides.

### Filtering Semantics Summary

| Setting | Behavior |
|---------|----------|
| `required_paths: ["docs"]` | Only include repos that contain at least one configured path (e.g. a `docs/` directory). |
| `required_paths: []` | Disable pruning: include all discovered repos regardless of documentation presence (missing paths are logged as WARN). |
| Omit `required_paths` | Defaults to `["docs"]` in v2 unless explicitly provided. |

Warnings are emitted if every repository is filtered out so you can adjust patterns quickly.

### Authentication Methods

1. **Token Authentication** (recommended):
   ```yaml
   auth:
     type: token
     token: "${GIT_ACCESS_TOKEN}"
   ```

2. **SSH Key Authentication**:
   ```yaml
   auth:
     type: ssh
     key_path: "~/.ssh/id_rsa"
   ```

3. **Basic Authentication**:
   ```yaml
   auth:
     type: basic
     username: "your-username"
     password: "${GIT_PASSWORD}"
   ```

## Commands

- `docbuilder build` - Build documentation site from configured repositories
- `docbuilder init` - Initialize a new configuration file
- `docbuilder build --incremental` - Update existing repositories instead of fresh clone

## Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Format code
make fmt

# Development cycle
make dev
```

## Metrics & Observability

DocBuilder exposes build and runtime metrics in two forms (both available when metrics are enabled in config):

1. JSON metrics snapshot (`/metrics` and `/metrics/detailed` on the admin port).
2. Prometheus metrics endpoint (`/metrics/prometheus`). Prometheus support is now always compiled in (build tags removed).

### Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `docbuilder_build_duration_seconds` | histogram | (none) | Total wall clock time of a full site build (from first stage start to completion). |
| `docbuilder_stage_duration_seconds` | histogram | `stage` | Duration of individual pipeline stages (e.g. `clone_repos`, `discover_docs`, `copy_content`, `run_hugo`). |
| `docbuilder_stage_results_total` | counter | `stage`, `result` | Count of stage executions by result (`success`, `warning`, `fatal`, `canceled`). |
| `docbuilder_build_outcomes_total` | counter | `outcome` | Final build outcomes: `success`, `warning`, `failed`, `canceled`. |
| `docbuilder_daemon_active_jobs` | gauge | (none) | Number of build jobs currently executing. |
| `docbuilder_daemon_queue_length` | gauge | (none) | Number of queued build jobs waiting for a worker. |
| `docbuilder_daemon_last_build_rendered_pages` | gauge | (none) | Pages rendered in the most recently completed build (snapshot). |
| `docbuilder_daemon_last_build_repositories` | gauge | (none) | Repositories processed in the most recently completed build (snapshot). |
| `docbuilder_clone_repo_duration_seconds` | histogram | `repo`,`result` | Per-repository clone duration (result = success or failed). |
| `docbuilder_clone_repo_results_total` | counter | `result` | Clone attempts by outcome (success or failed). |
| `docbuilder_clone_concurrency` | gauge | (none) | Effective clone concurrency used in the last `clone_repos` stage. |
| `docbuilder_build_retries_total` | counter | `stage` | Count of individual retry attempts for transient failures (excludes initial attempt). |
| `docbuilder_build_retry_exhausted_total` | counter | `stage` | Count of stages where all retry attempts were exhausted without success. |

Additional counters (JSON only currently) are derived from the internal `BuildReport` (e.g., cloned, failed, skipped repository counts, rendered pages). Two real‑time operational gauges (`docbuilder_daemon_active_jobs`, `docbuilder_daemon_queue_length`) provide active concurrency visibility. Retry attempt and exhaustion counters aid alerting around instability.

### Transient Error Classification

Each stage error is heuristically classified as transient or permanent (e.g., network clone failures, intermittent Hugo build issues). This classification is used internally for future retry logic and may later surface as labeled metrics.

### Enabling Metrics

Configuration snippet enabling metrics:

```yaml
version: "2.0"
monitoring:
  metrics:
    enabled: true
    path: /metrics   # JSON endpoint base path
```

Example scrape:

```fish
./docbuilder daemon -c config.yaml
curl http://localhost:8082/metrics/prometheus | head -40
```

Disable metrics at runtime (JSON + Prometheus) by setting `monitoring.metrics.enabled: false`.

### JSON Metrics Endpoints

| Endpoint | Description |
|----------|-------------|
| `/metrics` | Compact JSON snapshot (counters, gauges, basic histograms). |
| `/metrics/detailed` | Extended JSON including histogram summaries & system metrics (goroutines, memory). |
| `/api/daemon/status` | Includes latest build report with per-stage timings & counts. |

### Metric Naming Conventions

Prometheus metric names are prefixed with `docbuilder_` and use `_seconds` suffix for duration histograms. Stage label values mirror internal stage identifiers.

### Planned Additions

- Histogram bucket tuning / custom buckets via config.
- Transient vs permanent failure counters (per-stage labeled separation).
- Aggregation / sampling modes to reduce per-repository metric cardinality.

### Build Performance Tuning

`build` section houses performance & hygiene controls for repository acquisition and workspace optimization.

Supported fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `clone_concurrency` | int | 4 | Max repositories cloned/updated in parallel. Bounded to repo count; coerced to >=1. |
| `clone_strategy` | enum | `fresh` | How to treat existing repo directories: `fresh` (always reclone), `update` (incremental fetch + fast‑forward/hard reset), `auto` (update if dir exists else clone). |
| `shallow_depth` | int | 0 | If >0 performs shallow clones/fetches limited to that many commits (git `--depth`). 0 = full history. |
| `prune_non_doc_paths` | bool | false | Remove top‑level entries not part of any configured docs path segment (plus those allowed via `prune_allow`). Reduces workspace size. |
| `prune_allow` | []string | (empty) | Extra top‑level names or glob patterns to always keep when pruning (e.g. `LICENSE*`, `README.*`, `assets`). |
| `prune_deny` | []string | (empty) | Top‑level names or glob patterns to always remove (except `.git`). Takes precedence over allow + docs roots. |
| `hard_reset_on_diverge` | bool | false | If true and local branch diverged from origin, perform hard reset to remote head; else update fails with divergence error. |
| `clean_untracked` | bool | false | After a successful fast‑forward or hard reset, remove untracked files/dirs (like `git clean -fdx` sans ignored semantics). |
| `max_retries` | int | 2 | Extra retry attempts for transient clone/update failures (see retry settings below). |
| `retry_backoff` | enum | `linear` | Backoff mode: `fixed`, `linear`, or `exponential`. |
| `retry_initial_delay` | duration | `1s` | Initial retry delay. |
| `retry_max_delay` | duration | `30s` | Maximum backoff delay cap. |
| `workspace_dir` | string | `<output.directory>/_workspace` | Directory for cloning repositories during a build. If outside output dir it is not auto-cleaned; enables persistent clone cache. |

Example configuration demonstrating all knobs:

```yaml
build:
  clone_concurrency: 6            # bounded automatically by repository count
  clone_strategy: auto            # auto-update existing repos, clone missing ones
  shallow_depth: 10               # keep only last 10 commits (faster network + less storage)
  prune_non_doc_paths: true       # strip unrelated top-level dirs
  prune_allow:                    # keep these extra roots / files (glob supported)
    - LICENSE*
    - README.*
    - assets
  prune_deny:                     # always remove even if would be allowed
    - test
    - "*.bak"
  hard_reset_on_diverge: true     # forcefully realign if local diverged
  clean_untracked: true           # hygiene after update
  max_retries: 3                  # retry transient failures 3 times
  retry_backoff: exponential
  retry_initial_delay: 1s
  retry_max_delay: 45s
  workspace_dir: /var/cache/docbuilder/workspace   # persistent clone cache (NOT auto-cleaned)
```

Notes & semantics:

- Clone Strategy
  - `fresh`: Always reclone (ensures pristine state; slowest for large repos).
  - `update`: Reuse existing checkout; fetch, then fast‑forward or (if diverged) either hard reset (when enabled) or fail.
  - `auto`: Choose `update` when directory exists else `fresh` (practical default in long‑running daemon scenarios).
- Shallow Clones
  - Reduces bandwidth & disk; only latest N commits are fetched. Some features (e.g., generating links to deep history) may be limited.
  - Subsequent fetches reapply depth (best effort); no automatic deepening is performed yet.
- Pruning (`prune_non_doc_paths`)
  - Only removes top‑level entries. Docs roots are derived from the first path segment of each configured repo `paths` entry (e.g. `docs/api` => keep `docs/`).
  - Precedence: `.git` always kept → explicit deny (exact or glob) → docs roots → explicit allow (exact or glob) → removal.
  - Glob patterns support `*`, `?`, and character classes (`[]`) using Go's `filepath.Match` semantics.
  - Use with caution: assets outside allowed roots that are still referenced by Markdown may break links if removed.
- Divergence Handling
  - When `hard_reset_on_diverge` is true, local divergent branches are forcefully aligned to `origin/<branch>` using a hard reset.
  - When false, divergence surfaces as a stage error (reported in build issues) so you can investigate unexpected local mutations.
- Cleaning
  - `clean_untracked` removes untracked files after fast‑forward or hard reset updates—helpful when previous builds left generated artifacts inside repo directories.
- Retry Policy
  - Applies to clone/update operations; permanent failures (auth, repo not found, unsupported protocol) short‑circuit retries.
  - Metrics differentiate retries vs exhausted attempts (`docbuilder_build_retries_total`, `docbuilder_build_retry_exhausted_total`).

Minimal incremental-friendly snippet:

```yaml
build:
  clone_strategy: auto
  shallow_depth: 5
  prune_non_doc_paths: true
  prune_allow: [LICENSE*, README.*]
  # workspace_dir: ./site/_workspace  # (default) can be omitted
```

Disable all optimizations (legacy full clone behavior):

```yaml
build:
  clone_strategy: fresh
  shallow_depth: 0
  prune_non_doc_paths: false
  # workspace_dir: ./site/_workspace
```

If you encounter mysterious missing images or includes after enabling pruning, re-run with `prune_non_doc_paths: false` to confirm pruning as the cause, then add needed top-level directories or file globs to `prune_allow`.

### Retry Policy Configuration

Retry settings apply to transient stage failures (e.g., network clone issues):

```yaml
build:
  max_retries: 2              # number of retry attempts after the first try (default 2)
  retry_backoff: exponential  # fixed | linear | exponential (default linear)
  retry_initial_delay: 1s     # starting delay before first retry (default 1s)
  retry_max_delay: 30s        # cap for exponential growth (default 30s)
```

Metrics:
- `docbuilder_build_retries_total{stage="clone_repos"}` increments per retry attempt.
- `docbuilder_build_retry_exhausted_total{stage="clone_repos"}` increments when all retries fail.

`BuildReport` also includes `retries` and `retries_exhausted` aggregates for post-build introspection.

### Sample (Expanded) Direct Build Config

Below is an extended example combining repository definitions, Hugo settings, and the new build tuning options:

```yaml
repositories:
  - url: https://git.example.com/org/service-a.git
    name: service-a
    branch: main
    paths: [docs]
    auth:
      type: token
      token: ${GIT_ACCESS_TOKEN}
  - url: https://git.example.com/org/monorepo.git
    name: monorepo
    branch: main
    paths: [docs, documentation/guides]
    auth:
      type: token
      token: ${GIT_ACCESS_TOKEN}

build:
  clone_strategy: auto
  clone_concurrency: 6
  shallow_depth: 8
  prune_non_doc_paths: true
  prune_allow: [LICENSE*, README.*]
  prune_deny: ["*.bak", tmp]
  hard_reset_on_diverge: true
  clean_untracked: true
  max_retries: 3
  retry_backoff: exponential
  retry_initial_delay: 1s
  retry_max_delay: 30s

hugo:
  title: Unified Docs
  description: Combined service & monorepo documentation
  base_url: https://docs.example.com
  theme: hextra

output:
  directory: ./site
  clean: true
```
