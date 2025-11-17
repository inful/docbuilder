# Configuration Reference

This page enumerates the primary configuration sections and fields currently supported by DocBuilder's direct build mode.

## Top-Level Structure

```yaml
repositories: []    # List of repositories to aggregate
build: {}           # Performance & workspace tuning
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

## Hugo Section

| Field | Type | Description |
|-------|------|-------------|
| title | string | Site title. |
| description | string | Site description. |
| base_url | string | Hugo BaseURL. |
| theme | enum | Supported optimized themes (`hextra`, `docsy`). |

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

## Recommendations

- Use `clone_strategy: auto` for most CI and daemon scenarios.
- Pin commit or module versions externally until v1.0.0 stability.
- Compare successive `doc_files_hash` values to drive conditional downstream jobs.
