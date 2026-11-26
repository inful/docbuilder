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
fingerprint: 730a62ce893e9995990eef331a297b772e3fb2b356642539035fd7388417818f
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

The command asks the daemon that runs the site:

```bash
docbuilder reingest --config config.yaml
```

It answers with the build that carries the corpus:

```
Reingestion requested: the build manual-1764130000000000000 carries the corpus to ragabast.
```

The address of the daemon comes from, in this order:

1. `--admin-url`, or the environment variable `DOCBUILDER_ADMIN_URL`:
   the whole address.
2. The configuration: the port and the bind address of `daemon.http`.
   The values may hold `${VAR}` references, and the environment and the
   `.env` files fill them:

   ```yaml
   daemon:
     http:
       admin_port: ${ADMIN_PORT}
   ```

   ```bash
   # .env
   ADMIN_PORT=8082
   ```

3. The default: the loopback interface and the port `8082`.

A daemon on another host is named with `--admin-url`:

```bash
docbuilder reingest --admin-url http://ragabast-daemon:8082
```

The admin API of the daemon answers the request itself:

```bash
curl -X POST http://<daemon-host>:<admin_port>/api/ragabast/reingest
```

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
