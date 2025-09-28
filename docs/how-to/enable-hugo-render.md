# How To: Enable Hugo Rendering

By default DocBuilder scaffolds a Hugo site (content + config) without running the `hugo` binary. Enable automatic rendering to prebuild `public/`.

## Environment Flags

Precedence (highest first):

1. `DOCBUILDER_SKIP_HUGO=1` — forces skip
2. `DOCBUILDER_RUN_HUGO=1` — forces run
3. neither set — skip (scaffold only)

## Run With Rendering

```bash
DOCBUILDER_RUN_HUGO=1 ./bin/docbuilder build -c config.yaml
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
  DOCBUILDER_RUN_HUGO=1 ./bin/docbuilder build -c config.yaml
else
  ./bin/docbuilder build -c config.yaml
fi
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| No `public/` directory | Env flag not set | Export `DOCBUILDER_RUN_HUGO=1`. |
| Broken asset links | Theme modules not fetched | Ensure network access; rerun. |
| Build warning only | Hugo error surfaced | Read logs; fix Hugo config or content. |
