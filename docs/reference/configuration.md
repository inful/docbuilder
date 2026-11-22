---
aliases:
  - /_uid/d7e42918-9daa-47e0-9e29-8cda2e13dd7a/
categories:
  - reference
date: 2025-12-15T00:00:00Z
fingerprint: 060709d08b63210ed35abaf163e96fdd16bb55454af2a0779000f500d9e8bcb0
lastmod: "2026-11-22"
tags:
  - configuration
  - yaml
  - settings
title: Configuration Reference
uid: d7e42918-9daa-47e0-9e29-8cda2e13dd7a
---

This page enumerates the primary configuration sections and fields supported by DocBuilder for both direct build and daemon modes.

## Top-Level Structure

```yaml
repositories: []    # List of repositories to aggregate
build: {}           # Performance & workspace tuning
daemon: {}          # Daemon mode settings (link verification, sync, storage)
versioning: {}      # Multi-version documentation (optional)
hugo: {}            # Hugo site metadata & theme
monitoring: {}      # Health/metrics endpoints & logging
output: {}          # Output directory behavior
lint: {}            # Linter excludes (optional)
search: {}          # Where the search of the site runs (optional)
```

## Repositories

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| url | string | yes | Git clone URL. |
| name | string | yes | Unique repository name (used in content paths). Two repositories cannot share a name, and a name cannot be empty or contain a slash. |

Two repositories with the same name are refused when the configuration is
loaded. The name is the identity of a repository in a build: the clone
folder, the state map and the content path all use it, so two repositories
with one name would eat each other and one of them would vanish from the
site. Give each repository a name that says where it comes from, e.g.
`acme-docs` and `initech-docs`.

Repositories that are found on a forge do not need this: when two of them
share a name, the second one gets a name from its full path
(`platform/team-b/docs` becomes `platform-team-b-docs`).
| branch | string | no | Branch to checkout (default per remote). |
| paths | []string | no | Documentation root paths (default: ["docs"]). |
| namespace | string | no | Explicit forge namespace alias (e.g., "github", "gitlab"). Overrides forge_type detection. |
| auth.type | enum | no | Authentication mode: `token`, `ssh`, or `basic`. |
| auth.token | string | conditional | Required when `type=token`. |
| auth.username | string | conditional | Required when `type=basic`. |
| auth.password | string | conditional | Required when `type=basic`. |
| auth.key_path | string | conditional | SSH private key path when `type=ssh`. |

## Build Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| clone_concurrency | int | 4 | Parallel clone/update workers (bounded to repo count). |
| clone_strategy | enum | fresh | Repository acquisition mode: `fresh`, `update`, or `auto`. |
| shallow_depth | int | 1 | Shallow clone depth. Set to `0` to disable shallow cloning. |
| prune_non_doc_paths | bool | false | Remove non-doc top-level directories after clone. |
| prune_allow | []string | [] | Keep-listed directories/files (glob). |
| prune_deny | []string | [] | Force-remove directories/files (glob) except .git. |
| hard_reset_on_diverge | bool | true | Force align local branch to remote on divergence. |
| clean_untracked | bool | false | Remove untracked files after successful update. |
| max_retries | int | 2 | Retry attempts for transient clone/update failures. |
| retry_backoff | enum | linear | Backoff strategy: `fixed`, `linear`, or `exponential`. |
| retry_initial_delay | duration | 1s | First retry delay. |
| retry_max_delay | duration | 30s | Maximum backoff delay cap. |
| workspace_dir | string | derived | Explicit workspace override path. |
| namespace_forges | enum | auto | Forge prefixing: `auto`, `always`, or `never`. |
| max_asset_size | int64 | 52428800 | Maximum size in bytes for individual asset files (default 50MB). Files exceeding this are skipped. |
| skip_if_unchanged | bool | daemon:true, CLI:false | Skip builds when nothing changed (daemon only). |

## Monitoring

The `monitoring` section configures:

- Health/ready/metrics endpoints exposed by the daemon's **admin** HTTP server (see `daemon.http.admin_port`).
- Default daemon logging level and format.

