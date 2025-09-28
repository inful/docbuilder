# How To: Customize Index Pages

DocBuilder generates three index page kinds using template resolution with safe defaults. You can override any of them.

## Kinds

| Kind | Location | Purpose |
|------|----------|---------|
| main | `content/_index.md` | Global landing page |
| repository | `content/<repo>/_index.md` | Per repository overview |
| section | `content/<repo>/<section>/_index.md` | Section landing |

## Override Search Order

For `<kind>` = main | repository | section (first match wins):

1. `templates/index/<kind>.md.tmpl`
2. `templates/index/<kind>.tmpl`
3. `templates/<kind>_index.tmpl`

## Example Override

Create `templates/index/main.md.tmpl` before building:

```markdown
# {{ .Site.Title }}

{{ .Site.Description }}

{{ range $name, $files := .Repositories }}
## {{ $name }} ({{ len $files }} files)
{{ end }}
```

## Front Matter Behavior

If your template DOES NOT start with `---` DocBuilder injects generated front matter:

- title, description
- repository, section (where relevant)
- forge (when available/namespaced)
- date (generation timestamp)
- editURL (theme & metadata dependent)

If your template starts with `---` you assume full control; DocBuilder will not add another front matter block.

## Helper Functions

| Function | Description |
|----------|-------------|
| `titleCase` | Capitalize words (simple ASCII) |
| `replaceAll` | Wrapper around `strings.ReplaceAll` |

## Context Keys

| Key | Scope | Description |
|-----|-------|-------------|
| `.Site` | all | `{ Title, Description, BaseURL, Theme }` |
| `.FrontMatter` | all | Map of computed values prior to serialization |
| `.Repositories` | main | Go map `map[string][]DocFile` (repository name → files) |
| `.Files` | all | Slice of relevant DocFile entries |
| `.Sections` | repository | Go map `map[string][]DocFile` (key `root` for unsectioned files) |
| `.SectionName` | section | Name of current section |
| `.Stats` | main/repository | `{ TotalFiles, TotalRepositories }` |
| `.Now` | all | Build timestamp |

## DocFile Fields

`Name`, `Repository`, `Forge`, `Section`, `Path` (Hugo relative path base), plus internal metadata like `RelativePath`.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Default template still used | File path mismatch | Confirm placement matches search order. |
| Duplicate front matter | Custom template began with `---` plus injection | Remove manual fence or keep and remove conflicting keys. |
| Missing edit links | Repo metadata insufficient | Verify repo URL + branch in config. |
