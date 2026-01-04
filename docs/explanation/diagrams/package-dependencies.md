---
title: "Package Dependencies Diagram"
date: 2026-01-04
categories:
  - explanation
  - architecture
tags:
  - packages
  - dependencies
  - structure
---

# Package Dependencies Diagram

This document visualizes the dependency relationships between DocBuilder packages, showing how different layers interact and the import rules that must be followed.

**Last Updated:** January 4, 2026 - Reflects current package structure.

## Dependency Graph

```
┌──────────────────┐
│ cmd/docbuilder/  │
│   commands/      │
└────────┬─────────┘
         │
         ▼
┌───────────────────┐
│ internal/build/   │
│ internal/daemon/  │
└────────┬──────────┘
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
┌────────────────────────────────────────────────┐
│              Domain Layer                      │
│  ┌─────────┐  ┌──────────────┐  ┌───────────┐  │
│  │  docs/  │  │    hugo/     │  │   forge/  │  │
│  └────┬────┘  └──────┬───────┘  └─────┬─────┘  │
│       │              │                │        │
│       │         ┌────▼─────────┐      │        │
│       │         │  pipeline/   │      │        │
│       │         │  (transforms)│      │        │
│       │         └──────────────┘      │        │
└───────┼────────────────┼──────────────┼────────┘
        │                │              │
        └────────────────┴──────────────┘
                         │
        ┌────────────────┴─────────────┐
        │                              │
        ▼                              ▼
┌──────────────┐            ┌──────────────┐
│     git/     │            │  workspace/  │
└──────┬───────┘            └──────┬───────┘
       │                           │
       └─────────────┬─────────────┘
                     │
                     ▼
             ┌──────────────┐
             │ foundation/  │
             │   errors/    │
             └──────────────┘
```

## Layer Dependencies (Must Respect)

```
commands  →  services  →  domain  →  infrastructure
   ✓            ✓          ✓            ✓
   ✗            ✗          ✗            ✓
```

**Legend**:
- ✓ = Allowed to import
- ✗ = NOT allowed to import

## Import Rules

### ✅ Allowed Imports

**Command Layer** (`cmd/docbuilder/commands/`):
- ✅ `internal/build/` - Build service orchestration
- ✅ `internal/daemon/` - Daemon service
- ✅ `internal/config/` - Configuration loading
- ✅ `internal/foundation/errors` - Error handling

**Service Layer** (`internal/build/`, `internal/daemon/`):
- ✅ `internal/hugo/` - Hugo site generation
- ✅ `internal/docs/` - Documentation discovery
- ✅ `internal/git/` - Git operations
- ✅ `internal/config/` - Configuration access
- ✅ `internal/state/` - State management
- ✅ `internal/foundation/errors` - Error handling

**Domain Layer** (`internal/hugo/`, `internal/docs/`, `internal/forge/`):
- ✅ `internal/config/` - Configuration reading
- ✅ `internal/docs/` - Document models
- ✅ `internal/foundation/errors` - Error handling
- ✅ `internal/hugo/pipeline/` - Transform pipeline (hugo package only)

**Pipeline** (`internal/hugo/pipeline/`):
- ✅ `internal/config/` - Configuration reading
- ✅ `internal/foundation/errors` - Error handling
- ✅ No imports from `internal/hugo/` (except via interfaces)

**Infrastructure Layer** (`internal/git/`, `internal/workspace/`, `internal/forge/`):
- ✅ `internal/config/` - Configuration reading
- ✅ `internal/foundation/errors` - Error handling

**Foundation Layer** (`internal/foundation/`):
- ✅ Standard library only
- ❌ No internal/ package imports

### ❌ Forbidden Imports

**Configuration** (`internal/config/`):
- ❌ Cannot import `internal/build/`
- ❌ Cannot import `internal/hugo/`
- ❌ Cannot import `internal/git/`
- **Reason**: Config is a leaf domain package

**Git** (`internal/git/`):
- ❌ Cannot import `internal/build/`
- ❌ Cannot import `internal/hugo/`
- ❌ Cannot import `internal/daemon/`
- **Reason**: Infrastructure packages shouldn't know about application services

**Foundation** (`internal/foundation/`):
- ❌ Cannot import any `internal/` packages
- **Reason**: Foundation is the base layer - no upward dependencies

**Pipeline** (`internal/hugo/pipeline/`):
- ❌ Cannot import parent `internal/hugo/` (except through interfaces)
- **Reason**: Avoid circular dependencies

## Package Purposes

### Command Layer

#### `cmd/docbuilder/commands/`
**Purpose**: CLI command implementations

**Key Files**:
- `build.go` - Build command (main entry point)
- `daemon.go` - Daemon/watch mode
- `preview.go` - Live preview server
- `discover.go` - Documentation discovery analysis
- `lint.go` - Documentation linting

**Imports**: build, daemon, config

---

### Service Layer

#### `internal/build/`
**Purpose**: Build pipeline orchestration

**Key Components**:
- `BuildService` interface - Contract for build execution
- `DefaultBuildService` - Standard pipeline implementation
- Stage execution and error handling
- Build result aggregation

**Imports**: hugo, docs, git, config, state, forge

#### `internal/daemon/`
**Purpose**: Watch mode and HTTP server

