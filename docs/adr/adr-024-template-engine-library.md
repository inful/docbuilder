---
aliases:
  - /_uid/e1aa1f91-0bc0-4884-bc2b-fc88196af56a/
categories:
  - architecture-decisions
date: 2026-11-25T00:00:00Z
lastmod: "2026-11-25"
tags:
  - templates
  - library
  - api
  - architecture
title: 'ADR-024: The template engine as a library'
uid: e1aa1f91-0bc0-4884-bc2b-fc88196af56a
fingerprint: a3ba3fdf40ee5bea986a3e9abda554a7a0e2386c31bd63a4e2fdfbc3dc5ad50e
---

## Status

Accepted. Implemented on 2026-11-25.

## Context

The template engine of DocBuilder - the part that discovers a
template on a documentation site, parses it, asks for its inputs,
renders it and writes the file - lived in `internal/templates`, with
the tri-state boolean field in `internal/doctemplate/field` and the
index that suggests file names in `internal/doctemplate/suggest`.

The engine was already a library in everything but name. It is pure
`text/template` and `x/net/html`, its API is shaped for an outside
caller (the `Prompter` interface is an interface, so the terminal UI
can be replaced), and it depends on the application in exactly two
places: an atomic write and the redaction of the credentials of an
address.

The one thing that stopped it from being used elsewhere was the name
of the directory. Go refuses an import of a package under `internal/`
from another module, so an application outside DocBuilder could not
use the engine at all - not even by copying the code, because the
copy would drift.

## Decision

The engine moves to a public path in the module of DocBuilder:

| Before                          | After                                    |
|---------------------------------|------------------------------------------|
| `internal/templates`            | `templates`                              |
| `internal/doctemplate/field`    | `templates/field`                        |
| `internal/doctemplate/suggest`  | `templates/suggest`                      |

The import paths become:

```
git.home.luguber.info/inful/docbuilder/templates
git.home.luguber.info/inful/docbuilder/templates/field
git.home.luguber.info/inful/docbuilder/templates/suggest
```

The package names do not change, so the code of a consumer does not
change either - only the import lines.

The library keeps no dependency on the packages of the application:

- `foundation.WriteFileAtomic` becomes an unexported `writeFileAtomic`
  inside the library. The helper of the foundation package stays for
  the callers of the linter and the MCP server.
- `logfields.RedactURL` becomes an unexported `redactURL` inside the
  library. The helper of the logfields package stays for its callers.

This is the only duplication the move creates, and it is small: two
helpers of 40 and 8 lines. The alternative - a new public package for
the atomic write - would grow the public surface of the library for
the sake of one function.

What stays in DocBuilder:

- The terminal UI (`internal/doctemplate/app`), which is an
  application of the library and not a part of it.
- The lint-fix step of the service, which drags the whole linter into
  the dependency closure of the library. A consumer that wants the
  lint runs it itself.

The exported surface ships as it is. The exported types, functions
and constants of the three packages become the public API of the
library, documented in `templates/README.md`, without a promise of
stability: the module is `0.x`, and a change that breaks a consumer
needs a version bump.

### The later move into its own module

The library lives in the module of DocBuilder today, because a second
module means a second repository, a second release line and a
pseudo-version in the dependency of DocBuilder - a library bump would
force a release of the application.

When the library needs a version of its own, it moves into the module
`git.home.luguber.info/inful/doctemplate`, and the import path of a
consumer changes by one segment:

```
git.home.luguber.info/inful/docbuilder/templates
    → git.home.luguber.info/inful/doctemplate/templates
```

Go makes a move that costs the consumer nothing impossible: the path
of the module is a part of every import. The ADR names the future
path now, so that the later move is mechanical.

## Consequences

- Another application can use the engine without copying it: the
  engine is a package of a public module.
- DocBuilder consumes the library as a package like any other. The
  move changes no behaviour: the same code, at a different path.
- The library and the application drift apart on their own: the
  library takes no dependency on the application, so a change in the
  linter or in the UI cannot break it.
- The public surface of DocBuilder grows by the exported symbols of the
  three packages. They are the promise of a `0.x` module, and the
  README says which of them are the entry points.
- A consumer that wants the lint after the write runs the linter
  itself.

## Alternatives considered

1. **Its own module from the start.** The consumer would pin the
   version of the library and not the version of DocBuilder. The
   price: a second repository, a second release line, and DocBuilder
   consuming the library through a pseudo-version - every bump of the
   library would need a release of DocBuilder. Not worth it before
   there is a second consumer.

2. **A facade of a few functions over the engine.** The public
   surface would be small and deliberate. The price: the design of
   the facade is a guess about what the consumers need, and the
   engine's API is already shaped for them. The facade would be
   removed later, and it would be another thing to keep in step with
   the engine.

3. **Trimming the exported surface before publishing it.** The
   exported symbols of a package of `internal/` were never a promise.
   Publishing them makes them one. Trimming them first would have
   meant guessing which of them nobody needs, and the consumers of
   the engine are not known yet.

## Acceptance criteria

- `templates`, `templates/field` and `templates/suggest` hold the
  engine, and no package of `internal/` holds a part of it.
- The library imports no package of `internal/`.
- The `doctemplate` command, the `docbuilder template` command and the
  MCP server use the library at its new path, and behave as before.
- `scripts/check-library-import.sh` builds a module outside the
  repository that imports the library, and the CI runs it.
- `templates/README.md` holds the purpose of the library, its exported
  symbols and an example.
