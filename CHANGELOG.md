# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once a stable v1.0.0 is tagged. Pre-1.0 the API surface may change between
commits; pin to a SHA for reproducibility per the README.

## [Unreleased]

### Fixed
- **The reingestion command reads the environment variables.** Every
  command that reads the configuration loads the `.env` files first,
  and the reingestion command did not: the `${VAR}` references of the
  configuration were filled from the environment of the process alone.
  The command also insists on a configuration that the daemon itself
  would accept, and it does not need one: a configuration that cannot
  be read falls back to the default address of the admin API, and
  `DOCBUILDER_ADMIN_URL` (or `--admin-url`) names a daemon anywhere.

## [0.35.35] - 2026-11-26

### Added
- **A command for the reingestion into ragabast.** `docbuilder
  reingest` asks the daemon that runs the site to send the whole corpus
  to ragabast again. The address of the daemon comes from the
  configuration (the port and the bind address of `daemon.http`), and
  `--admin-url` names a daemon on another host. The command asks the
  daemon and not ragabast: the daemon that runs the site is the one
  that holds the corpus.

## [0.35.34] - 2026-11-26

### Added
- **The corpus can be sent to ragabast again on request.** `POST
  /api/ragabast/reingest` on the admin port moves the epoch of the
  ingest and asks for a build: every document then carries a
  fingerprint that ragabast has not seen, and ragabast embeds the whole
  corpus again. That is what an operator needs when the embedder of
  ragabast changed, or when ragabast lost its corpus - ragabast skips
  a document whose fingerprint did not change, and the daemon skips
  the document it uploaded before. The epoch lives in the state
  directory and survives a restart of the daemon.

## [0.35.33] - 2026-11-26

### Added
- **The permalink of a page has two copy buttons.** The permalink at
  the end of a page is a link to the stable alias of the page, and
  copying it meant selecting the URL and writing the markdown around
  it by hand. The permalink now carries two buttons: one copies the
  markdown link (`[Title](https://example.com/_uid/abc123/)`), the
  other copies the HTML link (`<a href="…">Title</a>`). The buttons
  look like the copy button of the code blocks of the theme, and they
  work on a site served over plain HTTP as well.

## [0.35.32] - 2026-11-25

### Fixed
- **The build fetches only the repository whose remote moved.** The
  clone stage of the build created its own git client, and the client
  had no cache of the remote heads: the check that skips the fetch of a
  repository that did not move was never taken, and every repository
  was fetched on every build. The daemon creates the cache once and
  shares it with the updater and with the build, so the check holds for
  the build too. It also fills the cache again: the cache of the round
  25 was never written, because the only client that had one never
  fetched.

## [0.35.31] - 2026-11-25

### Fixed
- **The debouncer reads the bus of the run under the lock.** The bus of
  the current run is written when the daemon starts and read by the
  goroutine of the debouncer: the pointer was read without the lock,
  and the race detector reported it.
- **The debouncer and the updater subscribe before their worker
  starts.** The bus has no queue: a request that is published before
  the consumer is subscribed is lost. The two subscribed inside their
  own goroutine, and the request that was published in the window was
  lost.
- **The server accepts the requests after the consumers are
  subscribed.** The server started before the consumers were
  subscribed, and a webhook that arrived in that window was thrown
  away.
- **The cache of the remote head holds what the clone holds.** The
  check wrote the SHA of the remote into the cache before the clone was
  updated. The build that followed saw no change and skipped the fetch:
  the site stayed at the old commit until the next sync of the daemon.
- **The edit path of the preview follows the links of the path.** A
  directory of the docs that is a link to another place took the file
  out of the docs directory: the preview server opened a file anywhere
  on the machine.
- **The cache of the remote head holds no token.** The address of a
  repository can hold a token, and the cache is written to disk: the
  token was in the state file for the life of the install.
- **A root-relative image carries the repository of its page.** A path
  that starts with a slash is relative to the root of the repository,
  and the links carry the repository (and the forge). The images did
  not: they pointed at the root of the site, where the image is not
  published.
- **The lint fails when a file cannot be read.** The error of the walk
  of the rules was overwritten by the error of the scan of the broken
  links: a file that could not be read was reported as a clean lint.
- **The lint of paths in different trees does not scan the disk.** The
  common root of the paths climbed to the root of the file system, and
  the scan of the broken links from there walked the whole disk.
- **The excludes hold for the paths of the lint too.** The readme of
  the repository was linted when the path was absolute and skipped when
  it was relative.
- **The renamer of the sequence rewrites the links of the file.** The
  list of the files of the tree held the old name: the links inside the
  renamed file were never rewritten, and the fixer reported an error.
- **A renamed template file keeps its uid binding.** The binding ran
  on the old name, the file was not there any more, and the fix was
  lost.

## [0.35.30] - 2026-11-25

### Added
- **The template engine is a library.** The engine, the tri-state
  boolean field and the index that suggests file names live in
  `templates/`, `templates/field` and `templates/suggest` now, and an
  application outside this repository can import them. Before, they
  were under `internal/`, and Go refused such an import from another
  module: the engine could not be reused at all. The library depends on
  no package of this application, the exported surface ships as it is,
  and `templates/README.md` holds the promise. The decision is in
  ADR-024.

## [0.35.29] - 2026-11-25

### Fixed
- **The code blocks of the site have their colors and their copy
  button.** The theme loads the styles of the code blocks through its own
  stylesheet, and the stylesheet we ship is built without them: the code
  blocks had no colors and their line numbers stood in a column of their
  own. The line numbers also stood between the theme and the code: the
  theme adds the copy button to the code of the block, and a table
  around the code hid it.
- **The path of a project of GitLab is escaped once.** The path held the
  escaped form, and the URL escaped it again: the request asked for the
  project "group%2Fproject", and GitLab answered 404. A repository that
  gained documentation never came back into the site through a webhook.
- **A file that breaks two rules of the name is renamed once.** The
  second rename used the path of the file before the first one, and the
  fixer failed on a file it had already renamed: the tree still broke
  the rule, and the next run of the fixer failed the same way.
- **The example of the syntax in an inline code is not rewritten.** The
  fixer looked for the last link of the line, and the example of the
  syntax was the last one: the example was rewritten and the real link
  pointed at a file that does not exist any more.
- **The check of the repository waits for room in the queue of the
  updater.** The request did not fit in the queue and was dropped: the
  forge does not send the webhook again, and the commit it announced
  was published only at the next sync of the daemon.
- **The consumers are subscribed when the workers start.** The bus has
  no queue: an event that is published before the consumer is subscribed
  is lost. The server accepts the requests before the consumers are
  subscribed, and a webhook that arrived in that window was thrown away.
- **The groups of GitLab are listed once.** The listing is paginated,
  and a listing per group burned the rate limit that the listing of the
  repositories needs.
- **The walk for the ancestor is bounded.** A branch that does not lead
  to the commit read the whole history: a repository with a hundred
  thousand commits took seconds for every update.
- **A failed clone leaves the old copy of the repository alone.** The old
  copy was removed before the clone, and a clone that failed took the
  working copy with it: the pages of the repository were gone until a
  later clone succeeded.
- **The broken links of the readme of the documentation are reported.**
  The walk of the rules and the walk of the broken links used a
  different base for the excludes: the readme was linted by one and
  skipped by the other, and its links were never checked.
- **The admin server refuses a configuration without the monitoring
  paths.** The paths of the monitoring endpoints come from the
  configuration, and a configuration without them took the process down
  when the server read the path.
- **The retry of the git client watches the cancellation.** A build that
  was canceled slept out the whole delay, and the daemon held a worker
  slot meanwhile.
- **The updater and the scheduler read their engine under the lock.**
  The engine of the scheduler is replaced when the daemon starts again,
  and the two schedule methods read it without the lock. The readiness
  signal of the updater belonged to the last run.
- **The stores are opened again when the daemon starts again.** The
  event store and the search index are closed when the daemon stops,
  and they are opened only in the constructor: a daemon that was started
  again after a stop wrote no events and no search index.
- **The report of the fixer names what changed.** The fixer changed the
  categories of a page, and the report said nothing about it: the
  operator saw a changed sidebar and no reason for it.
- **The job ids of the discovery runs are unique.** Two runs in the same
  nanosecond were the same job, and the debouncer dropped the second
  one.

### Changed
- **The rule and the fixer share the sequence directories.** The fixer
  had a second copy of the map: a change to one of them would let the
  linter and the fixer disagree.

### Documentation
- **The exit codes and the comment of the lint are the ones of the
  code.** The usage errors exit with 80 and not with 2, and the severity
  of the JSON report is upper case: every error was marked with the
  emoji of a warning in the comment of the pull request.
- **The excludes are the paths of the repository.** The two reference
  pages said different things about where the exclude patterns match,
  and neither of them was the behaviour of the code.
- **The documentation names no flag that does not exist.**


