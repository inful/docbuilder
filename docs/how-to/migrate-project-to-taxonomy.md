---
aliases:
  - /_uid/project-taxonomy-migration/
categories:
  - how-to
date: 2026-06-05T00:00:00Z
fingerprint: 6a1f3b8d4e9c2a5f8d7b1e4c9f3a6d2e5b8c1f4a7d9e2c5b8f1a4d7e0c3b6f9a
lastmod: "2026-06-05"
tags:
  - migration
  - sidebar
  - taxonomy
title: "Migrate `project:` to the new `projects:` front matter"
weight: 50
---

This guide explains the front-matter format change for the `project:` field and how to migrate existing docs.

## What changed

DocBuilder now treats `project` as a Hugo taxonomy and uses the **plural** `projects:` as the front-matter field name, following the Hugo singular-key / plural-value convention used by `tag: tags` and `category: categories`. The previous scalar form (`project: <value>`) is replaced by a list-form plural (`projects: [<value>]`). A doc with multiple projects (`projects: [a, b]`) appears in the sidebar under each value, and Hugo auto-generates listing pages at `/project/a/`, `/project/b/`, and so on.

## Two changes at once

The format change is actually two changes bundled together for the migration cost:

1. **Field rename**: `project:` → `projects:` (the field name becomes plural to match the Hugo convention).
2. **List form**: scalar `project: team_alpha` → list `projects: [team_alpha]`.

A doc that belongs to one project uses a one-element list. A doc that belongs to multiple projects lists all of them. Hugo's taxonomy machinery indexes every entry in the list, so the listing pages and the sidebar stay in sync.

## Format change

```diff
---
 title: Team Alpha Sync
 categories: [Minutes]
-project: team_alpha
+projects: [team_alpha]
 ---
```

```diff
---
 title: Cross-Team Doc
 categories: [Minutes]
-project: team_alpha
+projects: [team_alpha, team_beta]
 ---
```

A doc that belongs to one project uses a one-element list. A doc that belongs to multiple projects lists all of them.

## Why a list (not a scalar)

Hugo taxonomies are list-valued by design. A scalar `projects: foo` in the doc's front matter is not indexed as a taxonomy term by Hugo, so the `/project/foo/` listing page would be empty even though the doc exists. The list form is the canonical way to declare taxonomy membership, and the sidebar builder can also derive its grouping from the same list — one source of truth.

The sidebar builder still accepts a scalar value for backward compatibility (it normalises to a one-element list internally), but the taxonomy listing pages won't render correctly. The migration is the right time to switch to the list form.

## Why `projects` (plural), not `project` (singular)

Hugo's `taxonomies:` block follows a singular-key / plural-value convention:

```yaml
taxonomies:
  tag: tags          # the front-matter field is `tags:`
  category: categories  # the front-matter field is `categories:`
  project: projects     # the front-matter field is `projects:`
```

The key is a singular noun (the kind of taxonomy). The value is the plural noun that becomes the front-matter field name on your docs and the URL segment for the listing pages (`/projects/<value>/` is the conventional shape, but the project key keeps `/project/<value>/` because we set the value to `projects` not `project/<value>`). Following the convention keeps the docbuilder consistent with the rest of the Hugo ecosystem — every theme and template that inspects `.Site.Taxonomies.projects` "just works" without special-casing.

## Step 1: Find `project:` fields

A simple search across your content repos:

```bash
grep -rn '^project:' --include='*.md' .
```

You're looking for any line that has `project:` (with optional trailing whitespace) followed by either a single value (scalar) or a list. Each match needs migrating.

## Step 2: Rename and wrap in a list

For each match, rename the field to `projects` and wrap the value in square brackets:

```bash
# Manual, one file at a time
# Before: project: team_alpha
# After:  projects: [team_alpha]
```

If you have a small number of files (the typical case is one or two), this is fastest by hand. For a large fleet, a sed one-liner handles both changes in one pass:

```bash
# Convert `project: <value>` to `projects: [<value>]` (single value)
# AND `project: [...]` to `projects: [...]` (already a list)
# Review the diff before committing.
find . -name '*.md' -print0 | xargs -0 sed -i -E 's/^project: ([^[].*)$/projects: [\1]/; s/^project: \[/projects: [/'
```

The first expression handles scalar → list (the `[^[]` guard skips lines that already start a list, so it doesn't double-wrap). The second expression renames the field on lines that are already lists.

## Step 3: Update your `group_by` config

The config axis must match the new field name:

```diff
 hugo:
   sidebar:
     group_by:
-      minutes: [project]
+      minutes: [projects]
```

If you have multiple categories or a deep chain, the rename applies to every occurrence of the old axis name:

```diff
 hugo:
   sidebar:
     group_by:
-      minutes: [project, year]
-      team: [project]
+      minutes: [projects, year]
+      team: [projects]
```

The match is case-insensitive — `Minutes: [Projects]`, `MINUTES: [PROJECTS]`, and `minutes: [projects]` all resolve to the same chain. Pair them with the same casing in your docs (`Projects: [team_alpha]` or `projects: [team_alpha]`) and DocBuilder's case-insensitive lookup handles the rest.

## Step 4: Verify

Run a build with `-v` and check the categories menu debug log:

```
DEBUG Categories menu: configured group_by categories=1 group_by=map[minutes:[projects]]
```

And visit the new listing pages in the rendered site:

- `/project/team_alpha/` — listing of all docs whose `projects:` includes `team_alpha`.
- `/project/team_beta/` — same for `team_beta`.

If a listing page is empty for a value you expected, re-check the doc's front matter — the value must be inside the list under the renamed `projects:` key, not under the old `project:` key.

## What does NOT change

- The Repository fallback (A1) for docs without `projects:` is unchanged.
- The synthetic `_uncategorized` bucket is unaffected.
- The `categories:` taxonomy is unaffected (you don't need to rename that to `categorie` or anything — `categories` is already plural).

## What is new

- A doc with `projects: [a, b]` appears in the sidebar under both `Minutes > a` and `Minutes > b`. The doc link is duplicated, mirroring the natural taxonomy meaning.
- Hugo auto-generates `/project/<value>/` listing pages for every value that appears in any doc's `projects:` field.
- The `taxonomies:` block in the generated `hugo.yaml` includes `project: projects` by default. Authors who want a different taxonomy setup can override via `hugo.taxonomies` in their config.

## Disabling the `project` taxonomy

If you don't want the listing pages or the taxonomy behavior, override the `taxonomies:` block in your config:

```yaml
hugo:
  taxonomies:
    tag: tags
    category: categories
    # project intentionally omitted
```

The sidebar grouping still works — DocBuilder reads `projects:` from front matter regardless of whether `project` is declared as a taxonomy. The override only suppresses the listing pages.

## See also

- [Configuration reference — `group_by`](../reference/configuration.md#per-category-sub-grouping-group_by)
- [Configuration reference — multi-value axes](../reference/configuration.md#multi-value-axes-fan-out)
- [Configuration reference — the `project` taxonomy](../reference/configuration.md#the-project-taxonomy)
