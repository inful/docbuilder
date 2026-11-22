# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once a stable v1.0.0 is tagged. Pre-1.0 the API surface may change between
commits; pin to a SHA for reproducibility per the README.

## [Unreleased]

## [0.28.1] - 2026-11-22

### Fixed
- **A misplaced configuration key said only that it was unknown.** The
  configuration parser rejects keys the schema does not know, so that a
  typo or a key that moved cannot pass unnoticed. The error named the
  key but not where it belongs, which left an operator who had put a
  section at the wrong level with nothing to act on.

  A key that the schema knows somewhere else now says where it is
  accepted: a top-level `outbound` is answered with
  `"outbound" is accepted at "daemon.outbound"`.

  Note that a misplaced key was never applied. The daemon reads the
  outbound integration from `daemon.outbound.ragabast`, so a top-level
  `outbound` block configured nothing at all - it was dropped without
  a word, and the documents never reached ragabast.

## [0.28.0] - 2026-11-22

A repository audit turned up twenty findings. All of them are fixed here.

### Security
- **The MCP `get_config` tool returned the forge webhook secret.** The
  secret was masked along with the other credentials, but the forge
  block carried a second copy of it that was not masked. The forge
  webhook secrets are masked now.

- **A webhook without a verifiable signature was accepted.** The
  handler skipped the signature check when the configuration had no
  secret, when the forge was unknown, or when the client could not be
  reached. All three cases are refused now: a webhook that cannot be
  verified is not a webhook.

### Fixed
- **The daemon never skipped an unchanged build.** The per-repository
  state the skip check reads was never written: the generator recorded
  the commit it built, but nothing persisted it, so the check always
  found nothing to compare against. The state is persisted now, and a
  build whose repositories did not change is skipped.

- **The configuration parser expanded every `$` in a value.** A dollar
  sign in a password or a command was treated as the start of an
  environment variable and removed. `${VAR}` is still expanded; a bare
  `$` is only expanded when the variable is set.

- **A configuration key nobody read was silently ignored.** A typo in
  a key - `hugo.transform` instead of `hugo.transforms`, say - was
  accepted and had no effect. Unknown keys are refused now, with the
  name of the offending key.

- **The example configurations did not load.** The shipped examples
  were missing the `version` field and used keys that no longer
  exist, so a user copying one got a configuration error. The examples
  load now, and a test keeps them that way.

- **The build event log grew without bound.** The events are kept
  forever. They are pruned now, after thirty days by default.

- **The daemon state lived in the repository cache.** The state file,
  the event log and the remote-head cache shared a directory with the
  cloned repositories, so saving the state walked every file of every
  clone. The state has a directory of its own now.

- **The link verifier read every page's front matter on every build.**
  The front matter describes a broken link, so it was read for every
  rendered page although it is needed only when a link is broken. It
  is read when the link breaks now.

- **The site fetched its search library from a CDN.** The theme loads
  lunr from unpkg, so every published page made a request to a third
  party. The library is vendored and served from the site.

- **An unfingerprinted stylesheet was cached for a year.** The server
  gave every CSS and JavaScript file an immutable one-year cache, but
  only the files Hugo fingerprints change name when their content
  changes. The rest are revalidated now.

- **The "active jobs" counter counted the wrong thing.** It counted
  the discovery runs, and every surface that reported it - the
  Prometheus gauge, the status page, the health endpoint - called it
  the number of running builds. It counts the running builds now.

### Changed
- **The lint fixer re-read the tree for every run.** The parsed links
  of a file were thrown away at the start of every run, and the links
  of a file were sorted with a quadratic sort. The cache now outlives
  a run, and the sort is not quadratic.

- **The uid helpers were in two places.** The lint fixer had its own
  copies of the helpers that `frontmatterops` owns, and the alias path
  was built in three packages. They live in one place now.

- **The documentation described the old theme.** The README, the
  example configurations and the CI script still described Relearn,
  and the CI script wrote a configuration that no longer loads. They
  describe Docsy now.

### Removed
- **The build report promised fields nothing wrote.** The report
  carried `delta_decision`, `delta_changed_repos` and
  `delta_repo_reasons`, and the delta analyzer that was supposed to
  fill them was never wired into a build. The fields are gone.

- **The `hugo.transforms` configuration key.** It was documented at
  length and read by nothing. The key is refused now, and the guide
  that described it describes the pipeline that actually runs.

- **The unused transform registry and the delta analyzer.** Both were
  dead code: nothing outside their own tests imported them. Four
  thousand lines go with them.

- **Four exported symbols nothing called.** The no-op build
  observer, two discovery accessors and the configuration version
  probe had no caller.

## [0.27.0] - 2026-11-21

