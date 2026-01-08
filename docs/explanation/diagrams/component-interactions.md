---
categories:
    - explanation
    - architecture
date: 2026-01-04T00:00:00Z
id: 1b740fed-1103-44b3-9689-8785c4db8377
tags:
    - components
    - interactions
    - integration
title: Component Interactions Diagrams
---

# Component Interactions Diagrams

This document shows how specific components interact within DocBuilder, focusing on theme configuration, forge integration, and change detection.

**Last Updated:** January 4, 2026 - Reflects Relearn-only configuration.

## Relearn Theme Configuration

DocBuilder uses the Relearn theme exclusively with hardcoded default configuration.

```
┌────────────────────────────────────────────────────┐
│          Relearn Theme Configuration               │
│          (internal/hugo/config_writer.go)          │
│                                                    │
│  applyRelearnThemeDefaults(params)                 │
│                                                    │
│  Default Parameters:                               │
│  - themeVariant: ["auto", "zen-light", "zen-dark"]│
│  - themeVariantAuto: ["zen-light", "zen-dark"]    │
│  - showVisitedLinks: true                          │
│  - collapsibleMenu: true                           │
│  - alwaysopen: false                               │
│  - disableBreadcrumb: false                        │
│  - disableLandingPageButton: true                  │
│  - disableShortcutsTitle: false                    │
│  - disableLanguageSwitchingButton: true            │
│  - disableTagHiddenPages: false                    │
│  - disableGeneratorVersion: false                  │
│  - mermaid.enable: true                            │
│  - math.enable: true                               │
│  - enable_transitions: (if configured)             │
└────────────────────────────────────────────────────┘

Generation Flow:
1. Load config → title, baseURL, etc.
2. Apply Relearn defaults (via applyRelearnThemeDefaults)
3. User params deep merge (user values override defaults)
4. Add dynamic fields (build_date, doc_builder_version)
5. Add Hugo module: github.com/McShelby/hugo-theme-relearn
6. Configure i18n settings (defaultContentLanguage: "en")
7. Write hugo.yaml to staging directory
```

### Configuration Merging Strategy

**Deep Merge Algorithm**:
```
For each key in user params:
    If value is map:
        Recursively merge with default map
    Else:
        Override default value

Example:
    Default:  { mermaid: { enable: true, theme: "default" } }
    User:     { mermaid: { theme: "dark" } }
    Result:   { mermaid: { enable: true, theme: "dark" } }
```

**Non-Overridable Settings**:
- Hugo module path: Always `github.com/McShelby/hugo-theme-relearn`
- Language configuration: Always English (`en`)
- Markup configuration: Goldmark with specific extensions

### Theme-Specific Features

**Auto Theme Variant**:
- Detects OS light/dark preference
- Switches between configured variants
- Default: zen-light (light) / zen-dark (dark)

**Mermaid Diagrams**:
- Enabled by default
- Supports flowcharts, sequence diagrams, Gantt charts
- Theme-aware styling

**Math Support (MathJax)**:
- Enabled by default
- LaTeX-style equations
- Inline: `$equation$`
- Block: `$$equation$$`

**Search**:
- Lunr.js-powered offline search
- Searches titles, content, and tags
- Automatically indexes all pages

### Implementation Files
- `internal/hugo/config_writer.go` - Configuration generation
- `internal/hugo/modules.go` - Hugo module management
- `internal/config/typed/hugo_config.go` - Configuration validation

---

## Forge Integration

Forge clients provide repository metadata and edit link generation.

```
┌──────────────────────────────────────────────┐
│           Forge Factory                      │
│      (internal/forge/factory.go)             │
│                                              │
│  NewForge(config) → Forge                    │
└──────────────┬───────────────────────────────┘
               │
               │ Based on URL detection
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
```

### Forge Detection

**URL-Based Detection**:
```
Repository URL Analysis:
    │
    ├─ Contains "github.com" → GitHubClient
    ├─ Contains "gitlab.com" → GitLabClient
    ├─ Contains "forgejo" → ForgejoClient
    └─ Default → GenericForge
```

**Configuration Priority**:
1. Explicit `forge.type` in config (if provided)
2. URL pattern matching
3. API endpoint detection (if accessible)

### Operation Flow

