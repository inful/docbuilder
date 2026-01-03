# Package Dependencies

DocBuilder's package structure follows clean architecture principles with clear dependency direction.

## Package Organization

### Command Layer

**Package:** `cmd/docbuilder/commands`

**Purpose:** CLI command definitions and parsing.

**Dependencies:**
- `internal/config` - Configuration loading
- `internal/build` - Build service
- `internal/daemon` - Daemon service
- `internal/forge` - Repository discovery
- `internal/lint` - Documentation linting

**Exports:** CLI command structures for Kong parser.

### Service Layer

**Package:** `internal/build`

**Purpose:** Build orchestration and pipeline execution.

**Dependencies:**
- `internal/config` - Configuration
- `internal/hugo` - Hugo site generation
- `internal/docs` - Documentation discovery
- `internal/git` - Git operations
- `internal/workspace` - Workspace management
- `internal/state` - Build state management
- `internal/metrics` - Metrics recording

**Exports:**
- `BuildService` - Main build orchestrator
- `BuildRequest` - Build request parameters
- `BuildResult` - Build execution result

**Package:** `internal/daemon`

**Purpose:** Long-running daemon operations.

**Dependencies:**
- `internal/build` - Build execution
- `internal/server` - HTTP API handlers
- `internal/eventstore` - Event persistence
- `internal/state` - Daemon state management

**Exports:**
- `DaemonService` - Daemon lifecycle management
- `DaemonConfig` - Daemon configuration

### Processing Layer

**Package:** `internal/hugo`

**Purpose:** Hugo site generation and content processing.

**Dependencies:**
- `internal/config` - Hugo configuration
- `internal/docs` - Documentation files
- `internal/hugo/pipeline` - Transform pipeline
- `internal/hugo/content` - Content processing
- `internal/hugo/editlink` - Edit link generation
- `internal/metrics` - Metrics recording

**Exports:**
- `Generator` - Hugo site generator
- `Renderer` - Hugo rendering abstraction

**Package:** `internal/hugo/pipeline`

**Purpose:** Fixed transform pipeline for markdown files.

**Dependencies:**
- `internal/config` - Transform configuration
- `internal/docs` - Document representation

**Exports:**
- Transform functions (parse, normalize, rewrite, serialize)

**Package:** `internal/docs`

**Purpose:** Documentation discovery and representation.

**Dependencies:**
- `internal/config` - Repository configuration

**Exports:**
- `DocFile` - Documentation file representation
- `Discoverer` - Documentation discovery

### Domain Layer

**Package:** `internal/config`

**Purpose:** Configuration loading and validation.

**Dependencies:**
- `internal/foundation/errors` - Error handling

**Exports:**
- `Config` - Application configuration
- `Repository` - Repository configuration
- `HugoConfig` - Hugo configuration
- `ForgeConfig` - Forge configuration

**Package:** `internal/state`

**Purpose:** Build and daemon state management.

**Dependencies:**
- `internal/config` - Configuration structures

**Exports:**
- `BuildState` - Build execution state
- `GitState` - Git repository state
- `DocsState` - Documentation discovery state
- `PipelineState` - Pipeline execution state

### Infrastructure Layer

**Package:** `internal/git`

**Purpose:** Git operations and authentication.

**Dependencies:**
- `internal/config` - Repository and auth configuration
- `internal/workspace` - Workspace paths
- `github.com/go-git/go-git/v5` - Git library

**Exports:**
- `Client` - Git operations (clone, update, auth)

**Package:** `internal/forge`

**Purpose:** Forge API clients and repository discovery.

**Dependencies:**
- `internal/config` - Forge configuration
- Platform SDKs (GitHub, GitLab, Forgejo APIs)

**Exports:**
- `Client` - Forge client interface
- `ForgejoClient`, `GitHubClient`, `GitLabClient` - Implementations
- `DiscoveryService` - Repository auto-discovery

**Package:** `internal/eventstore`

**Purpose:** Event sourcing persistence.