## [0.35.28] - 2026-11-25

### Fixed
- **The pre-commit hook is installed where git reads it.** The path of
  the git directory kept the newline of the output of the command, and
  the hook was written to a directory that does not exist: the lint
  check never ran, and the command said it was installed.
- **A typo in the render mode is an error.** An unknown render mode
  became "auto", and in "auto" the build does not run hugo: the site
  was published with the scaffold only, and the build said it was fine.
- **The incremental build uses a workspace that stays.** The workspace
  of the command line was a directory of its own, and the clone of the
  repository was never found in it: every repository was cloned again,
  and the flag did nothing.
- **The build does not change the configuration of the caller.** The
  hash of the configuration holds the clone strategy, and the build
  changed it behind the back of the caller: the same configuration file
  had two hashes.
- **The name of a repository does not depend on the order of the
  discovery.** The forges are asked at the same time, and the order of
  the answer is the order of the race between them: two repositories
  that share a name swapped their names from one run to the next, and
  the name of a repository is the address of its pages.
- **The cache of the documentation check belongs to one forge.** Two
  forges hold repositories with the same full name, and the answer of
  one of them was used for the other one: the marker that keeps a
  repository out of the site took the repository of another forge out
  of it.
- **The discovery without a filtering configuration does not panic.**
  The warning about the filtered repositories read the configuration
  without a check, and the panic took the whole process down.
- **The admin server refuses a monitoring path with a wildcard.** The
  router panics on a path with a broken wildcard, and the panic took
  the whole daemon down.
- **The two links with the same target on one line are rewritten.** The
  line of the second link was looked for after the line of the first
  one: the fixer rewrote one link and left the other one pointing at a
  file that does not exist.
- **The lint of a single file honours the excludes.** The pre-commit
  hook lints every staged file on its own, and the readme of the
  repository was linted: the commit of every readme was blocked.
- **The lint of a single file resolves the links of the site.** The
  file was the root of the tree, and the link "/bar" was reported as
  broken: the commit of every page with a site-absolute link was
  blocked.
- **The excludes are the paths of the repository.** The same file was
  linted or not depending on the directory the linter was started in.
- **The query of a link is not part of the file.** The link
  "./exists.md?ref=foo" was reported as broken, and the build was
  blocked.
- **The link with an encoded name is rewritten.** The name of the file
  in a link is written the way a URL writes it ("%20"), and the fixer
  left such a link pointing at a file that was renamed.
- **The title of a page is not taken from a code block.** The comment of
  a shell command was the title of the page.
- **The fixer asks before it changes the files.** The command line
  changed the files of the operator without asking.
- **The dry run says what would happen.** The report of a dry run said
  that the fingerprints were updated and that the files were renamed:
  nothing was written.
- **The JSON report holds a list of issues and not a null.** The recipe
  of the documentation fails on a clean tree.
- **The usage of an undefined reference is reported.** The parser
  leaves the usage of a reference-style link whose label is not defined
  as plain text: the navigation was dead, and nothing reported it.
- **The broken link of a reference is reported once.** The definition
  of a reference and the usage of it point at the same file.
- **The handler behind the middleware can stream its answer.** The
  wrapper of the response held the answer in a buffer.
- **A damaged cache file does not take the daemon down.** The content
  "null" is a valid JSON, and it left the map of the cache nil: the
  next write panicked.
- **The state of the daemon is updated when the build has no report.** A
  build that failed before it wrote a report did not bring the site up
  to date: the daemon still said that the old commit was current, and
  the next webhook skipped the build.
- **The build of a removed repository is not refused.** The build
  service refused a build without repositories, and the build that
  follows the removal of the last repository has none: the pages of
  the removed repository stayed published for ever.
- **The webhook waits for room in the queue of the consumer.** The
  queue holds 32 events and the publish has a budget of two seconds: a
  burst of webhooks dropped the last ones, and the forge does not send
  them again.
- **The scheduler runs its jobs again after a restart.** The engine of
  the scheduler cannot be started again after it was shut down: the
  periodic work of the daemon stopped for good after a restart.
- **The stores stay open when the daemon stops.** The event store and
  the search index are closed when the daemon stops and opened only in
  the constructor: a daemon that was started again after a stop wrote
  no events and no search index.
- **The bus of the current run is read under a lock.** The bus is
  written when the daemon starts and read by the goroutines of the
  debouncer and of the updater: the race detector reported the three
  pointers.
- **The builds of the queue are counted.** The queue emitted the events
  through the emitter of the state, and nobody called the daemon: the
  counters of the builds read zero for ever.
- **The pending build of the last run is gone after a restart.** The
  snapshot of the last run survived the restart: the first build after
  it pinned the commit of the last run, and the new content appeared one
  build late.

### Changed
- **The configuration of the linter holds what the linter reads.** Four
  fields were written by the callers and read by nobody.
- **The code that nothing calls is gone.** The edit link resolver, the
  second link rewriter and the change detector of the documents had
  two implementations each, and only one of them was used.
- **The guard of the retry of the git client is gone.** The flag was
  written and read without a lock, and it guarded nothing.
- **The error variables of the forge package are gone.** The eight
  exported variables were read by nobody.

### Documentation
- **The documentation holds the command that exists.** The reference of
  the pipeline documented the command "visualize", which was removed.
- **The auto-fix of the file name is the one of the code.** The fixer
  removes the special characters instead of replacing them with
  hyphens.
- **The contribution guide points at a file that exists.**
- **The exit codes and the rules of the linter are the ones of the
  code.** The table of the exit codes named a code that never happens,
  and two rules of the frontmatter that never occur were documented.


## [0.35.27] - 2026-11-25

### Fixed
- **A pinned commit that is not in the shallow history is fetched.** The
  daemon pins the commit of the webhook, and a push that landed while
  the build ran is not the tip of the branch: the shallow clone did not
  have it, and the build failed over it.
- **The admin server refuses a monitoring path that the router panics
  on.** The paths of the monitoring endpoints come from the
  configuration: the router panics on a path without a leading slash
  and on a path that is already served, and the panic took the whole
  daemon down.
- **A server that fails to start releases the ports.** The listeners
  were bound before the servers were started: a server that failed left
  the ports of the others bound, and the daemon held a port that
  nobody answered on.
- **The first build is not lost.** The daemon asks for the first build
  when it starts, and the request goes through the bus: the bus has no
  queue, and a request that was published before the debouncer was
  subscribed was lost. The site was never built.
- **The handlers answer when the daemon is not there.** The method
  value of a daemon that is not there panics, and the guard that was
  meant to protect the handler never ran.
- **The build tree is not served before the first build.** The docs
  server answers on a public port: the sources of the site and the
  configuration of Hugo were served while the site was not rendered.
- **A daemon that is stopped without being started closes its stores.**
  The event store and the search index are opened when the daemon is
  built, and the file handles stayed open.
- **The public status does not tell where the site is written.** The
  answer held the output directory of the daemon, a path on the
  filesystem of the server.
- **The webhook answer says that a payload was not read.** A payload
  that could not be parsed answered "ignored", the same as an event
  that was understood and triggered no build.
- **Two discovery runs do not get the same job id.** The job id was the
  second of the run: two runs in the same second were the same job, and
  the debouncer drops a request whose job id it has already emitted.
- **Every route of the docs server answers through the middleware.** The
  routes beside the site got neither the log nor the answer to a
  panic, and a panic closed the connection without an answer.
- **The scheduler stops with the deadline of the stop.** The stop of
  the scheduler waited for the running job, whatever the deadline of
  the caller was.
- **A build that does not fit in the queue is not lost.** The queue was
  full, the build was dropped, and the site stayed as it was.
- **The failure of one subscriber does not keep the event from the
  others.** The publish stopped at the first subscriber that failed: the
  others never saw the event.
- **The bus is closed after the workers.** A worker that published
  while the daemon stopped found a closed bus, and the event was lost.
- **A link that wraps an image is rewritten.** The image inside the link
  was rewritten with the rules of a link, and the link itself was left
  alone: the image was in the wrong place and the link pointed at a
  file that does not exist.

### Documentation
- **The build queue is at /api/build/status.** The how-to told the
  operator to ask a route that does not exist.


## [0.35.26] - 2026-11-24

### Fixed
- **The daemon retries a transient failure.** The report of a failed
  build was thrown away: the queue tells a transient error from a
  permanent one by it, so the retries never happened in daemon mode.
- **A daemon that is started again after a stop builds again.** The bus
  and the stop channel of the queue were closed and never opened again:
  the second run never consumed a build request.
- **A failed build forgets the heads of the remotes.** The cache held
  the head of a remote whose build failed: the next webhook said that
  the remote did not move and skipped the build.
- **A namespaced build keeps the forge in its root-relative links.** A
  build of one repository that is namespaced by its forge keeps the
  forge in the path of the content, but the links at the root of the
  site and the images lost it: they pointed at pages that do not exist.
