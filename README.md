# docbuilder

A Go utility for creating documentation sites from multiple Git repositories using Hugo.

## Features

- Clone documentation from multiple Git repositories
- Support for various authentication methods (SSH, tokens, basic auth)
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

## Configuration

### Environment Variables

The application automatically loads environment variables from `.env` and `.env.local` files. Example:

```bash
# .env
GIT_ACCESS_TOKEN=your_git_access_token_here
```

### Configuration File

Example `config.yaml`:

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

### Daemon (V2) Configuration & Auto-Discovery

When running the daemon (`docbuilder daemon`) you use the v2 config format (version: "2.0").
Organizations / groups for a forge are OPTIONAL. If you omit both `organizations:` and `groups:` the daemon enters
an auto-discovery mode: it enumerates all organizations/groups your token can access and then lists their repositories.

Minimal Forgejo example with auto-discovery:

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

Notes:
- Provide explicit `organizations` / `groups` to limit scope and speed up discovery.
- Auto-discovery may increase API calls if you have access to many organizations.
- Per-forge discovery errors are exposed at `/status?format=json` under `discovery_errors`.
- To include repositories even if they have no matching documentation paths, set `required_paths: []` (empty array) which disables pruning.

### Static Site Rendering (Running Hugo Automatically)

By default DocBuilder only scaffolds a Hugo project (content + `hugo.yaml`). To also produce a pre-built static site (`site/public/`) you can opt in via environment variables:

```
DOCBUILDER_RUN_HUGO=1 docbuilder build -c config.yaml
```

You can skip execution explicitly (useful in CI matrix) with:

```
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
- Adds theme-specific params (search, edit links, UI defaults, offline search for Docsy, FlexSearch config for Hextra).
- Avoids using the legacy `theme` filesystem lookup; relies on modules for reproducible builds.

Pinning:
- Hextra is pinned to a stable version in `go.mod` automatically.
- Docsy currently floats (you can pin manually by editing `go.mod`).

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