---
title: "Architecture Diagrams"
date: 2025-12-15
categories:
  - explanation
tags:
  - architecture
  - diagrams
  - visualization
---

# Architecture Diagrams

> **⚠️ DEPRECATION NOTICE**: This document is partially outdated and requires significant updates to reflect the current transform-based architecture. Many package references (`cli/`, `pipeline/`, `services/`) no longer exist. The layer view diagram has been updated, but other sections still reference old architecture. See [architecture.md](architecture.md) and [functional-specification.md](../reference/functional-specification.md) for current information.

This document provides visual representations of DocBuilder's architecture using ASCII diagrams and Mermaid notation.

## Table of Contents

1. [High-Level System Architecture](#high-level-system-architecture)
2. [Pipeline Flow](#pipeline-flow)
3. [Package Dependencies](#package-dependencies)
4. [Data Flow](#data-flow)
5. [Component Interactions](#component-interactions)
6. [State Machine Diagrams](#state-machine-diagrams)

---

## High-Level System Architecture

### Layer View

```
┌──────────────────────────────────────────────────────────────────┐
│                      COMMAND LAYER                               │
│              (cmd/docbuilder/commands/)                          │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │  Build   │  │  Daemon  │  │  Preview │  │     Discover     │  │
│  │  (Kong)  │  │ (Watch)  │  │  (Live)  │  │   (Analysis)     │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
│       │             │             │                 │            │
└───────┼─────────────┼─────────────┼─────────────────┼────────────┘
        │             │             │                 │
        └─────────────┴─────────────┴─────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    SERVICE LAYER                                │
│             (internal/build, internal/daemon)                   │
│                                                                 │
│  ┌────────────────┐  ┌─────────────────┐  ┌──────────────────┐  │
│  │ BuildService   │  │ DaemonService   │  │ DiscoveryService │  │
│  │                │  │                 │  │                  │  │
│  │ - Run()        │  │ - Start()       │  │ - Discover()     │  │
│  │ - Validate()   │  │ - Stop()        │  │ - Report()       │  │
│  └────────┬───────┘  └────────┬────────┘  └────────┬─────────┘  │
│           │                   │                    │            │
└───────────┼───────────────────┼────────────────────┼────────────┘
            │                   │                    │
            └───────────────────┴────────────────────┘
                                │
┌───────────────────────────────▼─────────────────────────────────────┐
│                        PROCESSING LAYER                             │
│  (internal/hugo, internal/docs, internal/hugo/transforms)           │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                   Hugo Generator                             │   │
│  │                                                              │   │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐     │   │
│  │  │  Transform  │ │   Theme     │ │   Report Builder    │     │   │
│  │  │  Registry   │ │   System    │ │                     │     │   │
│  │  └──────┬──────┘ └──────┬──────┘ └──────────┬──────────┘     │   │
│  └─────────┼───────────────┼───────────────────┼────────────────┘   │
│            │               │                   │                    │
│  ┌─────────▼───────────────▼───────────────────▼──────────────┐     │
│  │                  Transform Pipeline                       │     │
│  │                                                          │     │
│  │  Parse → Build → Enrich → Merge → Transform →            │     │
│  │  Finalize → Serialize                                    │     │
│  └──────────────────────────────────────────────────────────┘     │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
┌─────────────────────────────────▼────────────────────────────────┐
│                          DOMAIN LAYER                            │
│  (internal/config, internal/state, internal/docs)                │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │  Config  │  │  State   │  │ DocFile  │  │  Repository      │  │
│  │          │  │          │  │          │  │                  │  │
│  │ - Hugo   │  │ - Git    │  │ - Path   │  │ - URL            │  │
│  │ - Build  │  │ - Docs   │  │ - Trans  │  │ - Branch         │  │
│  │ - Forge  │  │ - Build  │  │   forms  │  │ - Auth           │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
└───────┼─────────────┼─────────────┼─────────────────┼────────────┘
        │             │             │                 │
        └─────────────┴─────────────┴─────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                         │
│  (internal/git, internal/forge, internal/workspace)             │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────────┐  │
│  │   Git    │  │  Forge   │  │ Event    │  │  Workspace      │  │
│  │  Client  │  │ Clients  │  │ Store    │  │  Manager        │  │
│  │          │  │          │  │          │  │                 │  │
│  │ - Clone  │  │ - GitHub │  │ - Append │  │ - Create()      │  │
│  │ - Update │  │ - GitLab │  │ - Query  │  │ - Cleanup()     │  │
│  │ - Auth   │  │ - Forgejo│  │          │  │                 │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬────────┘  │
│       │             │             │                 │           │
│       └─────────────┴─────────────┴─────────────────┘           │
│                             │                                   │
│                    ┌────────▼────────┐                          │
│                    │   Foundation    │                          │
│                    │     Errors      │                          │
│                    │                 │                          │
│                    │ - ClassifiedErr │                          │
│                    │ - Categories    │                          │
│                    │ - Retry Logic   │                          │
│                    └─────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Pipeline Flow

### Sequential Stage Execution

```mermaid
graph TD
    A[Build Request] --> B[PrepareOutput Stage]
    B --> C[CloneRepos Stage]
    C --> D[DiscoverDocs Stage]
    D --> E[GenerateConfig Stage]
    E --> F[Layouts Stage]
    F --> G[CopyContent Stage]
    G --> H[Indexes Stage]
    H --> I{Render Mode?}
    I -->|always| J[RunHugo Stage]
    I -->|auto| K{Has Hugo?}
    K -->|yes| J
    K -->|no| L[Skip Hugo]
    I -->|never| L
    J --> M[Build Complete]
    L --> M
    M --> N[Generate Report]
    N --> O[Emit Events]
    O --> P[Return Result]
    
    style B fill:#e1f5ff
    style C fill:#e1f5ff
    style D fill:#e1f5ff
    style E fill:#e1f5ff
    style F fill:#e1f5ff
    style G fill:#e1f5ff
    style H fill:#e1f5ff
    style J fill:#e1f5ff
    style M fill:#c8e6c9
    style P fill:#c8e6c9
```

### Stage Detail: CloneRepos

```
CloneRepos Stage
    │
    ├─ For each repository config:
    │   │
    │   ├─ 1. Authenticate
    │   │   ├─ SSH Key
    │   │   ├─ Token
    │   │   └─ Basic Auth
    │   │
    │   ├─ 2. Check Incremental
    │   │   ├─ Compare HEAD ref
    │   │   ├─ Check doc hash
    │   │   └─ Skip if unchanged
    │   │
    │   ├─ 3. Clone or Update
    │   │   ├─ git clone (first time)
    │   │   └─ git pull (update)
    │   │
    │   ├─ 4. Read HEAD
    │   │   └─ Store ref in state
    │   │
    │   └─ 5. Emit Event
    │       ├─ RepositoryCloned
    │       └─ RepositoryUpdated
    │
    └─ Update GitState
```

### Stage Detail: CopyContent

```
CopyContent Stage
    │
    ├─ For each DocFile:
    │   │
    │   ├─ 1. Parse Front Matter
    │   │   ├─ Extract YAML header
    │   │   └─ Parse content
    │   │
    │   ├─ 2. Build Front Matter
    │   │   ├─ Add repository
    │   │   ├─ Add section
    │   │   ├─ Add forge
    │   │   └─ Add date
    │   │
    │   ├─ 3. Inject Edit Link
    │   │   ├─ Check forge capabilities
    │   │   ├─ Build edit URL
    │   │   └─ Add to front matter
    │   │
    │   ├─ 4. Merge Front Matter
    │   │   ├─ Combine parsed + built
    │   │   └─ User values override
    │   │
    │   ├─ 5. Apply Transforms
    │   │   ├─ Custom transformers
    │   │   └─ Replace patterns
    │   │
    │   ├─ 6. Serialize Front Matter
    │   │   ├─ Generate YAML
    │   │   └─ Combine with content
    │   │
    │   └─ 7. Write to content/
    │       └─ Create target file
    │
    └─ Update DocsState
```

---

## Package Dependencies

### Dependency Graph

```
┌──────────┐
│   cmd/   │─────────────────────┐
└────┬─────┘                     │
     │                           ▼
     │                    ┌──────────┐
     │                    │   cli/   │
     │                    └────┬─────┘
     │                         │
     ▼                         ▼
┌─────────────┐        ┌──────────────┐
│  services/  │◄───────│   server/    │
└──────┬──────┘        └──────────────┘
       │
       ▼
┌──────────────┐
│  pipeline/   │
└──────┬───────┘
       │
       ├────────────────────────────────┐
       │                                │
       ▼                                ▼
┌──────────────┐                 ┌──────────────┐
│    config/   │                 │    state/    │
└──────┬───────┘                 └──────┬───────┘
       │                                │
       ├────────────────────────────────┤
       │                                │
       ▼                                ▼
┌────────────────────────────────────────────┐
│              Domain Layer                  │
│  ┌─────────┐  ┌─────────┐  ┌────────────┐  │
│  │  docs/  │  │  hugo/  │  │    forge/  │  │
│  └────┬────┘  └────┬────┘  └─────┬──────┘  │
└───────┼────────────┼─────────────┼─────────┘
        │            │             │
        └────────────┴─────────────┘
                     │
        ┌────────────┴─────────────┐
        │                          │
        ▼                          ▼
┌──────────────┐          ┌──────────────┐
│     git/     │          │  workspace/  │
└──────┬───────┘          └──────┬───────┘
       │                         │
       └─────────────┬───────────┘
                     │
                     ▼
             ┌──────────────┐
             │ foundation/  │
             │   errors/    │
             └──────────────┘
```

### Import Rules

**Layer Dependencies (must respect):**
```
presentation  →  application  →  domain  →  infrastructure
     ✓               ✓             ✓            ✓
     ✗               ✗             ✗            ✓
```

**Package Rules:**
- ✅ `cli/` can import `services/`
- ✅ `services/` can import `pipeline/`
- ✅ `pipeline/` can import `config/`, `state/`, `docs/`, `hugo/`, `git/`
- ✅ `docs/` can import `config/`
- ✅ All packages can import `foundation/`
- ❌ `config/` cannot import `pipeline/`
- ❌ `git/` cannot import `services/`
- ❌ `foundation/` cannot import application packages

---

## Data Flow

### Configuration Loading

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant Config
    participant ENV
    participant Validator
    participant TypedConfig

    User->>CLI: docbuilder build -c config.yaml
    CLI->>Config: Load(configPath)
    Config->>ENV: Read .env files
    ENV-->>Config: Environment variables
    Config->>Config: Parse YAML
    Config->>Config: Expand ${VAR} references
    Config->>Config: Apply defaults
    Config->>Validator: ValidateConfig()
    Validator->>TypedConfig: HugoConfig.Validate()
    TypedConfig-->>Validator: ValidationResult
    Validator->>TypedConfig: DaemonConfig.Validate()
    TypedConfig-->>Validator: ValidationResult
    Validator-->>Config: Validation complete
    Config-->>CLI: Validated Config
    CLI->>CLI: Start build
```

### Build Execution

```mermaid
sequenceDiagram
    participant CLI
    participant BuildService
    participant Pipeline
    participant Git
    participant Docs
    participant Hugo
    participant EventStore

    CLI->>BuildService: Build(config)
    BuildService->>Pipeline: Run(stages)
    Pipeline->>EventStore: Emit BuildStarted
    Pipeline->>Git: CloneRepos()
    Git->>Git: Authenticate
    Git->>Git: Clone/Update
    Git-->>Pipeline: Repository ready
    Pipeline->>EventStore: Emit RepositoryCloned
    Pipeline->>Docs: DiscoverDocs()
    Docs->>Docs: Walk paths
    Docs->>Docs: Filter markdown
    Docs-->>Pipeline: DocFile list
    Pipeline->>EventStore: Emit DocumentationDiscovered
    Pipeline->>Hugo: GenerateConfig()
    Hugo->>Hugo: Apply theme params
    Hugo->>Hugo: Write hugo.yaml
    Hugo-->>Pipeline: Config ready
    Pipeline->>Hugo: CopyContent()
    Hugo->>Hugo: Transform files
    Hugo-->>Pipeline: Content ready
    Pipeline->>Hugo: RunHugo()
    Hugo->>Hugo: Execute hugo build
    Hugo-->>Pipeline: Site generated
    Pipeline->>EventStore: Emit BuildCompleted
    Pipeline-->>BuildService: BuildReport
    BuildService-->>CLI: Success
```

### State Persistence

```mermaid
sequenceDiagram
    participant Pipeline
    participant BuildState
    participant GitState
    participant StateStore
    participant FileSystem

    Pipeline->>BuildState: Create()
    Pipeline->>GitState: Update(repo, head)
    GitState->>BuildState: Merge update
    Pipeline->>BuildState: RecordStage(name, duration)
    Pipeline->>StateStore: Save(state)
    StateStore->>StateStore: Serialize to JSON
    StateStore->>FileSystem: Write .docbuilder/state.json
    FileSystem-->>StateStore: Success
    StateStore-->>Pipeline: State persisted
    
    Note over Pipeline,FileSystem: Later: Incremental build
    
    Pipeline->>StateStore: Load()
    StateStore->>FileSystem: Read .docbuilder/state.json
    FileSystem-->>StateStore: JSON data
    StateStore->>StateStore: Deserialize
    StateStore-->>Pipeline: Previous BuildState
    Pipeline->>Pipeline: Compare HEAD refs
    Pipeline->>Pipeline: Decide skip/clone
```

---

## Component Interactions

### Theme System

```
┌────────────────────────────────────────────────────┐
│              Theme Registry                        │
│                                                    │
│  themes = map[string]Theme{                        │
│    "hextra": &HextraTheme{},                       │
│    "docsy":  &DocsyTheme{},                        │
│  }                                                 │
└─────────────────┬──────────────────────────────────┘
                  │
                  │ GetTheme(name)
                  ▼
         ┌───────────────────┐
         │  Theme Instance   │ 
         │                   │
         │ - Name()          │
         │ - Features()      │
         │ - ApplyParams()   │
         │ - CustomizeRoot() │
         └────────┬──────────┘
                  │
    ┌─────────────┼─────────────┐
    │             │             │
    ▼             ▼             ▼
┌────────┐  ┌─────────┐  ┌──────────┐
│Hextra  │  │ Docsy   │  │ Custom   │
│Theme   │  │ Theme   │  │ Theme    │
└────────┘  └─────────┘  └──────────┘

Generation Flow:
1. Load config.hugo.theme → "hextra"
2. GetTheme("hextra") → HextraTheme
3. HextraTheme.Features() → {UsesModules: true, ...}
4. Core defaults → {title, baseURL, markup}
5. HextraTheme.ApplyParams(ctx, params)
6. User params deep merge
7. HextraTheme.CustomizeRoot(ctx, root)
8. Write hugo.yaml
```

### Forge Integration

```
┌──────────────────────────────────────────────┐
│           Forge Factory                      │
│                                              │
│  NewForge(config) → Forge                    │
└──────────────┬───────────────────────────────┘
               │
               │ Based on config.type
               │
    ┌──────────┼──────────┐
    │          │          │
    ▼          ▼          ▼
┌────────┐ ┌────────┐ ┌─────────┐
│GitHub  │ │GitLab  │ │Forgejo  │
│Client  │ │Client  │ │Client   │
└───┬────┘ └───┬────┘ └────┬────┘
    │          │           │
    └──────────┴───────────┘
               │
               │ All compose
               ▼
        ┌─────────────┐
        │ BaseForge   │
        │             │
        │ HTTP Client │
        │ Auth Header │
        │ Base URL    │
        └──────┬──────┘
               │
               │ Uses
               ▼
        ┌─────────────┐
        │http.Client  │
        │             │
        │- Timeout    │
        │- TLS Config │
        │- Transport  │
        └─────────────┘

Operation Flow:
1. Config specifies forge type: "github"
2. NewForge(config) creates GitHubClient
3. GitHubClient embeds BaseForge
4. BaseForge.NewRequest(method, path)
5. Add auth header: "Authorization: Bearer {token}"
6. Add custom headers: "X-GitHub-Api-Version: 2022-11-28"
7. BaseForge.DoRequest(req)
8. Parse response
9. Return Repository, error
```

### Change Detection

```
┌──────────────────────────────────────────────────┐
│          Change Detector                         │
└──────────────┬───────────────────────────────────┘
               │
               │ DetectChanges(repos)
               ▼
    ┌──────────────────────┐
    │  Load Previous State │
    │  - HEAD refs         │
    │  - Doc hashes        │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │  For each repository │
    └──────────┬───────────┘
               │
               ├─ Level 1: HEAD Comparison
               │  ├─ Read current HEAD
               │  ├─ Compare to previous
               │  └─ Changed? → Include
               │
               ├─ Level 2: Quick Hash
               │  ├─ Hash directory tree
               │  ├─ Compare to previous
               │  └─ Changed? → Include
               │
               ├─ Level 3: Doc Files Hash
               │  ├─ Discover docs
               │  ├─ Sort paths
               │  ├─ SHA-256 hash
               │  ├─ Compare to previous
               │  └─ Changed? → Include
               │
               └─ Level 4: Deletion Detection
                  ├─ Check removed files
                  └─ Deletions? → Include
               
               ▼
    ┌──────────────────────┐
    │    ChangeSet         │
    │                      │
    │ - ChangedRepos: []   │
    │ - SkippedRepos: []   │
    │ - Reasons: map[]     │
    └──────────────────────┘
```

---

## State Machine Diagrams

### Build State Machine

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Preparing: Build request
    Preparing --> Cloning: Output ready
    Cloning --> Discovering: Repos cloned
    Discovering --> Generating: Docs found
    Generating --> Processing: Config generated
    Processing --> Indexing: Content copied
    Indexing --> Rendering: Indexes created
    Rendering --> Complete: Hugo finished
    Processing --> Complete: Skip Hugo
    
    Cloning --> Failed: Clone error
    Discovering --> Failed: Discovery error
    Generating --> Failed: Config error
    Processing --> Failed: Copy error
    Rendering --> Failed: Hugo error
    
    Failed --> [*]
    Complete --> [*]
    
    note right of Cloning
        May skip unchanged repos
        in incremental mode
    end note
    
    note right of Rendering
        Optional based on
        render_mode setting
    end note
```

### Repository State

```mermaid
stateDiagram-v2
    [*] --> New
    New --> Cloning: First build
    Cloning --> Cloned: Success
    Cloned --> Checking: Incremental build
    Checking --> Unchanged: No changes
    Checking --> Updating: Changes detected
    Updating --> Updated: Pull success
    Updated --> Ready: Verified
    Unchanged --> Ready: Skip update
    Ready --> [*]
    
    Cloning --> Error: Network/auth failure
    Updating --> Error: Pull failure
    Error --> Retrying: Retryable
    Retrying --> Cloning: Retry clone
    Retrying --> Updating: Retry update
    Error --> Failed: Max retries
    Failed --> [*]
```

### Theme Loading

```mermaid
stateDiagram-v2
    [*] --> Loading
    Loading --> Resolving: Config loaded
    Resolving --> Found: Theme registered
    Resolving --> NotFound: Unknown theme
    NotFound --> Failed: Error
    Found --> CheckingFeatures: Theme instance
    CheckingFeatures --> ApplyingDefaults: Features loaded
    ApplyingDefaults --> MergingParams: Defaults applied
    MergingParams --> Customizing: User params merged
    Customizing --> Ready: Root customized
    Ready --> [*]
    Failed --> [*]
    
    note right of Found
        Lookup in theme registry
        e.g., "hextra" → HextraTheme
    end note
    
    note right of CheckingFeatures
        UsesModules, AutoMainMenu,
        SearchJSON, etc.
    end note
```

---

## Deployment Architecture

### Single Instance

```
┌────────────────────────────────────────┐
│          Server Host                   │
│                                        │
│  ┌───────────────────────────────────┐ │
│  │      DocBuilder Binary            │ │
│  │                                   │ │
│  │  ┌─────────┐      ┌────────────┐  │ │
│  │  │   CLI   │      │  Daemon    │  │ │
│  │  └─────────┘      └─────┬──────┘  │ │
│  │                         │         │ │
│  │                    ┌────▼─────┐   │ │
│  │                    │  Server  │   │ │
│  │                    │  :8080   │   │ │
│  │                    └──────────┘   │ │
│  └───────────────────────────────────┘ │
│                                        │
│  ┌───────────────────────────────────┐ │
│  │      Workspace                    │ │
│  │  /tmp/docbuilder-*/               │ │
│  └───────────────────────────────────┘ │
│                                        │
│  ┌───────────────────────────────────┐ │
│  │      State                        │ │
│  │  .docbuilder/state.json           │ │
│  └───────────────────────────────────┘ │
└────────────────────────────────────────┘
```

### High Availability

```
                    ┌──────────────┐
                    │ Load Balancer│
                    │   (nginx)    │
                    └──────┬───────┘
                           │
         ┌─────────────────┼────────────────┐
         │                 │                │
    ┌────▼────┐       ┌────▼────┐      ┌────▼────┐
    │ Worker 1│       │ Worker 2│      │ Worker 3│
    │         │       │         │      │         │
    │:8080    │       │:8080    │      │:8080    │
    └────┬────┘       └────┬────┘      └────┬────┘
         │                 │                │
         └─────────────────┼────────────────┘
                           │
                  ┌────────▼────────┐
                  │  Shared Storage │
                  │                 │
                  │  - Event Store  │
                  │  - State DB     │
                  │  - Output Files │
                  └─────────────────┘
```

---

## References

- [Comprehensive Architecture](comprehensive-architecture.md)
- [Architecture Overview](architecture.md)
- [Namespacing Rationale](namespacing-rationale.md)
- [Architecture Migration Plan](../../ARCHITECTURE_MIGRATION_PLAN.md)