**Dependencies:**
- `modernc.org/sqlite` - SQLite database

**Exports:**
- `Store` - Event store interface
- `SQLiteStore` - SQLite implementation
- `Event` - Event structure

**Package:** `internal/workspace`

**Purpose:** Temporary directory management.

**Dependencies:** None (leaf package)

**Exports:**
- `Manager` - Workspace lifecycle management

### Foundation Layer

**Package:** `internal/foundation/errors`

**Purpose:** Unified error handling with classification.

**Dependencies:** None (leaf package)

**Exports:**
- `ClassifiedError` - Error with category and context
- `Category` - Error category enumeration
- Builder API for error construction

**Package:** `internal/metrics`

**Purpose:** Metrics recording and Prometheus export.

**Dependencies:**
- `github.com/prometheus/client_golang` - Prometheus client

**Exports:**
- `Recorder` - Metrics recording interface
- `PrometheusRecorder` - Prometheus implementation

**Package:** `internal/observability`

**Purpose:** Logging and tracing utilities.

**Dependencies:**
- `log/slog` - Structured logging

**Exports:**
- Logging helpers and context utilities

## Dependency Rules

### Inward Dependencies Only

Outer layers depend on inner layers:
```
Command → Service → Processing → Domain → Infrastructure → Foundation
```

Inner layers never depend on outer layers.

### Interface Segregation

Infrastructure adapters implement domain interfaces:
- `internal/git` implements repository cloning
- `internal/forge` implements forge API access
- `internal/eventstore` implements event persistence

Domain layer defines interfaces, infrastructure implements them.

### No Circular Dependencies

Package structure prevents circular dependencies:
- Foundation packages have no internal dependencies
- Domain packages depend only on foundation
- Infrastructure packages depend on domain and foundation
- Service packages depend on all lower layers
- Command packages depend on service layer

## Key Interfaces

### Recorder (Metrics)

```go
type Recorder interface {
    RecordBuildDuration(duration time.Duration)
    RecordStageDuration(stage string, duration time.Duration)
    RecordStageResult(stage string, result string)
}
```

Implemented by:
- `PrometheusRecorder` - Production metrics
- `NoopRecorder` - Testing/disabled metrics

### Renderer (Hugo)

```go
type Renderer interface {
    Render(ctx context.Context, siteDir string) (RendererResult, error)
}
```

Implemented by:
- `BinaryRenderer` - Execute Hugo binary
- `MockRenderer` - Testing renderer

### BuildObserver

```go
type BuildObserver interface {
    OnStageStart(stage string)
    OnStageComplete(stage string, result StageResult)
    OnPageRendered()
}
```

Implemented by:
- `recorderObserver` - Bridges to metrics recorder
- Test observers - Verify stage execution

## Package Size Guidelines

| Package | Files | LOC | Purpose |
|---------|-------|-----|---------|
| `cmd/docbuilder/commands` | ~10 | ~1000 | CLI commands |
| `internal/build` | ~10 | ~1500 | Build orchestration |
| `internal/hugo` | ~50 | ~5000 | Hugo generation |
| `internal/config` | ~10 | ~1500 | Configuration |
| `internal/git` | ~5 | ~800 | Git operations |
| `internal/forge` | ~10 | ~2000 | Forge clients |
| `internal/eventstore` | ~5 | ~800 | Event persistence |

Total codebase: ~150 files, ~20,000 LOC (excluding tests).

## Testing Strategy

### Unit Tests

Each package has unit tests for:
- Public API behavior
- Error handling
- Edge cases

### Integration Tests

Integration tests in `test/integration/`:
- End-to-end pipeline execution
- Golden file testing for output verification
- Multi-repository scenarios

### Test Utilities

**Package:** `internal/testing`

Test helpers and builders:
- Configuration builders
- State builders
- Mock implementations

## Related Documentation

- [Architecture Overview](architecture-overview.md) - System architecture
- [Pipeline Architecture](pipeline-architecture.md) - Build pipeline
- [Style Guide](../style_guide.md) - Coding conventions
