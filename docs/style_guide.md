---
title: "Go Style Guide"
date: 2025-12-15
categories:
  - development
tags:
  - style-guide
  - coding-standards
  - go
---

# DocBuilder Go Style Guide

This document defines naming conventions and coding style for the DocBuilder project to ensure consistency across the codebase.

## Table of Contents

- [General Principles](#general-principles)
- [Variable Naming](#variable-naming)
- [Function Naming](#function-naming)
- [Type Naming](#type-naming)
- [Package Naming](#package-naming)
- [Error Handling](#error-handling)
- [Comments and Documentation](#comments-and-documentation)

## General Principles

1. **Clarity over cleverness**: Code should be self-documenting
2. **Consistency**: Follow existing patterns in the codebase
3. **Brevity with context**: Use short names in small scopes, descriptive names in larger scopes
4. **Go idioms**: Follow standard Go conventions from `golang.org/wiki/CodeReviewComments`

## Variable Naming

### Abbreviations

Use consistent abbreviations throughout the codebase:

| Full Term | Abbreviation | Usage |
|-----------|-------------|--------|
| application | `app` | Function parameters, local variables |
| argument | `arg` | Function parameters, local variables |
| authentication | `auth` | Function parameters, local variables |
| configuration | `cfg` | Struct fields, function parameters |
| context | `ctx` | Standard Go convention |
| database | `db` | Struct fields, function parameters |
| destination | `dst` | Function parameters, local variables |
| directory | `dir` | Function parameters, local variables |
| document | `doc` | Function parameters, local variables |
| environment | `env` | Struct fields, function parameters |
| error | `err` | Standard Go convention |
| identifier | `id` | Function parameters, local variables |
| information | `info` | Function parameters, local variables |
| maximum | `max` | Local variables |
| message | `msg` | Function parameters, local variables |
| minimum | `min` | Local variables |
| parameter | `param` | Function parameters, local variables |
| recorder | `rec` | Function parameters, local variables |
| reference | `ref` | Local variables only |
| repository | `repo` | Function parameters, local variables |
| request | `req` | Function parameters, local variables |
| response | `resp` | Function parameters, local variables |
| source | `src` | Function parameters, local variables |
| specification | `spec` | Function parameters, local variables |
| statistics | `stats` | Function parameters, local variables |
| temporary | `tmp` | Local variables only |

**Examples:**

```go
// ✅ Good
func (c *Client) CloneRepo(repo appcfg.Repository) (string, error)
func (c *Client) getAuth(authCfg *appcfg.AuthConfig) (transport.AuthMethod, error)
buildCfg := &appcfg.BuildConfig{}

// ❌ Bad - inconsistent abbreviation
func (c *Client) CloneRepository(repo appcfg.Repository) (string, error)
func (c *Client) getAuthentication(authConfig *appcfg.AuthConfig)
```

**Exception:** When using external library types, use the library's naming:
```go
// ✅ Correct - go-git uses "repository" in type name
repository, err := git.PlainOpen(repoPath)
var gitRepo *git.Repository
```

### Scope-Based Naming

- **Single-letter variables**: Only in very short scopes (< 10 lines)
  ```go
  for i, v := range items {
      // i and v are acceptable here
  }
  ```

- **Abbreviated names**: For function parameters and local variables
  ```go
  func processRepo(repo appcfg.Repository, cfg *BuildConfig) error
  ```

- **Descriptive names**: For package-level variables and struct fields
  ```go
  type Client struct {
      workspaceDir    string
      buildCfg        *appcfg.BuildConfig
      remoteHeadCache *RemoteHeadCache
  }
  ```

### Receiver Names

- Use **1-2 letter abbreviations** based on the type name
- Be **consistent** throughout a type's methods

```go
// ✅ Good
func (c *Client) CloneRepo(...)
func (c *Client) UpdateRepo(...)

// ✅ Good for multi-word types
func (rhc *RemoteHeadCache) Get(...)

// ❌ Bad - inconsistent
func (client *Client) CloneRepo(...)
func (c *Client) UpdateRepo(...)
```

### Configuration Variables

Always suffix with `Cfg` or `Config` depending on context:

```go
// ✅ Good - struct fields use abbreviated suffix
type Client struct {
    buildCfg *appcfg.BuildConfig
}

// ✅ Good - function parameters use abbreviated suffix
func NewGenerator(cfg *config.Config, outputDir string) *Generator

// ✅ Good - package-level uses full name for clarity
var DefaultConfig = &config.Config{...}

// ❌ Bad - no suffix
type Client struct {
    build *appcfg.BuildConfig
}
```

### Boolean Variables

Prefix with `is`, `has`, `should`, `can`, or `enable`:

```go
// ✅ Good
isValid := true
hasAuth := repo.Auth != nil
shouldRetry := attempt < maxRetries
canFastForward := true
enableCache := cfg.EnableCache

// ❌ Bad
valid := true
auth := repo.Auth != nil
retry := attempt < maxRetries
```

## Function Naming

### Private vs Public Functions

```go
// ✅ Public - descriptive, full words
func (c *Client) CloneRepo(repo appcfg.Repository) error
func ComputeRepoHash(repoPath string, commit string) (string, error)

// ✅ Private - can use abbreviations
func (c *Client) getAuth(authCfg *appcfg.AuthConfig) error
func classifyError(err error) error
func (c *Client) fetchOrigin(repo *git.Repository) error
```

### Getter/Setter Patterns

Go doesn't use `Get`/`Set` prefixes for simple accessors:

```go
// ✅ Good
func (c *Client) WorkspaceDir() string { return c.workspaceDir }
func (c *Client) SetWorkspaceDir(dir string) { c.workspaceDir = dir }

// ❌ Bad
func (c *Client) GetWorkspaceDir() string
func (c *Client) SetWorkspaceDir(dir string)
```

**Exception:** Use `Get` when fetching requires computation or I/O:

```go
// ✅ Correct - involves network I/O
func (c *Client) GetRemoteHead(repo appcfg.Repository) (string, error)

// ✅ Correct - involves cache lookup
func (c *RemoteHeadCache) Get(url, branch string) *RemoteHeadEntry
```

### Action Verbs

Use clear action verbs that describe what the function does:

| Action | Usage | Example |
|--------|-------|---------|
| `Clone` | Create a new copy | `CloneRepo` |
| `Update` | Modify existing | `UpdateRepo` |
| `Fetch` | Retrieve from remote | `FetchOrigin` |
| `Create` | Construct new instance | `CreateAuth` |
| `Build` | Construct complex object | `BuildConfig` |
| `Generate` | Produce output | `GenerateHugoSite` |
| `Process` | Transform data | `ProcessDocs` |
| `Compute` | Calculate value | `ComputeRepoHash` |
| `Classify` | Categorize | `ClassifyError` |
| `Resolve` | Determine value | `ResolveTargetBranch` |
| `Ensure` | Guarantee state | `EnsureWorkspace` |
| `Check` | Validate or test | `CheckRemoteChanged` |
| `Validate` | Verify correctness | `ValidateConfig` |

### Predicate Functions

Functions returning `bool` should read like questions:

```go
// ✅ Good
func (c *Client) IsAncestor(a, b plumbing.Hash) (bool, error)
func HasAuth(repo appcfg.Repository) bool
func ShouldRetry(err error) bool

// ❌ Bad
func (c *Client) Ancestor(a, b plumbing.Hash) (bool, error)
func Auth(repo appcfg.Repository) bool
```

### Error Classification Functions

Use consistent patterns for error classification:

```go
// ✅ Good - consistent "classify" pattern
func classifyFetchError(url string, err error) error
func classifyCloneError(url string, err error) error

// ✅ Good - consistent "is" pattern for boolean checks
func isPermanentGitError(err error) bool
func isTransientError(err error) bool

// ❌ Bad - mixing patterns
func classifyFetchError(url string, err error) error
func isPermanentGitError(err error) bool
func classifyTransientType(err error) string  // Different return type
```

### Constructor Functions

```go
// ✅ Standard pattern
func NewClient(workspaceDir string) *Client
func NewRemoteHeadCache(cacheDir string) (*RemoteHeadCache, error)

// ✅ Builder pattern - use "With" prefix
func (c *Client) WithBuildConfig(cfg *appcfg.BuildConfig) *Client
func (c *Client) WithRemoteHeadCache(cache *RemoteHeadCache) *Client
```

### Verb Ordering in Compound Function Names

When function names contain multiple actions, order verbs to reflect execution flow and emphasize the primary operation:

**1. Validation/Check Before Action**

Place validation verbs first when they guard the main operation:

```go
// ✅ Good - validation happens first
func ValidateAndCreate(cfg *Config) error
func CheckAndUpdate(repo Repository) error
func EnsureAndClone(dir string) error

// ❌ Bad - suggests action happens before validation
func CreateAndValidate(cfg *Config) error
func UpdateAndCheck(repo Repository) error
```

**2. Setup Before Execution**

Preparation verbs come before execution verbs:

```go
// ✅ Good - setup before main operation
func PrepareAndExecute(cmd Command) error
func InitializeAndRun(service Service) error
func LoadAndProcess(file string) error

// ❌ Bad - execution before preparation
func ExecuteAndPrepare(cmd Command) error
func RunAndInitialize(service Service) error
```

**3. Primary Action First**

When combining a main operation with a side effect, lead with the primary action:

```go
// ✅ Good - primary action leads
func CreateWithNotification(resource Resource) error
func UpdateWithLogging(entity Entity) error
func DeleteWithCleanup(path string) error

// ✅ Also acceptable - "And" pattern for equal importance
func CreateAndNotify(resource Resource) error
func UpdateAndLog(entity Entity) error
```

**4. CRUD Operation Ordering**

When implementing multiple CRUD operations, follow this conventional order:

```go
// ✅ Good - conventional CRUD order
func Create(...)
func Get(...) or Read(...)
func Update(...)
func Delete(...)

// For bulk operations
func CreateBatch(...)
func GetAll(...)
func UpdateBatch(...)
func DeleteBatch(...)
```

**5. Method Chaining Order**

Builder pattern methods should follow logical construction sequence:

```go
// ✅ Good - logical build sequence
client := NewClient().
    WithAuth(auth).
    WithConfig(cfg).
    WithRetry(3).
    Build()

// Methods ordered: identity → configuration → options → finalization
```

**6. Cleanup and Finalization**

Cleanup operations should be explicit in compound names:

```go
// ✅ Good - clear cleanup semantics
func CloseAndCleanup() error
func StopAndRemove() error
func CompleteAndArchive() error

// ❌ Bad - ambiguous cleanup timing
func CleanupAndClose() error  // Does cleanup happen before closing?
```

## Type Naming

### Struct Types

Use clear, descriptive names without abbreviations:

```go
// ✅ Good
type RemoteHeadCache struct { ... }
type RemoteHeadEntry struct { ... }
type BuildConfig struct { ... }

// ❌ Bad
type RHCache struct { ... }
type RHEntry struct { ... }
type BldCfg struct { ... }
```

### Interface Types

Prefer single-method interfaces with `-er` suffix:

```go
// ✅ Good
type Cloner interface {
    Clone(repo Repository) error
}

type Generator interface {
    Generate() error
}

// ✅ Good - multi-method interface
type GitClient interface {
    CloneRepo(repo Repository) error
    UpdateRepo(repo Repository) error
}
```

### Error Types

Suffix all error types with `Error`:

```go
// ✅ Good
type AuthError struct { ... }
type NotFoundError struct { ... }
type NetworkTimeoutError struct { ... }

// ❌ Bad
type AuthFailure struct { ... }
type NotFound struct { ... }
```

### Struct Field Names

```go
type Client struct {
    // ✅ Private fields - use abbreviated suffixes
    workspaceDir    string
    buildCfg        *appcfg.BuildConfig
    remoteHeadCache *RemoteHeadCache
    
    // ✅ Exported fields - full words for clarity
    Name        string
    Description string
    Repository  appcfg.Repository
}
```

## Package Naming

### Package Names

- Use **short, lowercase, single-word** names
- No underscores or camelCase
- Name should describe package purpose

```go
// ✅ Good
package git
package config
package hugo
package docs

// ❌ Bad
package gitOperations
package config_manager
package hugoSiteGenerator
```

### Package Import Aliases

Use consistent aliases when avoiding conflicts:

```go
// ✅ Good - descriptive prefix
import (
    appcfg "git.home.luguber.info/inful/docbuilder/internal/config"
    ggitcfg "github.com/go-git/go-git/v5/config"
)

// ❌ Bad - unclear abbreviation
import (
    cfg1 "git.home.luguber.info/inful/docbuilder/internal/config"
    cfg2 "github.com/go-git/go-git/v5/config"
)
```

## Error Handling

### Unified Error System

DocBuilder uses `internal/foundation/errors` package for all error handling. This provides:
- Type-safe error categories (`ErrorCategory`)
- Structured error context
- Retry semantics
- HTTP and CLI adapters

### Error Variables

Prefix package-level sentinel errors with `Err`:

```go
// ✅ Good
var (
    ErrNotFound      = errors.New(errors.CategoryNotFound, "repository not found").Build()
    ErrUnauthorized  = errors.New(errors.CategoryAuth, "authentication failed").Build()
    ErrInvalidConfig = errors.ValidationError("invalid configuration").Build()
)

// ❌ Bad
var (
    NotFoundError = errors.New("repository not found") // No category
    Unauthorized  = errors.New("authentication failed") // No category
)
```

### Error Construction

Use the fluent builder API:

```go
// ✅ Good - validation error with context
return errors.ValidationError("invalid forge type").
    WithContext("input", forgeType).
    WithContext("valid_values", []string{"github", "gitlab"}).
    Build()

// ✅ Good - wrapping errors
return errors.WrapError(err, errors.CategoryGit, "failed to clone repository").
    WithContext("url", repo.URL).
    WithContext("branch", repo.Branch).
    Build()

// ❌ Bad - raw errors
return fmt.Errorf("failed to clone: %w", err) // No category or context
```

### Error Messages

- Start with **lowercase**
- Be **specific and actionable**
- Add context using `WithContext(key, value)` instead of string formatting

### Error Categories

Available categories in `internal/foundation/errors/categories.go`:

- **User-facing**: `CategoryConfig`, `CategoryValidation`, `CategoryAuth`, `CategoryNotFound`, `CategoryAlreadyExists`
- **External**: `CategoryNetwork`, `CategoryGit`, `CategoryForge`
- **Build**: `CategoryBuild`, `CategoryHugo`, `CategoryFileSystem`
- **Runtime**: `CategoryRuntime`, `CategoryDaemon`, `CategoryInternal`

### Error Detection

```go
// ✅ Good - extract classified error
if classified, ok := errors.AsClassified(err); ok {
    if classified.Category() == errors.CategoryValidation {
        // Handle validation error
    }
}

// ✅ Good - check category using helper
if errors.HasCategory(err, errors.CategoryAuth) {
    // Handle auth error
}

// ❌ Bad - old pattern
var classified *foundation.ClassifiedError
if foundation.AsClassified(err, &classified) {
    // Old API - no longer exists
}
```

### Typed Errors

For package-specific errors, use the builder:

```go
// ✅ Good - use builder for custom errors
func NewAuthError(op, url string, cause error) error {
    return errors.WrapError(cause, errors.CategoryAuth, "authentication failed").
        WithContext("operation", op).
        WithContext("url", url).
        Build()
}

// Usage
return NewAuthError("clone", repo.URL, err)
```

## Comments and Documentation

### Package Documentation

Every package should have a package comment:

```go
// Package git provides a client for performing Git operations such as 
// clone, update, and authentication handling for DocBuilder's 
// documentation pipeline.
package git
```

### Function Documentation

Document all exported functions:

```go
// CloneRepo clones a repository to the workspace directory.
// If retry is enabled, it wraps the operation with retry logic.
func (c *Client) CloneRepo(repo appcfg.Repository) (string, error)
```

### Comment Style

- Use **complete sentences** with proper punctuation
- Start with the **name of the thing** being documented
- Explain **why**, not just **what** for complex logic

```go
// ✅ Good
// ComputeRepoHash computes a deterministic hash for a repository tree.
// The hash is based on the commit SHA and the tree structure of configured paths,
// enabling content-addressable caching where same commit + same paths = same hash.

// ❌ Bad
// computes hash
// This function hashes repos
```

### TODO Comments

Include context and optionally an issue number:

```go
// TODO(username): Add authentication support for remote.List
// TODO: Implement rate limiting (see issue #123)
```

## Code Organization

### File Naming

- Use **lowercase with underscores** for multi-word file names
- Match primary type or functionality

```go
// ✅ Good
remote_cache.go      // Contains RemoteHeadCache
typed_errors.go      // Contains error types
client.go            // Contains Client type

// ❌ Bad
RemoteCache.go       // Wrong case
remote-cache.go      // Use underscore, not hyphen
remotecache.go       // Hard to read
```

### Function Grouping

Group related functions together in files:

```go
// client.go - Core client operations
func NewClient()
func (c *Client) CloneRepo()
func (c *Client) UpdateRepo()

// retry.go - Retry logic
func (c *Client) withRetry()
func isPermanentGitError()
func classifyTransientType()

// remote_cache.go - Remote caching
type RemoteHeadCache
func NewRemoteHeadCache()
func (c *RemoteHeadCache) Get()
func (c *RemoteHeadCache) Set()
```

## References

This style guide is based on:
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://golang.org/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
- DocBuilder project conventions

## Enforcement

- Run `golangci-lint` before committing
- Review PRs for style consistency
- Update this guide as patterns emerge