- **A manual discovery is canceled when the daemon stops.**


## [0.35.25] - 2026-11-24

### Fixed
- **The categories are patched into the frontmatter too.** The two
  category fixers parsed the frontmatter and wrote it again: the comments
  of the file were lost, the keys were sorted and the date of the
  document changed its form ("2024-01-02" became "2024-01-02T00:00:00Z").


## [0.35.24] - 2026-11-24

### Fixed
- **A fix does not rewrite the whole frontmatter.** The uid fix and the
  fix that fills the required fields parsed the frontmatter and wrote it
  again: the comments of the file were lost and the keys were sorted
  alphabetically. The fields are patched into the raw frontmatter now.
- **The linter takes several paths.** The pre-commit hook of the
  documentation passes the staged files: the command took a single path
  and the hook failed with a usage error.
- **The output flag wins over the configuration.** The flag was ignored
  when the operator typed the default value.
- **The hash holds what decides the paths of the pages.** The namespace
  and the tags of a repository decide the path of every page of it: a
  change of them left the hash the same and the daemon served the old
  URLs.
- **A failed start stops what it started.** The HTTP server, the workers
  and the files of a daemon whose start failed halfway were left behind.
- **The report of a skipped build holds the numbers of the last build.**
- **The log of the discovery holds the number of the files of the
  repository.**
- **The fonts of KaTeX are published when a page with math needs them.**
- **The JSON report of the linter is described as it is.** The CI
  recipes that counted the errors always reported zero.
- **The pages of the linter say what the linter does.**


## [0.35.22] - 2026-11-24

### Fixed
- **The deadline of the drain belongs to the drain.** It was armed at the
  start-up of the dispatcher: a daemon that lived longer than it lost the
  outbound ingest for ever - every document failed with "context deadline
  exceeded".
- **The front matter of the readme is not in the landing page.** The
  generation phase runs before the front matter is parsed, so the block
  showed up as text in the page.
- **The retry of a webhook is a new request.** The retry published the
  event with the job id of the first attempt, and the debouncer drops a
  request whose job id it has already emitted: the site was never
  rebuilt for that push.
- **The goroutines of a webhook belong to the daemon.** The shutdown did
  not wait for them, and they published on a bus that was closed.
- **The build prints the warnings of the loader.**
- **The README lists the commands and the tools that exist.**
- **The style of the frontmatter has no field that nothing reads.**
- **The build command has an edit-url-base flag.**


## [0.35.20] - 2026-11-24

### Fixed
- **The retry backoff defaults to linear.** The normalizer had "fixed"
  as its fallback: a configuration that left the field out was
  rewritten to "fixed" and the configuration warned about it. The
  documented default is linear.
- **The images carry the version.** The Docker configuration injected
  the version into a variable that does not exist, so the image
  reported the version "unknown".
- **The .env file is loaded before the flags are parsed.** The
  variables in it name the configuration file and the log level, and
  kong reads them while it parses the flags. Loaded afterwards, they
  had no effect.
- **The flags of the commands are what the documentation says.** The
  preview command, the init command and the build command: the table of
  the preview command named a short port flag that does not exist, the
  init command wrote a file that every other command does not read,
  and the build built the local docs directory when the configuration
  file that was named did not exist.


## [0.35.19] - 2026-11-24

### Fixed
- **The message of a document without frontmatter says so.** The message
  said that the field "frontmatter" was missing, and the explanation
  asked for a "frontmatter" field in the frontmatter.
- **The staging directory is a sibling of the output.** The site is moved
  into its place with a rename, and a rename across directories fails
  when they are on different file systems.
- **The status job does not hold the lock of the lifecycle.** The
  /status endpoint and the shutdown waited behind the two writes.


## [0.35.18] - 2026-11-24

### Fixed
- **Every shortcode of the code is escaped.** Two forms of a shortcode
  reached Hugo and took the whole site build down: the percent form
  (`{{% alert %}}`) and a shortcode in an inline span.
- **The readme of a repository is not lost.** In a single repository build
  the readme was renamed to `_index` and the generated index was written
  over it: the content of the readme was nowhere in the site. It is the
  content of the landing page now.
- **The worker group is not thrown away under a running worker.** The
  reset replaced the wait group while a worker that outlived the
  deadline of the stop still held a reference to it: the counter went
  below zero and the process died.
- **The remote-head cache is saved exclusively.** Two saves wrote the
  same temporary file and renamed it over the cache.
- **A document that was not uploaded is not remembered as uploaded.** A
  document whose upload was refused by a 401 was never sent again, not
  even after the auth was fixed.
- **The drain of the outbound dispatcher uploads the documents that are
  left.** The context of the run was canceled before the dispatcher was
  stopped: every upload failed.
- **The build after a removal runs without a repository.** The pages of a
  removed repository stayed published until something else asked for a
  build.
- **The check of the head of a remote has a deadline.** A forge that
  never answered held the repo updater, and every webhook-driven build
  behind it.
- **The assets keep their name.** A file called `index.png` was published
  as `_index.png`, and the link to it pointed at nothing.
- **The state file says "stopped" after the shutdown.**


## [0.35.17] - 2026-11-24

### Fixed
- **The conventional names are not renamed.** The rule reported the
  uppercase letters of a README.md and the fixer renamed it to
  readme.md: the home page of the repository was gone. The names the
  git forges recognize are the exception - the exception was only in
  the text of the rule.
- **A renamed file is fingerprinted under its new name.** The fixer
  renamed the file and then tried to fingerprint the old path: the read
  failed and the command exited with 2 after it had done the work.
- **The wrong category is replaced, not joined.** The categories of a
  document are its doc mode, and a document has one mode. The fixer
  joined the category of the directory to the wrong one and produced a
  document that no fixer could repair.
- **A fingerprint fix does not rewrite the frontmatter.** The fix parsed
  the frontmatter and wrote it again: the comments of the file were lost
  and the keys were sorted alphabetically. Only the fingerprint and the
  lastmod are patched now.
- **The recommended link style is not reported.** The rule flagged every
  link that starts with a slash. The how-to guide calls the
  repository-root-relative links the best practice and the
  broken-link detector understands them. The rule is gone.
- **The broken-link check reads every directory once.** Every broken link
  read the listing of the directory of its target: 2000 documents with
  one broken link each read the listing of 2000 entries 2000 times.
- **The closing delimiter can be the last line of the file.** A document
  whose frontmatter ended without a newline behind the closing delimiter
  had no frontmatter at all.
- **The lint command reads the excludes of the configuration.** The
  documented `lint.excludes` was ignored: only the MCP server read it.
- **The allowed category pairs come from the configuration.** The rule
  had a field for the pairs that nobody could set.
- **The sequence name does not repeat the prefix.** A file called
  `adr-something.md` was renamed to `adr-001-adr-something.md`.
- **The page of the lint rules lists the rules that exist.** Six rules
  were documented that no code implements.


## [0.35.16] - 2026-11-24

### Fixed
- **The files that keep a repository out of the site are the ones of the
  configuration.** The field was normalised, put in the hash of the
  configuration and never read: every forge looked for ".docignore", so
  `ignore_files: [.myignore]` kept excluding `.docignore` and never
  excluded `.myignore`.
- **No retries is no retries.** `max_retries: 0` became two retries: the
  field had no memory of being written.
- **A self-hosted forge needs an api_url.** Without one the first request
  died with "unsupported protocol scheme", long after the daemon started.
- **One search index job at a time.** Two builds that finish close
  together started two index jobs, and the slower one wrote the pages of
  the older build into the index of the newer one.
- **The stage that does nothing does not spin.** The post process stage
  waited for the clock to move.
- **The skip evaluation lists the checks the pipeline really makes.**


## [0.35.15] - 2026-11-24

### Fixed
- **The hash of the configuration holds the versioning.** A change of the
  versioning left the hash the same, and the daemon served the old site.
- **The status of the daemon is written to its state.** Nothing wrote it:
  the state file said "starting" for the whole life of the daemon.
- **A self-hosted GitHub is a GitHub.** Only "github.com" was recognized,
  so a GitHub Enterprise was taken for a Forgejo and its edit links
  pointed at the wrong place.
- **The wait for the subscribers of the bus has an end.** The bus waits
  for the subscriber to take the event; the wait had no end, so a
  subscriber that did not take it held the request for ever.
- **The output directory may not hold the working directory.** The site
  is moved aside when the build is done: an output directory that holds
  the working directory of the process replaced it under its feet.
- **The LiveReload port takes a limited number of clients.** Every client
  holds a goroutine and a channel for as long as it lives, and any page
  of the internet can open them.


## [0.35.14] - 2026-11-24

### Fixed
- **The clone and the fetch take the context of the build.** The git
  operations took no context: a host that never answers kept the build
  worker busy until the operating system gave up. The build could not
  be canceled, and the daemon has one worker.
