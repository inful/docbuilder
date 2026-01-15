---
uid: 6549b7d7-c578-4b52-a202-d290d19be13c
aliases:
  - /_uid/6549b7d7-c578-4b52-a202-d290d19be13c/
title: "How To: Enable Hugo Rendering"
date: 2025-12-15
categories:
  - how-to
tags:
  - hugo
  - rendering
  - static-sites
fingerprint: 5dc3aeeac1b07d1b89f4a43c4bd5444caf637d0b1b64855c0573099bb696354e
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