### Monitoring Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| metrics.enabled | bool | true | Enable metrics endpoints on the admin server. |
| metrics.path | string | /metrics | Path for the basic metrics endpoint (JSON summary). |
| health.path | string | /health | Path for the basic health endpoint. |
| logging.level | enum | info | Log level: `debug`, `info`, `warn`, `error`. |
| logging.format | enum | json | Log output format: `json` or `text`. |

Example:

```yaml
monitoring:
  metrics:
    enabled: true
    path: "/metrics"
  health:
    path: "/health"
  logging:
    level: "info"
    format: "json"
```

### Monitoring Endpoints (Admin Server)

The admin server listens on the loopback interface only (`daemon.http.admin_bind`,
default `127.0.0.1`). It has no password and can start builds, so it is not
reachable from the network unless the operator sets `admin_bind: "0.0.0.0"` and
protects the port. Reach it through an SSH tunnel or a reverse proxy.

When `monitoring.metrics.enabled=true`, the admin server exposes:

- `GET <metrics.path>`: basic metrics JSON.
- `GET /metrics/detailed`: detailed daemon metrics JSON (includes counters, gauges, histograms, and custom metrics).
- `GET /metrics/prometheus`: Prometheus text exposition (counters + gauges + a few runtime gauges).

The admin server also exposes:

- `GET <health.path>` and `GET /healthz`: health.
- `GET /health/detailed`: enhanced health if available, otherwise basic health.
- `GET /ready` and `GET /readyz`: daemon readiness (ready once the daemon is running).
- `GET /site-ready`: render readiness (ready once `<output>/public` exists).

### Debouncer Metrics

Build debouncer metrics are emitted in daemon mode and show up under `GET /metrics/detailed` (and most counters/gauges also show up under `GET /metrics/prometheus`).

| Name | Type | Meaning |
|------|------|---------|
| debouncer_build_requests_total | counter | Total build requests observed by the debouncer. |
| debouncer_build_requests_deduped_total | counter | Requests ignored because they repeat the last emitted `JobID` with no pending build. |
| debouncer_builds_emitted_total | counter | Builds emitted by the debouncer (published as `BuildNow`). |
| debouncer_coalesced_requests_total | counter | Number of requests coalesced into emitted builds (adds `requestCount-1` per emitted build). |
| debouncer_pending | gauge | Whether a build is currently pending (0/1). |
| debouncer_pending_after_run | gauge | Whether a follow-up build is pending because a build was already running (0/1). |
| debouncer_planned_request_count | gauge | How many requests are currently coalesced into the next planned build. |
| debouncer_time_to_build_seconds | histogram | Time from first request in a burst until the build is emitted. (Available in detailed JSON; not exported as a Prometheus histogram yet.) |
| debouncer_last_debounce_cause | custom | Last trigger cause: `immediate`, `quiet`, `max_delay`, `after_running`. |

## Daemon Section

Configuration for daemon mode operation, including link verification, sync scheduling, and storage paths.

### HTTP Servers

The daemon runs three HTTP servers.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| docs_port | int | 8080 | Serves the rendered site. Listens on every interface. |
| webhook_port | int | 8081 | Receives forge webhooks. Listens on every interface. |
| admin_port | int | 8082 | Health, metrics and the build triggers. Listens on `admin_bind`. |
| livereload_port | int | 8083 | LiveReload SSE endpoint, only when `build.livereload` is on. |
| admin_bind | string | 127.0.0.1 | Address the admin server listens on. The admin API has no password, so it stays on the loopback interface. Use `0.0.0.0` to listen on every interface - only when something in front of the daemon restricts who can reach the port. |

### Build Queue

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| schedule | string | `0 */4 * * *` | Cron expression for the discovery/sync tick. |
| concurrent_builds | int | 3 | Number of build queue workers. The daemon runs **one build at a time** — a build owns the staging directory, so the other workers wait their turn. The value decides how many requests can be in flight, not how many builds run in parallel. |
| queue_size | int | 100 | How many build requests can wait in the queue. |
| build_on_discovery | bool | true | Whether a discovery run asks for a build. |

### Webhooks

