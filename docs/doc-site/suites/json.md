---
title: JSON
weight: 10
description: |
  RFC 8259, compiled with no patches: the generator, the mutations, the tags, and `DefaultLexer`.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- What does the JSON suite test, and what does it leave out?
- Which tags does it use, and at which stage?
- How were the `DefaultLexer` positions found?
- What would JSON need to ship as a full peer of YAML?

## Sections

- The grammar: `jsoncorpus.JSON`, `jsoncorpus.Text`; no patches
- The generator: `RandomValue`, `RandomStyle`, `Emit`; the {{% siteparam "metrics.json_mutations" %}} mutations
- Tags ({{% siteparam "metrics.json_tags" %}}): `number/out-of-range`, `string/lone-surrogate`, `structure/deep-nesting`,
  and the four `encoding/*` tags; link to [Tags](../../reference/tags/)
- `Normalize`: what a parser may strip before the grammar sees the bytes
- Corpus counts: smoke and full, from `metrics.yaml`
- `DefaultLexer`: measured, not declared; the two surprises (BOM tolerated, lone surrogate refused)
- Gaps: duplicate members, number precision, control characters, trailing data, empty input
