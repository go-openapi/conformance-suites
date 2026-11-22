---
title: Coverage and selection
weight: 30
description: |
  How the builder decides which generated documents to keep, and why the smoke corpus keeps 16 refusals per signature.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- What does the oracle record as coverage?
- Which documents does a smoke build keep, and which does a full build keep?
- What is a failure signature?
- Why does keeping one document per signature lose bugs?

## Sections

- Coverage buckets: production, and for YAML production by context; `Coverage`
- Generation: `jsoncorpus.Generate`, seed, documents, mutants per document; the mutations
- Smoke: valid documents that add coverage, `PerSignature: 16` refused documents, every encoding shape
- Full: every document ({{% siteparam "metrics.json_full_cases" %}} for JSON)
- Signatures: `Origin.Signature`
- The measurement: one per signature drops the known JSON colon defect, sixteen keeps it
- Encoding shapes: `stance.EncodingShapes`, why no coverage signal points at a byte order mark
