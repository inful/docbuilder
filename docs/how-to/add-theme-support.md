# How To: Add or Use Theme Support

DocBuilder currently provides optimized configuration for the `hextra` and `docsy` Hugo themes via Hugo Modules.

## Selecting a Theme

```yaml
hugo:
  theme: hextra   # or docsy
```

A `go.mod` is auto-created in the output directory with required module imports.

## Theme Features

### Hextra

- FlexSearch configuration for fast client-side search.
- Math support enabled in Goldmark.
- Edit link logic integrated per page when repository metadata allows.
- Default navbar with search & theme toggle.

### Docsy

- JSON output enabled for offline search index generation.
- Repository links and UI defaults auto-configured.
- Module import based resolution (no legacy `themes/` copy).

## Customizing Params

Edit the generated `hugo.yaml` after a build, or better: provide overrides via configuration fields (planned future expansion) and re-run the build.

## Overriding Index Templates

Place template overrides before running the build:

```text
outputDir/
  templates/
    index/
      main.md.tmpl
      repository.md.tmpl
      section.md.tmpl
```

DocBuilder searches these patterns (first match wins):
1. `templates/index/<kind>.md.tmpl`
2. `templates/index/<kind>.tmpl`
3. `templates/<kind>_index.tmpl`

If none match, an embedded default is used.

## Controlling Front Matter

If a template body starts with a YAML front matter fence (`---`), DocBuilder will NOT inject its own. Otherwise it prepends computed front matter (title, repository, section, forge, dates, edit link, etc.).

## Adding Support for a New Theme (Contributor Flow)

1. Extend theme dispatch in the Hugo generator (look for existing `hextra` / `docsy` param injection).
2. Add module import stanza.
3. Add theme-specific params (search, UI, etc.).
4. Add tests ensuring config merges safely.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Theme assets missing | Hugo not run | Set `DOCBUILDER_RUN_HUGO=1` or run `hugo` manually. |
| Edit links absent | Repo metadata incomplete | Ensure repo URL + branch were configured. |
| Wrong base URL | `hugo.base_url` mismatch | Update config and rebuild. |
