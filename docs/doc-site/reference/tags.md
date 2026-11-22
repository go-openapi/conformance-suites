---
title: Tags
weight: 20
description: |
  Every tag, its stage, and whether the specification settles it.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Which tags exist for each grammar?
- At which stage does each one arise?
- Which ones are settled, and by which clause?

## Sections

- Shared: `encoding/bom`, `encoding/not-utf8`, `encoding/utf16`, `encoding/utf32`
- JSON: `number/out-of-range`, `string/lone-surrogate`, `structure/deep-nesting`
- YAML: after the move
- One row per tag: name, stage, settled, what exhibits it

{{% notice style="info" %}}
This page will be generated from each grammar's `Vocabulary()` by a tool under `hack/`.
{{% /notice %}}
