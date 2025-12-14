# Package Architecture Guide

This document provides detailed information about each internal package in DocBuilder, including responsibilities, key types, interfaces, and usage patterns.

## Table of Contents

1. [Foundation Packages](#foundation-packages)
2. [Configuration & State](#configuration--state)
3. [Core Domain](#core-domain)
4. [Infrastructure](#infrastructure)
5. [Application Services](#application-services)
6. [Presentation Layer](#presentation-layer)
7. [Testing Support](#testing-support)

---

## Foundation Packages

### `internal/foundation/errors`

**Purpose:** Unified error handling system for all DocBuilder operations.

**Key Types:**

```go
type ClassifiedError struct {
    category ErrorCategory  // Type-safe category enum
    severity ErrorSeverity  // Fatal, Error, Warning, Info
    retry    RetryStrategy  // Never, Immediate, Backoff, RateLimit, User
    message  string
    cause    error
    context  ErrorContext   // map[string]any
}

type ErrorCategory string  // Type-safe enum
type ErrorSeverity string  // Type-safe enum
type RetryStrategy string  // Type-safe enum
```

**Error Categories:**
- `CategoryConfig` - Configuration errors
- `CategoryGit` - Git operations (clone, fetch, auth)
- `CategoryAuth` - Authentication failures
- `CategoryNotFound` - Resource not found errors
- `CategoryValidation` - Input validation errors
- `CategoryHugo` - Hugo generation failures
- `CategoryBuild` - Build pipeline errors
- `CategoryNetwork` - Network connectivity issues
- `CategoryFileSystem` - File I/O errors
- `CategoryDaemon` - Daemon/service errors
- `CategoryInternal` - Internal/unexpected errors

**Usage:**

```go
// Validation error with context
return errors.ValidationError("invalid theme").
    WithContext("input", theme).
    WithContext("valid_values", []string{"hextra", "docsy"}).
    Build()

// Wrap existing error with category
return errors.WrapError(err, errors.CategoryGit, "failed to clone repository").
    WithContext("repository", repoURL).
    WithContext("branch", branch).
    WithRetry(errors.RetryBackoff).
    Build()

// Extract and check error category
if classified, ok := errors.AsClassified(err); ok {
    if classified.Category() == errors.CategoryAuth {
        // Handle authentication error
    }
}
```

**Design Rationale:**
- Type-safe categories eliminate string-based error codes
- Fluent builder API makes error construction consistent
- Context map provides structured debugging information
- Retry strategy is built into error classification
- Severity levels support alerting and filtering
- HTTP/CLI adapters translate errors at system boundaries

---

## Configuration & State

### `internal/config`

**Purpose:** Configuration loading, validation, and normalization.

**Package Structure:**

```
config/
├── v2.go                  # Main config loading
├── validation.go          # Validation orchestration
├── normalize.go           # Fill implicit values
├── typed/                 # Domain-specific configs
│   ├── hugo_config.go
│   ├── daemon_config.go
│   ├── forge_config.go
│   └── build_config.go
└── defaults.go            # Default values
```

**Key Types:**

```go
type Config struct {
    Repositories []RepositoryConfig
    Hugo         HugoConfig
    Build        BuildConfig
    Output       OutputConfig
    Git          GitConfig
}

type RepositoryConfig struct {
    URL    string
    Name   string
    Branch string
    Paths  []string
    Auth   AuthConfig
}
```

**Configuration Flow:**

```
1. Load YAML file
2. Expand ${ENV_VAR} references
3. Parse into Config struct
4. Apply defaults
5. Normalize (fill implicit values)
6. Validate (all domains)
7. Return validated Config
```

**Validation Architecture:**

```go
// Top-level orchestration
func ValidateConfig(cfg *Config) error

// Domain-specific validation
func (hc *HugoConfig) Validate() foundation.ValidationResult
func (dc *DaemonConfig) Validate() foundation.ValidationResult
func (fc *ForgeConfig) Validate() foundation.ValidationResult
```

**Design Rationale:**
- 3-layer architecture: load → validate → typed
- Environment variable expansion enables secret management
- Normalization separates user intent from internal representation
- Domain validation keeps rules with domain logic

### `internal/state`

**Purpose:** Build state management and persistence.

**Package Structure:**

```
state/
├── build_state.go         # Root state
├── git_state.go           # Repository state
├── docs_state.go          # Documentation state
├── pipeline_state.go      # Execution metadata
└── store/                 # Persistence layer
    ├── interface.go
    ├── json_daemon_info_store.go
    ├── json_statistics_store.go
    └── helpers.go
```

**Key Types:**

```go
type BuildState struct {
    Git      *GitState
    Docs     *DocsState
    Pipeline *PipelineState
}

type GitState struct {
    Repositories map[string]*RepositoryState
}

type RepositoryState struct {
    Name       string
    HEAD       string
    LastUpdate time.Time
    DocHash    string
}

type DocsState struct {
    Files     []*DocFile
    TotalSize int64
}
```

**State Decomposition:**

```
BuildState (root)
    ├─ GitState
    │   ├─ Repository tracking
    │   ├─ HEAD references
    │   └─ Change detection
    │
    ├─ DocsState
    │   ├─ DocFile list
    │   ├─ Size metrics
    │   └─ Hash fingerprint
    │
    └─ PipelineState
        ├─ Stage durations
        ├─ Execution metadata
        └─ Configuration hash
```

**Store Interface:**

```go
type Store interface {
    Save(ctx context.Context, data any) error
    Load(ctx context.Context) (any, error)
    Delete(ctx context.Context) error
}
```

**Design Rationale:**
- Sub-states prevent god object
- Clear ownership boundaries
- JSON serialization for portability
- Store interface enables different backends

---

## Core Domain

### `internal/docs`

**Purpose:** Documentation file discovery and modeling.

**Key Types:**

```go
type DocFile struct {
    SourcePath  string            // Original file path
    HugoPath    string            // Target path in Hugo
    Repository  string            // Repository name
    Section     string            // Documentation section
    Content     []byte            // File content
    FrontMatter map[string]any    // Parsed YAML header
}

type DiscoveryConfig struct {
    Paths       []string
    Ignores     []string
    Extensions  []string
}
```

**Discovery Algorithm:**

```
1. Walk each configured path
2. For each file:
   a. Check extension (.md, .markdown)
   b. Apply ignore patterns
   c. Skip standard files (README, CONTRIBUTING, etc.)
   d. Create DocFile with paths:
      - SourcePath: repos/owner/repo/docs/guide.md
      - HugoPath: content/repo/docs/guide.md
3. Compute doc set hash (SHA-256 of sorted paths)
4. Return DocFile list + hash
```

**Standard Ignores:**
- `README.md`
- `CONTRIBUTING.md`
- `CHANGELOG.md`
- `LICENSE.md`
- `.github/`
- `node_modules/`

**Design Rationale:**
- DocFile is immutable after creation
- Hugo path computed at discovery time
- Hash enables efficient change detection
- Standard ignores prevent clutter

### `internal/hugo`

**Purpose:** Hugo site generation and content processing.

**Package Structure:**

```
hugo/
├── generator.go           # Main orchestrator
├── config.go              # hugo.yaml generation
├── content_copy.go        # Content processing
├── index.go               # Index page generation
├── runner.go              # Hugo binary execution
├── models/                # Transform pipeline
│   ├── frontmatter.go
│   ├── frontmatter_builder.go
│   ├── editlink.go
│   ├── transformers.go
│   └── typed_transformers.go
└── themes/                # Theme implementations
    ├── theme.go           # Interface
    ├── registry.go        # Registration
    ├── hextra/
    │   └── theme_hextra.go
    └── docsy/
        └── theme_docsy.go
```

**Key Interfaces:**

```go
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    ApplyParams(ctx ParamContext, params map[string]any)
    CustomizeRoot(ctx ParamContext, root map[string]any)
}

type ThemeFeatures struct {
    UsesModules     bool
    ModulePath      string
    AutoMainMenu    bool
    SearchJSON      bool
    EditLinkSupport bool
}

type Transformer interface {
    Transform(ctx context.Context, doc *DocFile) error
}
```

**Content Transform Pipeline:**

```go
type Pipeline struct {
    transformers []Transformer
}

// Built-in transformers:
1. FrontMatterParser      - Extract YAML
2. FrontMatterBuilder     - Add metadata
3. EditLinkInjector       - Generate edit URLs
4. FrontMatterMerger      - Combine metadata
5. CustomTransformers     - User-defined
6. FrontMatterSerializer  - Write YAML
```

**Hugo Config Generation:**

```
1. Core defaults
   ├─ title, baseURL
   ├─ languageCode
   └─ markup settings

2. Theme.ApplyParams()
   ├─ Theme-specific defaults
   └─ Feature configuration

3. User params merge (deep)

4. Dynamic fields
   └─ build_date

5. Module import (if UsesModules)

6. Automatic menu (if AutoMainMenu)

7. Theme.CustomizeRoot()
   └─ Final adjustments

8. Write hugo.yaml
```

**Design Rationale:**
- Theme interface enables extensibility
- Features flag capabilities declaratively
- Transform pipeline is composable
- Hugo binary execution is optional

### `internal/forge`

**Purpose:** Git hosting platform abstraction.

**Package Structure:**

```
forge/
├── base_forge.go          # Common HTTP operations
├── interface.go           # Forge interface
├── capabilities.go        # Feature detection
├── github.go              # GitHub implementation
├── gitlab.go              # GitLab implementation
└── forgejo.go             # Forgejo/Gitea implementation
```

**Key Interfaces:**

```go
type Forge interface {
    Name() string
    Type() string
    GetRepository(owner, repo string) (*Repository, error)
    GetFileContent(owner, repo, path, ref string) ([]byte, error)
    ListRepositories(org string) ([]*Repository, error)
    Capabilities() Capabilities
}

type Capabilities struct {
    Webhooks        bool
    AutoDiscovery   bool
    EditLinks       bool
    FileContent     bool
}

type BaseForge struct {
    client         *http.Client
    baseURL        string
    authHeader     string
    customHeaders  map[string]string
}
```

**HTTP Consolidation:**

All forge clients compose `BaseForge`:

```go
type GitHubClient struct {
    *BaseForge
}

func NewGitHubClient(config ForgeConfig) *GitHubClient {
    base := NewBaseForge(config.BaseURL, config.Token)
    base.SetCustomHeader("X-GitHub-Api-Version", "2022-11-28")
    return &GitHubClient{BaseForge: base}
}
```

**Common Operations:**

```go
// BaseForge provides:
func (bf *BaseForge) NewRequest(method, path string) (*http.Request, error)
func (bf *BaseForge) DoRequest(req *http.Request) ([]byte, error)
func (bf *BaseForge) DoRequestWithHeaders(req *http.Request) ([]byte, http.Header, error)
func (bf *BaseForge) SetAuthHeaderPrefix(prefix string)
func (bf *BaseForge) SetCustomHeader(key, value string)
```

**Design Rationale:**
- BaseForge eliminates HTTP duplication
- Composition over inheritance
- Capabilities enable feature detection
- Custom headers support API versioning

### `internal/git`

**Purpose:** Git operations and authentication.

**Package Structure:**

```
git/
├── git.go                 # Main client
├── auth.go                # Authentication
├── workspace.go           # Workspace management
└── head.go                # HEAD reference reading
```

**Key Types:**

```go
type Client struct {
    workspaceManager *workspace.Manager
    auth             AuthStrategy
}

type AuthStrategy interface {
    Name() string
    Apply(config *CloneConfig) error
}

type CloneConfig struct {
    URL       string
    Branch    string
    Depth     int
    Workspace string
}
```

**Authentication Methods:**

```go
// SSH keys
type SSHAuth struct {
    keyPath    string
    passphrase string
}

// Personal access tokens
type TokenAuth struct {
    token string
}

// Basic username/password
type BasicAuth struct {
    username string
    password string
}
```

**Git Operations:**

```go
// Clone or update repository
func (c *Client) CloneOrUpdate(ctx context.Context, config CloneConfig) error

// Read HEAD reference
func (c *Client) ReadRepoHead(repoPath string) (string, error)

// Check if repository exists
func (c *Client) RepositoryExists(path string) bool

// Clean workspace
func (c *Client) CleanWorkspace(path string) error
```

**Design Rationale:**
- Strategy pattern for authentication
- Workspace manager handles temp directories
- HEAD reading supports change detection
- Shallow clones optimize performance

---

## Infrastructure

### `internal/workspace`

**Purpose:** Temporary directory lifecycle management.

**Key Types:**

```go
type Manager struct {
    baseDir    string
    tempDir    string
    persistent bool
}

func NewManager(baseDir string) *Manager
func NewPersistentManager(baseDir, subdirName string) *Manager
func (m *Manager) Create() error
func (m *Manager) GetPath() string
func (m *Manager) Cleanup() error
```

**Workspace Lifecycle:**

```
Ephemeral Mode:
1. NewManager() → Configure base directory
2. Create() → /tmp/docbuilder-{timestamp}/
3. Use workspace for build
4. Cleanup() → Remove directory

Persistent Mode:
1. NewPersistentManager() → Configure fixed path
2. Create() → baseDir/subdirName (reused across builds)
3. Use workspace for build
4. Cleanup() → No-op (directory persists)
```

**Features:**
- Timestamped directories prevent conflicts (ephemeral mode)
- Persistent directories for incremental builds (daemon mode)
- Safe concurrent operations
- Automatic cleanup on error (ephemeral mode only)

**Design Rationale:**
- Simple focused interface
- Predictable naming convention
- Explicit cleanup (no GC reliance)

### `internal/storage` *(Removed)*

**Note:** This package was removed as part of simplifying the CLI build process. The daemon's skip evaluation system (using `internal/state`) provides equivalent functionality without the complexity of content-addressable storage.

**Historical Purpose:** Content-addressed storage abstraction for CLI incremental builds.


**Hash-Based Paths:**
- SHA-256 hash of content
- First 2 chars as directory
- Remainder as filename
- Natural deduplication

**Design Rationale:**
- Content-addressed prevents duplicates
- Flat hierarchy with bucketing
- GC supports cleanup
- Filesystem-based for simplicity

### `internal/eventstore`

**Purpose:** Immutable event log for build lifecycle.

**Key Types:**

```go
type Event struct {
    ID            string
    Timestamp     time.Time
    Type          EventType
    Data          json.RawMessage
    CorrelationID string
}

type EventStore interface {
    Append(ctx context.Context, event *Event) error
    Query(ctx context.Context, filter EventFilter) ([]*Event, error)
}

type EventFilter struct {
    Types         []EventType
    StartTime     *time.Time
    EndTime       *time.Time
    CorrelationID string
}
```

**Event Types:**

```go
const (
    EventBuildStarted            EventType = "build.started"
    EventBuildCompleted          EventType = "build.completed"
    EventBuildFailed             EventType = "build.failed"
    EventRepositoryCloned        EventType = "repository.cloned"
    EventRepositoryUpdated       EventType = "repository.updated"
    EventDocumentationDiscovered EventType = "documentation.discovered"
    EventHugoSiteGenerated       EventType = "hugo.generated"
)
```

**Implementations:**

```go
// In-memory (testing)
type MemoryEventStore struct {
    events []*Event
    mu     sync.RWMutex
}

// File-based (production)
type FileEventStore struct {
    filePath string
    mu       sync.RWMutex
}
```

**Design Rationale:**
- Events are immutable (append-only)
- Correlation ID traces related events
- Query interface supports projections
- JSON serialization for portability

---

## Application Services

### `internal/services`

**Purpose:** High-level business logic orchestration.

**Package Structure:**

```
services/
├── build_service.go       # Build orchestration
├── preview_service.go     # Preview server
└── discovery_service.go   # Discovery-only mode
```

**BuildService:**

```go
type BuildService struct {
    config     *config.Config
    pipeline   *pipeline.Runner
    eventStore eventstore.EventStore
}

func (s *BuildService) Build(ctx context.Context) (*BuildReport, error)
func (s *BuildService) Validate(ctx context.Context) error
func (s *BuildService) Clean(ctx context.Context) error
```

**PreviewService:**

```go
type PreviewService struct {
    config      *config.Config
    buildSvc    *BuildService
    hugoRunner  *hugo.Runner
    watcher     *fsnotify.Watcher
}

func (s *PreviewService) Start(ctx context.Context) error
func (s *PreviewService) Stop() error
```

**DiscoveryService:**

```go
type DiscoveryService struct {
    config *config.Config
    git    *git.Client
    docs   *docs.Discovery
}

func (s *DiscoveryService) Discover(ctx context.Context) (*DiscoveryReport, error)
```

**Design Rationale:**
- Services compose infrastructure
- Single responsibility per service
- Context-based cancellation
- Return domain types (reports)

### `internal/build` *(Current Pipeline)*

**Purpose:** Sequential pipeline for building documentation sites.

**Package Structure:**

```
build/
├── service.go             # BuildService implementation
├── default_service.go     # Default pipeline executor
├── stages.go              # Stage definitions
└── report.go              # Build reporting
```

**Pipeline Stages:**

```
PrepareOutput → CloneRepos → DiscoverDocs → GenerateConfig →
Layouts → CopyContent → Indexes → RunHugo (optional)
```

**Key Interfaces:**

```go
type BuildService interface {
    Run(ctx context.Context, req BuildRequest) (*BuildReport, error)
}

type BuildRequest struct {
    Config    *config.Config
    OutputDir string
}

type BuildReport struct {
    Status      BuildStatus
    StartTime   time.Time
    EndTime     time.Time  
    Stages      []StageReport
    Issues      []BuildIssue
    Summary     string
    Metrics     map[string]interface{}
}
type StageExecutor interface {
    Execute(ctx context.Context, state *state.BuildState) error
    Name() string
}

type Runner struct {
    stages      []StageExecutor
    config      *config.Config
    eventStore  eventstore.EventStore
}

func (r *Runner) Run(ctx context.Context) (*BuildReport, error)
```

**Stage Execution:**

```go
for _, stage := range r.stages {
    start := time.Now()
    
    // Emit start event
    r.eventStore.Append(ctx, &Event{
        Type: EventStageStarted,
        Data: stage.Name(),
    })
    
    // Execute stage
    err := stage.Execute(ctx, state)
    duration := time.Since(start)
    
    // Record in state
    state.Pipeline.RecordStage(stage.Name(), duration, err)
    
    // Emit complete event
    r.eventStore.Append(ctx, &Event{
        Type: EventStageCompleted,
        Data: StageResult{
            Name:     stage.Name(),
            Duration: duration,
            Error:    err,
        },
    })
    
    if err != nil {
        return nil, err
    }
}
```

**Change Detection:**

```go
type ChangeDetector struct {
    state *state.BuildState
}

type ChangeSet struct {
    ChangedRepos  []*config.RepositoryConfig
    SkippedRepos  []*config.RepositoryConfig
    Reasons       map[string]string
}

func (cd *ChangeDetector) DetectChanges(
    ctx context.Context,
    repos []*config.RepositoryConfig,
) (*ChangeSet, error)
```

**Design Rationale:**
- Sequential stage execution
- Event emission at stage boundaries
- Change detection enables incremental
- Context propagation for cancellation

### `internal/incremental` *(Removed)*

**Note:** This package was removed as part of simplifying the CLI build process. It provided change detection and signature caching for CLI incremental builds, but this functionality overlapped with the daemon's skip evaluation system. The daemon uses `internal/state` with rule-based validation instead, which is simpler and more maintainable.

**Historical Purpose:** Change detection and signature management for CLI incremental builds.

**Previous Design:**
- Multi-level detection (HEAD comparison, tree hashing, doc files hashing)
- Signature cache persisted across builds
- Hash-based comparison for determinism
- Optional detection levels for speed vs accuracy trade-offs


---

## Presentation Layer

### `cmd/docbuilder/commands`

**Purpose:** Command-line interface using Kong framework.

**Package Structure:**

```
cmd/docbuilder/
├── main.go                # CLI entry point
└── commands/
    ├── build.go           # Build command
    ├── init.go            # Init command  
    ├── discover.go        # Discovery command
    ├── daemon.go          # Daemon command
    ├── preview.go         # Preview command
    ├── generate.go        # Generate command
    ├── visualize.go       # Visualize command
    └── common.go          # Shared helpers
```

**Command Structure:**

```go
type CLI struct {
    Verbose bool `short:"v" help:"Verbose logging"`
    
    Build    BuildCmd    `cmd:"" help:"Build documentation site"`
    Init     InitCmd     `cmd:"" help:"Initialize configuration"`
    Discover DiscoverCmd `cmd:"" help:"Discover documentation"`
    Daemon   DaemonCmd   `cmd:"" help:"Run as daemon"`
    Preview  PreviewCmd  `cmd:"" help:"Preview documentation"`
    Generate GenerateCmd `cmd:"" help:"Generate assets"`
    Visualize VisualizeCmd `cmd:"" help:"Visualize pipeline"`
}

type BuildCmd struct {
    Config     string `short:"c" help:"Configuration file"`
    Output     string `short:"o" help:"Output directory"`
    RenderMode string `help:"Hugo render mode (always|auto|never)"`
}

func (cmd *BuildCmd) Run(ctx *Context) error {
    // Load config
    // Create build service
    // Execute build
    // Handle errors via foundation/errors CLI adapter
}
```

**Design Rationale:**
- Kong provides type-safe parsing
- Commands are small, focused
- Services handle business logic
- CLI only handles I/O and errors

### `internal/server`

**Purpose:** HTTP server for API and webhooks.

**Package Structure:**

```
server/
├── server.go              # Server setup
├── handlers/              # Request handlers
│   ├── webhook.go         # Forge webhooks
│   ├── build.go           # Build API
│   ├── status.go          # Status endpoint
│   └── metrics.go         # Metrics endpoint
├── middleware/            # HTTP middleware
│   ├── logging.go         # Request logging
│   ├── auth.go            # Authentication
│   ├── recovery.go        # Panic recovery
│   └── cors.go            # CORS headers
└── responses/             # Response types
    ├── json.go            # JSON responses
    └── errors.go          # Error responses
```

**Server Setup:**

```go
type Server struct {
    config  *config.Config
    router  *http.ServeMux
    buildSvc *services.BuildService
}

func (s *Server) Start(ctx context.Context) error {
    // Register routes
    s.router.HandleFunc("/api/v1/build", s.handleBuild)
    s.router.HandleFunc("/api/v1/status", s.handleStatus)
    s.router.HandleFunc("/webhook/github", s.handleGitHubWebhook)
    s.router.HandleFunc("/webhook/gitlab", s.handleGitLabWebhook)
    s.router.HandleFunc("/webhook/forgejo", s.handleForgejoWebhook)
    s.router.HandleFunc("/metrics", s.handleMetrics)
    
    // Apply middleware
    handler := s.applyMiddleware(s.router)
    
    // Start server
    return http.ListenAndServe(":8080", handler)
}
```

**Webhook Handling:**

```go
func handleForgeWebhook(
    w http.ResponseWriter,
    r *http.Request,
    eventHeader string,
    source string,
) {
    // Read event type
    eventType := r.Header.Get(eventHeader)
    
    // Parse payload
    var payload WebhookPayload
    json.NewDecoder(r.Body).Decode(&payload)
    
    // Trigger build if relevant event
    if isPushEvent(eventType) {
        buildService.Build(context.Background())
    }
    
    // Respond
    w.WriteHeader(http.StatusOK)
}
```

**Design Rationale:**
- Standard library HTTP server
- Middleware pattern for cross-cutting concerns
- Shared webhook handler eliminates duplication
- JSON responses with proper error codes

---

## Testing Support

### `internal/testing`

**Purpose:** Test utilities and builders.

**Package Structure:**

```
testing/
├── config_builder.go      # Fluent config builders
├── file_assertions.go     # File/directory assertions
├── cli_runner.go          # CLI integration testing
└── fixtures.go            # Test data
```

**ConfigBuilder:**

```go
type ConfigBuilder struct {
    t      *testing.T
    config *config.Config
}

func NewConfigBuilder(t *testing.T) *ConfigBuilder

func (b *ConfigBuilder) WithGitHubForge(
    name, token string,
) *ConfigBuilder

func (b *ConfigBuilder) WithRepository(
    url, name, branch string,
) *ConfigBuilder

func (b *ConfigBuilder) WithAutoDiscovery(
    org string,
) *ConfigBuilder

func (b *ConfigBuilder) Build() *config.Config

// Usage:
cfg := NewConfigBuilder(t).
    WithGitHubForge("github", "token").
    WithRepository("https://github.com/user/repo", "repo", "main").
    WithAutoDiscovery("myorg").
    Build()
```

**File Assertions:**

```go
func AssertFileExists(t *testing.T, path string)
func AssertDirExists(t *testing.T, path string)
func AssertFileContains(t *testing.T, path, content string)
func AssertHugoConfigValid(t *testing.T, path string)
```

**CLI Runner:**

```go
type CLIRunner struct {
    binary string
}

func (r *CLIRunner) Run(
    t *testing.T,
    args ...string,
) (stdout, stderr string, err error)

// Usage:
runner := NewCLIRunner("./docbuilder")
stdout, stderr, err := runner.Run(t, "build", "-c", "config.yaml")
assert.NoError(t, err)
assert.Contains(t, stdout, "Build complete")
```

**Design Rationale:**
- Fluent builders reduce test boilerplate
- Assertions fail with helpful messages
- CLI runner enables integration tests
- Fixtures provide realistic test data

### `internal/testforge`

**Purpose:** Mock forge implementation for testing.

**Key Types:**

```go
type MockForge struct {
    repositories map[string]*Repository
    files        map[string][]byte
    errors       map[string]error
}

func NewMockForge() *MockForge

func (m *MockForge) AddRepository(repo *Repository)
func (m *MockForge) AddFile(path string, content []byte)
func (m *MockForge) SetError(operation string, err error)
```

**Usage:**

```go
forge := testforge.NewMockForge()
forge.AddRepository(&forge.Repository{
    Name:  "test-repo",
    Owner: "test-owner",
})
forge.AddFile("README.md", []byte("# Test"))

// Use in tests
client := git.NewClient(forge)
repo, err := client.GetRepository("test-owner", "test-repo")
```

**Design Rationale:**
- Eliminates external dependencies in tests
- Predictable behavior for edge cases
- Error injection for failure testing
- Fast test execution

---

## Package Dependencies Summary

**Dependency Rules:**

✅ **Allowed:**
- Lower layers → Foundation
- Application → Domain
- Domain → Infrastructure (via interfaces)
- Presentation → Application

❌ **Prohibited:**
- Foundation → Any application package
- Domain → Application
- Infrastructure → Presentation

**Import Matrix:**

```
Package          | Can Import
-----------------|------------------------------------------
cmd/             | cli/, foundation/
cli/             | services/, config/, foundation/
server/          | services/, config/, foundation/
services/        | pipeline/, config/, state/, foundation/
pipeline/        | docs/, hugo/, git/, config/, state/
config/          | foundation/
state/           | foundation/
docs/            | config/, foundation/
hugo/            | config/, docs/, foundation/
forge/           | foundation/
git/             | workspace/, foundation/
workspace/       | foundation/
storage/         | foundation/
eventstore/      | foundation/
testing/         | All (test-only)
```

---

## References

- [Comprehensive Architecture](comprehensive-architecture.md)
- [Architecture Diagrams](architecture-diagrams.md)
- [Architecture Overview](architecture.md)
- [ADR-000: Uniform Error Handling](../adr/ADR-000-uniform-error-handling.md)
