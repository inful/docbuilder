# DocBuilder AI Coding Instructions

DocBuilder is a Go CLI tool that aggregates documentation from multiple Git repositories into a single Hugo static site. It supports themes like Hextra and Docsy with intelligent theme-specific configuration.

## Architecture Overview

The application follows a pipeline pattern:
1. **Configuration** (`internal/config/`) - Loads YAML config with environment variable expansion
2. **Workspace** (`internal/workspace/`) - Creates temporary directories for Git operations  
3. **Git Client** (`internal/git/`) - Handles repository cloning/updating with authentication
4. **Discovery** (`internal/docs/`) - Finds markdown files in configured paths within repos
5. **Hugo Generator** (`internal/hugo/`) - Creates Hugo sites with theme-specific optimizations

Key data flow: `Config → Git Clone → Doc Discovery → Hugo Site Generation`

## Development Patterns

### Command Structure
The CLI uses [Kong](https://github.com/alecthomas/kong) for command parsing. Main commands in `cmd/docbuilder/main.go`:
- `build` - Full pipeline execution (clone → discover → generate)
- `init` - Creates example configuration
- `discover` - Discovery-only mode for testing/debugging

Use `go run ./cmd/docbuilder <command> -v` for verbose logging during development.

### Configuration System
- YAML configuration with `${ENV_VAR}` expansion
- Auto-loads `.env` and `.env.local` files
- Repository-specific paths (defaults to `["docs"]`)
- Three auth types: `ssh`, `token`, `basic`

Example repository config:
```yaml
repositories:
  - url: https://github.com/org/repo.git
    name: repo-name
    branch: main
    paths: ["docs", "documentation"]
    auth:
      type: token
      token: "${GITHUB_TOKEN}"
```

### Theme System
Theme logic is implemented via a lightweight interface in `internal/hugo/theme` with concrete packages under `internal/hugo/themes/` (e.g. `hextra`, `docsy`). Each theme registers itself in `init()` and exposes:

```
type Theme interface {
  Name() config.Theme
  Features() ThemeFeatures             // capability flags (modules, math, search json, etc.)
  ApplyParams(ctx ParamContext, params map[string]any)  // inject default/normalized params
  CustomizeRoot(ctx ParamContext, root map[string]any)  // final root-level tweaks (optional)
}
```

Generation phases (`generateHugoConfig`):
1. Core defaults (title, baseURL, markup)
2. `ApplyParams` (theme fills or normalizes `params`)
3. User param deep-merge (config overrides)
4. Dynamic fields (`build_date`)
5. Theme module import block (if `Features().UsesModules`)
6. Automatic menu (if `Features().AutoMainMenu`)
7. `CustomizeRoot` (final adjustments)

Adding a new theme:
1. Create `internal/hugo/themes/<name>/theme_<name>.go`
2. Implement the interface and call `theme.RegisterTheme(Theme{})` in `init()`
3. Populate `ThemeFeatures` (set `UsesModules`, `ModulePath`, defaults)
4. Provide sane defaults in `ApplyParams` (avoid overwriting user-provided keys)
5. (Optional) adjust root in `CustomizeRoot`
6. Add/extend golden config test for `hugo.yaml`

Legacy helper functions (`addHextraParams`, `addDocsyParams`) have been removed; all new logic should go through the theme interface.

### File Discovery
Documentation discovery (`internal/docs/discovery.go`) walks configured paths and:
- Only processes `.md`/`.markdown` files
- Ignores standard files: `README.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `LICENSE.md`
- Preserves directory structure as Hugo sections
- Builds Hugo-compatible paths: `content/{repository}/{section}/{file}.md`

### Authentication Handling
Git client (`internal/git/git.go`) supports multiple auth methods:
- **SSH**: Uses `~/.ssh/id_rsa` by default or specified `key_path`
- **Token**: Username="token", Password=token (GitHub/GitLab pattern)
- **Basic**: Standard username/password auth

Environment variables are commonly used: `${GIT_ACCESS_TOKEN}`, `${GITHUB_TOKEN}`

## Common Development Tasks

### Adding New Hugo Theme Support
1. Update `addThemeParams()` logic in `hugo/generator.go`
2. Add module import pattern if theme supports Hugo Modules
3. Set theme-specific defaults (search, UI, etc.)
4. Test with example configuration

### Testing Changes
```bash
# Test with example config
make build
./bin/docbuilder init -c test-config.yaml
# Edit test-config.yaml with local repos
./bin/docbuilder build -c test-config.yaml -v

# Test discovery only
./bin/docbuilder discover -c test-config.yaml -v
```

### Debugging Git Issues
- Use `incremental` flag to avoid re-cloning during development
- Check authentication with verbose logging: `-v` flag
- Test with both public and private repositories

### Working with Configuration
- Always test environment variable expansion with `.env` files  
- Repository names become Hugo content sections - avoid spaces/special chars
- The `paths` array allows multiple doc directories per repo

## Code Conventions

**See [docs/STYLE_GUIDE.md](../docs/STYLE_GUIDE.md) for complete naming conventions and style rules.**

### Quick Reference

**Variable Naming:**
- Use consistent abbreviations: `repo` (not `repository`), `cfg` (not `config`/`configuration`), `auth` (not `authentication`), `dir` (not `directory`)
- Scope-based naming: single letters for very short scopes (< 10 lines), abbreviated for function parameters, descriptive for struct fields
- Boolean variables: prefix with `is`, `has`, `should`, `can`, `enable`
- Examples: `authCfg *config.AuthConfig`, `repoURL string`, `buildCfg *BuildConfig`

**Function Naming:**
- Public functions: descriptive, full words (e.g., `CloneRepo`, `UpdateRepo`, `ComputeRepoHash`)
- Private functions: can use abbreviations (e.g., `getAuth`, `fetchOrigin`)
- No `Get`/`Set` prefixes for simple accessors; use `Get` when fetching requires computation/I/O
- Predicate functions read like questions: `IsAncestor()`, `HasAuth()`, `ShouldRetry()`

**Type Naming:**
- Use full words without abbreviations: `RemoteHeadCache`, `BuildConfig`, `AuthProvider`
- Error types suffix with `Error`: `AuthError`, `NotFoundError`, `NetworkTimeoutError`
- Interface types prefer `-er` suffix: `Cloner`, `Generator`

**Error Handling:**
- Package-level errors prefix with `Err`: `ErrNotFound`, `ErrUnauthorized`
- Error messages start with lowercase, be specific, include context with `%w`
- Example: `fmt.Errorf("failed to clone repository %s: %w", repo.URL, err)`

**Other Conventions:**
- Use structured logging with `slog` package throughout
- File paths must be absolute when passed between packages
- Hugo paths use forward slashes even on Windows (`filepath.ToSlash()`)
- Error wrapping with `fmt.Errorf("context: %w", err)` pattern
- Configuration validation happens in `config.Load()` with sensible defaults

## Integration Points

**External Dependencies:**
- `github.com/go-git/go-git/v5` for Git operations
- `github.com/alecthomas/kong` for CLI parsing  
- `gopkg.in/yaml.v3` for configuration
- Hugo must be available in PATH for final site building

**File System Layout:**
- Temporary workspaces in `/tmp/docbuilder-{timestamp}/`
- Hugo sites generated in configured output directory
- Repository clones are ephemeral unless using incremental mode