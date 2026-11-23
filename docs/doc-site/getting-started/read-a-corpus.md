---
title: Read a corpus
weight: 10
description: |
  Read a corpus file from any language: the header, the cases, and what to check before scoring.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Where do I get a corpus file, and which tier do I want?
- How do I read it without Go?
- What must I check before I score anything?
- How do I turn a case into an expected outcome without the Go `stance` package?

## Sections

- Where corpora come from: released assets (not yet), `jsoncorpus.Full().Write` today, the embedded compact corpus
- Tiers: compact and full, with counts from `metrics.yaml`
- The layout in one screen: gzip, JSON Lines, header on line 1, one case per line, `src` in base64
- Reading it with `jq`: `gunzip -c`, `jq -c`, `@base64d` -- one command per question (how many cases, which tags)
- Reading it in another language: a short Python example
- Check the vocabulary first: every tag in `header.vocabulary` must be one you have a position on
- From case to outcome by hand: `wellFormed`, `tags`, `verdictAt`, `opaque`, and the `settled` field of a tag spec
- Pointer to [File format](../../reference/file-format/) for every field

{{% notice style="info" %}}
The YAML Test Suite view (a jq filter producing `in.yaml`, `in.json` and `error`) arrives with the YAML corpus.
{{% /notice %}}