A forge webhook endpoint needs a secret. The daemon refuses to start when a
forge has `webhook.path` (or a default path) but no `webhook.secret`, because
an endpoint without a secret accepts any POST and starts a build.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| secret | string | none | Shared secret used to sign the payload. Required when a webhook path is configured. |
| path | string | `/webhooks/<type>` | Path the forge posts to. |
| events | list | none | Event types to listen for. |

Webhook payloads are limited to 5 MiB. A bigger body is rejected before it is
read into memory.

### Link Verification

Automated link validation using NATS for caching and event publishing. Requires NATS server with JetStream enabled.

DocBuilder stores link verification results and page hashes in a JetStream KV bucket. The bucket is configured with a TTL so JetStream can automatically expire old entries and reclaim space over time. The bucket TTL is set to the larger of `cache_ttl` and `cache_ttl_failures`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Enable automatic link verification after builds. Opt-in: leave it off unless the daemon should check links. |
| nats_url | string | nats://localhost:4222 | NATS server connection URL (supports clustering). |
| subject | string | docbuilder.links.broken | NATS subject for publishing broken link events. |
| kv_bucket | string | docbuilder-link-cache | KV bucket name for caching link verification results. |
| cache_ttl | duration | 24h | TTL for successful link checks in cache. |
| cache_ttl_failures | duration | 1h | TTL for failed link checks in cache. |
| max_concurrent | int | 10 | Maximum concurrent link verification requests. |
| request_timeout | duration | 10s | HTTP timeout for link verification requests. |
| rate_limit_delay | duration | 100ms | Delay between link verification requests. |
| verify_external_only | bool | false | Verify only external links (skip internal links). |
| skip_edit_links | bool | true | Skip edit links that require authentication. |
| follow_redirects | bool | true | Follow HTTP redirects during verification. |
| max_redirects | int | 3 | Maximum number of redirects to follow. |

### Link Verification Examples

Link verification is **opt-in**. The daemon does not check links unless `enabled: true` is set; the examples below all turn it on.

**Basic Configuration:**

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "nats://localhost:4222"
```

**Remote NATS with Authentication:**

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "nats://username:password@nats.example.com:4222"
    subject: "docbuilder.links.broken"
    kv_bucket: "prod-link-cache"
```

**NATS Cluster Configuration:**

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "nats://server1:4222,nats://server2:4222,nats://server3:4222"
```

**TLS/Secure Connection:**

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "tls://nats.example.com:4222"
```

**Custom Verification Settings:**

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "nats://localhost:4222"
    cache_ttl: "48h"              # Cache successful checks for 2 days
    cache_ttl_failures: "30m"     # Recheck failures after 30 minutes
    max_concurrent: 20            # Increase parallelism
    request_timeout: "15s"        # Longer timeout for slow sites
    verify_external_only: true    # Skip internal link checks
    skip_edit_links: true         # Skip edit links (default, requires auth)
```

**Include Edit Links in Verification:**

```yaml
daemon:
  link_verification:
    enabled: true
    skip_edit_links: false        # Verify edit links (will likely fail without auth)
```

**Disable Link Verification:**

```yaml
daemon:
  link_verification:
    enabled: false
```

This is the default: omitting `link_verification` entirely, or setting `enabled: false`, leaves link checking off.

### NATS Requirements

Link verification requires **NATS with JetStream** enabled:

```bash
# Start NATS with JetStream
nats-server -js

# Or configure in nats-server.conf:
jetstream {
    store_dir: /var/lib/nats
    max_memory_store: 1GB
    max_file_store: 10GB
}
```

### Sync Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| schedule | string | 0 */4 * * * | Cron expression for periodic repository sync. |
| build_on_discovery | bool | true | When discovery finds repositories, enqueue a build for them. Set to false for discovery-only operation. |

The schedule is a standard 5-field cron expression (`minute hour day-of-month month day-of-week`) and is evaluated in the daemon process's local time (see `TZ`). Seconds are not supported.

`@every <duration>` expressions are not supported.

### Build Debouncing

Build debouncing controls how DocBuilder coalesces bursts of build requests into fewer builds.

Durations use Go duration syntax (e.g. `10s`, `1m`).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| quiet_window | duration | 10s | When non-immediate requests arrive, wait for a quiet period of this duration before triggering a build. |
| max_delay | duration | 60s | Upper bound on waiting; ensures a build still triggers even if requests keep arriving. Must be >= `quiet_window`. |
| webhook_immediate | bool | true | Whether webhook-triggered requests bypass the quiet window (but still coalesce while a build is running). |

Example:

```yaml
daemon:
  build_debounce:
    quiet_window: "15s"
    max_delay: "2m"
    webhook_immediate: false