**Key Components**:
- `DaemonService` - Watch and rebuild on changes
- HTTP server for preview
- LiveReload integration
- Build queue management

**Imports**: build, config, state, git

---

### Domain Layer

#### `internal/config/`
**Purpose**: Configuration models and validation

**Key Types**:
- `Config` - Root configuration
- `Repository` - Repository definition
- `HugoConfig` - Hugo-specific settings
- `DaemonConfig` - Daemon-specific settings

**Imports**: foundation/errors only

#### `internal/hugo/`
**Purpose**: Hugo site generation

**Key Components**:
- `Generator` - Main site generator
- Stage implementations (prepare, config, copy, indexes, render)
- `BuildState` - Mutable state during pipeline
- `BuildReport` - Build metrics and outcomes

**Imports**: config, docs, pipeline, forge, git, workspace

#### `internal/hugo/pipeline/`
**Purpose**: Content transformation pipeline

**Key Types**:
- `Processor` - Pipeline coordinator
- `Document` - Intermediate document representation
- `FileTransform` - Transform function type
- `FileGenerator` - Generator function type

**12 Transform Functions**:
1. `parseFrontMatter`
2. `normalizeIndexFiles`
3. `buildBaseFrontMatter`
4. `extractIndexTitle`
5. `stripHeading`
6. `escapeShortcodesInCodeBlocks`
7. `rewriteRelativeLinks`
8. `rewriteImageLinks`
9. `generateFromKeywords`
10. `addRepositoryMetadata`
11. `addEditLink`
12. `serializeDocument`

**Imports**: config, foundation/errors

#### `internal/docs/`
**Purpose**: Documentation discovery and file management

**Key Types**:
- `DocFile` - Documentation file model
- `Discovery` - File discovery service

**Key Functions**:
- `DiscoverDocs()` - Find markdown files
- `GetHugoPath()` - Convert source path to Hugo path

**Imports**: config, foundation/errors

#### `internal/forge/`
**Purpose**: Forge platform integration (GitHub, GitLab, Forgejo)

**Key Interfaces**:
- `Forge` - Platform abstraction
- Platform-specific implementations

**Key Operations**:
- Repository metadata fetching
- Edit link generation
- API authentication

**Imports**: config, foundation/errors

#### `internal/state/`
**Purpose**: Build state persistence and management

**Key Types**:
- `GitState` - Repository cloning state
- `DocsState` - Documentation discovery state
- `PipelineState` - Execution metadata

**Imports**: config, foundation/errors

---

### Infrastructure Layer

#### `internal/git/`
**Purpose**: Git repository operations

**Key Components**:
- `Client` - Git operations client
- Authentication handling (SSH, token, basic)
- Clone, update, and fetch operations
- Change detection

**Imports**: config, workspace, foundation/errors

#### `internal/workspace/`
**Purpose**: Temporary workspace management

**Key Operations**:
- Create temporary directories
- Cleanup after builds
- Path resolution

**Imports**: foundation/errors

#### `internal/forge/` (infrastructure aspects)
**Purpose**: HTTP clients for forge APIs

**Imports**: config, foundation/errors

---

### Foundation Layer

#### `internal/foundation/errors`
**Purpose**: Unified error handling

**Key Types**:
- `ClassifiedError` - Error with category and context
- `ErrorCategory` - Type-safe error categories

**Categories**:
- Config, Validation, Auth, NotFound
- Network, Git, Forge
- Build, Hugo, FileSystem
- Runtime, Daemon, Internal

**Imports**: Standard library only

#### `internal/logfields/`
**Purpose**: Structured logging field constants

**Imports**: Standard library only

#### `internal/metrics/`
**Purpose**: Metrics collection (currently NoopRecorder)

**Imports**: Standard library only

## Dependency Patterns

### Service Composition
```
BuildService
    ↓ uses
HugoGenerator
    ↓ uses
Pipeline Processor
    ↓ uses
Transforms
```

### Configuration Flow
```
CLI Command
    ↓ loads
Config
    ↓ passed to
BuildService
    ↓ passed to
Generator/Pipeline
```

### Error Propagation
```
Infrastructure Layer (git, forge)
    ↓ wraps errors
Domain Layer (hugo, docs)
    ↓ wraps errors
Service Layer (build)
    ↓ handles and reports
Command Layer (CLI)
```

## Circular Dependency Prevention

**Problem**: Package A imports B, B imports A (circular)

**Prevention Strategies**:

1. **Dependency Inversion**: Use interfaces
   ```go
   // Wrong: hugo imports build
   package hugo
   import "internal/build"
   
   // Right: build imports hugo via interface
   package build
   type HugoGenerator interface { ... }
   ```

2. **Shared Package**: Extract common types to lower layer
   ```go
   // Both hugo and build import config
   package config
   type Repository struct { ... }
   ```

3. **Event-Based**: Use event store for loose coupling
   ```go
   // Instead of: daemon → hugo → daemon
   // Use: daemon → event store ← hugo
   ```

## References

- [High-Level System Architecture](high-level-architecture.md)
- [Comprehensive Architecture](../comprehensive-architecture.md)
- [Go Style Guide](../../../docs/STYLE_GUIDE.md)
