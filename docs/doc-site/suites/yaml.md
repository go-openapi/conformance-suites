---
title: YAML
weight: 20
description: |
  YAML 1.2, patched for the rules the spec states in prose: the generator, the mutations, the tags and features,
  the readings, and the parsers the corpus is checked against.
---

The YAML suite tests whether a parser accepts exactly the documents YAML 1.2 admits, holds it to the positions it
declares on the questions the specification leaves open, and states what each valid document means.

| package | module | holds |
|---|---|---|
| `yamlcorpus` | `github.com/go-openapi/conformance-suites` (directory `yaml/`) | the oracle |
| `yamlgen` | `github.com/go-openapi/conformance-suites/yaml/yamlgen` | the generator: values, styles, the emitter, mutations for property tests |
| `yamlbuild` | the same module, `yaml/yamlgen/yamlbuild` | the corpus builder and the tag vocabulary |
| `yamlembed` | `github.com/go-openapi/conformance-suites/yaml/corpus` | the compact corpus, embedded: 1.6 MB, so it has a module of its own |

## The grammar

`yamlcorpus.YAML` is the grammar that [`yaml/yaml-grammar`](https://github.com/yaml/yaml-grammar) publishes, with
{{% siteparam "metrics.yaml_rules" %}} productions, compiled with {{% siteparam "metrics.yaml_patches" %}} patches
for rules the specification states only in prose. `yamlcorpus.Stream` checks one stream:

```go
ok := yamlcorpus.Stream(src).OK
```

[The oracle](../../concepts/oracle/) lists the patches, and the calibration against the YAML Test Suite:
{{% siteparam "metrics.yaml_yts_accept" %}} fixtures that must be accepted and
{{% siteparam "metrics.yaml_yts_reject" %}} that must be rejected, with {{% siteparam "metrics.yaml_yts_departures" %}}
declared departures.

## The generator

`yamlgen` draws a value first, then a way to write it:

- `Values` draws a value tree: scalars of every core type, sequences, mappings, mappings keyed by collections, and
  anchors with aliases in about a quarter of the documents.
- `Styles` draws a `Style`: block or flow, the indentation, the quoting, literal or folded scalars and their
  chomping, comments, line breaks, tags and how to spell them, number and time forms, escapes, directives, explicit
  keys, a byte order mark, and `%YAML 1.1`.
- `Emit` writes the value in that style. Every document it writes is valid YAML 1.2.

The value comes first, so the builder knows what each valid document means: see
[Meanings](../../concepts/meanings/).

`yamlbuild.Generate` draws through rapid's `Example`, which produces a value from a seed, so a seed rebuilds the
corpus byte for byte. Every test run rebuilds the compact corpus and compares it with the stored file, so a rapid
upgrade that changes the draws fails the build.

### Mutations

The builder breaks each document with one of {{% siteparam "metrics.yaml_mutations" %}} mutations: delete a
byte, delete a run, duplicate a byte, swap two neighbors, truncate, substitute a byte, substitute an indicator,
insert an indicator, indent a line further, outdent a line.

The byte mutations know nothing about YAML, on purpose: a mutation that knew the grammar would make only the
errors someone already thought of. The two indentation mutations are the exception. A random byte rarely lands in
the leading spaces of a line, and indentation is where parsers and this recognizer have had their bugs.

Most mutants are still valid YAML. The oracle sorts them, and the corpus keeps the ones that bring a new route
through the grammar: see [Coverage](../../concepts/coverage/).

`yamlgen.Mutate`, used in [property tests](../../getting-started/fuzz-and-properties/), is a separate set aimed at
YAML's structure: a tab in the indentation, a line moved sideways, a document marker inside a scalar. The corpus
builder does not use it.

### Rules broken on purpose

Some documents are valid by the grammar and still wrong, and a byte mutation almost never writes one. The builder
writes them with {{% siteparam "metrics.yaml_rule_breaks" %}} rewrites of a valid value, each with its tag:

| rewrite | tag |
|---|---|
| alias renamed to nothing | `anchor/alias-undefined` |
| a key repeated | `key/duplicate` |
| a duplicate entry appended | `key/duplicate` |
| a collection key written twice | `key/duplicate` |

### Enumerated shapes

{{% siteparam "metrics.yaml_compact_shapes" %}} cases are written by hand, named `shape/<family>/<what>`, and never
dropped by the builder. No coverage signal points at them: an alias enters the same productions whether its anchor
exists or not, and every plain scalar enters the same productions whatever it denotes. The families:

| family | covers |
|---|---|
| `anchor` | aliases undefined, forward, across documents and as keys; anchors redefined, unused or on an empty node; cycles |
| `directive` | repeated `%YAML` and `%TAG`, versions, reserved directives, handles out of scope |
| `escape` | an escape of the last code point, and one past it |
| `indentation`, `separation` | tabs between a property and its node, and tabs opening a block scalar's content |
| `key` | duplicates, before and after resolution; keys that are booleans, null, floats, tagged, aliases or collections |
| `merge` | `<<` keys: from an alias or a mapping written in place, a sequence of merges, overridden, quoted, alone |
| `reach` | productions the generator rarely reaches: explicit keys in flow, kept trailing breaks, more-indented folded lines |
| `schema` | plain scalars the YAML 1.1 and 1.2 schemas resolve differently: `yes`, `0777`, `1_000` |
| `tag` | verbatim, local, non-specific and secondary tags; handles and percent escapes; tags whose content does not fit, such as `!!int` on a word; `!!omap` |

## Encoding

The oracle reads UTF-8. A document that starts with a byte order mark carries `encoding/bom`. A document whose bytes
are not valid UTF-8, almost always a mutant, carries `encoding/not-utf8` and is marked `opaque`: the compact corpus
holds {{% siteparam "metrics.yaml_compact_opaque" %}} of them. The YAML generator writes no UTF-16 or UTF-32, so the
corpus carries no tag for them.

## Tags

The YAML vocabulary has {{% siteparam "metrics.yaml_tags" %}} tags. The specification settles
{{% siteparam "metrics.yaml_tags_reject" %}} of them as a rejection and {{% siteparam "metrics.yaml_tags_accept" %}}
as an acceptance, and leaves {{% siteparam "metrics.yaml_tags_open" %}} open:

| family | stages | the questions |
|---|---|---|
| `anchor/*` | parse, compose, construct | undefined and forward aliases (settled: reject), aliases as keys and redefined anchors (settled: accept), cyclic values (open) |
| `directive/*` | parse, compose | repeated directives (settled: reject), a later minor version (open), reserved directives (settled: accept) |
| `encoding/*` | parse | a byte order mark, bytes that are not UTF-8 |
| `escape/*`, `indentation/*`, `separation/*` | parse, compose | escapes past U+10FFFF (settled: reject), tabs |
| `key/*` | parse, compose, construct | duplicate keys (settled: reject), keys that are not strings or not scalars (open) |
| `merge/*` | construct | every form of `<<`: YAML 1.2 dropped merge keys, and many parsers still apply them |
| `representation/*`, `json/*` | construct | values a native type may not hold: wide integers, out-of-range floats, binary, timestamps, ordered maps |
| `schema/*` | construct | plain scalars the schemas resolve differently |
| `tag/*` | parse, compose, construct | undeclared handles (settled: reject), verbatim and non-specific tags (settled: accept), tags on content that does not fit them (open) |

[Tags](../../reference/tags/) lists every tag with its stage, its settled outcome and its description.

## Features

A YAML case also lists **features**: the constructs its document holds, from `presentation/block-literal` to
`node/anchor` and `value/merge-key`. The header lists all {{% siteparam "metrics.yaml_features" %}}. Features never
change an expected outcome. Use them to select a subset of the corpus, or to see what a failing document holds:

```sh
gunzip -c yaml-compact.jsonl.gz | tail -n +2 | jq -c 'select(.features | index("presentation/block-folded"))'
```

## Readings and meanings

{{% siteparam "metrics.yaml_compact_meanings" %}} compact cases carry a `meaning`, and
{{% siteparam "metrics.yaml_compact_meanings_split" %}} carry one per reading, under `yaml-1.2-core` and
`yaml-1.1`. [Meanings](../../concepts/meanings/) explains how to pick the one that applies to your parser.

## Other parsers

Two modules import real parsers. The root module imports none.

- `yaml/consensus` runs five implementations over the corpus: the YAML 1.2 reference parser (Perl), libfyaml,
  YAMLStar, `go.yaml.in/yaml/v3` and `github.com/go-openapi/go-yaml`. The
  [consensus report](../../reference/consensus/) shows where they agree with the corpus and with each other.
- `yaml/calibrate` holds tests only. They check the generator and the corpus against go-openapi/go-yaml: that
  every emitted document parses, that every style of one value reads the same, and that the stated meanings are
  what a parser reads.

## What the suite leaves out

- **The JSON and failsafe schemas.** The corpus states meanings under two readings only; see
  [Meanings](../../concepts/meanings/#readings).
- **Events and output.** The YAML Test Suite stores a parser's event stream (`test.event`) and a canonical
  rendering (`out.yaml`). The corpus stores neither: it states what a document means, not how a parser reports
  reading it.
- **UTF-16 and UTF-32 documents.** See Encoding above.
