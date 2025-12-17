---
title: "Configuration Reference"
date: 2025-12-15
categories:
  - reference
tags:
  - configuration
  - yaml
  - settings
---

# Configuration Reference

This page enumerates the primary configuration sections and fields supported by DocBuilder for both direct build and daemon modes.

## Top-Level Structure

```yaml
repositories: []    # List of repositories to aggregate
build: {}           # Performance & workspace tuning
daemon: {}          # Daemon mode settings (link verification, sync, storage)
versioning: {}      # Multi-version documentation (optional)
hugo: {}            # Hugo site metadata & theme
output: {}          # Output directory behavior
```

## Repositories

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| url | string | yes | Git clone URL. |
| name | string | yes | Unique repository name (used in content paths). |
| branch | string | no | Branch to checkout (default per remote). |
| paths | []string | no | Documentation root paths (default: ["docs"]). |
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
| shallow_depth | int | 0 | If >0 use shallow clones of that depth. |
| prune_non_doc_paths | bool | false | Remove non-doc top-level directories after clone. |
| prune_allow | []string | [] | Keep-listed directories/files (glob). |
| prune_deny | []string | [] | Force-remove directories/files (glob) except .git. |
| hard_reset_on_diverge | bool | false | Force align local branch to remote on divergence. |
| clean_untracked | bool | false | Remove untracked files after successful update. |
| max_retries | int | 2 | Retry attempts for transient clone/update failures. |
| retry_backoff | enum | linear | Backoff strategy: `fixed`, `linear`, or `exponential`. |
| retry_initial_delay | duration | 1s | First retry delay. |
| retry_max_delay | duration | 30s | Maximum backoff delay cap. |
| workspace_dir | string | derived | Explicit workspace override path. |
| namespace_forges | enum | auto | Forge prefixing: `auto`, `always`, or `never`. |
| skip_if_unchanged | bool | daemon:true, CLI:false | Skip builds when nothing changed (daemon only). |

## Daemon Section

Configuration for daemon mode operation, including link verification, sync scheduling, and storage paths.

### Link Verification

Automated link validation using NATS for caching and event publishing. Requires NATS server with JetStream enabled.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable automatic link verification after builds. |
| nats_url | string | nats://localhost:4222 | NATS server connection URL (supports clustering). |
| subject | string | docbuilder.links.broken | NATS subject for publishing broken link events. |
| kv_bucket | string | docbuilder-link-cache | KV bucket name for caching link verification results. |
| cache_ttl | duration | 24h | TTL for successful link checks in cache. |
| cache_ttl_failures | duration | 1h | TTL for failed link checks in cache. |
| max_concurrent | int | 10 | Maximum concurrent link verification requests. |
| request_timeout | duration | 10s | HTTP timeout for link verification requests. |
| rate_limit_delay | duration | 100ms | Delay between link verification requests. |
| verify_external_only | bool | false | Verify only external links (skip internal links). |
| follow_redirects | bool | true | Follow HTTP redirects during verification. |
| max_redirects | int | 3 | Maximum number of redirects to follow. |

### Link Verification Examples

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
```

**Disable Link Verification:**

```yaml
daemon:
  link_verification:
    enabled: false
```

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
| schedule | string | */5 * * * * | Cron expression for periodic repository sync. |

### Storage Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| output_dir | string | ./site | Output directory (must match `output.directory`). |
| repo_cache_dir | string | - | Persistent repository cache directory. |

### Daemon Configuration Example

```yaml
daemon:
  link_verification:
    enabled: true
    nats_url: "nats://localhost:4222"
    cache_ttl: "24h"
  sync:
    schedule: "*/10 * * * *"  # Sync every 10 minutes
  storage:
    repo_cache_dir: "./daemon-data/repos"
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
| params | map[string]any | Relearn theme parameters (optional). |
| taxonomies | map[string]string | Custom taxonomy definitions (optional). |

**Note:** Theme selection has been removed. DocBuilder uses the Relearn theme exclusively.

### Relearn Theme Parameters

Customize Relearn theme behavior via `hugo.params`:

```yaml
hugo:
  params:
    themeVariant: "relearn-dark"
    disableSearch: false
    collapsibleMenu: true
    showVisitedLinks: true
```

See [Use Relearn Theme](../how-to/use-relearn-theme.md) for complete parameter reference.

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
- `GET /ready`: readiness endpoint tied to render state.
  - Returns 200 only when `<output.directory>/public` exists.
  - Returns 503 before the first successful render or if the public folder is missing.
- When serving on the docs port, if the site is not yet rendered and the request path is `/`, DocBuilder returns a short 503 HTML placeholder indicating that the documentation is being prepared. This switches automatically to the rendered site once available.

## Kubernetes Probes

- Probe either the docs port or the admin port. Use `/ready` for readiness and `/health` for liveness.
- Prefer probing `/ready` instead of the docs root `/`, since the root may return a temporary 503 placeholder before the first render.

Example: probe the docs port (8080)

```yaml
containers:
  - name: docbuilder
    ports:
      - containerPort: 8080 # docs
      - containerPort: 8081 # webhooks
      - containerPort: 8082 # admin
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

## Recommendations

- Use `clone_strategy: auto` for most CI and daemon scenarios.
- Pin commit or module versions externally until v1.0.0 stability.
- Compare successive `doc_files_hash` values to drive conditional downstream jobs.