**Edit Link Generation**:
```
1. Document processed in pipeline
    │
    ▼
2. addEditLink transform called
    │
    ├─ Extract repository metadata
    │   ├─ Repository URL
    │   ├─ Source commit
    │   └─ Source branch
    │
    ├─ Detect forge type
    │   └─ GitHubClient
    │
    ├─ Build edit URL
    │   ├─ Base: https://github.com/user/repo
    │   ├─ Action: /edit/
    │   ├─ Branch: main
    │   └─ Path: /docs/file.md
    │   Result: https://github.com/user/repo/edit/main/docs/file.md
    │
    └─ Inject into front matter
        └─ editURL: https://...
```

**API Integration** (Future):
```
Forge Client
    │
    ├─ Authenticate
    │   ├─ Token header: "Authorization: Bearer {token}"
    │   └─ Custom headers (API version, User-Agent)
    │
    ├─ Fetch Repository Metadata
    │   ├─ GET /repos/{owner}/{repo}
    │   ├─ Parse response
    │   └─ Return: name, description, default_branch
    │
    └─ Fetch Commit Info
        ├─ GET /repos/{owner}/{repo}/commits/{sha}
        ├─ Parse response
        └─ Return: author, date, message
```

### Forge-Specific Patterns

**GitHub**:
- Edit URL: `/{owner}/{repo}/edit/{branch}/{path}`
- API: `https://api.github.com`
- Headers: `X-GitHub-Api-Version: 2022-11-28`

**GitLab**:
- Edit URL: `/{owner}/{repo}/-/edit/{branch}/{path}`
- API: `https://gitlab.com/api/v4`
- Headers: `PRIVATE-TOKEN: {token}`

**Forgejo**:
- Edit URL: `/{owner}/{repo}/_edit/{branch}/{path}`
- API: `https://forge.example.com/api/v1`
- Headers: `Authorization: token {token}`

### Implementation Files
- `internal/forge/factory.go` - Forge detection and creation
- `internal/forge/github/` - GitHub client
- `internal/forge/gitlab/` - GitLab client
- `internal/forge/forgejo/` - Forgejo client
- `internal/hugo/edit_link_resolver.go` - Edit link generation

---

## Change Detection System

Multi-level change detection optimizes incremental builds.

```
┌──────────────────────────────────────────────────┐
│          Change Detector                         │
│      (internal/hugo/doc_changes.go)              │
└──────────────┬───────────────────────────────────┘
               │
               │ DetectChanges(repos)
               ▼
    ┌──────────────────────┐
    │  Load Previous State │
    │  - HEAD refs         │
    │  - Doc hashes        │
    │  - Commit dates      │
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
               │  ├─ Changed? → Include
               │  └─ Cost: O(1) - single git command
               │
               ├─ Level 2: Quick Hash
               │  ├─ Hash directory tree
               │  ├─ Compare to previous
               │  ├─ Changed? → Include
               │  └─ Cost: O(n) - filesystem scan
               │
               ├─ Level 3: Doc Files Hash
               │  ├─ Discover docs
               │  ├─ Sort paths
               │  ├─ SHA-256 hash
               │  ├─ Compare to previous
               │  ├─ Changed? → Include
               │  └─ Cost: O(n*m) - read file contents
               │
               └─ Level 4: Deletion Detection
                  ├─ Compare file lists
                  ├─ Detect removed files
                  ├─ Deletions? → Include
                  └─ Cost: O(n) - set difference
               
               ▼
    ┌──────────────────────┐
    │    ChangeSet         │
    │                      │
    │ - ChangedRepos: []   │
    │ - SkippedRepos: []   │
    │ - Reasons: map[]     │
    │ - RequiresRebuild    │
    └──────────────────────┘
```

### Change Detection Algorithm

```go
func DetectChanges(repos []Repository, prevState BuildState) ChangeSet {
    changeSet := NewChangeSet()
    
    for _, repo := range repos {
        // Level 1: HEAD comparison
        currentHEAD := git.GetHEAD(repo)
        previousHEAD := prevState.Commits[repo.Name]
        
        if currentHEAD != previousHEAD {
            changeSet.AddChanged(repo, "HEAD changed")
            continue
        }
        
        // Level 2: Quick directory hash
        dirHash := computeDirectoryHash(repo.Path)
        prevHash := prevState.DirectoryHashes[repo.Name]
        
        if dirHash != prevHash {
            changeSet.AddChanged(repo, "directory structure changed")
            continue
        }
        
        // Level 3: Documentation files hash
        docs := DiscoverDocs(repo)
        docHash := computeDocHash(docs)
        prevDocHash := prevState.DocHashes[repo.Name]
        
        if docHash != prevDocHash {
            changeSet.AddChanged(repo, "documentation changed")
            continue
        }
        
        // Level 4: Deletion detection
        prevDocs := prevState.DocsByRepo[repo.Name]
        if detectDeletions(docs, prevDocs) {
            changeSet.AddChanged(repo, "files deleted")
            continue
        }
        
        // No changes detected
        changeSet.AddSkipped(repo, "no changes")
    }
    
    return changeSet
}
```

