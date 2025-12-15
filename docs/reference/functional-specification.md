# DocBuilder Functional Specification

**Version:** 2.0  
**Status:** Production (December 2025)  
**Last Updated:** December 14, 2025
**Document Purpose:** Complete specification enabling reimplementation of DocBuilder from scratch

**Architecture Note:** This specification describes the current transform-based architecture using dependency-ordered content processing pipeline.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Requirements](#2-system-requirements)
3. [Configuration Schema](#3-configuration-schema)
4. [Core Data Models](#4-core-data-models)
5. [Command Line Interface](#5-command-line-interface)
6. [Build Pipeline Specification](#6-build-pipeline-specification)
7. [Git Operations](#7-git-operations)
8. [Documentation Discovery](#8-documentation-discovery)
9. [Hugo Site Generation](#9-hugo-site-generation)
10. [Theme System](#10-theme-system)
11. [Forge Integration](#11-forge-integration)
12. [Content Processing](#12-content-processing)
13. [Change Detection](#13-change-detection)
14. [Build Report](#14-build-report)
15. [Daemon Mode](#15-daemon-mode)
16. [Preview Mode](#16-preview-mode)
17. [Error Handling](#17-error-handling)
18. [Observability](#18-observability)
19. [File System Outputs](#19-file-system-outputs)
20. [Algorithms](#20-algorithms)
21. [Test Requirements](#21-test-requirements)
22. [Package Architecture](#22-package-architecture)

---

## 1. Executive Summary

### 1.1 Purpose

DocBuilder is a documentation aggregation tool that:
- Clones documentation from multiple Git repositories
- Discovers markdown files in configured paths
- Generates a unified Hugo static site
- Optionally renders the site using the Hugo binary

### 1.2 Key Capabilities

| Capability | Description |
|------------|-------------|
| Multi-Repository Aggregation | Combine docs from unlimited Git repositories |
| Authentication | SSH keys, personal access tokens, basic auth |
| Forge Integration | GitHub, GitLab, Forgejo/Gitea support |
| Theme Support | Hextra, Docsy, and Relearn themes via Hugo Modules |
| Incremental Builds | Skip unchanged repositories |
| Change Detection | SHA-256 fingerprinting of documentation sets |
| Forge Namespacing | Prevent URL collisions across forges |
| Edit Links | Auto-generate "edit this page" URLs |
| Live Preview | File watcher with auto-rebuild |

### 1.3 Technology Stack

- **Language:** Go 1.21+
- **CLI Framework:** Kong
- **Configuration:** YAML with environment variable expansion
- **Git:** go-git library
- **Static Site Generator:** Hugo (external binary)
- **Output:** Standard Hugo site structure

---

## 2. System Requirements

### 2.1 Build Requirements

| Component | Version | Purpose |
|-----------|---------|---------|
| Go | 1.21+ | Compilation |
| Hugo | 0.120+ | Site rendering (optional) |
| Git | 2.0+ | Repository operations |

### 2.2 Runtime Requirements

| Component | Version | Purpose |
|-----------|---------|---------|
| Hugo | 0.120+ | Site rendering (when render_mode != never) |
| Network | - | Git clone operations |
| Filesystem | - | Write access to output directory |

### 2.3 Dependencies

```go
require (
    github.com/alecthomas/kong       // CLI parsing
    github.com/go-git/go-git/v5      // Git operations
    gopkg.in/yaml.v3                 // Configuration
    github.com/fsnotify/fsnotify     // File watching (preview)
)
```

---

## 3. Configuration Schema

### 3.1 Configuration File Format

**File:** `config.yaml` (default)  
**Format:** YAML 1.2  
**Version:** 2.0

### 3.2 Complete Schema

```yaml
version: "2.0"  # Required, must be "2.0"

# Direct repository list (for non-daemon builds)
repositories:
  - url: string          # Required: Git clone URL
    name: string         # Required: Unique identifier
    branch: string       # Optional: Branch to checkout
    paths: [string]      # Optional: Doc paths (default: ["docs"])
    auth:                # Optional: Authentication
      type: enum         # "token" | "ssh" | "basic"
      token: string      # For type: token
      username: string   # For type: basic
      password: string   # For type: basic
      key_path: string   # For type: ssh (default: ~/.ssh/id_rsa)
    tags:                # Optional: Metadata map
      forge_type: string # Used for namespacing

# Forge configuration (for auto-discovery)
forges:
  - name: string           # Friendly name
    type: enum             # "github" | "gitlab" | "forgejo"
    api_url: string        # API base URL
    base_url: string       # Web base URL
    organizations: [string]  # GitHub orgs to scan
    groups: [string]       # GitLab/Forgejo groups
    auto_discover: bool    # Enable full discovery
    auth:
      type: enum
      token: string
    webhook:
      secret: string
      path: string
      events: [string]

# Build configuration
build:
  clone_concurrency: int      # Default: 4
  clone_strategy: enum        # "fresh" | "update" | "auto"
  shallow_depth: int          # 0 = full clone
  prune_non_doc_paths: bool   # Default: false
  prune_allow: [string]       # Glob patterns to keep
  prune_deny: [string]        # Glob patterns to remove
  hard_reset_on_diverge: bool # Default: false
  clean_untracked: bool       # Default: false
  max_retries: int            # Default: 2
  retry_backoff: enum         # "fixed" | "linear" | "exponential"
  retry_initial_delay: duration  # Default: 1s
  retry_max_delay: duration   # Default: 30s
  workspace_dir: string       # Override workspace path
  namespace_forges: enum      # "auto" | "always" | "never"
  render_mode: enum           # "auto" | "always" | "never"
  incremental: bool           # Default: false
  detect_deletions: bool      # Default: true

# Hugo site configuration
hugo:
  title: string          # Site title
  description: string    # Site description
  base_url: string       # Hugo baseURL
  theme: enum            # "hextra" | "docsy" | "relearn"
  language_code: string  # Default: "en-us"
  params: map            # Pass-through to Hugo params

# Output configuration
output:
  directory: string      # Default: "./site"
  clean: bool            # Default: true

# Daemon configuration (optional)
daemon:
  http:
    docs_port: int       # Default: 8080
    webhook_port: int    # Default: 8081
    admin_port: int      # Default: 8082
  sync:
    schedule: string     # Cron expression
    concurrent_builds: int
    queue_size: int
  storage:
    state_file: string
    repo_cache_dir: string
    output_dir: string

# Filtering (optional)
filtering:
  required_paths: [string]
  ignore_files: [string]
  include_patterns: [string]
  exclude_patterns: [string]

# Monitoring (optional)
monitoring:
  metrics:
    enabled: bool
    path: string
  health:
    path: string
  logging:
    level: enum          # "debug" | "info" | "warn" | "error"
    format: enum         # "text" | "json"
```

### 3.3 Environment Variable Expansion

Configuration values support `${VAR}` syntax:

```yaml
auth:
  token: "${GITHUB_TOKEN}"
```

**Resolution Order:**
1. Process environment
2. `.env.local` file (if exists)
3. `.env` file (if exists)

### 3.4 Configuration Loading Algorithm

```
1. Check if config file exists, error if not
2. Load .env file (ignore errors)
3. Load .env.local file (ignore errors)
4. Read config file bytes
5. Expand ${VAR} references using os.ExpandEnv()
6. Parse YAML into Config struct
7. Validate version == "2.0"
8. Apply defaults to empty fields
9. Normalize paths and URLs
10. Validate all fields
11. Return Config or error
```

---

## 4. Core Data Models

### 4.1 DocFile

Represents a discovered documentation file:

```go
type DocFile struct {
    Path         string            // Absolute filesystem path
    RelativePath string            // Path relative to docs directory
    DocsBase     string            // Configured docs path (e.g., "docs")
    Repository   string            // Repository name
    Forge        string            // Forge namespace (or empty)
    Section      string            // Documentation section
    Name         string            // Filename without extension
    Extension    string            // File extension (.md)
    Content      []byte            // File content
    Metadata     map[string]string // Additional metadata
}
```

### 4.2 Repository

Configuration for a single repository:

```go
type Repository struct {
    URL    string            // Git clone URL
    Name   string            // Unique identifier
    Branch string            // Branch to checkout
    Auth   *AuthConfig       // Authentication settings
    Paths  []string          // Documentation paths
    Tags   map[string]string // Metadata (forge_type, etc.)
}
```

### 4.3 BuildReport

Output of a build operation:

```go
type BuildReport struct {
    SchemaVersion       int               // Always 1
    Repositories        int               // Repos with docs
    Files              int               // Total doc files
    Start              time.Time         // Build start
    End                time.Time         // Build end
    StageDurations     map[string]int64  // Stage → nanoseconds
    ClonedRepositories int               // Successful clones
    FailedRepositories int               // Failed clones
    SkippedRepositories int              // Filtered repos
    RenderedPages      int               // Pages written
    StaticRendered     bool              // Hugo succeeded
    Retries            int               // Total retries
    RetriesExhausted   bool              // Any exhausted budget
    Outcome            string            // success/warning/failed/canceled
    DocFilesHash       string            // SHA-256 fingerprint
    Issues             []Issue           // Structured issues
    SkipReason         string            // Early exit reason
}
```

### 4.4 Issue

Structured problem representation:

```go
type Issue struct {
    Code      string  // Issue code (CLONE_FAILURE, etc.)
    Stage     string  // Pipeline stage name
    Severity  string  // error/warning/info
    Message   string  // Human-readable message
    Transient bool    // Retryable issue
}
```

---

## 5. Command Line Interface

### 5.1 Binary Name

```
docbuilder
```

### 5.2 Global Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --config | -c | string | config.yaml | Configuration file path |
| --verbose | -v | bool | false | Enable debug logging |
| --version | | bool | false | Show version and exit |

### 5.3 Commands

#### 5.3.1 build

Build documentation site from configured repositories.

```bash
docbuilder build [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --output | -o | string | ./site | Output directory |
| --incremental | -i | bool | false | Incremental updates |
| --render-mode | | string | | Override render mode |

**Exit Codes:**
- 0: Success
- Non-zero: Error

#### 5.3.2 init

Create example configuration file.

```bash
docbuilder init [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| --force | bool | false | Overwrite existing file |
| --output | string | . | Output directory |

#### 5.3.3 discover

Discover documentation files without building.

```bash
docbuilder discover [flags]
```

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| --repository | -r | string | Specific repo to discover |

#### 5.3.4 daemon

Run as long-running service.

```bash
docbuilder daemon [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --data-dir | -d | string | ./daemon-data | State directory |

#### 5.3.5 preview

Preview local documentation with live reload.

```bash
docbuilder preview [flags]
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| --output | -o | string | ./site | Output directory |
| --port | -p | int | 1313 | Server port |
| --path | | string | . | Documentation source |

---

## 6. Build Pipeline Specification

### 6.1 Pipeline Stages

The build pipeline executes these sequential stages:

| Stage | Name | Description |
|-------|------|-------------|
| 1 | PrepareOutput | Initialize output directories |
| 2 | CloneRepos | Clone/update Git repositories |
| 3 | DiscoverDocs | Find markdown files and assets |
| 4 | GenerateConfig | Create hugo.yaml with theme settings |
| 5 | Layouts | Copy theme templates |
| 6 | CopyContent | **Execute transform pipeline on each file** |
| 7 | Indexes | Generate index pages |
| 8 | RunHugo | Execute Hugo (optional) |

**Note:** Stage 6 is where the transform system executes. Each file goes through:
- Parse stage (extract front matter)
- Build stage (generate defaults)
- Enrich stage (add metadata)
- Merge stage (apply patches)
- Transform stage (content modifications)
- Finalize stage (post-processing)
- Serialize stage (output YAML + markdown)

### 6.2 Stage 1: PrepareOutput

**Input:** Output directory path, clean flag

**Algorithm:**
```
1. If clean flag is true:
   a. Remove output directory recursively
2. Create output directory (mkdir -p)
3. Create staging directory: {output}_stage/
4. Return success
```

**Output:** Empty output and staging directories

### 6.3 Stage 2: CloneRepos

**Input:** Repository configurations, workspace directory

**Algorithm:**
```
1. Create workspace directory if not exists
2. For each repository (parallel, up to clone_concurrency):
   a. Determine clone strategy (fresh/update/auto)
   b. If fresh or repo doesn't exist:
      i. Clone repository to workspace/{name}
      ii. Checkout specified branch
   c. If update and repo exists:
      i. Fetch latest changes
      ii. Reset to origin/{branch}
   d. Record HEAD commit reference
   e. Handle authentication per auth type
   f. Retry on transient failures (network, etc.)
3. Return map of repo name → workspace path
```

**Output:** `map[string]string` of repository paths

### 6.4 Stage 3: DiscoverDocs

**Input:** Repository paths, configurations

**Algorithm:**
```
1. For each repository:
   a. Check for .docignore file (skip if present)
   b. Determine forge namespace (based on namespace_forges setting)
   c. For each configured docs path:
      i. Walk directory recursively
      ii. For each file:
         - Skip if not markdown (.md, .markdown)
         - Skip if hidden (starts with .)
         - Skip if in ignore list (README.md, etc.)
         - Create DocFile with computed paths
         - Add to results
2. Compute doc_files_hash (SHA-256 of sorted Hugo paths)
3. Return DocFile list
```

**Output:** `[]DocFile`, doc_files_hash

### 6.5 Stage 4: GenerateConfig

**Input:** Config, theme settings

**Algorithm:**
```
1. Get theme from registry
2. Build base config:
   - title, baseURL, languageCode
   - markup settings (goldmark)
   - build_date: current timestamp
3. Call theme.ApplyParams() for defaults
4. Deep merge user params
5. Add module import (if UsesModules)
6. Generate menu (if AutoMainMenu)
7. Call theme.CustomizeRoot()
8. Serialize to YAML
9. Write hugo.yaml to staging directory
```

**Output:** `hugo.yaml` file

### 6.6 Stage 5: Layouts

**Input:** Theme settings, staging directory

**Algorithm:**
```
1. Create layouts/ directory in staging
2. For index templates:
   a. Check user override
   b. Check theme default
   c. Use fallback if needed
3. Copy layouts to staging
```

**Output:** Populated layouts/ directory

### 6.7 Stage 6: CopyContent

**Input:** DocFiles, staging directory, transform configuration

**Algorithm:**
```
For each DocFile:
1. Load file content into Page struct
2. Create PageShim facade for transform pipeline
3. Build transform list from registry:
   a. Fetch all registered transforms
   b. Sort by stage and dependencies (topological order)
   c. Apply enable/disable filters from config
4. Execute transforms in order:
   a. Parse: Extract front matter → OriginalFrontMatter
   b. Build: Generate defaults via BuildFrontMatter()
   c. Enrich: Add repository/forge/section metadata
   d. Merge: Apply front matter patches
   e. Transform: Rewrite links, process content
   f. Finalize: Strip headings, escape shortcodes
   g. Serialize: Output YAML + markdown
5. Write transformed content to:
   content/{forge?}/{repo}/{section}/{file}.md
```

**Output:** Populated content/ directory with transformed markdown files

### 6.8 Stage 7: Indexes

**Input:** DocFiles, staging directory

**Algorithm:**
```
1. Generate main index (content/_index.md):
   - Front matter with cascade
   - Welcome content
2. For each unique repository:
   - Generate content/{repo}/_index.md
3. For each unique section:
   - Generate content/{repo}/{section}/_index.md
4. Apply theme-specific settings
```

**Output:** Index pages in content/

### 6.9 Stage 8: RunHugo

**Input:** Staging directory, render_mode

**Algorithm:**
```
1. Determine if Hugo should run:
   - always: yes
   - never: no
   - auto: check if hugo binary available
2. If running:
   a. Execute: hugo --source {staging}
   b. Capture stdout/stderr
   c. Check exit code
   d. Copy public/ to final output
3. Promote staging to output (atomic rename)
4. Record result in report
```

**Output:** Rendered site in public/

---

## 7. Git Operations

### 7.1 Authentication Methods

#### 7.1.1 Token Authentication

```go
type TokenAuth struct {
    Token string
}
```

**HTTP URL Transformation:**
```
https://github.com/owner/repo.git
→ https://token:{token}@github.com/owner/repo.git
```

#### 7.1.2 SSH Authentication

```go
type SSHAuth struct {
    KeyPath    string  // Default: ~/.ssh/id_rsa
    Passphrase string  // Optional
}
```

**Usage:** Uses go-git's SSH transport with key file.

#### 7.1.3 Basic Authentication

```go
type BasicAuth struct {
    Username string
    Password string
}
```

**HTTP URL Transformation:**
```
https://github.com/owner/repo.git
→ https://{username}:{password}@github.com/owner/repo.git
```

### 7.2 Clone Strategies

| Strategy | Behavior |
|----------|----------|
| fresh | Always delete and re-clone |
| update | Fetch + reset if exists, clone if not |
| auto | Use update if incremental, fresh otherwise |

### 7.3 HEAD Reference Reading

```go
func ReadRepoHead(repoPath string) (string, error)
```

**Algorithm:**
```
1. Read .git/HEAD file
2. If starts with "ref: ":
   a. Extract ref path (e.g., refs/heads/main)
   b. Read .git/{ref path} file
   c. Return commit hash
3. Else:
   a. Return content as detached HEAD hash
```

---

## 8. Documentation Discovery

### 8.1 Discovery Algorithm

```
For repository with paths ["docs"]:

1. Build full path: {workspace}/{repo}/docs
2. Walk directory tree:
   For each entry:
     - Skip directories
     - Check: isMarkdownFile(name)
       - .md extension
       - .markdown extension
     - Check: !isIgnoredFile(name)
       - README.md
       - CONTRIBUTING.md
       - CHANGELOG.md
       - LICENSE.md
       - CODE_OF_CONDUCT.md
     - Check: !isHidden(name)
       - Starts with "."
     - If all pass:
       - Compute relative path
       - Compute Hugo path
       - Create DocFile
3. Return []DocFile
```

### 8.2 Path Computation

**Given:**
- Workspace: `/tmp/docbuilder-123`
- Repository: `my-repo`
- Doc path: `docs`
- File: `/tmp/docbuilder-123/my-repo/docs/guide/intro.md`
- Forge: `github` (if namespacing)

**Computed:**
```
Path:         /tmp/docbuilder-123/my-repo/docs/guide/intro.md
RelativePath: guide/intro.md
DocsBase:     docs
Repository:   my-repo
Forge:        github
Section:      guide
Name:         intro
Extension:    .md

HugoPath:     content/github/my-repo/guide/intro.md
              (or content/my-repo/guide/intro.md without namespacing)
```

### 8.3 .docignore File

If `.docignore` file exists in repository root, skip entire repository.

```
{repo}/.docignore exists → skip repository
```

### 8.4 Forge Namespacing Logic

```go
func shouldNamespaceForges(mode string, repos []Repository) bool {
    switch mode {
    case "always":
        return true
    case "never":
        return false
    case "auto":
        // Count unique forge_type tags
        forges := map[string]bool{}
        for _, repo := range repos {
            if ft := repo.Tags["forge_type"]; ft != "" {
                forges[ft] = true
            }
        }
        return len(forges) > 1
    }
    return false
}
```

---

## 9. Hugo Site Generation

### 9.1 Output Directory Structure

```
{output}/
├── hugo.yaml              # Generated configuration
├── go.mod                 # Module definition (for Hugo Modules)
├── content/
│   ├── _index.md          # Main index
│   ├── {repo-1}/
│   │   ├── _index.md      # Repository index
│   │   ├── {section}/
│   │   │   ├── _index.md  # Section index
│   │   │   └── page.md    # Documentation page
│   │   └── page.md
│   └── {repo-2}/
│       └── ...
├── layouts/
│   └── ...                # Optional custom layouts
├── static/
│   └── ...                # Static assets
├── assets/
│   └── ...                # Theme assets
├── public/                # Rendered site (after Hugo run)
│   └── ...
├── build-report.json      # Machine-readable report
└── build-report.txt       # Human-readable summary
```

### 9.2 Hugo Configuration Generation

**Generated `hugo.yaml`:**

```yaml
# Core settings
baseURL: "{config.hugo.base_url}"
languageCode: "{config.hugo.language_code}"
title: "{config.hugo.title}"

# Markup settings
markup:
  goldmark:
    renderer:
      unsafe: true    # Allow HTML in markdown
  highlight:
    noClasses: false

# Module import (theme-specific)
module:
  imports:
    - path: github.com/imfing/hextra  # For Hextra

# Theme parameters (merged from theme defaults + user params)
params:
  # Theme-specific defaults...
  # User overrides...
  build_date: "2025-12-07T00:00:00Z"  # Dynamic

# Menu (if AutoMainMenu)
menu:
  main:
    - identifier: repo-1
      name: Repo 1
      url: /repo-1/
      weight: 10
```

### 9.3 go.mod Generation

For themes using Hugo Modules:

```go
module documentation-site

go 1.21
```

---

## 10. Theme System

### 10.1 Theme Interface

```go
type Theme interface {
    Name() string
    Features() Features
    ApplyParams(ctx Context, params map[string]any)
    CustomizeRoot(ctx Context, root map[string]any)
}

type Features struct {
    Name            string
    UsesModules     bool
    ModulePath      string
    AutoMainMenu    bool
    SearchJSON      bool
    EditLinkSupport bool
    MathSupport     bool
}
```

### 10.2 Hextra Theme

**Module Path:** `github.com/imfing/hextra`

**Features:**
- UsesModules: true
- AutoMainMenu: true
- SearchJSON: true
- EditLinkSupport: true (per-page editURL)
- MathSupport: true

**Default Parameters:**
```yaml
params:
  navbar:
    displayTitle: true
    displayLogo: true
  page:
    width: normal
  footer:
    displayPoweredBy: true
  theme:
    default: system
    displayToggle: true
  search:
    enable: true
    type: flexsearch
  editURL:
    enable: true       # Enables per-page editURL
```

**Cascade (applied to content):**
```yaml
cascade:
  type: docs
```

### 10.3 Docsy Theme

**Module Path:** `github.com/google/docsy`

**Features:**
- UsesModules: true
- AutoMainMenu: false
- SearchJSON: false (uses different search)
- EditLinkSupport: true (global pattern)

**Default Parameters:**
```yaml
params:
  ui:
    navbar_logo: true
    sidebar_search_disable: false
    breadcrumb_disable: false
  links:
    user: []
    developer: []
  version_menu: Releases
```

### 10.4 Theme Registration

Themes self-register in init():

```go
func init() {
    theme.Register(&HextraTheme{})
}
```

---

## 11. Forge Integration

### 11.1 Forge Interface

```go
type Forge interface {
    Name() string
    Type() string  // "github", "gitlab", "forgejo"
    
    // Repository operations
    ListRepositories(ctx context.Context, org string) ([]*Repository, error)
    GetRepository(ctx context.Context, owner, repo string) (*Repository, error)
    
    // File operations
    GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
    
    // Capabilities
    Capabilities() Capabilities
}

type Capabilities struct {
    Webhooks      bool
    AutoDiscovery bool
    EditLinks     bool
    FileContent   bool
}
```

### 11.2 BaseForge HTTP Client

All forges share common HTTP operations:

```go
type BaseForge struct {
    httpClient       *http.Client
    apiURL           string
    token            string
    authHeaderPrefix string            // "Bearer " or "token "
    customHeaders    map[string]string
}

func (b *BaseForge) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error)
func (b *BaseForge) DoRequest(req *http.Request) ([]byte, error)
func (b *BaseForge) DoRequestWithHeaders(req *http.Request) ([]byte, http.Header, error)
```

### 11.3 GitHub Implementation

**API URL:** `https://api.github.com`

**Auth Header:** `Authorization: Bearer {token}`

**Custom Headers:**
```
X-GitHub-Api-Version: 2022-11-28
Accept: application/vnd.github+json
```

**Endpoints:**
- List repos: `GET /orgs/{org}/repos`
- Get repo: `GET /repos/{owner}/{repo}`
- Get file: `GET /repos/{owner}/{repo}/contents/{path}?ref={ref}`

### 11.4 GitLab Implementation

**API URL:** `https://gitlab.com/api/v4` (or custom)

**Auth Header:** `Authorization: Bearer {token}`

**Endpoints:**
- List repos: `GET /groups/{group}/projects`
- Get repo: `GET /projects/{id}`
- Get file: `GET /projects/{id}/repository/files/{path}?ref={ref}`

### 11.5 Forgejo/Gitea Implementation

**API URL:** `https://{host}/api/v1`

**Auth Header:** `Authorization: token {token}`

**Endpoints:**
- List repos: `GET /orgs/{org}/repos`
- Get repo: `GET /repos/{owner}/{repo}`
- Get file: `GET /repos/{owner}/{repo}/contents/{path}?ref={ref}`

### 11.6 Edit Link Generation

**Algorithm:**
```go
func GenerateEditLink(file DocFile, repo Repository) string {
    forgeType := repo.Tags["forge_type"]
    baseURL := repo.Tags["base_url"]
    
    switch forgeType {
    case "github":
        return fmt.Sprintf("%s/%s/%s/edit/%s/%s/%s",
            baseURL, owner, repo, branch, docsBase, relativePath)
    case "gitlab":
        return fmt.Sprintf("%s/%s/%s/-/edit/%s/%s/%s",
            baseURL, owner, repo, branch, docsBase, relativePath)
    case "forgejo", "gitea":
        return fmt.Sprintf("%s/%s/%s/_edit/%s/%s/%s",
            baseURL, owner, repo, branch, docsBase, relativePath)
    }
    return ""
}
```

---

## 12. Content Processing

### 12.0 Design Principles

**Dual Compatibility Goal:**

DocBuilder's content processing system is designed to ensure documentation works correctly in **both** contexts:

1. **Source Forge Context** - Links and content render correctly when viewing files directly in GitHub, GitLab, or Forgejo web interfaces
2. **Generated Hugo Site Context** - The same links and content work correctly in the rendered static site after transformation

This dual compatibility is achieved through the transform pipeline, which automatically rewrites links and adjusts content during the build process. Users write standard relative markdown links (e.g., `[guide](../guide.md)`) that work in their forge, and the pipeline transforms them to Hugo-compatible paths (e.g., `[guide](../../guide/)`) for the static site.

**Key Design Decisions:**

- Relative links (page-relative and repository-root-relative) are preserved in source for forge compatibility
- Transform pipeline handles all conversions to Hugo URL structure
- No manual link maintenance required - write once, works everywhere
- Transformations are deterministic and testable via golden tests

### 12.1 Transform Pipeline Architecture

**Location:** `internal/hugo/transforms/`

Content processing uses a **registry-based transform pipeline** with dependency ordering:

**Transform Stages:**
1. **Parse** - Extract front matter from markdown
2. **Build** - Generate default front matter fields
3. **Enrich** - Add repository/forge metadata
4. **Merge** - Apply front matter patches
5. **Transform** - Content transformations (link rewriting, etc.)
6. **Finalize** - Post-processing (heading stripping, escaping)
7. **Serialize** - Final YAML + content output

**Transform Interface:**
```go
type Transformer interface {
    Name() string
    Stage() TransformStage
    Dependencies() TransformDependencies
    Transform(PageAdapter) error
}
```

**PageAdapter Pattern:**
Transforms operate on `PageShim` which provides:
- `GetContent() string` / `SetContent(string)`
- `GetOriginalFrontMatter() map[string]any`
- `SetOriginalFrontMatter(map[string]any, bool)`
- `AddPatch(FrontMatterPatch)` - Queue front matter changes
- `ApplyPatches()` - Merge all patches
- `Serialize() error` - Final output

**Processing Flow:**
```
1. Load markdown file → DocFile
2. Create Page with raw content
3. Build PageShim facade
4. Execute transforms in dependency order:
   - front_matter_parser: Extract YAML
   - front_matter_builder_v2: Generate defaults
   - edit_link_injector_v2: Add edit URLs (if enabled)
   - relative_link_rewriter: Fix markdown links
   - strip_first_heading: Remove H1 (optional)
   - shortcode_escaper: Escape Hugo shortcodes
   - hextra_type_enforcer: Set type=docs (Hextra only)
   - front_matter_serialize: Output final YAML + content
5. Write to content/{forge?}/{repo}/{section}/file.md
```

**Example Input:**
```markdown
---
title: Existing Title
custom: value
---

# Content here
```

**Example Output:**
```markdown
---
title: Existing Title
custom: value
repository: my-repo
forge: github
section: guides
date: 2025-12-07T00:00:00Z
editURL: https://github.com/owner/repo/edit/main/docs/guides/page.md
---

# Content here
```

### 12.2 Transform System Details

**Registry Location:** `internal/hugo/transforms/`

**Transform Lifecycle:**

1. **Registration** - Transforms register in `init()`:
```go
func init() {
    transforms.Register(FrontMatterParser{})
}
```

2. **Dependency Resolution** - Build pipeline via topological sort:
```go
type TransformDependencies struct {
    MustRunAfter  []string  // Must execute after these transforms
    MustRunBefore []string  // Must execute before these transforms
    // Capability flags
    RequiresOriginalFrontMatter bool
    ModifiesContent             bool
    ModifiesFrontMatter         bool
    RequiresConfig              bool
    RequiresThemeInfo           bool
    RequiresForgeInfo           bool
    RequiresEditLinkResolver    bool
    RequiresFileMetadata        bool
}
```

3. **Execution** - Process page through transforms:
```go
transformList, err := transforms.List() // Sorted by dependencies
for _, transform := range transformList {
    if err := transform.Transform(pageShim); err != nil {
        return err
    }
}
```

**Built-in Transforms:**

| Transform | Stage | Purpose |
|-----------|-------|---------|
| front_matter_parser | Parse | Extract YAML from markdown |
| front_matter_builder_v2 | Build | Generate default fields |
| edit_link_injector_v2 | Enrich | Add editURL (theme-specific) |
| relative_link_rewriter | Transform | Fix relative markdown links |
| strip_first_heading | Finalize | Remove H1 if configured |
| shortcode_escaper | Finalize | Escape Hugo shortcodes |
| hextra_type_enforcer | Finalize | Set type=docs (Hextra) |
| front_matter_serialize | Serialize | Output final YAML+content |

**Transform Filtering:**

Configuration can enable/disable transforms:
```yaml
hugo:
  transforms:
    enable:  ["front_matter_parser", "relative_link_rewriter"]
    disable: ["strip_first_heading"]
```

### 12.3 Front Matter Patch System

**Location:** `internal/hugo/fmcore/`

Transforms add patches instead of directly mutating front matter:

```go
type FrontMatterPatch struct {
    Key           string
    Value         any
    Mode          MergeMode
    ArrayStrategy ArrayStrategy
    Source        string  // Transform name
}
```

**Merge Modes:**
- `MergeDeep` - Recursively merge maps
- `MergeReplace` - Overwrite value
- `MergeSetIfMissing` - Only set if key absent

**Patch Application:**
```go
shim.AddPatch(fmcore.FrontMatterPatch{
    Key:   "editURL",
    Value: "https://github.com/...",
    Mode:  fmcore.MergeSetIfMissing,
    Source: "edit_link_injector_v2",
})
```

All patches are applied during merge stage, with conflict tracking.

### 12.4 Title Generation from Filename

```go
func titleFromFilename(name string) string {
    // Remove extension
    base := strings.TrimSuffix(name, filepath.Ext(name))
    
    // Replace underscores with hyphens
    base = strings.ReplaceAll(base, "_", "-")
    
    // Split on hyphens
    parts := strings.Split(base, "-")
    
    // Title case each part
    for i, part := range parts {
        if len(part) > 0 {
            parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
        }
    }
    
    return strings.Join(parts, " ")
}
```

**Examples:**
- `getting-started.md` → `Getting Started`
- `api_reference.md` → `Api Reference`
- `README.md` → `Readme` (but README is ignored)

### 12.5 Index Page Generation

**Main Index (`content/_index.md`):**
```yaml
---
title: "{site_title}"
cascade:
  type: docs
---

Welcome to the documentation.
```

**Repository Index (`content/{repo}/_index.md`):**
```yaml
---
title: "{repo_name}"
weight: {index}
---

Documentation for {repo_name}.
```

**Section Index (`content/{repo}/{section}/_index.md`):**
```yaml
---
title: "{section_title}"
weight: {index}
---
```

---

## 13. Change Detection

### 13.1 Detection Levels

| Level | Speed | Accuracy | Method |
|-------|-------|----------|--------|
| 1 | Fast | Low | HEAD ref comparison |
| 2 | Medium | Medium | Directory tree hash |
| 3 | Slow | High | Doc files hash |
| 4 | Slow | Full | Deletion detection |

### 13.2 HEAD Comparison

```go
func detectHeadChange(repo, previousHead string) bool {
    currentHead := readRepoHead(repo)
    return currentHead != previousHead
}
```

### 13.3 Doc Files Hash

```go
func computeDocFilesHash(files []DocFile) string {
    // Sort by Hugo path for determinism
    paths := make([]string, len(files))
    for i, f := range files {
        paths[i] = f.HugoPath()
    }
    sort.Strings(paths)
    
    // Compute SHA-256
    h := sha256.New()
    for _, p := range paths {
        h.Write([]byte(p))
        h.Write([]byte{'\n'})
    }
    
    return hex.EncodeToString(h.Sum(nil))
}
```

### 13.4 Skip Conditions

Build can be skipped if ALL conditions are true:
1. All repository HEAD refs unchanged
2. Doc files hash unchanged
3. No deletions detected (if detect_deletions enabled)
4. Previous output directory is valid:
   - build-report.json exists
   - public/ directory non-empty
   - content/ has at least one .md file

---

## 14. Build Report

### 14.1 JSON Schema

```json
{
  "schema_version": 1,
  "repositories": 5,
  "files": 127,
  "start": "2025-12-07T00:00:00Z",
  "end": "2025-12-07T00:01:30Z",
  "stage_durations": {
    "prepare_output": 1000000,
    "clone_repos": 5000000000,
    "discover_docs": 100000000,
    "generate_config": 10000000,
    "layouts": 5000000,
    "copy_content": 200000000,
    "indexes": 50000000,
    "run_hugo": 2000000000
  },
  "cloned_repositories": 5,
  "failed_repositories": 0,
  "skipped_repositories": 0,
  "rendered_pages": 127,
  "static_rendered": true,
  "retries": 0,
  "retries_exhausted": false,
  "outcome": "success",
  "doc_files_hash": "abc123...",
  "issues": [],
  "skip_reason": ""
}
```

### 14.2 Issue Codes

| Code | Description |
|------|-------------|
| CLONE_FAILURE | Repository clone failed |
| PARTIAL_CLONE | Some repos failed, some succeeded |
| ALL_CLONES_FAILED | No repositories cloned |
| DISCOVERY_FAILURE | Documentation discovery failed |
| NO_REPOSITORIES | No repositories configured |
| HUGO_EXECUTION | Hugo build failed |
| BUILD_CANCELED | Build was canceled |
| AUTH_FAILURE | Authentication failed |
| REPO_NOT_FOUND | Repository does not exist |
| UNSUPPORTED_PROTOCOL | Unknown URL protocol |
| REMOTE_DIVERGED | Local branch diverged |
| GENERIC_STAGE_ERROR | Unclassified stage error |

### 14.3 Text Summary

```
DocBuilder Build Report
=======================
Outcome: success
Repositories: 5 (0 failed, 0 skipped)
Documentation Files: 127
Rendered Pages: 127
Static Site: rendered
Duration: 1m 30s
Doc Files Hash: abc123...
```

---

## 15. Daemon Mode

### 15.1 Overview

Long-running service that:
- Periodically polls for repository changes
- Handles webhook events for immediate rebuilds
- Serves generated documentation
- Provides admin/status endpoints

### 15.2 HTTP Endpoints

| Port | Path | Method | Purpose |
|------|------|--------|---------|
| docs_port | /* | GET | Serve static site |
| webhook_port | /webhook/github | POST | GitHub webhooks |
| webhook_port | /webhook/gitlab | POST | GitLab webhooks |
| webhook_port | /webhook/forgejo | POST | Forgejo webhooks |
| admin_port | /health | GET | Health check |
| admin_port | /metrics | GET | Prometheus metrics |
| admin_port | /status | GET | Build status |
| admin_port | /build | POST | Trigger build |

### 15.3 Webhook Handling

**Algorithm:**
```go
func handleWebhook(r *http.Request, eventHeader, source string) {
    // Read event type
    eventType := r.Header.Get(eventHeader)
    
    // Validate signature (if secret configured)
    if !validateSignature(r, secret) {
        return 401
    }
    
    // Parse payload
    var payload WebhookPayload
    json.Decode(r.Body, &payload)
    
    // Check if relevant event
    if isPushEvent(eventType) {
        // Queue rebuild
        buildQueue <- BuildRequest{
            Repository: payload.Repository.Name,
            Branch: payload.Ref,
        }
    }
    
    return 200
}
```

### 15.4 Scheduling

Uses cron expressions for periodic discovery:

```yaml
sync:
  schedule: "*/15 * * * *"  # Every 15 minutes
```

---

## 16. Preview Mode

### 16.1 Overview

Local development mode that:
- Watches local documentation directory
- Rebuilds on file changes
- Serves via Hugo's built-in server
- No Git operations

### 16.2 Workflow

```
1. Create synthetic repository config:
   - URL: local path
   - Name: "local"
   - Paths: [configured path or "."]

2. Initial build:
   - Discover docs in local path
   - Generate Hugo site
   - Start Hugo server

3. Watch loop:
   - Monitor source directory
   - On change:
     - Rebuild site
     - Hugo hot-reloads

4. On exit:
   - Stop Hugo server
   - Clean up
```

### 16.3 File Watching

Uses fsnotify for cross-platform file watching:

```go
watcher.Add(sourcePath)

for {
    select {
    case event := <-watcher.Events:
        if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
            if isMarkdownFile(event.Name) {
                rebuild()
            }
        }
    case err := <-watcher.Errors:
        log.Error("watcher error", err)
    }
}
```

---

## 17. Error Handling

**Location:** `internal/foundation/errors/`

### 17.1 Error Type

```go
type ClassifiedError struct {
    category ErrorCategory
    severity ErrorSeverity
    retry    RetryStrategy
    message  string
    cause    error
    context  ErrorContext
}

type ErrorContext map[string]any

type ErrorCategory string
const (
    CategoryConfig        ErrorCategory = "config"
    CategoryValidation    ErrorCategory = "validation"
    CategoryAuth          ErrorCategory = "auth"
    CategoryNotFound      ErrorCategory = "not_found"
    CategoryAlreadyExists ErrorCategory = "already_exists"
    CategoryNetwork       ErrorCategory = "network"
    CategoryGit           ErrorCategory = "git"
    CategoryForge         ErrorCategory = "forge"
    CategoryBuild         ErrorCategory = "build"
    CategoryHugo          ErrorCategory = "hugo"
    CategoryFileSystem    ErrorCategory = "filesystem"
    CategoryRuntime       ErrorCategory = "runtime"
    CategoryDaemon        ErrorCategory = "daemon"
    CategoryInternal      ErrorCategory = "internal"
)

type ErrorSeverity string
const (
    SeverityFatal   ErrorSeverity = "fatal"
    SeverityError   ErrorSeverity = "error"
    SeverityWarning ErrorSeverity = "warning"
    SeverityInfo    ErrorSeverity = "info"
)

type RetryStrategy string
const (
    RetryNever      RetryStrategy = "never"      // Permanent failure, don't retry
    RetryImmediate  RetryStrategy = "immediate"  // Retry immediately
    RetryBackoff    RetryStrategy = "backoff"    // Retry with exponential backoff
    RetryRateLimit  RetryStrategy = "rate_limit" // Retry after rate limit window
    RetryUserAction RetryStrategy = "user"       // Requires user intervention
)
```

### 17.2 Error Construction

```go
// Create new error with category
func NewError(category ErrorCategory, message string) *ErrorBuilder

// Wrap existing error
func WrapError(err error, category ErrorCategory, message string) *ErrorBuilder

// Convenience constructors
func ValidationError(message string) *ErrorBuilder
func ConfigError(message string) *ErrorBuilder
func AuthError(message string) *ErrorBuilder
func NotFoundError(message string) *ErrorBuilder

// Builder methods (fluent API)
func (b *ErrorBuilder) WithContext(key string, value any) *ErrorBuilder
func (b *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder
func (b *ErrorBuilder) WithRetry(strategy RetryStrategy) *ErrorBuilder
func (b *ErrorBuilder) Build() *ClassifiedError

// Example usage
err := errors.ValidationError("invalid forge type").
    WithContext("input", forgeType).
    WithContext("valid_values", []string{"github", "gitlab"}).
    Build()
```

### 17.3 Retry Logic

```go
func withRetry(fn func() error, config RetryConfig) error {
    var lastErr error
    delay := config.InitialDelay
    
    for attempt := 0; attempt <= config.MaxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        if !isRetryable(err) {
            return err
        }
        
        lastErr = err
        time.Sleep(delay)
        
        switch config.Backoff {
        case "linear":
            delay += config.InitialDelay
        case "exponential":
            delay *= 2
        }
        
        if delay > config.MaxDelay {
            delay = config.MaxDelay
        }
    }
    
    return lastErr
}
```

---

## 18. Observability

### 18.1 Logging

Uses `log/slog` structured logging:

```go
slog.Info("Documentation discovered",
    slog.String("repository", repoName),
    slog.Int("files", count),
)
```

**Log Levels:**
- DEBUG: Detailed debugging information
- INFO: Normal operation events
- WARN: Warning conditions
- ERROR: Error conditions

### 18.2 Metrics

Prometheus-compatible metrics:

| Metric | Type | Description |
|--------|------|-------------|
| docbuilder_builds_total | Counter | Total builds by outcome |
| docbuilder_build_duration_seconds | Histogram | Build duration |
| docbuilder_repos_processed | Counter | Repositories processed |
| docbuilder_docs_discovered | Gauge | Documentation files |
| docbuilder_clone_duration_seconds | Histogram | Clone operation duration |

### 18.3 Health Checks

**Endpoint:** `/health`

**Response:**
```json
{
  "status": "healthy",
  "checks": {
    "git": "ok",
    "hugo": "ok",
    "filesystem": "ok"
  }
}
```

---

## 19. File System Outputs

### 19.1 Workspace Layout

```
/tmp/docbuilder-{timestamp}/
├── {repo-1}/
│   ├── .git/
│   ├── docs/
│   │   └── ...
│   └── ...
├── {repo-2}/
│   └── ...
└── ...
```

### 19.2 Staging Layout

```
{output}_stage/
├── hugo.yaml
├── go.mod
├── content/
│   └── ...
├── layouts/
├── static/
└── assets/
```

### 19.3 Final Output Layout

```
{output}/
├── hugo.yaml
├── go.mod
├── content/
│   └── ...
├── layouts/
├── static/
├── assets/
├── public/           # After Hugo run
│   ├── index.html
│   ├── {repo-1}/
│   └── ...
├── build-report.json
└── build-report.txt
```

### 19.4 State Persistence

```
.docbuilder/
└── state.json
```

**State Contents:**
```json
{
  "last_build": "2025-12-07T00:00:00Z",
  "repositories": {
    "repo-1": {
      "head": "abc123...",
      "doc_hash": "def456...",
      "last_update": "2025-12-07T00:00:00Z"
    }
  },
  "doc_files_hash": "ghi789..."
}
```

---

## 20. Algorithms

### 20.1 Atomic Directory Promotion

```go
func promoteStaging(staging, output string) error {
    // Staging is sibling: {output}_stage
    
    // 1. If output exists, rename to backup
    backup := output + "_backup"
    if exists(output) {
        rename(output, backup)
    }
    
    // 2. Rename staging to output
    if err := rename(staging, output); err != nil {
        // Restore backup
        rename(backup, output)
        return err
    }
    
    // 3. Remove backup
    removeAll(backup)
    
    return nil
}
```

### 20.2 Content Hash Computation

```go
func computeContentHash(content []byte) string {
    h := sha256.Sum256(content)
    return hex.EncodeToString(h[:])
}
```

### 20.3 Path Normalization

```go
func normalizeHugoPath(path string) string {
    // Convert backslashes to forward slashes
    path = filepath.ToSlash(path)
    
    // Remove leading ./
    path = strings.TrimPrefix(path, "./")
    
    // Ensure no double slashes
    for strings.Contains(path, "//") {
        path = strings.ReplaceAll(path, "//", "/")
    }
    
    return path
}
```

### 20.4 Environment Variable Expansion

Uses Go's `os.ExpandEnv`:

```go
// ${VAR} → value of VAR
// $VAR → value of VAR
expanded := os.ExpandEnv(input)
```

---

## 21. Test Requirements

### 21.1 Unit Tests

**Coverage Areas:**
- Configuration parsing and validation
- DocFile path computation
- Front matter processing
- Theme parameter merging
- Hash computation
- Error construction

### 21.2 Integration Tests

**Coverage Areas:**
- Full build pipeline execution
- Git clone operations (with test repos)
- Hugo site generation
- Incremental build behavior
- Webhook handling

### 21.3 Golden Tests

For Hugo configuration generation:

```go
func TestHugoConfigGeneration(t *testing.T) {
    // Generate config
    config := generateHugoConfig(input)
    
    // Compare to golden file
    golden := readGoldenFile("testdata/hugo-hextra.yaml")
    
    if config != golden {
        t.Errorf("config mismatch")
    }
}
```

### 21.4 End-to-End Tests

```bash
# Build with test configuration
./docbuilder build -c test/config.test.yaml -o /tmp/test-output

# Verify output
test -f /tmp/test-output/hugo.yaml
test -d /tmp/test-output/content
test -f /tmp/test-output/build-report.json
```

---

## 22. Package Architecture

### 22.1 Core Packages

| Package | Location | Purpose |
|---------|----------|---------|
| **CLI Commands** | `cmd/docbuilder/commands/` | Command implementations (build, init, daemon, preview, etc.) |
| **Build Service** | `internal/build/` | Build pipeline orchestration |
| **Configuration** | `internal/config/` | YAML parsing and validation |
| **Git Client** | `internal/git/` | Repository clone/update operations |
| **Documentation Discovery** | `internal/docs/` | File discovery and path computation |
| **Hugo Generator** | `internal/hugo/` | Hugo site generation |
| **Transform System** | `internal/hugo/transforms/` | Content processing pipeline |
| **Front Matter Core** | `internal/hugo/fmcore/` | Front matter patching and merging |
| **Theme System** | `internal/hugo/theme/` | Theme interface and implementations |
| **Forge Integration** | `internal/forge/` | GitHub/GitLab/Forgejo clients |
| **Error Foundation** | `internal/foundation/errors/` | Classified error system |
| **Daemon** | `internal/daemon/` | Long-running service mode |
| **Workspace** | `internal/workspace/` | Temporary directory management |

### 22.2 Transform Package Structure

```
internal/hugo/transforms/
├── registry.go              # Transform registration
├── dependencies.go          # Transformer interface
├── toposort.go              # Dependency ordering
├── defaults.go              # PageShim facade
├── adapters.go              # PageAdapter interface
├── front_matter_parser.go   # (built-in)
├── front_matter_builder_v2.go
├── edit_link_v2.go
├── relative_link_rewriter.go
├── strip_heading.go
├── shortcode_escaper.go
├── hextra_type.go
└── front_matter_serialize.go
```

### 22.3 Key Interfaces

**Transformer Interface:**
```go
// internal/hugo/transforms/dependencies.go
type Transformer interface {
    Name() string
    Stage() TransformStage
    Dependencies() TransformDependencies
    Transform(PageAdapter) error
}
```

**Theme Interface:**
```go
// internal/hugo/theme/theme.go
type Theme interface {
    Name() config.Theme
    Features() ThemeFeatures
    ApplyParams(ctx ParamContext, params map[string]any)
    CustomizeRoot(ctx ParamContext, root map[string]any)
}
```

**Forge Interface:**
```go
// internal/forge/forge.go
type Forge interface {
    Name() string
    Type() string
    ListRepositories(ctx context.Context, org string) ([]*Repository, error)
    GetRepository(ctx context.Context, owner, repo string) (*Repository, error)
    GetFileContent(ctx context.Context, owner, repo, path, ref string) ([]byte, error)
    Capabilities() Capabilities
}
```

### 22.4 Data Flow

```
Configuration (config.yaml)
    ↓
CLI Commands (cmd/docbuilder/commands/)
    ↓
Build Service (internal/build/)
    ├→ Git Client (internal/git/) → Clone repositories
    ├→ Documentation Discovery (internal/docs/) → Find markdown files
    └→ Hugo Generator (internal/hugo/)
        ├→ Theme System (theme/) → Apply theme defaults
        ├→ Transform Pipeline (transforms/) → Process content
        │   └→ Front Matter Core (fmcore/) → Patch merging
        └→ Hugo execution → Rendered site
```

---

## Appendix A: Example Configuration

```yaml
version: "2.0"

repositories:
  - url: https://github.com/org/repo1.git
    name: repo1
    branch: main
    paths: ["docs"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"
    tags:
      forge_type: github
      base_url: https://github.com

  - url: https://github.com/org/repo2.git
    name: repo2
    paths: ["documentation", "docs"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"
    tags:
      forge_type: github
      base_url: https://github.com

build:
  clone_concurrency: 4
  clone_strategy: auto
  namespace_forges: auto
  render_mode: auto
  incremental: true

hugo:
  title: "My Documentation"
  description: "Aggregated documentation from multiple repositories"
  base_url: "https://docs.example.com"
  theme: hextra
  params:
    navbar:
      displayTitle: true
    search:
      enable: true

output:
  directory: "./site"
  clean: true
```

---

## Appendix B: Error Recovery Procedures

### B.1 Clone Failure

```
1. Check network connectivity
2. Verify authentication credentials
3. Confirm repository URL is correct
4. Check for rate limiting
5. Review retry configuration
```

### B.2 Hugo Generation Failure

```
1. Check Hugo installation
2. Verify theme module availability
3. Review hugo.yaml for syntax errors
4. Check for invalid front matter
5. Run hugo manually for detailed errors
```

### B.3 State Corruption

```
1. Delete .docbuilder/state.json
2. Clean output directory
3. Run fresh build with clean: true
```

---

## Document History

| Version | Date | Changes |
|---------|------|---------|
| 2.0 | December 2025 | Initial comprehensive specification |
| 2.0.1 | December 14, 2025 | Updated to reflect transform-based architecture, corrected error handling, added package architecture section |

---

**End of Functional Specification**
