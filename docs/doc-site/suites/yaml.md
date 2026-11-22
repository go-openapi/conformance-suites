---
title: YAML
weight: 20
description: |
  YAML 1.2: the recognizer is here; the corpus and its generator are moving in from go-openapi/go-yaml.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- What ships today for YAML, and what is still in go-yaml?
- How was the recognizer calibrated?
- What will the YAML corpus hold?

## Sections

- Today: `grammar.YAML`, `grammar.Stream`, calibration against the vendored YAML Test Suite
- Moving in: `yamlcorpus`, `yamlgen`, and the `yaml/corpus` module holding the embedded corpus
- The vocabulary: 54 YAML tags in `anchor/*`, `tag/*`, `merge/*`, `key/*`, `directive/*`, `schema/*`, `separation/tab`
- Readings: `yaml-1.2-core`, `yaml-1.1`
- The YAML Test Suite view: the jq filter

{{% notice style="info" %}}
Written in full after the YAML side moves in.
{{% /notice %}}
