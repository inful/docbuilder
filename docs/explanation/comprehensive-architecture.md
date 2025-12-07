# Comprehensive Architecture Documentation

## Table of Contents

1. [Overview](#overview)
2. [Core Principles](#core-principles)
3. [System Architecture](#system-architecture)
4. [Package Structure](#package-structure)
5. [Data Flow](#data-flow)
6. [Key Subsystems](#key-subsystems)
7. [Extension Points](#extension-points)
8. [Operational Considerations](#operational-considerations)

---

## Overview

DocBuilder is a Go CLI tool and daemon that aggregates documentation from multiple Git repositories into a unified Hugo static site. It implements a staged pipeline architecture with event sourcing, typed configuration/state management, and comprehensive observability.

### Key Characteristics

- **Event-Driven**: Build lifecycle modeled as events in an event store
- **Type-Safe**: Strongly typed configuration and state (no `map[string]any` in primary paths)
- **Observable**: Unified error system, structured logging, metrics, and tracing
- **Incremental**: Change detection and partial rebuilds for performance
- **Multi-Tenant**: Supports forge namespacing and per-repository configuration
- **Theme-Aware**: Hugo theme integration via modules (Hextra, Docsy)

---

## Core Principles

### 1. Clean Architecture

The codebase follows clean architecture principles with clear dependency direction:

```
presentation → application → domain → infrastructure
     ↓              ↓           ↓            ↓
   cmd/        services/    forge/       git/
   cli/        pipeline/    config/      storage/
   server/                  state/       workspace/
```

**Dependency Rules:**
- Inner layers never depend on outer layers
- Domain logic has no infrastructure dependencies
- Infrastructure adapters implement domain interfaces

### 2. Event Sourcing

Build lifecycle is captured as events in an immutable event store:

```go
type Event struct {
    ID        string
    Timestamp time.Time
    Type      EventType
    Data      json.RawMessage
}
```

**Event Types:**
- `BuildStarted`, `BuildCompleted`, `BuildFailed`
- `RepositoryCloned`, `RepositoryUpdated`
- `DocumentationDiscovered`
- `HugoSiteGenerated`

### 3. Typed State Management

State is decomposed into focused sub-states:

```go
type BuildState struct {
    Git      *GitState      // Repository management
    Docs     *DocsState     // Documentation discovery
    Pipeline *PipelineState // Execution metadata
}
```

Each sub-state has:
- Clear ownership boundaries
- Validation methods
- JSON serialization
- Test builders

### 4. Unified Error Handling

All errors use `foundation.DocBuilderError`:

```go
type DocBuilderError struct {
    Code       ErrorCode
    Message    string
    Cause      error
    Context    map[string]any
    Severity   Severity
    Retryable  bool
}
```

**Error Categories:**
- Configuration errors (non-retryable)
- Network errors (retryable)
- Filesystem errors (transient)
- Validation errors (non-retryable)

---

## System Architecture

### High-Level Components

```
┌───────────────────────────────────────────────────────────┐
│                     Presentation Layer                    │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  │
│  │   CLI    │  │  Daemon  │  │  Server   │  │  Tests   │  │
│  └─────┬────┘  └─────┬────┘  └─────┬─────┘  └─────┬────┘  │
└────────┼─────────────┼─────────────┼──────────────┼───────┘
         │             │             │              │
         └─────────────┴─────────────┴──────────────┘
                       │
         ┌─────────────▼─────────────┐
         │    Service Layer          │
         │  ┌─────────────────────┐  │
         │  │  BuildService       │  │
         │  │  PreviewService     │  │
         │  │  DiscoveryService   │  │
         │  └──────────┬──────────┘  │
         └─────────────┼─────────────┘
                       │
         ┌─────────────▼─────────────┐
         │   Pipeline Layer          │
         │  ┌─────────────────────┐  │
         │  │  StageExecutor      │◄─┤ PrepareOutput
         │  │  PipelineRunner     │◄─┤ CloneRepos
         │  │  ChangeDetector     │◄─┤ DiscoverDocs
         │  └─────────┬───────────┘◄─┤ GenerateConfig
         └────────────┼──────────────┤ Layouts
                      │              ┤ CopyContent
         ┌────────────▼──────────────┤ Indexes
         │   Domain Layer            ┤ RunHugo
         │  ┌─────────────────────┐  │
         │  │  Config             │  │
         │  │  State              │  │
         │  │  DocFile            │  │
         │  │  Repository         │  │
         │  └─────────┬───────────┘  │
         └────────────┼──────────────┘
                      │
         ┌────────────▼──────────────┐
         │  Infrastructure Layer     │
         │  ┌─────────────────────┐  │
         │  │  Git Client         │  │
         │  │  Forge Clients      │  │
         │  │  Storage            │  │
         │  │  Workspace Manager  │  │
         │  │  Event Store        │  │
         │  └─────────────────────┘  │
         └───────────────────────────┘
```

### Pipeline Stages

The build process executes 8 sequential stages:

```
1. PrepareOutput    → Initialize directories
2. CloneRepos       → Git operations
3. DiscoverDocs     → Find markdown files
4. GenerateConfig   → Create hugo.yaml
5. Layouts          → Copy theme templates
6. CopyContent      → Process markdown with transforms
7. Indexes          → Generate index pages
8. RunHugo          → Render static site (optional)
```

Each stage:
- Implements `StageExecutor` interface
- Records duration and outcome
- Emits events to event store
- Returns typed errors

---

## Package Structure

### Foundation Packages

#### `internal/foundation/`
Core types used across all layers:

- **errors/** - Unified error system (`DocBuilderError`)
- **validation/** - Validation result types
- **logging/** - Structured logging setup

#### `internal/config/`
Configuration management:

```
config/
├── v2.go              # YAML loading with env expansion
├── validation.go      # Top-level validation orchestration
├── typed/             # Domain-specific config structs
│   ├── hugo_config.go
│   ├── daemon_config.go
│   └── forge_config.go
└── normalize.go       # Configuration normalization
```

**Key Types:**
- `Config` - Root configuration
- `RepositoryConfig` - Per-repo settings
- `HugoConfig` - Hugo site configuration
- `BuildConfig` - Build behavior settings

#### `internal/state/`
State management:

```
state/
├── build_state.go     # Root build state
├── git_state.go       # Repository state
├── docs_state.go      # Documentation discovery state
├── pipeline_state.go  # Execution metadata
└── store/             # State persistence
    ├── json_*.go      # JSON-based stores
    └── helpers.go     # Common store utilities
```

### Core Domain Packages

#### `internal/forge/`
Git hosting platform abstraction:

```
forge/
├── base_forge.go      # Common HTTP operations
├── github.go          # GitHub implementation
├── gitlab.go          # GitLab implementation
├── forgejo.go         # Forgejo/Gitea implementation
└── capabilities.go    # Feature detection
```

**Key Abstractions:**
- `Forge` interface - Platform operations
- `BaseForge` - Shared HTTP client
- `Capabilities` - Feature flags (webhooks, tokens, etc.)

#### `internal/git/`
Git operations:

```
git/
├── git.go             # Client implementation
├── auth.go            # Authentication strategies
├── workspace.go       # Workspace management
└── head.go            # HEAD reference reading
```

**Auth Methods:**
- SSH keys
- Personal access tokens
- Basic username/password

#### `internal/docs/`
Documentation discovery:

```
docs/
├── discovery.go       # File discovery logic
├── doc_file.go        # DocFile model
└── filters.go         # Ignore patterns
```

**Discovery Rules:**
- Walk configured paths
- Filter `.md` and `.markdown` files
- Ignore `README.md`, `CONTRIBUTING.md`, etc.
- Respect `.docignore` files

#### `internal/hugo/`
Hugo site generation:

```
hugo/
├── generator.go       # Main generator
├── config.go          # hugo.yaml generation
├── content_copy.go    # Content processing
├── index.go           # Index page generation
├── runner.go          # Hugo binary execution
├── models/            # Transform pipeline
│   ├── frontmatter.go
│   ├── editlink.go
│   └── transformers.go
└── themes/            # Theme implementations
    ├── hextra/
    └── docsy/
```

**Content Pipeline:**
```
Parse Front Matter
    ↓
Build Front Matter (metadata injection)
    ↓
Edit Link Injection
    ↓
Merge Front Matter
    ↓
Apply Transforms
    ↓
Serialize Front Matter
```

### Infrastructure Packages

#### `internal/workspace/`
Temporary directory management:

```go
type Manager struct {
    basePath string
}

func (m *Manager) Create() (string, error)
func (m *Manager) Cleanup() error
```

**Lifecycle:**
- Creates timestamped temp directories
- Tracks creation for cleanup
- Safe concurrent operations

#### `internal/storage/`
Content-addressed storage:

```
storage/
├── fs_store.go        # Filesystem-based storage
└── interface.go       # Storage abstraction
```

**Features:**
- Hash-based object paths (`objects/{hash[:2]}/{hash[2:]}`)
- Put/Get/Delete/List operations
- Garbage collection support

#### `internal/eventstore/`
Event persistence:

```
eventstore/
├── store.go           # Event store interface
├── memory.go          # In-memory implementation
└── file.go            # File-based implementation
```

**Operations:**
- Append events (immutable)
- Query by type, time range, correlation ID
- Projections for state reconstruction

### Application Packages

#### `internal/services/`
High-level business logic:

```
services/
├── build_service.go       # Build orchestration
├── preview_service.go     # Preview server
└── discovery_service.go   # Discovery-only mode
```

#### `internal/pipeline/`
Pipeline execution:

```
pipeline/
├── runner.go          # Pipeline orchestration
├── executor.go        # Stage execution
├── stages/            # Stage implementations
│   ├── prepare.go
│   ├── clone.go
│   ├── discover.go
│   ├── generate.go
│   ├── layouts.go
│   ├── content.go
│   ├── indexes.go
│   └── hugo.go
└── change_detector.go # Incremental build logic
```

#### `internal/incremental/`
Change detection:

```
incremental/
├── detector.go        # Change detection logic
├── signature.go       # Content fingerprinting
└── cache.go           # Signature cache
```

**Detection Strategy:**
- Compare repository HEAD refs
- Hash discovered documentation files
- Skip unchanged repositories
- Optional deletion detection

### Presentation Packages

#### `internal/cli/`
Command-line interface:

```
cli/
├── root.go            # Root command
├── build.go           # Build command
├── init.go            # Init command
├── discover.go        # Discovery command
├── daemon.go          # Daemon command
├── preview.go         # Preview command
└── version.go         # Version command
```

Uses [Kong](https://github.com/alecthomas/kong) for parsing.

#### `internal/server/`
HTTP server:

```
server/
├── server.go          # Server setup
├── handlers/          # Request handlers
│   ├── webhook.go
│   ├── build.go
│   ├── status.go
│   └── metrics.go
├── middleware/        # Middleware
│   ├── logging.go
│   ├── auth.go
│   └── recovery.go
└── responses/         # Response types
```

### Testing Packages

#### `internal/testing/`
Test utilities:

```
testing/
├── config_builder.go      # Fluent config builders
├── file_assertions.go     # File/directory assertions
├── cli_runner.go          # CLI integration testing
└── fixtures.go            # Test data
```

#### `internal/testforge/`
Forge test doubles:

```
testforge/
├── mock_forge.go          # Mock forge implementation
└── README.md
```

---

## Data Flow

### Build Flow

```
User invokes CLI/API
    ↓
BuildService.Build(config)
    ↓
PipelineRunner.Run(stages)
    ↓
┌─────────────────────────────────┐
│ Stage 1: PrepareOutput          │
│  - Create/clean output dirs     │
│  - Initialize staging           │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 2: CloneRepos             │
│  - Authenticate with forges     │
│  - Clone/update repositories    │
│  - Detect changes               │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 3: DiscoverDocs           │
│  - Walk documentation paths     │
│  - Filter markdown files        │
│  - Build DocFile list           │
│  - Compute doc set hash         │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 4: GenerateConfig         │
│  - Load theme capabilities      │
│  - Apply theme params           │
│  - Merge user config            │
│  - Write hugo.yaml              │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 5: Layouts                │
│  - Copy custom layouts          │
│  - Set up index templates       │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 6: CopyContent            │
│  - Parse front matter           │
│  - Inject metadata              │
│  - Add edit links               │
│  - Apply transforms             │
│  - Write to content/            │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 7: Indexes                │
│  - Generate _index.md files     │
│  - Repository indexes           │
│  - Section indexes              │
└────────────┬────────────────────┘
             ↓
┌─────────────────────────────────┐
│ Stage 8: RunHugo (optional)     │
│  - Execute hugo build           │
│  - Generate public/             │
└────────────┬────────────────────┘
             ↓
BuildReport generated
    ↓
Events persisted
    ↓
Metrics updated
    ↓
Return to caller
```

### Configuration Loading Flow

```
1. Load YAML file
    ↓
2. Expand ${ENV_VAR} references
    ↓
3. Parse into Config struct
    ↓
4. Apply defaults
    ↓
5. Normalize (fill implicit values)
    ↓
6. Validate (orchestration)
    ├→ ValidateHugoConfig()
    ├→ ValidateDaemonConfig()
    ├→ ValidateForgeConfig()
    └→ ValidateRepositories()
    ↓
7. Return validated Config
```

### State Persistence Flow

```
In-Memory State (BuildState)
    ↓
Sub-State Updates
    ├→ GitState.Update()
    ├→ DocsState.AddDocFile()
    └→ PipelineState.RecordStage()
    ↓
State Store Operations
    ├→ DaemonInfoStore.Save()
    ├→ StatisticsStore.Save()
    └→ JSONStore.Save()
    ↓
Filesystem Persistence
    └→ .docbuilder/state.json
```

### Event Flow

```
Pipeline Stage Execution
    ↓
Emit Event(s)
    ├→ BuildStarted
    ├→ RepositoryCloned
    ├→ DocumentationDiscovered
    └→ BuildCompleted
    ↓
Event Store Append
    ↓
Event Handlers (async)
    ├→ Metrics Update
    ├→ Webhook Notification
    └→ Log Aggregation
```

---

## Key Subsystems

### 1. Theme System

Themes implement a lightweight interface:

```go
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    ApplyParams(ctx ParamContext, params map[string]any)
    CustomizeRoot(ctx ParamContext, root map[string]any)
}
```

**Registration:**
```go
func init() {
    theme.RegisterTheme(&HextraTheme{})
}
```

**Generation Phases:**
1. Core defaults (title, baseURL, markup)
2. `ApplyParams()` - Theme fills/normalizes params
3. User param deep-merge
4. Dynamic fields (build_date)
5. Module import (if `Features().UsesModules`)
6. Automatic menu generation
7. `CustomizeRoot()` - Final adjustments

**Adding New Theme:**
1. Create `internal/hugo/themes/<name>/theme_<name>.go`
2. Implement `Theme` interface
3. Register in `init()`
4. Populate `ThemeFeatures`
5. Add golden test for `hugo.yaml`

### 2. Forge Integration

Forges implement the `Forge` interface:

```go
type Forge interface {
    Name() string
    Type() string
    GetRepository(owner, repo string) (*Repository, error)
    GetFileContent(owner, repo, path, ref string) ([]byte, error)
    ListRepositories(org string) ([]*Repository, error)
    Capabilities() Capabilities
}
```

**HTTP Consolidation:**
All forge clients use `BaseForge` for common operations:

```go
type BaseForge struct {
    client      *http.Client
    baseURL     string
    authHeader  string
    customHeaders map[string]string
}
```

**Auth Methods:**
- GitHub: Bearer token + custom headers
- GitLab: Bearer token
- Forgejo: Token prefix + dual event headers

### 3. Change Detection

The incremental system uses multiple strategies:

```go
type ChangeDetector interface {
    DetectChanges(repos []*RepositoryConfig) (*ChangeSet, error)
}
```

**Detection Levels:**
1. **Repository HEAD** - Git ref comparison
2. **Quick Hash** - Fast directory tree hashing
3. **Doc Files Hash** - SHA-256 of sorted Hugo paths
4. **Deletion Detection** - Optional file removal tracking

**Skip Conditions:**
- Unchanged HEAD ref
- Identical doc set hash
- No deletions detected (if enabled)

### 4. Content Transform Pipeline

Content processing uses a pipeline pattern:

```go
type Transformer interface {
    Transform(ctx context.Context, doc *DocFile) error
}
```

**Built-in Transformers:**
- `FrontMatterParser` - Extract YAML headers
- `FrontMatterBuilder` - Add metadata
- `EditLinkInjector` - Generate edit URLs
- `FrontMatterMerger` - Combine metadata
- `FrontMatterSerializer` - Write YAML

**Custom Transformers:**
Users can add custom transforms in config:

```yaml
hugo:
  content_transforms:
    - type: replace
      pattern: "{{OLD}}"
      replacement: "{{NEW}}"
```

### 5. Observability Stack

#### Logging
Structured logging with `slog`:

```go
logger.Info("Documentation discovered",
    "repository", repoName,
    "files", fileCount,
)
```

#### Metrics
Prometheus-compatible metrics:

```go
buildDuration.Observe(duration.Seconds())
buildsTotal.WithLabelValues("success").Inc()
reposProcessed.WithLabelValues(repoName).Inc()
```

#### Tracing
Context-based distributed tracing:

```go
ctx, span := tracer.Start(ctx, "clone-repository")
defer span.End()
```

#### Error Handling
All errors use unified type:

```go
return foundation.NewDocBuilderError(
    foundation.ErrCodeGitClone,
    "failed to clone repository",
    err,
    foundation.WithContext(map[string]any{
        "repository": repoURL,
        "branch": branch,
    }),
    foundation.WithRetryable(true),
)
```

---

## Extension Points

### 1. Custom Stages

Add new pipeline stages:

```go
type CustomStage struct {
    config *config.Config
}

func (s *CustomStage) Execute(ctx context.Context, state *state.BuildState) error {
    // Custom logic
    return nil
}

func (s *CustomStage) Name() string {
    return "custom"
}
```

Register in pipeline configuration.

### 2. Custom Transformers

Implement transformer interface:

```go
type CustomTransformer struct {}

func (t *CustomTransformer) Transform(ctx context.Context, doc *docs.DocFile) error {
    // Modify doc.Content or doc.FrontMatter
    return nil
}
```

Register in content copy stage.

### 3. Custom Stores

Implement state store interface:

```go
type CustomStore struct {}

func (s *CustomStore) Save(ctx context.Context, data any) error {
    // Custom persistence
    return nil
}

func (s *CustomStore) Load(ctx context.Context) (any, error) {
    // Custom retrieval
    return nil, nil
}
```

Register in state management.

### 4. Event Handlers

Subscribe to build events:

```go
type CustomHandler struct {}

func (h *CustomHandler) Handle(ctx context.Context, event *eventstore.Event) error {
    // React to events
    return nil
}
```

Register with event store.

---

## Operational Considerations

### Performance

**Incremental Builds:**
- Enable with `build.incremental: true`
- Typically 10-100x faster for unchanged repos
- Requires persistent workspace

**Pruning:**
- Enable with `pruning.enabled: true`
- Removes non-doc directories
- Reduces workspace size by 50-90%

**Shallow Clones:**
- Enable with `git.shallow: true`
- Depth 1 clones
- Faster for large repositories

### Scalability

**Multi-Tenancy:**
- Per-tenant configuration
- Isolated workspaces
- Resource quotas

**Horizontal Scaling:**
- Stateless build workers
- Shared event store
- Load balancer distribution

**Resource Limits:**
- Memory: ~100MB base + 10MB per repo
- CPU: 1-2 cores for typical builds
- Disk: 500MB workspace + output size

### Reliability

**Error Recovery:**
- Retryable errors auto-retry (3x default)
- Partial build state preserved
- Atomic staging promotion

**Health Checks:**
- `/health` endpoint
- Workspace availability
- Git connectivity
- Hugo binary presence

**Monitoring:**
- Build success/failure rates
- Stage durations
- Repository update lag
- Disk usage

### Security

**Authentication:**
- Token-based API auth
- SSH key management
- Credential encryption at rest

**Authorization:**
- Repository access control
- API endpoint permissions
- Webhook signature verification

**Secrets Management:**
- Environment variable expansion
- `.env` file support
- External secret stores (planned)

### Maintenance

**Configuration Updates:**
- Hot reload daemon config
- Validate before apply
- Rollback on error

**State Management:**
- State stored in `.docbuilder/`
- JSON format for portability
- Manual cleanup supported

**Dependency Management:**
- Go modules for dependencies
- Hugo binary external
- Theme modules auto-fetched

---

## Architecture Decision Records

See `docs/adr/` for detailed architectural decisions:

- [ADR-000: Uniform Error Handling](../adr/ADR-000-uniform-error-handling.md)
- [ADR-001: Forge Integration Daemon](../../plan/adr-001-forge-integration-daemon.md)

---

## Migration Status

The architecture has undergone significant evolution. See `ARCHITECTURE_MIGRATION_PLAN.md` for:

- 19 completed phases (A-M, O-P, R-S-T-U)
- 2 deferred phases (Q, J)
- ~1,290 lines eliminated
- Zero breaking changes

**Current State:** Architecture migration complete. Codebase follows:
- Event-driven patterns
- Typed configuration/state
- Unified observability
- Single execution pipeline
- Clean domain boundaries

---

## References

- [Getting Started Tutorial](../tutorials/getting-started.md)
- [CLI Reference](../reference/cli.md)
- [Configuration Reference](../reference/configuration.md)
- [Build Report Reference](../reference/report.md)
- [Theme Development](../how-to/add-theme-support.md)
- [Forge Integration](../how-to/configure-forge-namespacing.md)
