---
title: The oracle
weight: 10
description: |
  A recognizer compiled from the grammar each specification publishes, used to label every generated document.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Why does a generated corpus need an oracle, and why not an existing parser?
- Where do the grammars come from?
- What does the oracle return, and what does it not return?
- Where does the YAML grammar need patches, and why does JSON need none?

## Sections

- The labelling problem: a mutated YAML document is usually still valid YAML
- Why not a parser: the corpus would score every parser against one parser's bugs
- Sources: `yaml-spec-1.2.json` from `yaml/yaml-grammar` ({{% siteparam "metrics.yaml_rules" %}} productions),
  `rfc8259.json` transcribed from RFC 8259 ({{% siteparam "metrics.json_rules" %}} rules)
- What `Result` holds; no tree, no error messages
- Patches: the YAML rules the spec states only in prose
- Cost in one paragraph: compiled to closures, memoized
- Summary of `internal/grammar/doc.go`, with links to pkg.go.dev for out-parameters, measured indentation and memoization
