---
aliases:
  - /_uid/fc3644e7-a7b4-42d6-9174-db359b9eb05d/
categories:
  - explanation
date: 2026-11-22T00:00:00Z
fingerprint: 9387056f1993b0d1bd1fabbb7fd4bd87fab23ec7fe2711f3badb0e0566f3e875
lastmod: "2026-11-22"
tags:
  - search
  - performance
title: Server-side Search
uid: fc3644e7-a7b4-42d6-9174-db359b9eb05d
---

## Overview

The search of the site runs on the server. The daemon keeps an index of
the rendered pages and answers the queries; the visitor's browser asks
for the results and never sees the index.

The theme it replaces built the index in the browser. Hugo wrote the
text of every page into a JSON file, the browser downloaded that file
and built a search index on the reader's machine. The file holds the
full text of the site: on a site with a few hundred pages it is already
close to a megabyte, and it grows with every page. Every page view paid
for the download, the parse and the index build.

## The two providers

| Provider | Where the search runs | When to use it |
|----------|-----------------------|----------------|
| `server` | The daemon answers the query from its own index | The daemon serves the site. This is the default when a daemon is configured. |
| `lunr` | The browser searches the index the theme ships | The site is served as a static tree - exported and handed to a web server - and has no server to ask. |

```yaml
search:
  provider: server
  max_results: 10
```

The provider decides two things: what the site ships, and what the
daemon does.

- With `server`, the site ships its own search client and an empty
  index file. The theme's index is never built, so Hugo does not spend
  the memory on it either.
- With `lunr`, the site ships the theme's client and the theme's index,
  and the daemon keeps no index.

## How the index is built

The daemon indexes the pages after every successful build. It reads the
transformed markdown of the build - the same files Hugo rendered - and
takes from each of them:

- the address the page is served at,
- the title, the categories and the tags from the front matter,
- the text of the page.

Only pages that were actually rendered are indexed: a page that is not
in the rendered tree cannot be reached, so it must not show up in a
search result. A page that sets `exclude_search: true` in its front
matter is left out as well.

The taxonomy pages Hugo generates - `/tags/<tag>/`, `/categories/<category>/`
- are not part of the content tree and are therefore not indexed. They
repeat the text of every page that carries the tag, and in a site with
many tags they were more than half of what the browser-side search
indexed. A search for a word that appears in a tag name no longer
returns a page that lists every page with that tag.

The index lives in the daemon's state directory, so it survives a
restart: the daemon answers a query right away, without waiting for the
next build. Every build replaces the index as a whole, so a page that
is gone from the site is gone from the index with it.

## The endpoint

The docs server answers queries at `/api/search`:

```console
$ curl 'http://localhost:8080/api/search?q=install&limit=10'
{
  "query": "install",
  "results": [
    {
      "url": "/how-to/install-the-daemon/",
      "title": "Installing the daemon",
      "excerpt": "Install the daemon with the package manager on Linux.",
      "score": 0.938627669854439
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `query` | The query that was asked. |
| `results` | The pages that match it, the best match first. |
| `results[].url` | The address of the page. |
| `results[].title` | The title of the page. |
| `results[].excerpt` | The passage of the page that matched. |
| `results[].score` | How well the page matches. Bigger is better. |

An empty query is answered with an empty list, not with an error.

## How the results are found

The index is a SQLite FTS5 table. SQLite is already part of the
binary, so the search costs no extra dependency.

- Every word of the query has to appear somewhere in the page.
- The words are stemmed, so "installing" finds "install".
- The last word may be a prefix, so "kuber" finds "Kubernetes" while
  the reader is still typing it.
- A hit in the title counts more than a hit in the body. The ranking is
  BM25, the same measure the search engines use.

## The query language

A query is read as it is written, so the reader can narrow the search
down.

| Written | Means |
|---------|-------|
| `daemon kubernetes` | Both terms have to be in the page. |
| `daemon AND kubernetes` | The same, written out. |
| `daemon OR kubernetes` | Either term is enough. |
| `daemon NOT kubernetes` | The first term, without the second. |
| `"package manager"` | The words, in that order, next to each other. |
| `title:kubernetes` | The term has to be in the title. |
| `body:kubernetes` | The term has to be in the text of the page. |
| `tags:adr` | The term has to be in the tags of the page. |
| `categories:how-to` | The term has to be in the category of the page. |
| `kuber*` | The words that start with it. |

The operators are the words of the index, so they are written in upper
case: a lower case "and" is a word the pages have to hold.

The words are stemmed, and any term may be a prefix - the last term of
a query gets one on its own, so the reader finds the pages while they
are still typing.

A query the index cannot read - a half-written one, most of the time -
is searched the forgiving way: every word of the query has to appear
somewhere in the page. A search box holds half-written queries, and
half a query still finds the pages it can.

## The limits

- There is no typo tolerance beyond the prefix of a term.
- There are no facets: the results cannot be filtered by category or
  repository.
- The index holds the text of the pages, not their code blocks in
  their original form. The words of a code block are searchable, but
  not its punctuation.
- A query is at most 256 characters long. A longer one is cut short:
  a search box is for words, not for programs.

A stronger engine - one with fuzzy matching and facets - can replace the
index behind the same endpoint. The endpoint, the client and the way the
index is built do not change.
