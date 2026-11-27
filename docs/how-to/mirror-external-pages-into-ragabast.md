---
categories:
  - how-to
date: 2026-11-27T00:00:00Z
tags:
  - ragabast
  - ingest
  - daemon
title: 'How To: Mirror The External Pages Of A Document Into Ragabast'
lastmod: "2026-11-27"
aliases:
  - /_uid/d01b1ba5-656d-4584-819c-ccc29b4c5e6e/
uid: d01b1ba5-656d-4584-819c-ccc29b4c5e6e
fingerprint: 2e6df226e2dc811d2a2b5e246320c1e9aa8c8a8c3f964db7c61cc0415949350c
---

A document often belongs with a page that lives elsewhere: a
specification, an API reference, a changelog. Ragabast can mirror those
pages into the corpus, and a question about them is then answered with
the page itself and not only with the words of the document.

## The request

The document asks for the pages in its front matter:

```yaml
---
uid: 3f2a1b4c-5d6e-4f70-8192-a3b4c5d6e7f8
title: Semantic Versioning
categories:
  - reference
tags: []
web_sources:
  - https://semver.org/spec/v2.0.0.html
---
```

The request travels with the document. The daemon uploads the document
as it is, and ragabast reads the field out of it. Nothing else has to
be configured on the docbuilder side.

The field is a list of http(s) URLs. A single URL written as a string is
not read, and the lint says so:

```
ERROR: The web sources must be a list
```

`docbuilder lint --fix` turns a lone URL into a list of one. An entry
that is not an http(s) URL is reported and left to the author: the
fixer does not guess a scheme and does not drop a page the author asked
for.

## The allowance

The request is the author's decision; what may be fetched is the
operator's. Ragabast fetches a page only when its host is on the list
of `web_ingest.allowed_hosts`:

```yaml
# The configuration of ragabast
web_ingest:
  allowed_hosts:
    - semver.org
```

An empty list means the feature is off: ragabast never fetches
anything. A document that names a host the operator did not allow is
ingested all the same — the mirror simply does not exist.

## Checking the mirrors

Ragabast answers with the effective list: the pages that were
requested, allowed and fetched, with the documents that asked for each
one and the outcome of the last fetch.

```bash
curl http://<ragabast-host>:<port>/api/web-mirrors
```

```json
{
  "allowed_hosts": ["semver.org"],
  "mirrors": [
    {
      "source_url": "https://semver.org/spec/v2.0.0.html",
      "uid": "web-...",
      "title": "Semantic Versioning 2.0.0",
      "declared_by": ["3f2a1b4c-5d6e-4f70-8192-a3b4c5d6e7f8"],
      "fetched_at": "2026-11-27T10:00:00Z"
    }
  ]
}
```

A page that is missing from the list was either not requested or not
allowed. The `error` field of an entry says why the last fetch failed.

## What it does not do

- It does not fetch anything on its own. The pages are fetched when the
  document that asks for them is ingested, and again when ragabast
  refreshes them.
- It does not change the document. The page is a mirror next to the
  document, not a part of it.
- It does not work without ragabast. The daemon holds the corpus, and
  the corpus is what is sent to ragabast. See [How To: Reingest The
  Corpus Into Ragabast](reingest-into-ragabast.md).