### Optimization Strategies

**Early Exit**:
- Stop at first detected change
- Don't compute expensive hashes if HEAD differs
- Saves computation time

**Caching**:
- Cache HEAD references
- Cache directory hashes
- Cache file content hashes
- Reuse across builds

**Parallel Detection**:
- Check multiple repositories concurrently
- Use worker pool pattern
- Aggregate results

### Implementation Files
- `internal/hugo/doc_changes.go` - Change detection logic
- `internal/hugo/early_skip.go` - Early skip evaluation
- `internal/state/git_state.go` - State persistence
- `internal/git/git.go` - HEAD reference reading

---

## Build State Management

Build state tracks progress and enables incremental builds.

```
┌─────────────────────────────────────────────────┐
│              BuildState                         │
│      (internal/hugo/build_state.go)             │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          GitState                         │  │
│  │  - WorkspaceDir: /tmp/docbuilder-xyz/    │  │
│  │  - Repositories: []Repository            │  │
│  │  - Commits: map[repo]string              │  │
│  │  - CommitDates: map[repo]time.Time       │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          DocsState                        │  │
│  │  - Files: []DocFile                      │  │
│  │  - IsSingleRepo: bool                    │  │
│  │  - FilesByRepository: map[repo][]DocFile │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          PipelineState                    │  │
│  │  - ConfigHash: string                    │  │
│  │  - ExecutedStages: []string              │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          BuildReport                      │  │
│  │  - Status: BuildStatus                   │  │
│  │  - StageDurations: map[stage]Duration    │  │
│  │  - Errors: []BuildIssue                  │  │
│  │  - Warnings: []BuildIssue                │  │
│  │  - Summary: string                       │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          Generator                        │  │
│  │  - Reference to Hugo generator           │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

### State Lifecycle

```
1. Initialization
    │
    ├─ Create BuildState
    ├─ Initialize GitState
    ├─ Initialize DocsState
    ├─ Initialize PipelineState
    └─ Create BuildReport
    │
    ▼
2. Stage Execution
    │
    ├─ For each stage:
    │   ├─ Record start time
    │   ├─ Execute stage function
    │   ├─ Update sub-states
    │   ├─ Record duration
    │   └─ Handle errors
    │
    ▼
3. Persistence
    │
    ├─ Serialize GitState
    ├─ Serialize DocsState
    ├─ Serialize PipelineState
    └─ Write to .docbuilder/state.json
    │
    ▼
4. Next Build
    │
    ├─ Load previous state
    ├─ Compare for changes
    └─ Decide incremental/full
```

### State Update Patterns

**GitState Updates**:
```go
// After cloning repository
bs.Git.Commits[repo.Name] = headCommit
bs.Git.CommitDates[repo.Name] = commitTime

// After all repos cloned
bs.Report.ClonedRepositories = len(bs.Git.Commits)
```

**DocsState Updates**:
```go
// After discovery
bs.Docs.Files = discoveredFiles
bs.Docs.IsSingleRepo = (len(repos) == 1)
bs.Docs.FilesByRepository = groupByRepo(discoveredFiles)

// After processing
bs.Report.FilesProcessed = len(bs.Docs.Files)
```

**PipelineState Updates**:
```go
// After config generation
bs.Pipeline.ConfigHash = computeHash(config)

// After each stage
bs.Pipeline.ExecutedStages = append(bs.Pipeline.ExecutedStages, stageName)
```

### Implementation Files
- `internal/hugo/build_state.go` - BuildState definition
- `internal/state/git_state.go` - Git-specific state
- `internal/state/docs_state.go` - Documentation-specific state
- `internal/state/pipeline_state.go` - Pipeline metadata

---

## References

- [Pipeline Flow Diagrams](pipeline-flow.md)
- [Data Flow Diagrams](data-flow.md)
- [State Machine Diagrams](state-machines.md)
- [High-Level System Architecture](high-level-architecture.md)