```

Semantics:

- “Update one, rebuild all”: webhook-triggered activity may update/check a single repository, but the build renders and publishes the full site (all configured/discovered repositories).
- Debounce timing:
  - `Immediate: true` requests bypass the quiet window and try to trigger a build as soon as possible.
  - `Immediate: false` requests wait for a quiet period (`quiet_window`), but a build will still fire by `max_delay`.
- Build-running behavior: if a build is already running, DocBuilder will not start a concurrent build; it coalesces all requests into at most one follow-up build after the running build completes.
- Job IDs under coalescing: when multiple requests map to one build, DocBuilder reuses the debouncer’s planned job ID so logs and webhook responses remain stable across bursts.
- Eventual consistency: by default, builds use the HEAD of each configured branch at build time. DocBuilder may optionally pin repositories to specific commit SHAs for stricter “what was built” semantics (snapshot builds).

### Storage Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| output_dir | string | ./site | Output directory (must match `output.directory`). |
| repo_cache_dir | string | - | Persistent repository cache directory. Holds the cloned repositories. |
| state_dir | string | ./daemon-data | Where the daemon keeps its state: the state file, the event log and the remote-head cache. Separate from the repository cache, so a cache wipe does not take the state with it. |
| event_retention | string | 720h | How long build events are kept in the event store (`events.db`). `0` keeps every event. |

If `daemon-state.json` cannot be read, the daemon quarantines it as `daemon-state.json.corrupt-<timestamp>` and starts with empty state rather than failing.

### Daemon Configuration Example

```yaml
daemon:
  link_verification:
    enabled: false                # Opt-in: link checking is off by default
    nats_url: "nats://localhost:4222"
    cache_ttl: "24h"
  sync:
    schedule: "*/10 * * * *"  # Sync every 10 minutes
    build_on_discovery: true  # Default: enqueue builds for discovered repositories
  storage:
    repo_cache_dir: "./daemon-data/repos"
    state_dir: "./daemon-data/state"  # State, events and remote-head cache
    event_retention: "720h"         # Keep build events for 30 days
```

## Versioning Section

Enables multi-version documentation by discovering and building multiple branches/tags from each repository.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Enable multi-version documentation. |
| strategy | enum | branches_and_tags | Version selection: `branches_and_tags`, `branches_only`, or `tags_only`. |
| default_branch_only | bool | false | Build only the default branch (overrides strategy). |
| branch_patterns | []string | [\"*\"] | Branch name patterns to include (glob). |
| tag_patterns | []string | [\"*\"] | Tag name patterns to include (glob). |
| max_versions_per_repo | int | 10 | Maximum versions to build per repository. |

### Versioning Examples

```yaml
versioning:
  enabled: true
  strategy: branches_and_tags
  max_versions_per_repo: 5
  tag_patterns:
    - \"v*\"           # Match semantic versions
    - \"[0-9]*\"       # Match numeric tags
  branch_patterns:
    - \"main\"
    - \"develop\"
    - \"release/*\"
