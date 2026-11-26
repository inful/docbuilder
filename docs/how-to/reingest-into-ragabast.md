---
categories:
  - how-to
date: 2026-11-26T00:00:00Z
tags:
  - ragabast
  - ingest
  - daemon
title: 'How To: Reingest The Corpus Into Ragabast'
lastmod: "2026-11-26"
aliases:
  - /_uid/08f012ea-eca0-4c24-a4b6-1755aa4d23fb/
uid: 08f012ea-eca0-4c24-a4b6-1755aa4d23fb
fingerprint: 8ea6325b1bb592b0f542714e261e574a2a2e4603f29675436f62d66502c51989
---

Ragabast skips a document whose fingerprint did not change, and the
daemon skips the document it uploaded before. A reingestion moves the
epoch of the ingest: every document then carries a fingerprint that
ragabast has not seen, and ragabast embeds the whole corpus again.

## When you need it

- The embedder of ragabast changed. The vectors of the corpus were made
  by a model that is gone, and the search answers with them.
- Ragabast lost its corpus. Its database was wiped, or it was restored
  from a backup that does not hold the documents.

## Asking for a reingestion

The reingestion is a request to the admin API of the daemon:

```bash
curl -X POST http://<daemon-host>:<admin_port>/api/ragabast/reingest
```

The answer names the build that carries the corpus:

```json
{
  "status": "triggered",
  "job_id": "manual-1764130000000000000"
}
```

The build runs even when nothing moved: the corpus is what is sent to
ragabast, not the site. The build sends every document, and ragabast
embeds every document again. The price is one build and one upload per
document, and the embedding of the whole corpus on the ragabast side.

## What it does not do

- It does not change the documents. The fingerprint of a document stays
  the fingerprint of its content; only the copy that goes to ragabast
  carries the epoch.
- It does not delete anything in ragabast. The documents of the corpus
  are replaced, not duplicated: ragabast drops the chunks of a document
  before it embeds it again.
- It does not work without ragabast. The daemon answers that the ingest
  is not configured when `daemon.outbound.ragabast` is missing.

## The epoch

The epoch is a counter in the state directory of the daemon
(`ragabast-ingest-epoch.json`). It moves with every reingestion, and it
survives a restart of the daemon: without it, every restart would send
the whole corpus again.

Losing the state directory resets the epoch to zero. The corpus is then
sent once more, because ragabast holds fingerprints of an epoch that is
not the current one, and the epoch settles again.
