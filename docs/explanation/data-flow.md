# Data Flow

This document describes how data flows through DocBuilder from configuration to rendered static site.

## Build Request Flow

### 1. Configuration Loading

**Input:** YAML configuration file
**Process:**
1. Load YAML from file
2. Expand environment variables (`.env`, `.env.local`)
3. Validate configuration structure
4. Apply defaults
5. Create `Config` object

**Output:** Validated `Config` object

**Data:**
```go
type Config struct {
    Repositories []Repository
    Forges       []ForgeConfig
    Hugo         HugoConfig
    Build        BuildConfig
    Output       OutputConfig
}
```

### 2. Repository Discovery

**Input:** `Config` with forge definitions
**Process:**
1. Initialize forge clients (GitHub, GitLab, Forgejo)
2. Query forge APIs for organizations/groups
3. List repositories in each organization
4. Filter by `required_paths` (e.g., "docs")
5. Convert forge repositories to `Repository` config

**Output:** List of `Repository` objects

**Data:**
```go
type Repository struct {
    URL    string
    Name   string
    Branch string
    Paths  []string
    Auth   AuthConfig
}
```

### 3. Repository Cloning

**Input:** List of `Repository` objects
**Process:**
1. Authenticate with configured method
2. Clone repository to workspace
3. Read current HEAD ref (Git SHA)
4. Store clone metadata in `GitState`

**Output:** Local repository clones

**Data:**
```go
type GitState struct {
    Repositories map[string]RepositoryGitInfo
    WorkspaceDir string
}

type RepositoryGitInfo struct {
    Path   string
    Ref    string  // HEAD SHA
    Branch string
}
```

### 4. Documentation Discovery

**Input:** Cloned repositories with configured `paths`
**Process:**
1. Walk each configured path (e.g., `docs/`)
2. Find markdown files (`.md`, `.markdown`)
3. Exclude standard files (README, CONTRIBUTING, etc.)
4. Extract file metadata (repository, section, path)
5. Create `DocFile` objects

**Output:** List of `DocFile` objects

**Data:**
```go
type DocFile struct {
    Name       string
    Path       string
    HugoPath   string  // content/repo/section/file.md
    Repository string
    Forge      string  // When multi-forge namespacing
    Section    string  // Subdirectory within docs/
    SourcePath string  // Absolute path in workspace
}
```

### 5. Hugo Configuration Generation

**Input:** `Config.Hugo` and discovered repositories
**Process:**
1. Load Hugo configuration from YAML config
2. Apply theme defaults (Relearn theme)
3. Deep merge user parameters
4. Configure Hugo modules import
5. Set up taxonomies (tags, categories)
6. Generate `go.mod` for Hugo modules

**Output:** `hugo.yaml` and `go.mod` files

**Data:**
```yaml
title: "My Documentation"
baseURL: "https://docs.example.com"
theme: "relearn"
module:
  imports:
    - path: "github.com/McShelby/hugo-theme-relearn"
params:
  themeVariant: "auto"
  editURL: "..."
```

### 6. Content Transformation

**Input:** `DocFile` objects with source content
**Process:** For each file, apply transform pipeline:

1. **Parse Front Matter**
   - Extract YAML headers
   - Preserve original content

2. **Normalize Index Files**
   - Rename README.md → _index.md
   - Mark as index file

3. **Build Base Front Matter**
   - Add title (from filename or H1)
   - Add date (current timestamp)
   - Add repository metadata

4. **Extract Index Title**
   - Parse H1 heading from content
   - Use for index page titles

5. **Strip Heading**
   - Remove H1 from content body
   - Avoid duplicate titles

6. **Rewrite Relative Links**
   - Convert `.md` links to `/`
   - Make paths Hugo-compatible

7. **Rewrite Image Links**
   - Convert to repository-relative paths
   - Fix broken image references

8. **Generate Keywords**
   - Extract from `@keywords` directive
   - Add to front matter

9. **Add Repository Metadata**
   - Inject repository name
   - Add forge type (if multi-forge)
   - Add branch information

10. **Add Edit Link**
    - Generate edit URL based on forge
    - Add to front matter as `editURL`

11. **Serialize Document**
    - Format front matter as YAML
    - Combine with transformed content

**Output:** Transformed markdown files in Hugo content directory

**Data:**
```markdown
---
title: "API Reference"
date: 2025-01-03T10:00:00Z
repository: "my-service"
forge: "github"
editURL: "https://github.com/org/repo/edit/main/docs/api.md"
---

API documentation content...
```

### 7. Index Page Generation

