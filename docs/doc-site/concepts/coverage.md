---
title: Coverage and selection
weight: 30
description: |
  How the builder decides which generated documents to keep, and why the compact corpus keeps 16 refusals per signature.
---

A build draws far more documents than a corpus can hold. This page explains what the oracle records while it
reads a document, and how the builder uses that record to choose the documents to keep.

## What the oracle records

Given a `Coverage` vector, the oracle counts each production it enters while it reads a document. A bucket is
not the production alone:

- **YAML**: the production, the context it was entered in (`block-in`, `flow-key`, ...), and the indentation
  `n` and `m`, each reduced to nine classes.
- **JSON**: the production. JSON has no context and no indentation.

The context is part of the bucket because the same rule decides differently in `block-in` and in `flow-key`.
The indentation is part of it because two documents that differ only in how far they are indented once produced
the same record, and the builder kept one of them.

Each bucket counts attempts and successes apart, so a backtracked attempt still counts as an entry.

```go
rec := yamlcorpus.NewRecognizer(4096)
cov := yamlcorpus.NewCoverage()
rec.Cover(cov)

for _, doc := range docs {
	rec.Stream(doc)
}
```

`Reach` computes the buckets that can be entered at all, from a start production in a context. From a YAML
stream, {{% siteparam "metrics.yaml_buckets" %}} production-by-context buckets are reachable, and the compact
corpus enters every one. A test fails if a document enters a bucket `Reach` declares unreachable.

## Signatures

A document's **signature** is a digest of the set of buckets it entered. Two documents with one signature took
the same route through the grammar. For a refused document, the route is close to a statement of how the
document is wrong.

The signature that put a case in the corpus is stored in `origin.signature`. Bucket numbers change when the
grammar changes, so the corpus stores digests and documents, never vectors.

## How a build keeps a document

`Generate(seed, documents, mutantsEach)` draws valid documents and breaks each one `mutantsEach` times. The
builder then reads each document with the oracle and keeps it if any of these holds:

1. it enters a bucket no kept document has entered;
2. its signature has fewer than `PerSignature` kept documents;
3. it holds a combination of constructs no kept document holds, such as a tab, an escape or a line break style
   (in JSON, this applies to valid documents only);
4. in YAML, it carries a [meaning](../meanings/).

The construct set catches what the route cannot. The compact YAML corpus enters every reachable bucket, so a new
document rarely reaches a new one, and a document holding a tab takes the same route as one holding a space.

A document with a meaning is never dropped. Two documents can take the same route and denote different values:
they are one test for a recognizer and two for a decoder. A mutant carries no meaning, because nobody knows what
it denotes, so the quota applies to it.

The **enumerated shapes** go in before any drawn document and are never dropped: the JSON encoding and value
shapes, and the YAML families such as anchor patterns and schema scalars. No coverage signal points at them. A
byte order mark appears in no grammar production, and an alias enters the same productions whether its anchor
exists or not.

## Tiers

A tier is a `Build` recipe. The three tiers draw from the same seed and differ in how many documents they draw
and how many they keep:

| tier | JSON | YAML |
|---|---|---|
| compact | `Compact()`: 8,000 documents, 24 mutants each, up to {{% siteparam "metrics.json_per_signature" %}} per signature: {{% siteparam "metrics.json_compact_cases" %}} cases | `Compact()`: 3,000 documents, 16 mutants each, up to {{% siteparam "metrics.yaml_per_signature" %}} per signature: {{% siteparam "metrics.yaml_compact_cases" %}} cases |
| extended | 40,000 documents, 24 mutants each, up to 16 per signature: {{% siteparam "metrics.json_extended_cases" %}} cases | 15,000 documents, 16 mutants each, up to 16 per signature: {{% siteparam "metrics.yaml_extended_cases" %}} cases, {{% siteparam "metrics.yaml_extended_size" %}} |
| full | `Full()`: the compact draw, every document kept: {{% siteparam "metrics.json_full_cases" %}} cases | `Full()`: 6,000 documents, 12 mutants each, every document kept |

- **Compact** lives in the repository, and the Go modules embed it. Every test run rebuilds it and compares the
  result with the stored file.
- **Extended** draws five times the compact documents and keeps the same quota, so it reaches the shapes the
  generator draws rarely. The quota caps the refused documents, so five times the documents gives about three times
  the JSON cases and four times the YAML ones. Run against the YAML extended tier on 2026-11-23, go-yaml showed
  seven candidate defects that the compact tier had missed. Extended is written as a `Build` literal for now, not as a function;
  it is meant to ship with each release.
- **Full** drops nothing, valid documents included. No build calls `Full()` today.

```go
err := yamlbuild.Build{Tier: "extended", Seed: 1, Documents: 15000, MutantsEach: 16, PerSignature: 16}.Write(w)
```

A build reads no clock and calls no parser, so the same recipe writes the same bytes every time.

## Why sixteen per signature

A signature is how the oracle sees a document. Keeping one document per signature throws away everything two
documents differ by that the oracle cannot see, and parsers fail in exactly those places.

The quota was set by replaying each build through go-yaml and counting the distinct error messages it produced:

| documents kept per signature | 1 | 4 | 16 | 32 |
|---|---|---|---|---|
| distinct messages | 14 | 19 | 22 | 23 |

Going from 16 to 32 found one more message and cost 37 KB. Drawing more documents found more than raising the
quota did, because most messages appear in about one document in twenty thousand. So the compact build keeps 16
per signature and draws more documents.
