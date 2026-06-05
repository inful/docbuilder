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
title: "Migrate `project:` to the new list-form front matter"
weight: 50
---

This guide explains the front-matter format change for the `project:` field and how to migrate existing docs.

## What changed

DocBuilder now treats `project` as a Hugo taxonomy and supports list-valued `project:` in your docs. The previous scalar form (`project: <value>`) is replaced by a list (`project: [<value>]`). A doc with multiple projects (`project: [a, b]`) appears in the sidebar under each value, and Hugo auto-generates listing pages at `/project/a/`, `/project/b/`, and so on.

## Format change

```diff
---
 title: Team Alpha Sync
 categories: [Minutes]
-project: team_alpha
+project: [team_alpha]
 ---
```

```diff
---
 title: Cross-Team Doc
 categories: [Minutes]
-project: team_alpha
+project: [team_alpha, team_beta]
 ---
```

A doc that belongs to one project uses a one-element list. A doc that belongs to multiple projects lists all of them. Hugo's taxonomy machinery indexes every entry in the list, so the listing pages and the sidebar stay in sync.

## Why a list (not a scalar)

Hugo taxonomies are list-valued by design. A scalar `project: foo` in the doc's front matter is not indexed as a taxonomy term by Hugo, so the `/project/foo/` listing page would be empty even though the doc exists. The list form is the canonical way to declare taxonomy membership, and the sidebar builder can also derive its grouping from the same list — one source of truth.

The sidebar builder still accepts a scalar value for backward compatibility (it normalises to a one-element list internally), but the taxonomy listing pages won't render correctly. The migration is the right time to switch to the list form.

## Step 1: Find scalar `project:` fields

A simple search across your content repos:

```bash
grep -rn '^project: ' --include='*.md' .
```

You're looking for lines that have `project:` followed by a single value, not a list. The pattern is unambiguous: a list starts with `[` and a scalar does not.

If you want to be thorough, you can also catch any inline lists and make sure they parse correctly:

```bash
grep -rn '^project: \[' --include='*.md' .   # already in list form
grep -rn '^project: [a-zA-Z]' --include='*.md' .   # scalar form, needs migration
```

## Step 2: Wrap each scalar in a list

For each match, wrap the value in square brackets:

```bash
# Manual, one file at a time
# Before: project: team_alpha
# After:  project: [team_alpha]
```

If you have a small number of files (the typical case is one or two), this is fastest by hand. For a large fleet, a sed one-liner works:

```bash
# Convert `project: <value>` to `project: [<value>]`
# Review the diff before committing.
find . -name '*.md' -print0 | xargs -0 sed -i -E 's/^project: ([^[].*)$/project: [\1]/'
```

The `[^[]` guard skips lines that already start a list (`project: [...]`), so it's safe to run on a mixed-content tree.

## Step 3: Verify

Run a build with `-v` and check the categories menu debug log:

```
DEBUG Categories menu: configured group_by categories=1 group_by=map[minutes:[project]]
```

And visit the new listing pages in the rendered site:

- `/project/team_alpha/` — listing of all docs whose `project:` includes `team_alpha`.
- `/project/team_beta/` — same for `team_beta`.

If a listing page is empty for a value you expected, re-check the doc's front matter — the value must be inside the list, not outside it.

## What does NOT change

- The `group_by` config shape is unchanged. `group_by: minutes: [project]` still works; the only difference is the input value is now a list.
- The sidebar grouping semantics for `project:` are unchanged for the single-value case. A doc with `project: [team_alpha]` lands in the same `Minutes > team_alpha` bucket it always did.
- The Repository fallback (A1) for docs without `project:` is unchanged.
- The synthetic `_uncategorized` bucket is unaffected.

## What is new

- A doc with `project: [a, b]` appears in the sidebar under both `Minutes > a` and `Minutes > b`. The doc link is duplicated, mirroring the natural taxonomy meaning.
- Hugo auto-generates `/project/<value>/` listing pages for every value that appears in any doc's `project:` field.
- The `taxonomies:` block in the generated `hugo.yaml` includes `project: project` by default. Authors who want a different taxonomy setup can override via `hugo.taxonomies` in their config.

## Disabling the `project` taxonomy

If you don't want the listing pages or the taxonomy behavior, override the `taxonomies:` block in your config:

```yaml
hugo:
  taxonomies:
    tag: tags
    category: categories
    # project intentionally omitted
```

The sidebar grouping still works — DocBuilder reads `project:` from front matter regardless of whether `project` is declared as a taxonomy. The override only suppresses the listing pages.

## See also

- [Configuration reference — `group_by`](../reference/configuration.md#per-category-sub-grouping-group_by)
- [Configuration reference — multi-value axes](../reference/configuration.md#multi-value-axes-fan-out)