**Input:** Transformed `DocFile` objects grouped by repository and section
**Process:**
1. Generate main landing page (`content/_index.md`)
   - List all repositories
   - Show statistics
2. Generate repository index pages (`content/<repo>/_index.md`)
   - List sections
   - List root files
3. Generate section index pages (`content/<repo>/<section>/_index.md`)
   - List files in section

**Output:** Index markdown files

**Template Context:**
```go
type IndexContext struct {
    Site         SiteInfo
    Repositories map[string][]DocFile  // main only
    Files        []DocFile
    Sections     map[string][]DocFile  // repository only
    SectionName  string                // section only
    Stats        StatsInfo
    Now          time.Time
}
```

### 8. Hugo Rendering

**Input:** Complete Hugo site structure
**Process:**
1. Execute Hugo binary
2. Parse build output
3. Count rendered pages
4. Capture any errors

**Output:** Static site in `public/` directory

**Data:**
```
public/
├── index.html
├── repo-name/
│   ├── index.html
│   ├── section/
│   │   ├── index.html
│   │   └── file.html
└── static/
    └── images/
```

### 9. Build Report Generation

**Input:** Build execution metadata from all stages
**Process:**
1. Collect stage timings
2. Aggregate statistics (files, repositories, pages)
3. Collect issues (warnings, errors)
4. Compute delta reasons for incremental builds
5. Calculate global documentation hash

**Output:** `BuildReport` object and `build-report.json` file

**Data:**
```go
type BuildReport struct {
    Status           BuildStatus
    StartTime        time.Time
    EndTime          time.Time
    Duration         time.Duration
    Stages           []StageResult
    RepositoryStats  RepositoryStats
    DocumentStats    DocumentStats
    Issues           []BuildIssue
    DocFilesHash     string
    DeltaRepoReasons map[string]string
}
```

## Incremental Build Flow

### Change Detection

**Input:** Previous build state and current configuration
**Process:**
1. Load previous `BuildReport` and `GitState`
2. For each repository:
   - Compare HEAD ref with previous build
   - If unchanged, skip cloning
   - If changed, clone and discover
3. Compute documentation hash
4. Compare with previous hash
5. Determine delta reason

**Output:** List of changed repositories

**Delta Reasons:**
- `unknown` - First build
- `quick_hash_diff` - Content changed
- `config_changed` - Configuration modified
- `forced` - Forced rebuild
- `assumed_changed` - Could not determine

### Partial Builds

**Input:** Changed repository list
**Process:**
1. Skip cloning unchanged repositories
2. Discover docs only in changed repositories
3. Update global documentation set
4. Regenerate affected index pages
5. Run Hugo on complete site

**Output:** Updated static site with minimal rebuilding

## Event Flow

### Event Emission

Events are emitted at key points in the build:

1. `BuildStarted` - Build begins
2. `StageStarted` - Stage begins
3. `RepositoryCloned` - Repository cloned
4. `RepositoryUpdated` - Repository updated
5. `DocumentationDiscovered` - Docs found
6. `StageCompleted` - Stage finished
7. `BuildCompleted` - Build finished
8. `BuildFailed` - Build failed

### Event Storage

**Input:** `Event` objects
**Process:**
1. Append event to SQLite database
2. Include timestamp, type, and JSON data
3. Maintain event order

**Output:** Persistent event log

**Data:**
```go
type Event struct {
    ID        string
    Timestamp time.Time
    Type      EventType
    Data      json.RawMessage
}
```

### Event Projection

**Input:** Event stream
**Process:**
1. Read events from store
2. Replay events to rebuild state
3. Compute derived metrics

**Output:** Projected state (build history, statistics)

## Metrics Flow

### Metric Collection

Metrics are collected throughout the build:

1. **Stage Timings** - Record start/end for each stage
2. **Stage Results** - Count success/warning/fatal outcomes
3. **Repository Counts** - Cloned, skipped, failed
4. **Document Counts** - Discovered files, rendered pages
5. **Retry Attempts** - Retry counts per stage

### Metric Export

**Input:** Collected metrics
**Process:**
1. Store in Prometheus registry
2. Expose via `/metrics/prometheus` endpoint
3. Format as Prometheus text format

**Output:** Prometheus-compatible metrics

**Example:**
```
docbuilder_build_duration_seconds_sum 45.23
docbuilder_stage_duration_seconds{stage="clone_repos"} 12.45
docbuilder_build_outcomes_total{outcome="success"} 42
```

## Related Documentation

- [Architecture Overview](architecture-overview.md) - System architecture
- [Pipeline Architecture](pipeline-architecture.md) - Build pipeline stages
- [Build Report Reference](../reference/report.md) - Report format details