### Changed
- **The theme is Docsy.** The site was built with the Relearn theme; it
  is built with [Docsy](https://github.com/docsy/docsy) now, and there
  is no theme to choose from: `hugo.theme` is gone.

  The navigation is the content tree - the sections and pages of the
  repositories - instead of a sidebar with one block per category. The
  categories are a taxonomy, so their landing pages
  (`/categories/<name>/`) list their documents with their descriptions,
  across every repository the build aggregates. The order of the
  sidebar comes from the `weight` front matter of the section index
  pages.

  What the build does for the theme: it imports it as a Hugo Module,
  writes the layout overrides (the custom CSS, the view transitions and
  the template metadata in the theme's head hook, the stylesheet link,
  and the edit link from the page's own forge), enables the theme's
  offline search, and ships the stylesheet prebuilt - Hugo 0.170 has no
  Sass support of its own, so the compiled stylesheet, the Bootstrap
  JavaScript and the two web fonts it references are vendored in the
  repository.

  What it costs: the per-page edit link works for every forge, but the
  "categories sidebar" - the `hugo.sidebar.*` settings, the
  `group_by` axes and the `_uncategorized` bucket - is gone with the
  theme it belonged to. The `hugo.chrome` setting replaces the shared
  navigation: `full` (the default) renders the navigation on every
  page, `shared` renders it once and restores it in the browser.

### Removed
- **The Relearn theme integration**: the theme parameters, the
  categories sidebar (3,600 lines and its tests), the shared navigation
  fragment with its vendored copy of the theme's menu template (1,000
  lines), and the `hugo.sidebar.*` configuration.

## [0.26.1] - 2026-11-21

### Fixed
- **The Docsy site had no dark mode and no edit link.** Relearn
  offers a light/dark switch and an "Edit this page" link out of the
  box; Docsy leaves both off, and its own edit link only works for
  GitHub.

  The Docsy build now switches the theme variant on, folds the
  sidebar sections the way Relearn does, and renders the edit link
  from the URL DocBuilder computes for the page's forge - so GitLab
  and Gitea pages get one too.

## [0.26.0] - 2026-11-21

### Added
- **A second theme: Docsy**. `hugo.theme: docsy` builds the site with
  the Docsy theme; `relearn` stays the default. The choice is
  experimental until the default flips, after Docsy has been verified
  by use.

  The two themes disagree about how a site is put together, so the
  build adapts in five places: the module import (Docsy lives in the
  `theme` subdirectory of its repository and is its own Go module), the
  shortcodes the generated pages carry (Docsy has no `children` and no
  `badge`, so the permalink is written as plain markdown), the layout
  overrides (Docsy takes the custom CSS, the view transitions and the
  template metadata in its head-end hook), the navigation (Docsy's
  navigation is the content tree; the categories stay a taxonomy, so
  their landing pages exist either way) and the search (Docsy's
  offline index needs no JSON output format).

  Hugo 0.170 has no Sass support of its own, so the four files Docsy
  needs — the compiled stylesheet, the Bootstrap JavaScript and the
  two web fonts the stylesheet references — are vendored in the
  repository. `scripts/vendor-docsy-assets.sh` rebuilds them; the
  build needs neither Sass nor Node.

## [0.25.1] - 2026-11-21

### Security
- **The image carries a Hugo with twelve unpatched advisories**: the
  build pinned Hugo 0.152.2, seventeen releases old. Hugo 0.170.1 is
  the version now. It is built with Go 1.27.4, which carries twelve
  security fixes.

  Hugo 0.170.0 ended the extended edition — one binary with every
  feature but LibSass — so the build takes the standard archive now
  instead of the alias the extended name resolves to.

### Changed
- **The taxonomy pages show a double space in each child's path**.
  Hugo 0.169.0 rewrote `plainify`: text that a tag separated now gets
  a space between the parts. Relearn 9.0.3 prints each child's path
  with `plainify`, so `Documentation > adr` became `Documentation >  adr`
  on the 131 listing pages. The fix belongs in the theme and comes with
  the Relearn 9.5 bump; the rest of the site is unchanged, page for
  page.

## [0.25.0] - 2026-11-19

### Fixed
- **The documentation passes its own lint**: 111 warnings, all of them
  real. Ninety-two pages started with a top-level H1 that duplicated
  the heading the theme renders from the `title` frontmatter; the rule
  that says so has been in the repository all along and the pages did
  not follow it. The H1 is gone from each of them, the fingerprint
  follows the content, and no link pointed at one of the removed
  headings. The pages still render with one H1: the title.

- **A link in an example is not a link in the document**: the
  internal-link-style rule flagged every site-rooted link, also the
  ones inside code blocks and inline code. The how-to guides show what
  a site-rooted link looks like, so the rule flagged its own examples:
  nineteen warnings in three files. The rule now skips fenced code
  blocks and inline code spans.

## [0.24.4] - 2026-11-19

### Security
- **The dependencies govulncheck flagged are updated**:
  `golang.org/x/net` v0.56.0 → v0.62.0 (six advisories),
  `golang.org/x/crypto` v0.53.0 → v0.58.0 (two), and with them
  `golang.org/x/text` v0.43.0, `golang.org/x/sys` v0.49.0,
  `golang.org/x/term` v0.47.0 and `github.com/klauspost/compress`
  v1.20.4. The scan says the daemon calls none of the vulnerable
  code, but the gate runs on the whole dependency set.

  The standard library advisories (net/http, crypto/tls, crypto/x509,
  net/textproto, html/template) are fixed in go1.26.11, so the module
  says so: `go 1.26.0` with `toolchain go1.26.11`. A build with an
  older toolchain cannot ship a vulnerable binary — the go command
  switches to the pinned one.

- **The Go toolchain in the Docker image is patched**: the image
  carried Go 1.24.0, which has those standard library advisories
  open. The toolchain in the image runs Hugo Modules, and the module
  proxy is reached over HTTP. Go 1.26.11 has the fixes.

### Changed
- **The CI runs the image**: it built the image and never started it.
  A broken image — a work directory the daemon cannot write, a home
  directory the Go module cache cannot use — passed the build and
  failed at the first start. The smoke test starts the image and
  checks that the binary runs, that the daemon starts and answers on
  the docs port, and that the toolchain in the image works with a
  writable module cache. It needs no network.

- **The CI lints with the version `mise.toml` asks for** (v2.14.0),
  so the local gate and the one that decides agree.

  The test job still starts from Go 1.24 on the runner: the branch
  protection requires a check with that name, and the toolchain
  directive in `go.mod` decides the version the build actually uses
  (go1.26.11). Renaming the job needs the check renamed in the
  repository settings too.

## [0.24.3] - 2026-11-19

### Changed
- **The Docker image is a sixth smaller**: the runtime stage chowned
  everything after the copy, and a chown writes the ownership of every
  file into a layer of its own. The Go toolchain alone is about ten
  thousand files, so that layer held nothing but metadata — 25 MB of the
  image. `COPY --chown` sets the ownership while the files are copied,
  so the metadata lands in the layer that holds the content.

  Measured on the arm64 image: **495.0 MB → 424.7 MB** uncompressed,
  **149.4 MB → 124.8 MB** gzipped.

  The image also passes `hadolint` now, which it did not before: the
  download stage uses wget for both downloads instead of wget and curl,
  the binary lookup uses `find` with a path pattern so the path can
  stay quoted, and the daemon runs as the numeric user id with the
  home directory the name used to give it (the Go module cache for
  Hugo Modules lives there).

## [0.24.2] - 2026-11-19

### Fixed
- **A mistake in the configuration says what to fix**: every classified
  error was reported as "Internal error occurred (use -v for details)", so
  a user whose configuration the daemon refused had to guess that `-v`
  existed to learn why. A configuration or validation error now shows its
  message: `Error: validation failed: duplicate repository name`.

## [0.24.1] - 2026-11-19

### Removed
- **The build store that only its own test used**: the JSON state store
  had a store for build records — create, read, update, list, delete,
  clean up. Nothing in the daemon ever wrote a build to it: the build
  history comes from the event store, and the admin API reads that. The
  state file kept a `builds` object that was always empty.

### Fixed
- **The lint exclude globs have a test**: the excludes are globs, and a
  wrong glob means the wrong files are skipped.
- **The cache header of the docs server has a test**: the header
  decides what a reader's browser does with a page. A wrong one either
  serves stale content for a year or re-downloads everything on every
  visit.

## [0.24.0] - 2026-11-19

### Security
- **The MCP server only fetches templates from the template host**: the
  template tools took a URL and fetched it. The URL comes from the LLM,
  and the LLM reads whatever it is told to read - including text in a
  document. So the server could be made to fetch any address: the cloud
  metadata service with the credentials in it, the daemon's own admin
  port, anything else on the network. The answer came back in the tool
  result. A template URL now has to be on the host of the configured
  template base URL - the same host that `list_templates` already uses.

### Fixed
- **The NATS reconnect no longer writes the cache bucket without the
  lock**: the reconnect handler rebuilt the bucket and stored it in the
  client without taking the lock, while every reader takes the bucket
  under that lock. A reconnect that landed while a build was reading
  the cache was a race on an interface value, and a torn read of that
  crashes the daemon.

### Changed
- **The `group:` field is gone**: the content path had a group segment
  for the case where two repositories share a name. It never got there:
  the group was only used when two repositories in the build had the
  same name, and the build keys its repositories by name, so one of the
  two was already gone before the group could be used. A configuration
  with two repositories of one name is now refused when it is loaded,
  and discovered repositories that share a name get one of their own,
  so the case the group was built for can no longer happen.
  **Breaking**: the `group:` field does nothing and has been removed.
  Give each repository a name that says where it comes from, e.g.
  `acme-docs` and `initech-docs`.

### Removed
- **The helpers that nobody calls**: eight helpers and one whole file
  had no callers anywhere in the tree - not in the code, not in the
  tests, not in the configuration.

## [0.23.1] - 2026-11-19

### Changed
- **Hugo is only asked for its debug output when it is read**: every
  build ran Hugo with `--logLevel debug`, and the daemon held the
  output in a buffer, split it into lines and dropped every line,
  because they are logged at debug level and the daemon usually logs at
  info level. Hugo is now asked for info output, and for debug output
  only when the daemon logs at debug level.

## [0.23.0] - 2026-11-19

### Security
- **The preview server keeps its admin API to itself**: `docbuilder
  preview` builds its configuration by hand, so the admin address was
  never set and the admin API - which has no password and can start a
  build - listened on every interface. It now uses the same loopback
  address as the daemon. The docs port stays open, so the site can
  still be looked at from another machine.
- **The GitLab webhook secret is compared in constant time**: GitLab
  sends the secret in a header instead of a signature, so the client
  compared it with `==`. That comparison stops at the first byte that
  differs, so the secret could be learned one byte at a time by
  timing the answer. The other two forges already compared in
  constant time.

### Fixed
- **Two repositories with the same name no longer eat each other**:
  the name of a repository is its identity in a build - the clone
  folder, the state map and the content path all use it. The same
  project name in two GitLab subgroups, or the same name on two
  forges, meant that the second clone wiped the first one's folder
  and one repository's documentation vanished from the site while the
  build said it succeeded. Discovered repositories now get a name
  nobody else has.
- **The docs server stops holding every response in memory**: the
  server read the whole file into memory before writing it out, so a
  big file and a few readers at once was a lot of memory for nothing.
  Only a LiveReload request needs its response captured now.

### Changed
- **A hand-written configuration with two repositories of the same name
  is refused**: the configuration is checked when it is loaded, and so
  is a repository name that is empty or that points outside the
  workspace. **Breaking**: a configuration that used two repositories
  with one name now fails to load with a clear message.
- **The lint fixer reads every markdown file once, not once per
  renamed file**: it looked for the links to a renamed file once per
  renamed file, and every look walked the whole tree and read and
  parsed every file in it. A tree with 2,000 pages and 50 files to
  rename was read 102,500 times; it now reads 2,099.

### Removed
- **The state layer that nobody used**: the state package had two ways
  to do the same thing, and the second one was never built. With it
  went the method that was supposed to keep the state file small (it
  was never called, so the file grew for as long as the daemon ran),
  the statistics getter and the service lifecycle interface that only
  the state service implemented, for nobody to run.

## [0.22.0] - 2026-11-15

### Security
- **The admin API is no longer open to the network**: it has no password
  and it can start a build, but it listened on every interface. It now
  binds to `daemon.http.admin_bind`, which defaults to `127.0.0.1`. Reach
  it through an SSH tunnel or a reverse proxy, or set
  `admin_bind: "0.0.0.0"` and protect the port yourself.
- **Webhook payloads are limited to 5 MiB**: the webhook port is the one
  port that has to be reachable from the internet, and it read the whole
  body into memory with no limit. The read timeout limited the time, not
  the size.
- **A forge webhook now needs a secret**: an endpoint without a secret
  accepted any POST and started a build - usually because the config
  pointed at an environment variable that was not set and expanded to
  nothing. The daemon refuses to start when a forge has a webhook path
  but no secret, and the error names the forge and the field to set.
  **Breaking**: set `forges[].webhook.secret`, or remove
  `forges[].webhook.path` to disable the endpoint.

### Fixed
- **Link verification looks in the right folder**: it walked
  `daemon.storage.output_dir` while the daemon writes the site to
  `output.base_directory` + `output.directory`. With a base directory
  set, it found nothing, logged one warning and did nothing at all.
- **Link verification reads every page once**: it read each rendered page
  for the change hash and then read it again to pull the links out.
- **`/api/build/status` and `/api/repositories` answer with real data**:
  they always reported zero builds and an empty repository list. They now
  come from the daemon's state, and `/api/daemon/status` carries the
  last build report with its per-stage timings, as the README promised.
- **The state store no longer offers a transaction that deadlocks**:
  `WithTransaction` took the store lock and then called your function
  with the store, and every store method took the same lock again. Its
  own test was switched off with "FIXME: Deadlock". It had no caller, so
  it is gone.
- **The outbound dispatcher no longer keeps its "close once" flag in a
  global map** that could only grow.

### Changed
- **Discovery does not ask the forge about repositories that did not
  move**: the documentation check costs two API calls per repository and
  the answer only changes when the repository changes. The result is
  now cached per repository and invalidated by its last-updated
  timestamp.
- **Pushing a document to ragabast costs nothing when it did not
  change**: the dispatcher remembers the fingerprint it uploaded and
  skips both the preflight and the upload, so the outbound cost follows
  the number of changes instead of the number of pages. Limit: if
  ragabast loses a document on its own, docbuilder does not notice until
  the document changes or the daemon restarts.
- **`concurrent_builds` is documented honestly**: the daemon runs one
  build at a time - a build owns the staging directory - so the extra
  queue workers wait their turn.
- **The hugo version is detected once per process** instead of three
  times per build, and the detection has a timeout: a stuck `hugo`
  binary used to hang the build with no way out.

### Removed
- **The old index generator is gone** (719 lines plus 534 lines of its
  own tests): the content pipeline has generated the index pages since
  ADR-003, and the old copy was reachable only from its own tests. The
  per-page `TransformedBytes` copy that only it read is gone too.

## [0.21.2] - 2026-11-11

### Fixed
- **The shared navigation keeps its scroll position**: the theme scrolls the
  active entry into view after every page load, so the navigation jumped on
  every navigation. The browser now remembers the entry the reader was
  looking at and puts it back. The position is anchored on the entry, not
  on a pixel offset: the tree opens the path to the current page, so the
  same offset would show a different part of the tree on the next page.
  When the reader jumps to a different part of the tree, the navigation
  follows the current page instead.

## [0.21.1] - 2026-11-10

### Fixed
- **The shared sidebar is no longer re-downloaded on every navigation**: the
  navigation fragment is the same on every page of a build, so the browser
  now keeps it in its local storage, keyed by the build id, and restores it
  instead of fetching it again. Measured on the 1517-page site: the first
  page downloads the 560 KB fragment, every page after that restores it in
  14 ms with no request at all. A rebuild changes the build id and
  invalidates the copy.

## [0.21.0] - 2026-11-10

### Added
- **Shared sidebar mode**, now the **default**: the navigation is rendered
  once into a `/sidebar.html` fragment and injected into each page by a
  small script, instead of being embedded in every page. Relearn renders
  the whole navigation tree into every page, so both the render time and the
  published site grow with the square of the page count. Measured on a
  1517-page site: render step **3m42s → 4.0s**, published site
  **868 MB → 31 MB**. The navigation content is unchanged and the current
  entry is marked in the browser. The trade-off: without JavaScript the pages
  show no sidebar — set `hugo.sidebar.mode: categories` to keep the previous
  behaviour (the navigation embedded in every page, server-rendered), or
  `default` for Relearn's legacy single-page menu.

### Changed
- **Kubernetes example resources**: the example deployment limits the
  container to 1 CPU. The Hugo render is CPU-bound and parallel (~2.7 cores
  for ~1500 pages), so that quota stretched a 3-minute render to ~9 minutes.
  The example now requests 500m and allows 4000m CPU / 2Gi memory.

### Fixed
- **Embedded assets are now content-addressed**: the shared navigation
  script and the view-transitions stylesheet were published under stable
  URLs, but the docs server serves `.js` and `.css` with a one-year
  `immutable` cache header - safe only for content-hashed URLs. A browser
  that had cached them kept the old file after a DocBuilder upgrade. Both
  are now published through Hugo's resource pipeline, so their URLs carry a
  content hash.
- **Shared sidebar was not scrollable**: the theme binds to the sidebar's
  contents on `DOMContentLoaded`, which happens before the shared fragment
  arrives, so its menu scrollbar was never mounted and the navigation could
  not be scrolled. The shared navigation script now re-runs the theme's
  sidebar initialisation after injecting the fragment.
- **Hugo log lines are no longer reported as warnings**: Hugo writes its
  progress output — including the per-step build timings — to stderr, and
  every one of those lines was logged at `WARN`. Each line is now logged at
  the level Hugo reported it at.
- **Docker image tags**: the release workflow publishes the version
  **without** a `v` prefix (`0.20.0`, `0.20`, `0`) plus `latest`, but the
  documentation showed `v1.2.3` / `v1.2` / `v1` — tags that were never
  published, so every documented `docker pull` command 404'd. The docs now
  use the real tag names, and the workflow additionally publishes a
  `v`-prefixed alias (`v0.20.0`) so both spellings resolve.

## [0.20.0] - 2026-11-10

### Changed
- **Link verification is now opt-in**: `daemon.link_verification.enabled`
  defaults to `false`, so a daemon no longer checks links (and no longer
  connects to NATS) unless the config explicitly sets `enabled: true`.
  Previously the defaults pass forced the flag back to `true`, which also
  made the documented `enabled: false` opt-out impossible.
- **Daemon internal architecture**: `internal/daemon.Daemon` is now a
  thin façade over six per-concern sub-structs (`lifecycleStack`,
  `stateStack`, `buildStack`, `discoveryStack`, `serveStack`,
  `optionalStack`) instead of a 34-field flat struct. The
  `NewDaemonWithConfigFile` constructor is now a 35-line recipe
  over twelve per-concern `init*` methods (previously 191 lines,
  cyclomatic complexity 22, 14 returns). No public API change;
  the single external caller
  (`cmd/docbuilder/commands/daemon.go`) is unchanged. See
  [ADR-023](docs/adr/adr-023-daemon-decomposition.md).
- **Daemon test seam**: introduced `daemonForTest(t, opts)` helper
  in `internal/daemon/daemon_test_helpers.go` to replace the
  hand-rolled `&Daemon{lifecycle: ..., serve: ..., ...}`
  literals that the build-service-unification F5 review flagged.
  The helper centralises the seams a test wires up and defaults
  unused sub-structs to non-nil empties, so the Daemon's
  sub-struct shape is the single source of truth for the test
  surface.
- **Build pipeline unification**: All build entry points (CLI full,
  CLI local `-d`, daemon, preview) now route through a single
  `build.BuildService` interface. Previously, the CLI constructed
  `hugo.NewGenerator` directly and the daemon went through an
  indirection adapter; both paths now use the canonical service.
- **CLI gains skip-evaluation**: `docbuilder build` now honors
  `build.skip_if_unchanged` the same way the daemon does (via
  `BuildService.Run`). Previously the daemon alone benefited
  from the cached-report skip path; the CLI rebuilt every time.
  Operators who explicitly want a full rebuild every run should
  set `build.skip_if_unchanged: false` in their config.
- **Build pipeline internals**: dropped the `HugoGeneratorFactory` and
  `SkipEvaluatorFactory` injection hooks on `DefaultBuildService`,
  the `daemon.SkipEvaluator` wrapper, the `daemon.BuildServiceAdapter`,
  and the daemon's `build_queue_aliases` / `discovery_aliases` /
  `delta_compat` re-export shims. `BuildService` now imports
  `internal/hugo` directly (no cycle exists) and constructs the
  generator itself. `BuildRequest` gains `SkipState` and
  `KeepWorkspace` fields, and `DirectBuildRequest` gains
  `KeepStaging`, to carry the options those shims used to inject.

### Added
- **Placeholder page while the first site is still building**: while no
  rendered site exists, the docs server now serves the "Documentation is
  being prepared" page for every documentation URL instead of only for
  `/`. Previously every other URL fell through to the file server and
  404'd against the Hugo scaffold. The page shows the daemon status and
  the build queue depth, so it reads as "working on it" rather than a
  dead end. Asset-looking paths still 404 instead of returning HTML.
- **BuildService.RunDirect**: a new entry point on `BuildService`
  for callers that have already-discovered doc files (preview
  mode, `docbuilder build -d`, any future embedder). It skips the
  clone and discovery stages.
- **Git corruption self-heal**: `Client.UpdateRepo` now detects
  corrupt on-disk git state (truncated pack, bad `.idx` checksum,
  `plumbing.ErrObjectNotFound` from an unreadable pack index, etc.)
  and transparently wipes the workspace and re-clones from the
  remote. Previously a single interrupted fetch or a flaky
  filesystem could leave the workspace in a state where every
  subsequent `docbuilder build` surfaced
  `[git:error] failed to checkout existing branch: invalid checksum`
  to the user. The recovery is logged at WARN with the original
  error and the path, then the build proceeds normally. Sourced
  from go-git's pack/idx/index decoder error strings, so future
  go-git versions that add new corruption signatures will need
  `gitCorruptionPatterns` in `internal/git/recovery.go` extended
  to match.

### Fixed
- **Daemon readiness no longer depends on a rendered site**: `GET /ready`
  (and `/readyz`) now report whether the daemon is running and able to
  serve. They previously returned 503 until `<output>/public` existed,
  which meant a daemon whose first build had not finished (or could not
  finish — an unreachable forge, a failing clone) never became ready. As
  a Kubernetes startup probe that is a restart loop: the container is
  killed, the build restarts, it fails again, forever. The render-state
  check is still available under the new `GET /site-ready` path for
  deployments that want to hold traffic until the site is built.
- **A failed initial discovery is retried instead of waiting for the
  next cron tick**: with a `forges:` configuration the first build only
  happens after a successful discovery, and a failed discovery left the
  daemon without a build until the next sync tick (default: every 4
  hours). The daemon now retries the initial discovery with a backoff
  ladder (30s, 1m, 2m, then every 5m) until it succeeds. Forge-level
  failures count as failures here: they are reported inside the
  discovery result (`result.Errors`) rather than as an error return, so
  an unreachable forge previously looked like a successful discovery
  that found zero repositories.
- **Daemon state load no longer bricks startup**: an unreadable
  `daemon-state.json` (corrupt JSON, or a `format_version` this build
  does not understand) previously aborted daemon startup with
  `failed to load daemon state` (category `internal`). The file is now
  moved aside to `daemon-state.json.corrupt-<timestamp>` and the daemon
  continues with empty state, logging the cause and the quarantine path
  at WARN. The original file is preserved, so no state data is lost.
  Trade-off: a file written by a *newer* binary is quarantined rather
  than refused, so running an older binary against a newer state file
  resets the live state (the newer file stays recoverable in the
  `.corrupt-*` copy).

## [0.19.2] - 2026-09-30

### Changed
- **Build (gitignore)**: excluded local goreleaser build artifacts.
  Added `/-/` (the literal root-level scratch dir local goreleaser runs
  write to when the output dir is overridden — this operator's setup)
  and `*.tar.gz` (goreleaser archive pattern — verified no `.tar.gz`
  files exist anywhere else in the repo, so the broad pattern is
  safe). Without these, every local release build regenerated ~100 MB
  of `.tar.gz` files in `git status` as untracked-but-real entries.

## [0.19.1] - 2026-09-30

### Added
- **Ragabast ingester (config)**: new `ragabast_base_url` field for the
  ragabast host. The dispatcher derives both endpoints from this single
  value: `{base}/api/ingest/file` for the upload and
  `{base}/api/documents/<uid>/fingerprint` for preflight. Path
  components on the field are stripped, so the "POST to / because the
  operator forgot the path" foot-gun is eliminated by construction.
  The endpoint paths are now constants in the dispatcher (matching
  ragabast's stable API contract) rather than something the operator
  has to spell out.

### Deprecated
- **Ragabast ingester (config)**: `ingest_url` (the v0.19.0 form) is
  deprecated. When set without `ragabast_base_url`, the dispatcher
  derives scheme+host from it (path is discarded), logs a deprecation
  warning, and proceeds — so existing configs keep working. Migrate by
  replacing `ingest_url: https://ragabast.example.com/api/ingest/file`
  with `ragabast_base_url: https://ragabast.example.com`.

### Fixed
- **Ragabast ingester (config validation)**: an unparseable
  `ingest_url` (or unparseable `ragabast_base_url`) now fails the
  dispatcher construction with an explicit error rather than
  silently disabling preflight. The previous "garbage URL" path was
  the only test case for that, but the new behavior makes
  misconfigurations visible at startup instead of after the dispatcher
  has been silently running with no preflight base.

## [0.19.0] - 2026-09-30

### Fixed
- **Hugo (auto-generated section `_index.md`)**: stopped emitting today's
  date. The `generateSectionIndex` pipeline stage and the
  `buildBaseFrontMatter` transform were both falling back to `time.Now()`
  when an auto-generated section `_index.md` had no `date` in its
  FrontMatter. For local builds (no git history, so
  `doc.CommitDate.IsZero()`), that meant the build time leaked into
  the rendered site — sitemap `<lastmod>`, RSS `<pubDate>`, OG meta
  tags (`datePublished`, `dateModified`, `article:published_time`,
  `article:modified_time`), and the visible `dateModified` all
  showed today. Replaced both `time.Now()` fallbacks with the
  deterministic epoch `2024-01-01T00:00:00Z` (date) /
  `2024-01-01` (lastmod) — matching what `generateMainIndex` and the
  legacy `internal/hugo/indexes.go` were already doing. Authored
  docs are untouched (their `date` / `lastmod` are preserved
  verbatim). For docs that do have a `lastmod`, Hugo's `.Lastmod`
  template variable continues to drive `dateModified`, sitemap
  `lastmod`, and `article:modified_time` automatically — authors
  who want latest-change-date semantics in the rendered output can
  switch their template from `.Date` to `.Lastmod` without any
  docbuilder changes.

### Added
- **Ragabast ingester (preflight fingerprint check)**: when
  `daemon.outbound.ragabast` is configured, the dispatcher consults
  `GET <base>/api/documents/<uid>/fingerprint` before POSTing each
  document and skips the upload when the stored fingerprint matches
  the document's frontmatter. Saves the upload on rebuilds where
  content hasn't changed. Failure policy is fail-open: 5xx, network
  errors, and parse failures fall through to the upload
  (`preflight_errors_total++`); 401/403 are counted as `fails_total`
  without uploading; 404 (new doc) falls through. The fingerprint
  and uid are parsed from the document's YAML frontmatter via
  `internal/frontmatter` (never recomputed). Preflight is on by
  default when a base URL can be derived from `ingest_url`; set
  `preflight_enabled: false` to opt out, or `preflight_base_url:
  <host>` to use a different host for preflight than for uploads.

### Changed
- **Ragabast ingester (multipart upload migration)**: switched the
  upload path from `POST /api/ingest/async` (JSON `{"content":"..."}`)
  to `POST /api/ingest/file` (multipart/form-data with a `file`
  field). The old endpoint returns 405 to POSTs and is no longer
  supported upstream; ragabast's example script uses the new one.
  The upload path now uses `mime/multipart.Writer` with
  `CreateFormFile("file", path)`. `Content-Type` is set from
  `FormDataContentType()` so the boundary is correct; `application/json`
  is no longer set. Go's multipart parser strips directory components
  from the filename, so the receiver sees just the basename
  (`index.md` rather than `docs/index.md`) — same as `curl -F`
  behaves. Auth via `Authorization: Bearer <token>` is unchanged.
  **Operators must update `daemon.outbound.ragabast.ingest_url` from
  `…/api/ingest/async` to `…/api/ingest/file`** — `config.example.yaml`
  has been updated.

## [0.18.0] - 2026-09-28

### Changed
- **Lint (internal-link-style rule, scoped down)**: dropped the
  missing-`.md` half of `internal-link-style` and its auto-fix. The
  rule used to flag any relative link without `.md` extension as
  WARNING and offered "Append `.md`" as the fix, but that contradicted
  the README's documented `./name/` directory-link pattern (Hugo
  resolves `[text](./api/)` to `api/_index.md`) and the auto-fix
  actively rewrote `[API](./api)` → `[API](./api.md)`, breaking valid
  links to `api/_index.md`. Non-existence is now caught by the
  `broken-links` rule, which already does the right `.md` fallback
  (and also handles directory targets via `os.Stat`). The rule still
  fires for site-rooted links (starting with `/`), which is the only
  remaining cross-renderer portability concern. Removed the
  `applyInternalLinkStyleFixes` function, `appendMarkdownExtension`,
  `LinkStyleUpdate` type, `FixResult.LinkStyleUpdates` field, and the
  wiring in `fixer.go`. The `internal-link-style` entry was removed
  from the MCP `autoFixableRules` map and the closed-set test in
  `lint_fix_manual_test.go`. The rule description in
  `docbuilder-best-practices.md` was updated. Closes #76.

### Removed
- **Lint (missing-index-page rule)**: dropped the WARNING that flagged
  directories with ≥3 `.md` children and no `_index.md`, plus its
  auto-fixer that generated a placeholder landing page. The rule
  produced more lint noise than value: the auto-generated placeholder
  itself failed `frontmatter-required-fields` and surfaced as an ERROR
  in the `Detect Lint Rule Drift` workflow, requiring a follow-up fix
  in 0.17.1. The placeholder was also opinion-driven (curated landing
  page vs. Hugo's auto-generated listing). `DocBuilder` continues to
  generate Hugo `_index.md` files at the site, repository, and section
  levels via the build pipeline; authors who want curated section
  indexes can add them by hand. Removed the `DirectoryRule` interface
  and `runDirectoryRules` helper from the linter, the
  `MissingIndexPages` field from `FixResult`, the `itoa` helper was
  moved into `rule_tag_count.go` (its remaining caller), and the rule
  entries in `tools.go`, `lint_fix_manual_test.go`, and four docs
  (`lint-rules.md`, `lint-rules-changelog.md`, `migrate-to-linting.md`,
  `docbuilder-best-practices.md`) were updated. Closes #70.

## [0.17.1] - 2026-09-27

### Fixed
- **Hugo (auto-generated _index.md)**: the `generateMainIndex` function in
  the pipeline now pre-populates the root `_index.md` Document with all
  `frontmatter-required-fields` rule fields (`date`, `lastmod`,
  `categories`, `tags`, `uid`). Previously the downstream
  `transformFrontmatter` only filled `date`; the generated page failed
  the new lint rule and the `Detect Lint Rule Drift` workflow flagged it
  as ERROR. `date` and `lastmod` are pinned to a deterministic epoch
  (matching the manual index path) so generated index frontmatter is
  stable across rebuilds.

### Changed
- **Lint (idiomatic cleanups)**: replaced manual loops with
  `slices.Contains` and `strings.Cut`, dropped `+=` string concatenation
  for `strings.Builder` in three test helpers, removed unused fields
  (`FilenameRule.cfg`, `FixResult.uidTargetsAdded`) and the unused
  `issueCounts` parameter on `applyMissingIndexPageFixes`, extracted
  `runDirectoryRules` from `LintPath` to keep branch complexity under
  the `nestif` threshold, and renamed the local `min` shadowing the
  builtin in `MissingIndexPageRule` to `threshold`. No rule semantics
  changed.

### Tests
- **Lint (golden fixtures + fixture swap)**: `TestIntegration_RenameWithLinkUpdates`
  was vacuous against the default excludes — its `docs/README.md`
  fixture was being skipped from the fix pass. Renamed it to
  `docs/introduction.md` so the integration actually exercises the
  rename-update path. `TestFixer_RenameFile` assertion updated
  (`ErrorsFixed: 3 → 4`) to count the new `frontmatter-required-fields`
  fixer pass. Golden files (`fix-dry-run.golden.txt`,
  `fix-with-links.golden.json`) updated to reflect the new error count
  and shifted link-update line numbers from the auto-generated
  `_index.md` frontmatter block.

## [0.x] — Recent themes

The last 90 days focused on four themes:

### Hugo sidebar: categories become the new default
- `feat(hugo)`: make categories sidebar the new default with `_uncategorized` triage bucket.
- `feat(hugo)`: add categories-mode sidebar wiring.
- `feat(hugo)`: per-category sub-grouping via `hugo.sidebar.group_by`.
- `feat(hugo)`: support N-level max-depth nesting in `hugo.sidebar.group_by`.
- `feat(hugo)`: extend `sidebar.group_by` with sibling-category and multi-value axes.
- `fix(hugo)`: render category titles via top-level wrapper entry.
- `fix(hugo)`: align sidebar identifier with Hugo menu key for Relearn lookup.
- `fix(hugo)`: load doc content on demand in the categories-menu stage.
- `fix(hugo)`: filter categories sidebar by `daemon.content.public_only`.
- `fix(hugo)`: group categories case-insensitively, keep first-seen spelling.
- `fix(hugo)`: make `group_by` case-insensitive on config keys and field names.
- `fix(hugo)`: rename project taxonomy to plural `projects` (Hugo convention).
- `fix(hugo)`: preserve `$...$` inline math through Goldmark passthrough.

### Doctemplate TUI (template generation)
- `feat(doctemplate)`: add Bubble Tea TUI for template generation.
- `feat(doctemplate)`: use preview taxonomy API for tag/category suggestions.
- `feat(doctemplate)`: left/right arrows cycle suggestions for `string_list` fields.
- `feat(templates)`: add `GlobSuggestion` field to `SchemaField`.
- `fix(tui)`: only suggest directories for glob patterns ending with `*/`.
- `fix(templates)`: support metadata headers without transitions.

### MCP server for LLM integration (v0.14.0–v0.14.3)
- **New `docbuilder-mcp` binary** in `cmd/mcp-server` speaking the
  [Model Context Protocol](https://modelcontextprotocol.io/) over stdio.
  Surfaces 10 tools — 6 read-only (`get_config`, `list_templates`,
  `describe_template`, `resolve_template_inputs`, `lint_docs`,
  `read_doc`) and 4 mutating (`create_from_template`, `lint_fix`,
  `create_doc`, `update_doc`) — plus a `config://current` resource.
  Mutating tools advertise `destructiveHint` and require `confirm=true`
  so hosts gate them behind user prompts. Auth tokens, passwords, and
  key paths are redacted as `***` in every response. Write tools are
  path-contained to the configured `--docs-dir` (read & lint tools
  accept any path the caller provides, since lint is a general tool).
  Built on `github.com/mark3labs/mcp-go`.
- **Patch releases in this series:**
  - v0.14.0 — initial MCP server release.
  - v0.14.1 — added `docbuilder-mcp` to GoReleaser config so the
    release tarball ships it.
  - v0.14.2 — added a workflow + safety + pitfalls instructions
    string sent in the `initialize` response so LLMs learn the
    recommended tool order at session start.
  - v0.14.3 — security bump: `go-git` v5.19.1 → v5.19.2, clearing two
    Dependabot advisories (malicious reference names; worktree
    symlink follow).

### Daemon & webhooks
- `fix(daemon)`: recheck filtered repos on webhook (#61).
- `feat`: implement asset size limits and forge namespace stability.

### Build pipeline hygiene
- `refactor(hugo)`: centralize content-tree path construction in `docs.HugoContentPath`.
- `refactor(concurrency)`: use `WaitGroup.Go` in async paths.
- `feat`: add doctemplate build configuration.
- `fix(hugo)`: lowercase repository names in index paths.
- `fix(docs)`: only include GitLab group in content path when needed.

## [0.17.0] - 2026-09-27

### Added
- **Lint (cross-forge excludes)**: new `lint.excludes` config block with a
  cross-forge union of git-forge-conventional filenames (GitHub / GitLab /
  Forgejo — `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`,
  `LICENSE`, etc.). The walk skips these files from schema checks and the
  `filename-conventions` rename suggestion. Closes #77.
- **Lint (new rules)**:
  - `body-h1` (WARNING) — flags `# Title` at the start of a body, which
    duplicates the rendered H1 from frontmatter `title`. Closes #68.
  - `missing-index-page` (WARNING) — flags directories with ≥3 `.md`
    children and no `_index.md`. Closes #70.
  - `sequence-prefix-filename` (ERROR) — flags files in sequence-numbered
    directories (`adr/` by default) that don't match `prefix-NNN-slug.md`.
    Closes #73.
  - `tag-count` (WARNING) — flags `tags:` lists with more than 10 entries.
    Closes #75.
  - `internal-link-style` (WARNING) — flags `[text](relative/path)` without
    `.md` extension or site-rooted (`/foo`) targets. Closes #76.
  - `category-naming` (ERROR) — flags categories that don't match
    `^[a-z][a-z0-9-]*$` (kebab-case). Closes #71.
  - `cross-mode-category` (ERROR) — flags docs whose `categories:` lists
    two distinct doc modes without a configured parent/child relation.
    Closes #74.
  - `directory-category-consistency` (ERROR) — flags docs in mode-specific
    directories (`how-to/`, `reference/`, etc.) whose `categories:` is
    missing the expected mode value. Closes #69.
  - `frontmatter-required-fields` (ERROR for missing/empty, WARNING for
    malformed dates) — verifies `title`, `date`, `lastmod`,
    `categories`, `tags` per the doc-builder contract. Closes #72.
- **Lint (auto-fixers)**: new `lint_fix` phases for the auto-fixable
  rules (`frontmatter-required-fields`, `category-naming`,
  `directory-category-consistency`, `internal-link-style`,
  `sequence-prefix-filename`, `missing-index-page`). The fixer's
  audit-test path (`lint_docs` → `lint_fix` → `lint_docs`) now reaches
  zero ERRORs without manual intervention for the auto-fixable subset.
- **MCP (lint-fix hints)**: server `instructions` and the `lint_docs`
  tool description now mention that each issue carries a `fix` field
  with a concrete remediation hint. `lint_fix`'s response includes a
  `manual_required` array listing issues that couldn't be auto-fixed
  (typically `body-h1`, `tag-count`, `cross-mode-category`) so the LLM
  can surface them with their fix hints verbatim.
- **Daemon (ragabast integration)**: opt-in outbound dispatcher that pushes
  each generated document to ragabast's `/api/ingest/async` endpoint
  immediately after it has been written to disk. Daemon-mode-only and
  off by default; operators enable it via `daemon.outbound.ragabast` in
  config. The bearer token is read from `RAGABAST_INGEST_TOKEN` at
  daemon startup; the dispatcher is bounded, drops on overflow, and
  honours ragabast's persistent job queue + `uid`/`fingerprint` dedup
  for self-healing delivery.
- **Tests**: direct unit tests for `nodeFromAny` (`internal/frontmatter/serialize.go`)
  covering all scalar, sequence, and map branches.
- **Tests**: edge-case coverage for `FromMap` (`internal/hugo/models/frontmatter.go`)
  including wrong-type coercion, invalid date strings, and mixed-type taxonomy arrays.
- **Tests**: focused unit tests for `inferRenameMappingFromGitHead`
  (`internal/lint/broken_link_healer.go`) covering all 17 branches without requiring
  a git repository.
- **Tests**: direct unit tests for `repoAbsPath`
  (`internal/lint/git_uncommitted_rename_detector.go`) including traversal
  rejection.

### Changed
- **Lint (issue iteration)**: the fixer's issue-collection loop now keeps
  both `SeverityError` and `SeverityWarning` so the new warning-level
  rules participate in target building.
- **Lint (filename matcher)**: the cross-forge `excludes` list supports
  recursive globs (`**`) for nested template directories
  (`.github/ISSUE_TEMPLATE/**/*.md`, `.gitlab/issue_templates/*.md`,
  `.gitea/ISSUE_TEMPLATE/**/*.md`).
- **Refactor (hugo/docs)**: extracted the duplicated `normalizeTaxonomyValues` /
  `normalizeSnippetTaxonomyValues` into a single shared `docs.NormalizeTaxonomyValues`.
  Both `internal/doctemplate/app/taxonomy_client.go` and
  `internal/docs/taxonomy_snippets.go` now call the shared helper.
- **Refactor (forge)**: broke a call-graph cycle between
  `internal/forge/enhanced_mock.go` and `internal/forge/enhanced_mock_factory.go`
  by moving the `EnhancedMockBuilder` type and the `CreateRealistic*Mock`
  constructors into `enhanced_mock.go` (where `EnhancedMockForgeClient` lives).
- **Refactor (stages)**: broke a call-graph cycle between
  `internal/hugo/stages/repo_fetcher.go` and `internal/hugo/stages/stage_clone.go`
  by removing the deprecated `readRepoHead` wrapper. Callers now use
  `gitpkg.ReadRepoHead` directly.

### Removed
- Six stale coverage artefacts at the repo root (`coverage.out`,
  `coverage_cmd.out`, `coverage_final.out`, `coverage_new.out`,
  `coverage_review.out`, `coverage.html`). They were never tracked by git;
  `.gitignore` already excluded them. This file (`CHANGELOG.md`) is now the
  source of truth for release notes.
- Dead code surfaced by the `analyze kind=dead_code` graph pass (each item
  verified to have zero in-tree callers, including tests):
  - `internal/hugo/models/typed_transformers.go` (entire file) — four unused
    `V2`/`V3` transformers (`FrontMatterParserV2`, `FrontMatterBuilderV3`,
    `EditLinkInjectorV3`, `ContentProcessorV2`) and their tests.
  - `internal/hugo/stages/stage_execution.go` (entire file) —
    `StageExecution` type and outcome helpers (`ExecutionSuccess*` /
    `ExecutionFailure*`).
  - `internal/hugo/models/early_skip.go` (entire file) — `EarlySkipDecision`
    type and `EvaluateEarlySkip`, `NoSkip`, `SkipAfter` (the runner already
    inlines the early-skip decision at line 59 of
    `internal/hugo/stages/runner.go`).
  - Per-forge webhook handlers `HandleGitHubWebhook`, `HandleGitLabWebhook`,
    `HandleForgejoWebhook` in `internal/server/handlers/webhook.go` — the
    dispatcher `HandleForgeWebhook` (wired by config) and the generic
    `HandleWebhook` (wired at `/webhook`) remain the single entry points.
  - `foundation.Option.Match` / `UnwrapOrElse`,
    `foundation.Result.ToTuple`, `ContentPage.GetOriginalFrontMatter` /
    `SetOriginalFrontMatter` / `AddTransformationRecord` /
    `GetTransformationHistory` / `HasBeenTransformed`,
    `TransformationPipeline.SetContext`, `TransformationResult.SetSource`,
    `Pipeline.AddIf`, `Statistics.UpdateDiscoveryStats`,
    `Manager.WithAutoSave`, `Service.GetScheduleStore` /
    `GetDaemonInfoStore`, `Resolve.WithObserver`,
    `Generator.WithObserver`, `GitState.SetCommitDate`, `Report.GetDocBuilderVersion`,
    `MigrationHelper.ConvertLegacyFrontMatter`,
    `EditorLinkResolver.NewResolverWithChain`, `CommitDetector.Clear`, and
    the testutil fluent helpers `WithTitle` / `WithTheme` / `WithOutputDir` /
    `WithEnv` / `AssertFailure`.

> **Note (retroactive)**: this section was amended after the v0.17.0 tag
> was published. The ragabast dispatcher, taxonomy/forge/stages refactors,
> dead-code removal, coverage-artefact cleanup, and additional test
> coverage all shipped in v0.17.0 but were originally listed under
> `[Unreleased]`. The release-notes commit (`965105c`) that closed out
> v0.17.0 didn't roll those entries forward. This entry corrects the
> record; the binaries are unchanged.

[0.18.0]: https://github.com/inful/docbuilder/compare/v0.17.2...v0.18.0
[0.17.1]: https://github.com/inful/docbuilder/compare/v0.17.0...v0.17.1
[0.17.0]: https://github.com/inful/docbuilder/compare/v0.16.1...v0.17.0
[Unreleased]: https://github.com/inful/docbuilder/compare/v0.18.0...HEAD
