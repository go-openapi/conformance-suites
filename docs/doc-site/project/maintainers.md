---
title: Maintainers
weight: 30
description: |
  Package layout, where new code goes, and how a release is cut.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Where does a new grammar, generator or helper go?
- Which packages are public, and why?
- When does a second module appear?
- How is a corpus regenerated, and how is a release cut?

## Sections

- One directory per grammar; `suite`, `stance`, `grammar` beside them
- Public: `suite` and `stance` (a consumer reads a file and declares a table); the oracle's result types
- The engine moves to the module's `internal` once the YAML side is here; grammar packages re-export
  `type Result = grammar.Result`
- Package names carry the grammar: `jsoncorpus`, `yamlcorpus`, `yamlgen`
- No parser imported in the root module, tests included; `consensus` is a module of its own
- One module until a dependency or an embed forces a split (`yaml/corpus`)
- Regenerating the smoke corpus, and the regenerate-and-diff test
- Updating `metrics.yaml`
- Releasing: `bump-release.yml` or a signed semver tag; corpus assets once released
