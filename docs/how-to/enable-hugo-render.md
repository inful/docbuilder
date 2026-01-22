---
aliases:
  - /_uid/6549b7d7-c578-4b52-a202-d290d19be13c/
categories:
  - how-to
date: 2025-12-15T00:00:00Z
fingerprint: f1f297d9390decda15571e957020e413644931cb9e41083363cc8ed3f1064104
lastmod: "2026-01-22"
tags:
  - hugo
  - rendering
  - static-sites
title: 'How To: Enable Hugo Rendering'
uid: 6549b7d7-c578-4b52-a202-d290d19be13c
---

# How To: Enable Hugo Rendering

By default DocBuilder scaffolds a Hugo site (content + config) without running the `hugo` binary. Enable automatic rendering to prebuild `public/`.

## Render Mode

Precedence (highest first):

1. `build.render_mode` in config (`never`, `auto`, `always`).
2. `--render-mode` CLI flag, which overrides config for a single run.

## Run With Rendering

```bash
./bin/docbuilder build -c config.yaml --render-mode always
```

Result: `public/` under the output directory plus a `build-report.json` with `static_rendered: true`.

## Verify

Open `public/index.html` in a browser or run a local server:

```bash
(cd site && hugo server)
```

## When Builds Fail

If Hugo execution fails, DocBuilder logs a warning and leaves the scaffold intact. You can inspect and run `hugo` manually.

## CI Pattern

Skip rendering in pull request validation (faster), run rendering only on main branch merges:

```bash
if test "$CI_COMMIT_BRANCH" = "main"; then
  ./bin/docbuilder build -c config.yaml --render-mode always
else
  ./bin/docbuilder build -c config.yaml --render-mode never
fi
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| No `public/` directory | Render mode not set to `always` | Run with `--render-mode always` or set `build.render_mode: always`. |
| Broken asset links | Theme modules not fetched | Ensure network access; rerun. |
| Build warning only | Hugo error surfaced | Read logs; fix Hugo config or content. |
