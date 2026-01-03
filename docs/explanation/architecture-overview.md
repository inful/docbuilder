# Architecture Overview

DocBuilder follows a layered architecture with clear separation of concerns.

## System Layers

The application is organized into five distinct layers:

### Command Layer
Location: `cmd/docbuilder/commands/`

CLI commands implemented with Kong:
- `build` - Build documentation sites
- `daemon` - Continuous update daemon
- `preview` - Live preview with file watching
- `discover` - Documentation discovery
- `lint` - Documentation linting
- `generate` - Static site generation

### Service Layer
Location: `internal/build`, `internal/daemon`

Core business logic:
- `BuildService` - Orchestrates the build pipeline
- `DaemonService` - Manages long-running daemon operations
- `DiscoveryService` - Repository auto-discovery from forges

### Processing Layer
Location: `internal/hugo`, `internal/docs`, `internal/hugo/pipeline`

Content processing and transformation:
- `Generator` - Hugo site generation orchestration
- `Pipeline` - Fixed transform pipeline for markdown files
- `Theme` - Theme configuration and defaults
- `ReportBuilder` - Build report generation

### Domain Layer
Location: `internal/config`, `internal/state`, `internal/docs`

Core business entities:
- `Config` - Application configuration
- `BuildState` - Build execution state
- `DocFile` - Documentation file representation
- `Repository` - Repository configuration

### Infrastructure Layer
Location: `internal/git`, `internal/forge`, `internal/workspace`, `internal/eventstore`

External integrations:
- `GitClient` - Git operations (clone, update, authentication)
- `ForgeClients` - Forge API clients (GitHub, GitLab, Forgejo)
- `EventStore` - Event sourcing persistence
- `WorkspaceManager` - Temporary directory management

## Build Pipeline

The build process executes eight sequential stages:

1. **PrepareOutput** - Initialize output directories
2. **CloneRepos** - Clone or update repositories with authentication
3. **DiscoverDocs** - Find markdown files in configured paths
4. **GenerateConfig** - Create Hugo configuration
5. **Layouts** - Copy layout templates
6. **CopyContent** - Process and transform markdown files
7. **Indexes** - Generate index pages (main, repository, section)
8. **RunHugo** - Execute Hugo renderer (optional)

Each stage can succeed, produce warnings, or fail. Failures in critical stages halt the pipeline.

## Content Transform Pipeline

Documentation files undergo a fixed transform sequence:

1. Parse front matter (YAML headers)
2. Normalize index files (README.md → _index.md)
3. Build base front matter (defaults)
4. Extract H1 title for index pages
5. Strip H1 heading from content
6. Rewrite relative links (.md → /)
7. Rewrite image links (absolute paths)
8. Generate keywords from @keywords
9. Add repository metadata
10. Add edit link URL
11. Serialize document (front matter + content)

## Key Design Patterns

### Dependency Injection
Services and generators use constructor injection with optional WithX() methods for dependencies.

### Interface Segregation
Interfaces define minimal contracts (Recorder, Renderer, BuildObserver).

### Error Classification
All errors use `ClassifiedError` with categories (Auth, Git, Hugo, Validation, etc.).

### Event Sourcing
Build lifecycle events are appended to an event store for audit and replay.

## Configuration Management

Configuration flows through three levels:

1. **File-based** - YAML configuration files with environment variable expansion
2. **Defaults** - Sensible defaults applied by the system
3. **Runtime** - CLI flags override configuration values

## Authentication

Three authentication methods supported:

- **Token** - Bearer token or personal access token
- **SSH** - SSH key-based authentication
- **Basic** - Username/password

Authentication is per-repository and per-forge configurable.

## Incremental Builds

Change detection operates at three levels:

1. **Repository-level** - Git ref comparison (HEAD SHA)
2. **Content-level** - Documentation file hash comparison
3. **Config-level** - Configuration fingerprint comparison

Unchanged repositories are skipped during clone and discovery stages.

## Observability

Three observability mechanisms:

1. **Structured Logging** - slog-based structured logging
2. **Metrics** - Prometheus metrics for build performance
3. **Build Reports** - JSON reports with stage timings and statistics

## Daemon Mode

Daemon mode adds:

- HTTP API for status and control
- Webhook receivers for forge events
- File system watching for local development
- Scheduled builds with cron expressions
- Repository metadata persistence

## Deployment Patterns

### CLI Mode
Single-shot execution for CI/CD pipelines.

### Daemon Mode
Long-running service for continuous documentation updates.

### Container Mode
Docker container with volume mounts for configuration and output.

## Related Documentation

- [Pipeline Architecture](pipeline-architecture.md) - Detailed pipeline stages
- [Package Architecture](package-architecture.md) - Package structure and dependencies
- [Comprehensive Architecture](comprehensive-architecture.md) - Complete system design