```

With versioning enabled, DocBuilder:
- Discovers available branches/tags from each repository
- Expands each repository into multiple versioned builds
- Clones each version separately (branches use `refs/heads/`, tags use `refs/tags/`)
- Organizes content under repository-version paths
- Generates Hugo configuration with version metadata for version switchers

## Hugo Section

| Field | Type | Description |
|-------|------|-------------|
| title | string | Site title. |
| description | string | Site description. |
| base_url | string | Hugo BaseURL. |
| chrome | string | How the navigation is rendered: `full` (the default) or `shared`. See [Chrome](#chrome). |
| params | map[string]any | Theme parameters (optional). |
| taxonomies | map[string]string | Custom taxonomy definitions (optional). |
| custom_css | string | Site-wide CSS inlined in the theme's head hook. See [Custom CSS](#custom-css). |
| enable_page_transitions | bool | Animate the move between pages with the View Transitions API. |

### The theme

The site is built with the [Docsy theme](https://github.com/docsy/docsy),
imported as a Hugo Module (`github.com/docsy/docsy/theme`, version
`v0.18.1`). There is nothing to install and nothing to choose: the
theme ships with the build.

Docsy ships its stylesheet prebuilt. Hugo 0.170 has no Sass support of
its own, so DocBuilder vendors the compiled stylesheet, the Bootstrap
JavaScript and the two web fonts the stylesheet references;
`scripts/vendor-docsy-assets.sh` rebuilds them.

See [Use the Docsy Theme](../how-to/use-docsy-theme.md) for the
complete guide.

### Chrome

| Mode | What it does |
|------|--------------|
| `full` (the default) | The navigation is rendered on every page. It is there without JavaScript. |
| `shared` | The navigation is rendered on one page and restored in the browser. The published site is much smaller, and the navigation needs JavaScript. |

```yaml
hugo:
  chrome: shared
```

The theme calls its shared mode experimental and recommends `full` for
production, so that is the default. The saving is real on a large
site: the navigation is carried by every page, so a site with a
thousand pages carries it a thousand times.

### Taxonomies

Hugo taxonomies allow you to classify and organize content. DocBuilder automatically configures the default Hugo taxonomies (`tags` and `categories`) but you can customize or extend them.

**Default Configuration:**

```yaml
hugo:
  taxonomies:
    tag: tags
    category: categories
```

**Custom Taxonomies:**

```yaml
hugo:
  taxonomies:
    tag: tags
    category: categories
    author: authors      # Custom taxonomy
    topic: topics        # Custom taxonomy
```

The taxonomy key (e.g., `tag`, `author`) is the singular form used in Hugo's templates and URLs, while the value (e.g., `tags`, `authors`) is the plural form used for the collection.

**Usage in Frontmatter:**

Once configured, you can use taxonomies in your markdown frontmatter:

```yaml
---
title: "My Documentation Page"
tags: ["api", "golang", "tutorial"]
categories: ["guides"]
authors: ["john-doe"]
topics: ["backend-development"]
---
```

DocBuilder's FrontMatter model supports `tags`, `categories`, and `keywords` fields by default. Custom taxonomies can be added through the `Custom` field or by extending the FrontMatter structure.

### Custom CSS

Use `hugo.custom_css` to ship site-wide CSS. DocBuilder inlines the value as a `<style>` block in the theme's head hook, which the theme includes in every page head — so the rule applies site-wide without further wiring.

| `custom_css` value | Effect |
|--------------------|--------|
| unset (default) | No `<style>` block is written. The theme's own styling applies. |
| set to a non-empty string | The value is inlined verbatim. Use this to ship your own rules. |
| set to a whitespace-only string | The `<style>` block is suppressed entirely. The rest of the hook (template metadata, view-transitions link if enabled) is still written. |

**Example — your own rules:**

```yaml
hugo:
  custom_css: |
    /* My site-wide override */
    .td-sidebar { font-size: 0.95rem; }
```

**Example — opt out of the `<style>` block (keep all other hook contents):**

```yaml
hugo:
  custom_css: "   \n   "   # any whitespace-only value
```

### The navigation

The sidebar is the content tree: the sections and pages of the
repositories, in the order the documents declare. A document's
`categories:` front matter is a **taxonomy**, not a sidebar block, so
every category gets a landing page under `/categories/<name>/` that
lists its documents with their descriptions — across every repository
the build aggregates.

To order the sidebar, use the `weight` front matter of the section
index pages.

## Output Section

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| directory | string | ./site | Output root. |
| clean | bool | true | Remove directory before build. |

### Output Directory Unification

- DocBuilder treats `output.directory` as the canonical output root for both direct builds and daemon mode.
- In daemon mode, `daemon.storage.output_dir` must match `output.directory`. If not provided, it is derived from `output.directory`.
- A validation check enforces this equality (after path normalization). Mismatches cause configuration loading to fail.
- Recommendation: set only `output.directory`; avoid setting `daemon.storage.output_dir` unless absolutely necessary.

## Search Section

Configures where the search of the site runs.

The daemon serves the site, so the search runs there: the daemon keeps
an index of the rendered pages and answers the queries. The index never
reaches the visitor, and the search stays fast however large the site
is.

A site that is served as a static tree - exported and handed to a web
server - has no server to ask, so it uses the browser-side index the
theme ships. That index holds the text of every page, so it grows with
the site and every visitor downloads it.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `provider` | string | the builder decides | Where the search runs: `server` (the daemon's own index) or `lunr` (the browser-side index the theme ships). Left unset, the daemon chooses `server` and a plain build chooses `lunr`. |
| `max_results` | int | `10` | How many results a search returns. |

```yaml
search:
  provider: server
  max_results: 10