- **The remote-head cache remembers what the remote holds.** The cache
  was never filled, so the check that says "the repository did not
  move" always said it did, and every webhook rebuilt the whole site.
  The daemon saves the cache with its other state now.
- **The parent of the output directory is made before the site is moved
  aside.** A directory that cannot be made cost the published site: the
  build failed and the daemon had nothing to serve.
- **A removed repository leaves the site.** The discovery asked for a
  build only when it found a repository, so the pages of a deleted
  repository stayed until something else asked for a build.
- **The site is published in the same place for everybody.** The command
  line joined the base directory onto an absolute directory, the daemon
  and the server did not: the site was published in two places.
- **The docs server does not hold a whole response in memory.** A
  request with the LiveReload cookie held the whole file in memory:
  104 MiB for a 20 MiB file, on the public port, from any client.
- **A webhook path with a wildcard does not take the process down.** The
  HTTP router panicked on a malformed path, and the daemon died with a
  stack trace instead of a clean error.
- **Versioning that is turned off stays off.** A strategy in the
  configuration turned the versioning on, even when the operator wrote
  `enabled: false`. The generated configuration holds both.
- **The edit links can be verified.** `skip_edit_links: false` did
  nothing: the field was a plain bool and the default turned every
  false into a true.
- **The Forgejo repositories keep their order.** The list was built by
  walking a map, so the same repository changed its name - and its
  place in the site - from one run to the next.
- **The build error page escapes the error.** The message of a build
  failure went into the page as markup, so a file whose name holds a
  script ran it in the browser of the reader.
- **The webhook answers what it did.** A body that is too large was
  answered with 400 and a wrong signature with 400 as well; they say
  413 and 401 now. A request that triggered no build answered
  "received": the forge believed the build was on its way.


## [0.35.13] - 2026-11-24

### Fixed
- **Every shortcode of a code block is escaped.** The check for an escaped
  shortcode was per line, so a second, unescaped shortcode on that line
  reached Hugo - and Hugo fails the whole site build when a shortcode has
  no template. The shortcodes are escaped one by one now, and the indented
  code blocks are escaped as well. The document keeps its bytes: the
  transform no longer adds a newline to every page, which changed the
  fingerprint of every page and was not idempotent.
- **The links and the images of a code block are left alone.** A markdown
  example inside a fenced block was rewritten into a site-absolute path, and
  the example showed a path that does not exist. ADR-012 keeps the links of
  the code blocks out of the rewriting.
- **The H1 of a document replaces the title of the file name.** A file called
  `install.md` lost the H1 of the author, while `Install.md` with the same
  content kept it. The two files are the same page.
- **The hash of the configuration holds the settings that change the site.**
  A change of the custom CSS, of the params, of the menu or of the URL of a
  repository left the hash the same, and the daemon skipped the build and
  served the old site. The documentation says that a change of any of them
  triggers a rebuild.
- **The parent of the output directory is made.** An output directory nested
  under the base directory failed every build with "no such file or
  directory".


## [0.35.12] - 2026-11-24

### Fixed
- **The prune keeps the `.docignore` of the repository.** The file is read
  from the clone: it is what keeps a repository out of the site. The prune
  removed it like every other top-level entry, so a repository the
  operator excluded was published.
- **A percent-encoded link target is decoded.** A link target is a URL:
  the space of a file name is written as `%20`. The target was looked for
  as it was written and never found, so every link to a file with a space
  in its name was a broken link - and the fixer could not heal it.
- **The link updater keeps the mode of the file.** It wrote the file with
  the mode `0600`, unlike every other write path of the fixer. A document
  that another user could read became one only its owner could read.
- **The clock of the fixer is UTC.** It returned the local time while its
  own documentation says UTC, and the dates it writes are documented as
  UTC. The same document got a different date - and a different
  fingerprint - on two machines.
- **A history that cannot be read is aligned to the remote.** The update
  failed before it even asked for `hard_reset_on_diverge`, a setting that
  is on by default and says that the local branch is aligned to the
  remote. A shallow clone whose deepening fetch failed failed the whole
  build instead.


## [0.35.11] - 2026-11-24

### Fixed
- **The outbound dispatcher does not send on a closed queue.** The queue
  was closed by the stop of the dispatcher and never opened again. A
  document that arrived after the stop went into the closed channel: a
  send on a closed channel panics, and the panic happens in a build
  worker, outside the recovery of the HTTP server - the first document of
  the first build after a restart took the whole daemon down. The queue is
  guarded by a mutex now: a document that arrives after the stop is
  dropped with a counter, and a dispatcher that runs again gets a new
  queue.
- **The stop of the build queue is bounded by its context.** The queue
  threw the context away and waited for its workers without an end. A
  build that does not stop - a clone to a host that never answers - kept
  the daemon from shutting down, and the state file, which is written
  after the queue is stopped, was lost with it. The stop waits for the
  deadline of its context now and then gives up.


## [0.35.10] - 2026-11-24

### Fixed
- **The `--data-dir` flag of the daemon decides where the state lives.**
  The flag was dead: the daemon only logged it and always wrote its state
  to the `state_dir` of the configuration. `docbuilder daemon -d ./elsewhere`
  wrote its state somewhere else entirely, and a daemon in a read-only
  working directory failed to start. The flag now wins over the
  configuration; without it the configuration decides, as before.


## [0.35.9] - 2026-11-23

### Fixed
- **The index of a section is not generated when a document has the
  name of the section.** The document and the index publish at the same
  page, and Hugo keeps one of them without a word: the index was a file
  in the content tree that published nowhere, and it could take the page
  of the document away. The document is the page of the section now, and
  the build says so.


## [0.35.8] - 2026-11-23

### Fixed
- **The landing page of a repository is never empty.** The generated
  index is written next to the documents it describes: the tree of the
  documents decides the tree of the index, and not the metadata of a
  repository name. Two repositories that share a name - the same name
  on two forges - each get their own landing page. A landing page
  without an index was rendered by Hugo as a bare section page: no
  content, and no navigation at all.


## [0.35.7] - 2026-11-23

### Fixed
- **The "Edit this page" link of the preview works again.** The VS Code
  CLI can leave a process behind that holds its output open: the command
  was then never finished, the request hung forever, and the browser
  waited forever. The command itself was done - it exited with a zero
  status - and the file was opened all the same. The pipes are closed
  after the deadline, and a command that was done is a success.

- **The failure of the edit handler says what failed.** The message
  alone ("the IPC socket was not found") does not say which one, and the
  page in the browser is what the developer sees: the details are part
  of the answer now.


## [0.35.6] - 2026-11-23

### Fixed
- **The build says when the Hugo is too old for the theme.** The theme
  asks the site for the direction of its language, and Hugo added that
  field in v0.158.0. An older Hugo fails on the first page with
  "can't evaluate field Direction in type *langs.Language", and the
  build says nothing about the version that is too old. The build
  checks the version of the Hugo binary before it renders and refuses
  the build with a clear message.


## [0.35.5] - 2026-11-23

### Fixed
- **The failure of Hugo says what Hugo said.** The operator saw "hugo
  execution failed: exit status 1" and the reason was only in the log of
  the daemon. The message carries the errors of Hugo now, and the
  preview shows them on its error page.

- **The paths of the files and of the links have the same shape.** The
  files of the tree and the files of the issues do not have the same
  shape: one of them is relative to the working directory of the
  process and the other one is absolute. The comparison of the two never
  matched: the links inside a renamed file were never rewritten, and the
  broken links to a file that was renamed were never healed.

- **The log of a skipped fetch survives an empty hash.** The remote has
  no branch of that name, and the slice of the empty hash stopped the
  daemon.

- **The token of the address of the site stays out of the answers.** The
  address of a private site can hold a token, and the error messages of
  the server and of the template fetcher carried it as it is.

- **The addresses of the daemon hold no credentials in the log.** The
  URL of NATS holds a password, and the address of the ragabast can
  hold credentials.

- **The fallback of the rewrite touches the last segment of the link.**
  The replacement of the first occurrence of the name of the file
  anywhere in the target changed the directory of the link and the query
  of the link.


## [0.35.4] - 2026-11-23

