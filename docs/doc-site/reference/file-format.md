---
title: File format
weight: 10
description: |
  Format 3: every field of the header and of a case, and the compatibility rules.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- What is every field, its type, and when is it absent?
- How do I tell that a file is truncated or stale?
- What may change between format versions, and what may not?

## Sections

- Container: gzip, JSON Lines, UTF-8; `suite.Format`
- Header: `format`, `grammar`, `digest`, `generator`, `seed`, `tier`, `cases`, `vocabulary`, `readings`, `features`
- Tag spec: `tag`, `stage`, `settled`, `because`
- Case: `name`, `src`, `wellFormed`, `opaque`, `tags`, `features`, `verdictAt`, `meaning`, `meanings`, `origin`
- Origin: `document`, `mutation`, `signature`
- Meaning: `under`, `json`, `cyclic`
- Checks: `cases` against the line count, `digest` against the grammar
- Go: `suite.Reader`, `suite.Writer`, `suite.ReadAll`, `suite.FromBytes`
- Stability: what a reader of format 3 can rely on

{{% notice style="warning" %}}
Do not document case names or seed reproduction as stable. Both change when the corpus becomes append-only.
{{% /notice %}}