```

See [Server-side search](../explanation/server-side-search.md) for how
the index is built and what each provider does.

## Lint Section

Configures the documentation linter's exclude list — files that should
be skipped from lint and schema enforcement.

The linter applies a built-in cross-forge union of git-forge-conventional
filenames by default (README, CONTRIBUTING, SECURITY, CHANGELOG, LICENSE,
and variants across GitHub / GitLab / Forgejo). The `lint` block lets
you override that list or extend it.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `excludes` | `[]string` | (built-in union) | Glob patterns matching files to skip from lint and schema enforcement. An explicit empty list disables all excludes. |

### Default excludes (when `lint` is absent)

The built-in default list covers the cross-forge union of
git-forge-conventional files. Exact globs (e.g. `README.md`) match only
at the repository root — nested files with the same name are normal
doc-pages and are linted normally. Recursive globs (with `**`) match
anywhere in the tree.

```yaml
# Files matched at the repo root only:
- README.md
- README
- README.rst
- README.txt
- README.adoc
- CONTRIBUTING.md
- CONTRIBUTING
- SECURITY.md
- SECURITY
- CODE_OF_CONDUCT.md
- CODE_OF_CONDUCT
- SUPPORT.md
- CHANGELOG.md
- CHANGELOG
- LICENSE
- LICENSE.md
- LICENSE.txt
- LICENSE.rst
- COPYING
- COPYING.md
- AUTHORS
- AUTHORS.md
- CONTRIBUTORS
- CONTRIBUTORS.md
- NOTICE
- NOTICE.md

# Recursive globs (match anywhere in the tree):
- .github/ISSUE_TEMPLATE/**/*.md
- .github/PULL_REQUEST_TEMPLATE.md
- .gitlab/issue_templates/*.md
- .gitlab/merge_request_templates/*.md
- .gitea/ISSUE_TEMPLATE/**/*.md
- .gitea/PULL_REQUEST_TEMPLATE.md
```

### Examples

```yaml
# Defaults still apply; add an additional pattern alongside:
lint:
  excludes:
    - "README.md"
    - "vendor-docs/**/*.md"
```

```yaml
# Empty list — disable all excludes (every file is linted):
lint:
  excludes: []
```

```yaml
# Custom list — replaces the defaults:
lint:
  excludes:
    - "INTERNAL.md"
    - ".team/**/*.md"
