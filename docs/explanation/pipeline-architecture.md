# Pipeline Architecture

DocBuilder's build pipeline consists of eight sequential stages that transform repository content into Hugo static sites.

## Pipeline Stages

### Stage 1: PrepareOutput

**Purpose:** Initialize output directories and clean previous builds.

**Operations:**
- Create output directory structure
- Remove previous build artifacts if `output.clean: true`
- Set up staging directory for atomic promotion
- Initialize workspace for repository clones

**Result:** Clean workspace ready for new build.

### Stage 2: CloneRepos

**Purpose:** Clone or update Git repositories with authentication.

**Operations:**
- Authenticate using configured method (SSH, token, basic)
- Check incremental build eligibility:
  - Compare HEAD ref with previous build
  - Check documentation content hash
  - Skip if unchanged (when incremental enabled)
- Clone new repositories or update existing ones
- Read current HEAD ref and store in build state
- Emit `RepositoryCloned` or `RepositoryUpdated` events

**Concurrency:** Repositories cloned in parallel (configurable `clone_concurrency`).

**Result:** Local clones of all configured repositories.

### Stage 3: DiscoverDocs

**Purpose:** Find markdown files in configured documentation paths.

**Operations:**
- Walk configured `paths` in each repository
- Identify markdown files (`.md`, `.markdown`)
- Exclude standard files (`README.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `LICENSE.md`)
- Build `DocFile` objects with metadata (path, repository, section)
- Compute content hashes for incremental detection
- Emit `DocumentationDiscovered` event

**Result:** List of `DocFile` objects representing documentation files.

### Stage 4: GenerateConfig

**Purpose:** Create Hugo configuration with theme defaults.

**Operations:**
- Load Hugo config from YAML configuration
- Apply theme-specific defaults (Relearn theme only)
- Deep merge user-provided parameters
- Set up Hugo modules import
- Configure taxonomies (tags, categories)
- Generate sanitized module name from base URL
- Write `hugo.yaml` to output directory
- Create/update `go.mod` for Hugo modules

**Result:** Hugo configuration files ready for site generation.

### Stage 5: Layouts

**Purpose:** Copy layout templates to Hugo site.

**Operations:**
- Copy embedded default layouts (if not overridden)
- Copy user-provided layout overrides
- Set up index page templates (main, repository, section)
- Configure template search order

**Result:** Layout templates available for Hugo rendering.

### Stage 6: CopyContent

**Purpose:** Process and transform markdown files.

**Operations:**
For each `DocFile`:
1. Parse front matter (YAML headers)
2. Normalize index files (README.md → _index.md)
3. Build base front matter with defaults
4. Extract H1 title for index pages
5. Strip H1 heading from content body
6. Rewrite relative markdown links (.md → /)
7. Rewrite image links with repository-relative paths
8. Generate keywords from @keywords directive
9. Add repository metadata (name, forge, branch)
10. Add edit link URL based on forge capabilities
11. Serialize document with final front matter

**Result:** Transformed markdown files in Hugo content directory.

### Stage 7: Indexes

**Purpose:** Generate repository and section index pages.

**Operations:**
- Generate main landing page (`content/_index.md`)
- Generate per-repository index pages (`content/<repo>/_index.md`)
- Generate per-section index pages (`content/<repo>/<section>/_index.md`)
- Use configured templates or embedded defaults
- Populate template context with repository and file metadata
- Apply front matter wrapping (if template doesn't provide it)

**Template Context:**
- `.Site` - Site metadata (title, description, base URL)
- `.Repositories` - Map of repository name to files (main index only)
- `.Files` - Files for current scope
- `.Sections` - Section map for repository index
- `.Stats` - Statistics (total files, total repositories)

**Result:** Complete set of index pages for navigation.

### Stage 8: RunHugo

**Purpose:** Execute Hugo to render static site.

**Operations:**
- Check render mode (`never`, `auto`, `always`)
- Verify Hugo binary availability (if `auto` or `always`)
- Execute Hugo build command
- Capture build output and parse statistics
- Count rendered pages from Hugo output
- Handle build failures gracefully

**Render Modes:**
- `never` - Skip Hugo execution (generate scaffold only)
- `auto` - Run if Hugo binary is available
- `always` - Run Hugo or fail the build

**Result:** Rendered static site in `public/` directory (if Hugo executed).

## Stage Execution

### Sequential Execution

Stages execute in strict order. A stage failure halts the pipeline unless the stage is non-critical.

### Stage Results

Each stage returns one of:
- **Success** - Stage completed without issues
- **Warning** - Stage completed with non-fatal issues
- **Fatal** - Stage failed, halt pipeline
- **Canceled** - Build was canceled

### Stage Retries

Transient failures in CloneRepos stage trigger automatic retries:
- Configurable `max_retries` (default: 2)
- Backoff strategies: fixed, linear, exponential
- Only transient errors are retried (network, timeout)
- Permanent errors fail immediately (auth, not found)

## Error Handling

### Error Classification

Errors are classified by category:
- `CategoryAuth` - Authentication failures
- `CategoryGit` - Git operation failures
- `CategoryHugo` - Hugo rendering failures
- `CategoryValidation` - Configuration validation errors
- `CategoryNotFound` - Repository or file not found
- `CategoryTimeout` - Operation timeout
- `CategoryNetwork` - Network failures

### Error Propagation

Errors bubble up through layers:
1. Infrastructure layer throws classified errors
2. Service layer wraps with context
3. Command layer handles and formats for user

### Build Issues

Non-fatal issues are collected in `BuildReport.Issues`:
- Malformed front matter (recoverable)
- Missing image files (warning)
- Broken internal links (warning)
- Template rendering issues (warning)

## Incremental Builds

### Change Detection

Incremental builds skip unchanged repositories:

**Repository-level:**
- Compare HEAD ref (Git SHA)
- Skip if HEAD unchanged and no forced rebuild

**Content-level:**
- Compute hash of documentation file tree
- Skip if content hash unchanged

**Config-level:**
- Fingerprint configuration (repository list, paths, Hugo config)
- Force rebuild if config changed

### Delta Reasons

Build report includes per-repository delta reasons:
- `unknown` - First build, no previous state
- `quick_hash_diff` - Content hash changed
- `assumed_changed` - Couldn't determine (assume changed)
- `config_changed` - Configuration modified
- `forced` - Forced rebuild requested

### Deletion Detection

Optional deletion detection (`build.detect_deletions: true`):
- Detect when documentation files removed from repository
- Update global documentation hash
- Remove stale files from Hugo content directory

## Observability

### Stage Metrics

Each stage emits metrics:
- `docbuilder_stage_duration_seconds` - Stage execution time
- `docbuilder_stage_results_total` - Stage result counts (success, warning, fatal)
- `docbuilder_build_retries_total` - Retry attempt count

### Stage Events

Event store records stage lifecycle:
- `StageStarted` - Stage began execution
- `StageCompleted` - Stage finished successfully
- `StageFailed` - Stage failed
- `StageWarning` - Stage completed with warnings

### Build Report

Generated after pipeline execution:
- Stage timings and results
- Repository statistics (cloned, failed, skipped)
- Documentation statistics (files discovered, pages rendered)
- Issue list with severity and locations
- Delta reasons for incremental builds

## Related Documentation

- [Architecture Overview](architecture-overview.md) - System architecture
- [Package Architecture](package-architecture.md) - Package structure
- [Build Report Reference](../reference/report.md) - Report format