### Fixed
- **A readme next to an index is a page of its own.** The discovery
  maps an index.md to _index.md, and the pipeline renames a README to
  _index as well: the two of them are the same page, and one of them was
  lost. The check of the collisions compared the path before the rename,
  so it stayed quiet - and then the build failed on every repository
  that has a README and an index in the same directory (issue #147).

- **The rename of a file of a repository moves it with git.** The paths
  of git are relative to the directory of the -C flag, and the paths of
  the fixer are relative to the working directory of the process: the
  rename failed with "fatal: bad source" on every file of a repository,
  and the lint --fix exited with an error and blocked the commit.

- **A repository without a name is not cloned.** The name of a
  repository is the name of the directory the clone goes to: a
  repository whose name was refused was cloned into the root of the
  workspace, and the removal of the old clone deleted every other clone
  of the portal.

- **The build does not skip when the site is behind the workspace.** A
  build that failed after the clone stage left the commits of the next
  build in the workspace, and the next build saw "no HEAD changes" and
  skipped: the content never reached the site.

- **A build that did not publish is a failed build.** A clone failure
  that the classification does not recognize was only a warning, and
  the report in the site said warning although a repository was missing
  from it.

- **The password of the NATS URL stays out of the answer of the MCP
  server.** The link verification redacts that URL in its own logs.

- **The path of a link is cut by the characters of its suffix.** The
  lowercase of a character can have another length ("İ" is two bytes
  and "i" is one): the link lost a character of its path.

- **The hook lints the staged files where they are.** The hook copied
  the staged files into a temporary directory: the copy has no
  neighbors, and every link to a file that was not staged is a broken
  link. The hook rejected the commit of every documentation that links
  to a file of its own repository.

- **The list of the files of the tree follows a rename.** After a rename
  the list held a path that does not exist any more, and the links
  inside the renamed file were never rewritten.

- **A replacement of the frontmatter keeps the identity of the page.**
  The uid and the aliases are what the links of the other pages point
  at.

- **A link to the readme of the repository is broken.** The readme of
  the repository is above the documentation, and the site has no page
  for it.

- **A GitLab group is resolved to its numeric id.** A group that the
  operator names by its path was not found, and the discovery of the
  whole forge failed.


## [0.35.3] - 2026-11-23

### Fixed
- **The multiplication of the backoff does not overflow.** The shift of
  the exponential backoff was capped, but the product of the shift and
  the initial delay overflows the duration as well: with an initial
  delay of ten seconds and thirty-one retries the delay was negative,
  the sleep returned at once, and the retry loop hammered the failing
  service without a backoff.

- **The preview daemon does not trigger a discovery.** The preview
  daemon has no discovery runner: the trigger dereferenced a nil
  pointer inside a spawned goroutine and killed the process.

- **A failed build updates the state.** The state was only updated
  after a successful build: every repository still reported the result
  of the last successful build, and the time of the last build stayed
  the time of the last successful one.

- **The status page shows the repositories of an explicit
  configuration.** The list was built from the discovery result alone:
  an explicit configuration has no discovery result, and the page showed
  no repository at all.

- **The URL of a repository is logged without its credentials.** A
  repository URL can carry a token, and the log line ends up in a log
  collector. Ten places logged the URL as it is.

- **The index of a repository lands in the tree of its pages.** The
  forge of a repository is the one the discovery gave it: the discovery
  names a repository only when the configuration asks for it.

- **A sibling readme or index points at the section.** The pipeline
  renames a readme to _index, and the page is published at the
  directory of the document.

- **A path with brackets is not cut at the first one.** The closing
  bracket of a link was the first one of the path, so "./guide(2).md"
  was cut at the bracket of the path, and the extension of the file
  was never taken off.

- **The error of a repository is written to the state.** The status of
  a repository whose clone failed was "ok", because the counter of the
  last successful build was not zero.

- **The metrics of the build are recorded.** The recorder of the build
  was never given to anybody: every call went to the no-op recorder,
  and the metrics of the build stayed at zero.

- **The page waits for its links.** The hash of the page says that the
  links of the page are checked: a page that is marked as checked while
  its links are still in the pool is never checked again.

- **The attempts to connect to NATS are spaced out.** The cache of every
  link of every page asked for a connection, and a server that is down
  was asked thousands of times in one build.

- **The path of the document and its body have one uid.** The builtins
  were resolved by every render: the path and the body each had their
  own uuid.

### Refactoring
- The forge factory of the tests is test code and is not compiled into
  the binary any more.


## [0.35.2] - 2026-11-23

### Fixed
- **The status page counts the builds.** The builds that ran and the ones
  that failed were serialized and never filled: the page reported zero
  of both while the daemon was building and failing.

- **The preview reads no state.** The preview daemon has no state
  manager, and the repositories were read from it: a request to
  `/api/repositories` panicked.

- **The time to live of the cache is read once.** It was parsed on every
  lookup, and a value that does not parse left it at zero: every link
  was checked again on every build, even though the cache was full.

- **The backoff does not overflow.** The shift of the exponential
  backoff was not capped: a shift of 63 overflows the duration, the
  delay became negative, and the retry loop spun without a backoff.

### Performance
- The patterns of the headings and of the images are compiled once and
  not for every document of every build.


## [0.35.1] - 2026-11-23

### Fixed
- **A file that cannot be read does not stop the scan for the links of
  the other files.** The file was renamed and no link was rewritten.

- **The trigger of a build answers that nothing was queued** when no job
  was queued: the daemon is not running, or the event did not reach the
  orchestration bus.

- **The stop channel is closed once.** Two concurrent stops closed it
  twice, and the second close stopped the daemon with a panic.

- **The status of a daemon that was stopped while the loop was ending
  stays "stopped".**

- **A replacement of the frontmatter with nothing is refused.** It
  deleted the title, the uid and the aliases of the page.

- **The name of a discovered repository is a single path segment.** The
  clone directory is built from it, and the removal of the old clone
  deleted what was outside of the cache.

- **A generated file is written without a window in which it is half
  written.**

- **The broken link detection uses the excludes of the linter.**

- **A build with `render_mode: always` and no hugo binary fails.** It
  published a staging tree without a rendered site, and the site that
  was published was gone.

- **The metadata of a repository carries the forge the discovery gave
  it.** The generated index landed in another tree than the pages.

- **The guard of a lost repository does not fire for a version.** The
  versions of a repository share its URL, so the state holds the count
  of the last one of them.

- **A link with a title is rewritten.** It kept the extension of the
  file, and Hugo publishes no .md files.

- **The namespace of a repository is a path segment.**

- **The organizations of GitHub are read page by page.** Every
  organization after the thirtieth was never scanned.


## [0.35.0] - 2026-11-23

### Fixed
- **The forge of a repository is the namespace,** and the tag is the
  fallback. The metadata of the build and the discovery read them in
  different orders, so the pages and the generated index of a repository
  with a namespace landed in different trees.

- **A build that did not publish is a failed build.** The report in the
  site said warning although a repository was missing from it.

- **The early skip sees a repository that was removed from the
  configuration.** Its pages stayed published forever.

- **The counters of the builds are raised.** The metric of the builds
  stayed at 0 on a portal that had run thousands of them.

- **A link that timed out is checked again later.** It was cached as
  valid for the whole time to live and was never published as broken.

- **The link verification runs in a worker** the daemon waits for, and
  the verifier is closed after the workers stopped.

- **The status page shows the state of the repositories.** Every
  repository was shown as healthy, also the ones whose clone failed.


## [0.34.9] - 2026-11-23

### Fixed
- **A template comes from the site it was discovered on.** The
  discovery page names the templates, and the CLI and the TUI fetched
  whatever it named: a site that lists a template of another host had
  its body written into the documentation of the operator.

- **A generated file stays in the docs directory.** The write checked
  the path as it was written, and a directory of the docs that is a link
  to another place took the file outside of it.

- **A link to the root of the repository is rewritten.** A link that
  points at the root has no extension ("/Guide Setup") and the name of
  the file has one ("Guide Setup.md"): the link kept the old name of a
  file that was renamed.

- **The tree is walked once per run, not once per renamed file.**

- **The daemon stops.** Stop returned at once when the status was
  "stopping", and that is the status when the main loop ends: the
  teardown never ran, the state file was not written and the ports
  stayed bound.

- **The guard of a repository that lost its documentation works.** It
  walked the repositories that have files, and a repository that lost
  all of them has none.

- **The URL of NATS holds a password and stays out of the log.**

- **A live reload client that breaks leaves no entry behind.**

### Performance
- The fixer collects the markdown files of the tree once per run: a run
  that renames a hundred files walked the tree a hundred times.


## [0.34.8] - 2026-11-23

### Fixed
- **Two files that land on the same page are refused.** The pipeline
  renames a README to _index, so a repository with a README and an
  index in the same directory sends two documents to the same page and
  one of them is lost. The check of the collisions compared the path
  before the rename, and it stayed quiet.

- **A repository that lost its documentation fails the build.** The
  number of documents a repository had in the last build is in the
  state, and the build compares it with the number it found: a
  repository whose directory was renamed would have taken its pages out
  of the site, and the site that still had them was replaced.

- **The git commands of the fixer run in the repository of the file.**
  The fixer looked for the file in the index of the working directory of
  the process and moved it there: an MCP server started in another
  repository than the docs directory renamed the wrong file.


## [0.34.7] - 2026-11-23

### Fixed
- **The pool of the links of the verification does not block and does not
  crash.** The defer that released the semaphore released a semaphore
  nothing acquired any more: every link worker blocked in it after its
  first link, the verification never ended, and one goroutine leaked per
  build. The queue of the links is closed after the pages are done with
  it: a page that still had links sent to a closed channel, and the
  daemon died.

- **The fixer writes the files of the user without a window in which they
  are half written.** The write goes through a file next to it and a
  rename now, the way the MCP server writes. A file that the owner
  marked read-only is not replaced.

- **Three crashes and a lost repository in the clone.** The head of the
  repository and the reference of the branch were read without their
  error: a workspace with an unreadable .git/HEAD stopped the daemon
  with a panic. The prune of the paths took the first segment of the
  path: a repository whose documentation is at its root had every top
  level entry removed. The local forge said that it has no
  documentation, and the filter dropped the repository.

- **The extension of a file is lowercased** like the name: a markdown
  file with .MD was never rendered by Hugo, and a file published at
  /repo/diagram.PNG was not found at /repo/diagram.png, which is where
  the link points.

- **The state file is private to its owner.** It holds the URL of the
  repositories, and the URL can hold a token.

- **A build that is canceled stops.** The fetcher threw the context
  away: a build that was canceled or that timed out kept cloning every
  repository, and the queue waited for it.

- **The debouncer keeps the newest request.** It cleared its pending
  flag after it published the event: a request that arrived during the
  publish was dropped.

- **The preview does not panic and the state survives the shutdown.**
  The preview daemon left the build, the state and the discovery stacks
  nil, and the monitoring endpoints read them. The state was saved
  before the workers stopped.

- **The pipes of Hugo are closed when the deadline of the build
  passes:** a child of Hugo keeps them open, and the build waited for it
  past the deadline.

- **The staging directory must not be the output directory:** it is
  emptied before the build, and the site would be deleted.


## [0.34.6] - 2026-11-23

### Fixed
- **The state of the daemon is written once per build.** Every setter
  re-marshalled, wrote, synced and renamed the whole state file: a build
  writes the state of every repository about five times, so a portal
  with a hundred repositories rewrote the whole file hundreds of times
  per build. The changes mark the store dirty now, and the file is
  written when the state is saved: the daemon saves it after every
  build, and the store writes it when it closes.

- **The paths of the pages are not stored.** The state kept the Hugo
  path of every page of every repository, and nothing read them back:
  the file grew to megabytes and was rewritten on every change. The
  hash of the files of the repository covers the change detection.


## [0.34.5] - 2026-11-23

### Fixed
- **A build of Hugo has a deadline.** A Hugo that hangs - a module
  download that never answers - held the build of the daemon forever,
  and every later build queued behind it until the queue was full.

### Tests
- **A stage that is fast enough measures zero.** The clock of the
  machine is the limit, and the test of the timings failed the build of
  the project every few runs.


## [0.34.4] - 2026-11-23

### Fixed
- **The credentials of a repository URL stay out of the answer of the
  MCP server and out of the logs.** The URL of a repository holds a
  token in the documentation, and the answer of the `get_config` tool
  and of the `config` resource carried it as it is.

- **The link of a renamed file is rewritten.** The path of the walk of
  the lint is relative and the resolved link is absolute, so the
  precise rewrite was skipped and the link kept the old name of the
  file that was renamed.

- **Two versions do not share a name.** The path of a version is a
  lossy version of its name, and the second clone of a name deleted the
  workspace of the first one.

- **The short hash of a commit does not panic** when the stored hash is
  short or empty.

- **The links of a page are verified by the workers of the pool** and
  not by one goroutine per link.

- **The event bus delivers and shuts down without a race.** The bus
  held its read lock while it sent to a subscriber whose channel was
  full: the write lock that unsubscribe and Close need was never
  given, and the daemon did not stop.

- **The event store is closed after the workers.** An event that a
  worker wrote after the close was lost from the build history.

- **A build that lost a repository fails also when there is no site
  yet.** A portal without one of its repositories is worse than no
  portal.

- **A repository without a commit counts as a failure** in the status
  of the repositories.


## [0.34.3] - 2026-11-23

### Fixed
- **The pages are verified by a fixed number of workers.** One goroutine
  per page: a portal with thousands of pages spawned thousands of
  goroutines, and every one of them held the links of its page in
  memory while it waited for its turn.

- **An explicit `follow_redirects: false` is honoured.** The default
  turned every false into a true, so the check that reads the value
  could never be false.

- **The entries of the state are changed under the lock of the store.**
  The adapter read a repository, changed the copy and wrote it back, and
  a write of another field that landed in between was silently reverted:
  the build wrote the commit of a repository while the discovery wrote
  the time of its last discovery.


## [0.34.2] - 2026-11-23

### Fixed
- **The in-memory event store keeps one connection.** A second connection
  of the pool opened a database of its own, and every statement on it
  failed with "no such table: events".

- **The password of the NATS URL stays out of the logs.** The URL holds
  a password in the documentation, and it was logged as it is.

- **The discovery does not empty the number of documents.** The
  discovery does not know the number, and it wrote a zero over the
  number the build had written.

- **A broken link of a page without a document does not panic.**

- **A repository that could not be cloned counts as changed.** The build
  skipped and the site stayed without the pages of that repository.

- **A build that does not publish writes a report that says so.** The
  next build reads the report to decide whether it may skip, and the
  site was behind the workspace: only a build that runs publishes it.

- **A link that goes above the root of the repository keeps no "../" in
  its address.**

- **The name of a versioned repository is a path.** It was built from the
  display name of the version, which holds spaces and may hold a slash,
  so two versions could share one name and overwrite each other.

- **The counters of the repositories are raised.** Every repository
  stayed "pending" on the status page, and the operator could not tell
  which of them was built.


### Fixed
- **The findings of the code scanning are gone.** The alerts of the
  CodeQL scan of the repository are addressed: every workflow says what
  the token of GitHub may do, the link checker knows the schemes a
  browser runs as code, the webhook handler keeps the headers of a
  request out of the log, and the walk for the nearest page stays under
  the root of the documentation.

## [0.34.1] - 2026-11-23

### Fixed
- **The build does not ask the network for the assets of the theme.**
  The theme fetches the KaTeX stylesheet and its fonts from a CDN, and
  the mermaid script from another one, with `resources.GetRemote`. A
  build without a network died on it with "Could not retrieve KaTeX css
  file from CDN" (issue #129).

  The site serves its own copy of KaTeX, the way it already serves the
  stylesheet, the Bootstrap JavaScript, the fonts and the search
  library, and the mermaid partial of the theme is replaced by one that
  does not fetch anything: the browser fetches the script anyway, so
  the check that the version exists was the only thing that needed the
  network.


## [0.34.0] - 2026-11-22

### Added
- **The MCP server can read the published site.** Two tools,
  `search_site` and `list_site_pages`, ask the documentation site the
  daemon serves. An agent that writes documentation can see what the
  site already holds - across every repository the daemon aggregated -
  before it writes a page, so it neither duplicates a page nor misses
  the one that covers the topic.

- **The docs server answers with the inventory of the site.** The new
  endpoint `/api/pages` lists the pages of the last published build
  with their addresses and their titles.

### Fixed
- **A build that lost a repository does not take the site down.** A
  clone that failed was only a warning: the build went on, published a
  site without the pages of that repository, and reported success. When
  every clone failed it published a site with nothing in it but the
  home page. A build that lost a repository now fails, and the site of
  the last build stays published.

- **A repository that was added to the configuration is published.** The
  check that decides whether anything moved only looked at the
  repositories that had a head before, so a new one did not count and
  the build was skipped. The site kept serving the old content.

- **The site keeps its shape when a clone fails.** The count of the
  repositories that could be cloned decided whether the site was a
  single-repository site, so every page of the other repositories moved
  to another address. The count of the configured repositories decides
  it now.

- **A page without a date carries the date of its commit.** The date
  was fetched and thrown away, so every such page carried the same old
  date and every list page sorted by it was in the wrong order.

- **The links point at the addresses the pages are published at.** The
  content path is lowercased when it is written, but three of the link
  rewrites kept the case of the repository, the forge and the file. A
  link that goes up with `../` was resolved without the directory of
  the document, so it pointed at the root of the repository instead of
  the section of the page.

- **The versioned repositories are part of the build.** The metadata of
  the repositories was built from the configured ones, and the
  versioned pages looked their metadata up by their own name: they were
  published without an edit link and without the landing page of their
  repository.

- **Every second build was a full build.** The report of a build that
  was skipped holds no files, and the next build reads that report to
  decide whether it may skip: a report without files is read as "the
  site is empty". The report of a skipped build keeps the numbers of
  the last build that ran.

- **The site carries the children shortcode.** The documentation of the
  repositories was written for a theme that had the shortcode, and a
  shortcode without a template fails the build of the whole site
  (issue #126).

- **The fixer rewrites the link and not the prose.** The line of a link
  was found by searching the body for the text of its target, so a page
  that mentioned a file in its prose and linked to it had its prose
  rewritten and its link left pointing at a file that was renamed.

- **A link to the root of the repository is found.** The lint looked for
  the file in a directory named "content", a layout that only exists in
  a built site, and reported every such link as broken: a valid doc
  failed the lint and the CI.

- **The values of the state survive a restart.** The fields of an
  Option are unexported and the type had no JSON methods, so the
  encoder wrote `{}` for every Option and the decoder read them back as
  empty: the daemon lost the commit of every repository, the time of
  its last build, its last error and its doc-files hash on every save.

- **The state file survives a crash.** It was renamed into place
  without being synced, so a crash left a file that was empty or half
  written and the daemon continued with an empty state.

- **A state write that fails is not silent.** Every setter threw the
  result of the write away, so a state directory that could not be
  written left the daemon carrying on with a state that never reached
  the disk.

- **Two builds in the same second do not share a workspace.** The
  workspace was named after the second it was made in, so the first
  build to finish deleted the workspace of the other while it was
  cloning: the build published a site without pages.

- **A repository that is versioned by a tag survives the build.** The
  update path fetched a branch, and a tag is not one: the pages of the
  tag vanished from the site after the first build.

- **The paths of the documentation are the configured ones.** The forge
  clients looked for a folder named "docs" and nothing else, so an
  operator whose repositories keep their documentation elsewhere had
  every repository dropped and the site came out empty.

- **The `.env.local` file is loaded as well.** The loader returned
  after the first file it found, so an operator who kept the live token
  in `.env.local` ran the daemon with the old one, and every call to
  the forge failed with a 401 that pointed at nothing.

- **The credentials of a repository URL stay out of the logs.** The
  repository URL is the one field where an operator puts a token, and it
  was logged as it is and put into every git error.

- **The daemon does not stop on a race or a panic.** The number of live
  reload clients was read outside the lock, the time of the last
  discovery was written by one goroutine and read by another, a
  discovery that panicked was reported as a discovery that succeeded,
  the error of the health and the metrics endpoints was written with a
  missing request, a configuration with a negative number of link
  verifications stopped the daemon at startup, and the preview server
  stopped with a panic when a file was saved and the preview was
  stopped within 300 milliseconds.

- **The base directory holds the output everywhere.** The docs root
  joined the base directory with the output directory and the other
  places did not: a deployment with a base directory never showed the
  status page and served an empty tree instead.

- **The taxonomies of the site are not read on every request.** Every
  request to `/api/taxonomies.json` walked the content of the site and
  parsed the front matter of every file. The answer is kept until the
  content directory changes.

- **The admin API refuses the requests of another origin.** Any page
  the operator visited could post a form to the admin port and have the
  portal rebuilt.

- **The MCP server stays inside the docs directory.** The check that a
  path is inside the docs directory compared the path as it was
  written, so a symlink inside the directory that pointed outside it
  let the tools read and rewrite a file anywhere on the machine.

- **The MCP server does not throw a frontmatter away.** The update read
  the frontmatter of the file and, when it could not be parsed, wrote
  the body alone: the title, the uid and the aliases of the page were
  gone and the tool answered that it had written the file.


### Added
- **The MCP server can read the published site.** Two tools,
  `search_site` and `list_site_pages`, ask the documentation site the
  daemon serves. An agent that writes documentation can see what the
  site already holds - across every repository the daemon aggregated -
  before it writes a page, so it neither duplicates a page nor misses
  the one that covers the topic.

- **The docs server answers with the inventory of the site.** The new
  endpoint `/api/pages` lists the pages of the last published build
  with their addresses and their titles. A `prefix` keeps only the
  pages whose address starts with it.

## [0.33.1] - 2026-11-22

### Fixed
- **A long query no longer ties up the search endpoint.** The query
  was cut to 256 characters on one path but not on the other, so the
  fallback read a query of any length. A 500 KB query cost 15 seconds
  of one CPU core per request; the endpoint is public, so a handful
  of them made the docs server unresponsive. The cut now happens
  once, before either path reads the query.

- **Two builds finishing at the same time no longer break the search
  index.** The wait for the lock was set through a statement, which
  reaches only the one connection that ran it - every other connection
  the pool opened had no wait at all. The index now waits for the lock
  on every connection, takes the lock at the start of the transaction
  instead of on its first write, and writes one build at a time.

- **A static export ships a working search again.** The provider
  followed the configuration file: a file with a `daemon:` block made
  the `docbuilder build` command ship the search client that asks a
  server, and no server was there to answer. The provider now follows
  who builds the site: the daemon, which serves the site it builds,
  chooses the server; a plain build, which produces a static tree,
  chooses the browser-side index the theme ships.

- **The daemon now releases the search index when it stops.** The index
  kept its file handle on the state directory, and its write-ahead log
  was only folded back into the index when the last connection went.

- **A page under a non-ASCII address gets a readable title again.** The
  title of a page without one of its own comes from its address, and
  the first letter was upper-cased as a byte - which cut a multi-byte
  letter in half and left a broken character in the search results.

- **The search results hold the words of the page, not the markup.**
  The page was indexed as it was written, so an excerpt showed the
  reader `**package manager**` and the fences of the code blocks. The
  index now holds the text of the page - the same text the theme's own
  index holds, so a reader finds the same pages either way.

- **A field filter on a hyphenated value finds the page again.**
  `categories:how-to` asked the index for a field named `to`, which
  does not exist, so the query found nothing at all. The value of a
  field filter is now read as one term, so the filter means what the
  reader wrote.

- **The `limit` of the search request is honoured.** The client sent
  the number of results it wanted and the server ignored it, so the
  site's own limit had no effect. The visitor may now ask for fewer
  results than the configuration allows; the limit of the configuration
  stays the ceiling.

- **The search index is written in the background.** The walk over
  every page of the site took seconds in the middle of the build
  queue, so the next build waited for it. The index is now written the
  way the link verification runs.

- **The search tip is only shown when it helps.** A reader who already
  writes `OR`, `NOT`, a phrase or a field filter does not need to be
  reminded of them on every keystroke.

- **A field filter keeps its meaning.** The quoting of the value of a
  field filter took the wildcard of the value with it, and a colon
  inside a quoted phrase was read as a field filter. A wildcard stays
  a wildcard and a phrase stays a phrase.

- **A change of the search provider rebuilds the site.** The provider
  decides what the site ships - the search client and the index - but
  it was not part of the configuration the build compares. A site that
  switched provider kept the old client until something else changed.

- **A page that asks for an address of its own is searchable again.**
  A page with `url` in the front matter is rendered at that address,
  and the index looked for it under the name of its file. Such a page
  could not be reached through the search.

## [0.31.0] - 2026-11-22

### Added
- **The search understands operators.** A query is read as it is
  written now:

  | Written | Means |
  |---------|-------|
  | `daemon AND kubernetes` | Both terms have to be in the page. |
  | `daemon OR kubernetes` | Either term is enough. |
  | `daemon NOT kubernetes` | The first term, without the second. |
  | `"package manager"` | The words, in that order, next to each other. |
  | `title:kubernetes` | The term has to be in the title. |
  | `body:kubernetes` | The term has to be in the text of the page. |
  | `tags:adr` | The term has to be in the tags of the page. |
  | `kuber*` | The words that start with it. |

  The operators are the words of the index, so they are written in
  upper case - a lower case "and" is a word the pages have to hold.

  A query the index cannot read - a half-written one, most of the time
  - is searched the forgiving way: every word of the query has to
  appear somewhere in the page. A search box holds half-written
  queries, and half a query still finds the pages it can.

  The search box tells the reader about the operators, so the syntax
  does not have to be guessed.

### Fixed
- **A search without results answered with nothing instead of an empty
  list.** The answer of the endpoint held `null` where a reader of the
  answer expects `[]`.

## [0.30.0] - 2026-11-22

### Added
- **A "Create documentation issue" link on every page.** The link
  opens the new-issue page of the forge the page was written in, with
  the title of the page filled in.

  The issue belongs to the repository the page came from: a site that
  gathers several repositories sends the reader to the tracker of the
  one they are reading. GitHub, GitLab and Forgejo each name that page
  differently, so the address is built per forge.

  The link replaces the theme's own, which only worked for GitHub and
  only for a single repository.

## [0.29.0] - 2026-11-22

The search of the site now runs on the server.

### Added
- **A server-side search.** The daemon keeps an index of the rendered
  pages and answers the queries at `/api/search`. The index never
  reaches the visitor: the browser asks for the results and receives
  them.

  The index is a SQLite FTS5 table, so the search costs no new
  dependency - SQLite was already part of the binary. The ranking is
  BM25, a hit in the title counts more than a hit in the body, the
  words are stemmed and the last word of a query may be a prefix.

  The index is built after every build, from the transformed markdown
  of that build, and it lives in the daemon's state directory, so it
  survives a restart.

### Fixed
- **The search index was more than half taxonomy pages.** The index the
  theme built held an entry for every tag and every category, and each
  of those entries repeats the text of every page that carries the
  tag. A site with many tags was mostly a list of tag pages, and they
  competed with the real documents in every search.

  The taxonomy pages are not part of the content tree and are not
  indexed now. A search for a word that appears in a tag name returns
  the pages with that tag, not the page that lists them.

- **Every visitor downloaded the text of the whole site.** The theme
  wrote the full text of every page into a JSON file that every page
  view fetched, parsed and indexed in the browser. The file is megabytes
  on a site of a few hundred pages, and it grows with every page. It
  stays empty now: the browser asks the server instead.

### Configuration
- **`search.provider`** decides where the search runs: `server` (the
  daemon's own index) or `lunr` (the browser-side index the theme
  ships). The default follows the deployment: a daemon serves the site
  and answers the queries, a site that is built as a static tree has no
  server to ask and searches in the browser.
- **`search.max_results`** is how many results a search returns. The
  default is 10.

## [0.28.6] - 2026-11-22

More findings from the fifth audit of the whole repository.

### Fixed
- **A failed promotion took the whole site away.** The build renames
  the current site to a backup before it moves the new one into place.
  When that move failed, the site stayed at the backup and the daemon
  served nothing until the next build. The old site is put back now.

- **A page without a document file crashed the daemon.** The broken
  link event reads the name of the document the broken link is in. A
  page that has no document file - which a caller that only knows the
  rendered page does not have - made the daemon panic. The event is
  published with empty source fields now.

## [0.28.5] - 2026-11-22

A fifth audit of the whole repository.

### Fixed
- **The site lost a repository when one API call failed.** The
  discovery asks the forge whether a repository has a documentation
  folder. When that call failed - a rate limit, a hiccup, a server
  error - the repository was treated as one without documentation and
  left out of the build. The site was then rebuilt without it, and its
  pages disappeared until the next discovery run.

  A failed answer is not an answer. The repository is kept now, and the
  build looks at the paths again: a repository without documentation
  contributes nothing, as before.

## [0.28.4] - 2026-11-22

A fourth audit of the whole repository. Two of the findings broke the
site; the rest were a setting that did nothing and a skip that never
fired.

### Fixed
- **The daemon never rebuilt the site.** The daemon decided whether a
  build could be skipped before it looked at the repositories, so it
  compared the last build with itself and always concluded that nothing
  had changed. Every build after the first one was skipped: a push to a
  repository, a scheduled sync, a manual trigger - the site kept serving
  the first version forever.

  The decision is made by the pipeline now, right after the
  repositories are cloned, where the commit each repository was at
  before the clone and the commit it is at now are both known. The
  broken decision path is gone.

- **The MCP `lint_fix` tool could rewrite files anywhere on the
  machine.** The tool took a path and renamed and rewrote the files it
  found there, without checking that the path was inside the
  documentation directory. Every other tool that touches a file keeps
  it there. The fixer does too now.

### Changed
- **`skip_if_unchanged: false` was ignored.** The daemon's defaults
  overrode the setting, and the pipeline skipped a build whatever the
  setting said. The setting decides now: with it off, every build runs.
  An explicit `false` in the configuration is left alone.

- **The pipeline could never see that nothing had moved.** The commit
  of the clone the last build used was only read with one of the two
  clone strategies, so the daemon - which uses the other one - always
  rebuilt. It reads the commit now.

## [0.28.3] - 2026-11-22

A third audit of the whole repository. Two of the findings broke a
build; the rest were slow paths, drift and code that did nothing.

### Fixed
- **A repository that moved two commits was reported as diverged.** The
  default clone keeps one commit of history, so the daemon could not
  read the history it needed to tell a fast-forward from a divergence.
  Every repository that moved two or more commits between two builds
  took the destructive path, and `hard_reset_on_diverge: false` - the
  setting that is supposed to be the safe one - failed the update
  outright. The daemon deepens the history when it has to and asks
  again now.

- **The queue length on the status page was a number nobody could
  trust.** Two counters wrote to it: one counted the jobs waiting, the
  other counted every job that had ever been queued and was only
  corrected every 30 seconds. The number on the status page, in the
  build status endpoint and in the Prometheus gauge grew with every
  build. The build queue is the only owner of that number now.

- **Every markdown file was read twice per fix run.** The linter found
  the broken links and the fixer looked for them again, parsing the
  whole tree a second time. The interactive path parsed every file four
  times. The fixer works from the findings of the lint run now.

- **The link rate limit did not limit anything.** The delay was
  applied between pages, not between requests, so a page with thirty
  links sent all thirty at once. It also sat on the critical path, so
  a site of 171 pages waited 17 seconds before the last page was even
  looked at. The delay is in front of the request now.

- **The preview showed one set of changes and another set was
  applied.** The interactive fix ran the whole fixer twice, and the
  second run read the tree again. If anything changed in between, the
  applied fixes were not the ones the user had approved. Both runs
  work from the same lint run now.

- **A font that changed kept the old one in the browser.** Stylesheets
  and scripts are cached for a year only when their name carries a hash
  of their content. Fonts were cached that way whatever their name
  was, so a theme that changed a font never reached a browser that had
  it. Fonts follow the same rule now.

- **The Prometheus gauges reported the wrong daemon.** They read a
  package global, so a second daemon in the same process - a test, for
  instance - made both report the numbers of the last one. The gauges
  belong to their daemon now.

- **The event log was scanned after every build.** The delete that
  removes the old events ran after every build, even when nothing was
  old enough to remove. It runs on a schedule now, and the rebuild at
  startup reads the recent events instead of the whole log.

### Changed
- **The documentation check cache grew with every commit.** The cache
  key held the timestamp it was supposed to be invalidated by, so every
  commit left an entry behind that was never used again. The
  repository keeps one entry now.

- **The histogram shifted its whole buffer on every value.** A
  histogram that holds a thousand values copied all thousand of them on
  every new value. It overwrites the oldest value instead.

- **The map of already uploaded documents grew forever.** The daemon
  remembered every document it had ever sent to ragabast, and never
  forgot one. The map is bounded now.

### Removed
- **A webhook endpoint that did nothing.** The daemon answered `POST
  /webhook` with a cheerful acknowledgement and threw the payload
  away: no signature was checked and no build was started. It is gone,
  so a request to it gets a 404 instead of a misleading answer. The
  webhook paths of the configured forges are unchanged.

- **A copy of the reference lister without authentication.** The
  version manager could lose the authentication of a private
  repository without saying so. The parameter that carries it is typed
  now, and the copy of the lister that had no authentication at all is
  gone.

- **A package that held one copied interface.** The package existed
  for a single interface that repeated four methods another package
  already had. The package is gone.

## [0.28.2] - 2026-11-22

A second audit of the whole repository. Two of the findings were real
bugs, the rest were documentation that had drifted, code that did
nothing and logic that no test covered.

### Security
- **The edit endpoint answered requests that came from the network.**
  The preview server keeps the documentation port open so a developer
  can look at the site from another machine, and the endpoint that
  opens a file in the editor was on that port. Opening a file runs a
  command on the machine, so anyone who could reach the port could make
  the editor open a file. The endpoint now answers only requests that
  come from the machine itself.

- **The CI actions were pinned to tags.** A tag can be moved, so the
  code that runs in CI - with the repository's token - was not
  necessarily the code that was reviewed. Every action is pinned to the
  commit its tag names now, with the tag kept beside it as a comment.

### Fixed
- **A repository whose documentation lives at its root never rebuilt on
  a push.** The webhook filter compares the changed files with the
  documentation paths of the repository. The path that means "the whole
  repository" is `.`, and no file name starts with `./`, so every push
  was ignored and the site stayed stale. A repository configured with
  `paths: ["."]` is rebuilt now, for every change.

- **A marker in a page was quietly removed.** The content pipeline had
  a transform that looked for an `@glossary` marker, deleted it and
  generated nothing in its place. A page that used the marker lost it
  without gaining a glossary. The transform is gone.

- **The README promised a report field and three metrics that do not
  exist.** The delta analyzer that filled them in went with the unused
  code in v0.28.0, and the feature list still named them. It names the
  metrics the Prometheus endpoint really exports now.

### Changed
- **The status snapshot has no `cpu_usage_percent` field.** It was
  never filled in, so it reported a flat zero that read like a
  measurement. A reader that expects the field has to read the
  Prometheus endpoint instead, where `process_cpu_seconds_total` is the
  real number.

### Removed
- **A build stage that did nothing.** The pipeline had an "indexes"
  stage that returned without work: the indexes are generated while
  the content is copied. The stage is gone, a build no longer reports
  it, and the documentation lists the stages the code really runs.

- **A CPU usage that was always zero.** The status snapshot carried a
  `cpu_usage_percent` field that was never filled in, so it reported a
  flat zero that read like a measurement. The Prometheus endpoint has
  the real numbers (`process_cpu_seconds_total`), and the field is
  gone.

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