```

Note: omitting the `lint` block entirely keeps the defaults. To replace
them, supply an explicit `excludes` list.

## Build Report Fields (Selected)

| Field | Purpose |
|-------|---------|
| cloned_repositories | Successful clones/updates |
| failed_repositories | Failed clone/auth attempts |
| rendered_pages | Markdown pages written |
| static_rendered | Hugo run succeeded |
| doc_files_hash | Fingerprint of docs set |
| issues[] | Structured issue list |

## Environment Variable Expansion

Values like `${GIT_ACCESS_TOKEN}` in YAML are expanded using the current process environment. `.env` and `.env.local` files are loaded automatically (last one wins on conflicts).

## Health and Readiness Endpoints

- Endpoints are exposed on both the docs port and the admin port.
- `GET /health`: basic liveness endpoint; returns 200 when the server is responsive.
- `GET /ready`: daemon readiness; returns 200 once the daemon is running and able to serve.
  - It does **not** depend on the rendered site. A daemon whose forge is unreachable, or
    whose first build has not finished, is still a working daemon.
- `GET /site-ready`: render readiness; returns 200 only when `<output.directory>/public` exists,
  503 before the first successful render or if the public folder is missing.
  - Use this instead of `/ready` when traffic must wait for a rendered site.
- When serving on the docs port, if no rendered site exists yet (the first build is still running, or the previous one was lost), DocBuilder returns a short 503 HTML placeholder for every documentation URL, showing the daemon's status and build queue depth. Asset paths still 404. The placeholder is replaced automatically by the rendered site once it exists.
- During a rebuild the previous site keeps being served from `<output>.prev/public`, so the site never goes blank while a new version renders.

## Kubernetes Probes

- Probe either the docs port or the admin port.
- Use `/health` for the **startup** and **liveness** probes. A startup probe on `/ready`
  ties container startup to a successful build: if the build fails, Kubernetes restarts
  the container, which restarts the build, which fails again — a loop that never resolves.
- Use `/ready` for the **readiness** probe when the daemon's API should only receive
  traffic once it is up, or `/site-ready` when traffic must additionally wait for a
  rendered site.
- Prefer probing `/ready` instead of the docs root `/`, since the root may return a temporary 503 placeholder before the first render.

Example: probe the docs port (8080)

```yaml
containers:
  - name: docbuilder
    ports:
      - containerPort: 8080 # docs
      - containerPort: 8081 # webhooks
      - containerPort: 8082 # admin
    startupProbe:
      httpGet:
        path: /health
        port: 8080
      periodSeconds: 5
      failureThreshold: 30
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      periodSeconds: 5
      failureThreshold: 3
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 10
```

Example: probe the admin port (8082)

```yaml
containers:
  - name: docbuilder
    startupProbe:
      httpGet:
        path: /health
        port: 8082
      periodSeconds: 5
      failureThreshold: 30
    readinessProbe:
      httpGet:
        path: /ready
        port: 8082
      periodSeconds: 5
      failureThreshold: 3
    livenessProbe:
      httpGet:
        path: /health
        port: 8082
      initialDelaySeconds: 5
      periodSeconds: 10
```

Swap `path: /ready` for `path: /site-ready` in the readiness probe to hold traffic
until the first render has produced `<output.directory>/public`.

## Namespacing Behavior

When `namespace_forges=auto` and more than one distinct forge is present across repositories, content paths are written under `content/<forge>/<repo>/...`. Otherwise they remain `content/<repo>/...`.

## Skip Evaluation (Daemon Mode)

When running in daemon mode, builds are automatically skipped when nothing has changed:

1. **State Tracking**: Repository commits, config hash, and doc file hashes are saved after each successful build
2. **Skip Rules**: Validates version, config, repository commits, and content integrity
3. **Automatic**: Enabled by default in daemon mode (`skip_if_unchanged: true`)
4. **CLI Mode**: Not used - CLI builds always run when explicitly requested

This prevents unnecessary rebuilds when daemon polls/watches for changes but repositories haven't been updated.

For CLI mode, simply don't run `docbuilder build` if you don't want a build. No caching is needed.

## Content Policies (Daemon Mode)

The daemon can apply global content filters to the documentation site.

### Public-Only Filter

When enabled, only documents explicitly marked as public are included in the generated site.

```yaml
daemon:
  content:
    public_only: true
```

**Guardrails & Behavior:**

1.  **Strict Boolean Check**: Documents must have `public: true` in their YAML frontmatter. `public: "true"` (string) or missing keys are treated as `false` and excluded.
2.  **Structural Scoping**: If a repository or section contains no public documents, that entire navigation branch/section index is omitted from the generated site to prevent "ghost" links.
3.  **Cross-Repo Consistency**: This is a global setting. If enabled, it applies to all repositories fetched by the daemon instance.
4.  **Content Discovery**: Assets (images, PDFs) adjacent to filtered Markdown files are still copied if they are within a documentation path, as they may be referenced by other public documents.

## Recommendations

- Use `clone_strategy: auto` for most CI and daemon scenarios.
- Pin commit or module versions externally until v1.0.0 stability.
- Compare successive `doc_files_hash` values to drive conditional downstream jobs.
