---
title: JSON
weight: 10
description: |
  RFC 8259, compiled with no patches: the generator, the mutations, the tags, and `DefaultLexer`.
---

The JSON suite tests whether a parser accepts exactly the documents RFC 8259 admits, and holds it to the
positions it declares on the questions the RFC leaves open. The package is `jsoncorpus`, in the root module.

## The grammar

`jsoncorpus.JSON` is the grammar of RFC 8259, compiled from `json/testdata/rfc8259.json` with no patches: every
production runs as the RFC writes it. It has {{% siteparam "metrics.json_rules" %}} rules. `jsoncorpus.Text`
checks one document against it:

```go
ok := jsoncorpus.Text(src).OK
```

### Calibration

Before the grammar decides a single case, `TestRecognizerAgreesWithJSONTestSuite` scores it against the vendored
copy of Nicolas Seriot's JSONTestSuite:

| files | count | the grammar |
|---|---|---|
| `y_`, must accept | {{% siteparam "metrics.json_jts_accept" %}} | accepts every one |
| `n_`, must reject | {{% siteparam "metrics.json_jts_reject" %}} | refuses every one |
| `i_`, implementation-defined | {{% siteparam "metrics.json_jts_impl" %}} | accepts {{% siteparam "metrics.json_jts_impl_accepted" %}}; the test records them and asserts nothing |

A corpus stores the grammar's verdicts, so a grammar wrong on one document would put a false accusation into every
corpus built from it.

## The generator

`jsoncorpus.Generate(seed, documents, mutantsEach)` draws documents and breaks each one:

- `RandomValue` draws a value tree, and `RandomStyle` draws how to write it: indentation and spacing, the
  whitespace around the top value, and which characters are escaped.
- `Emit` writes the value in that style.
- `Mutate` applies one of {{% siteparam "metrics.json_mutations" %}} mutations to the bytes: delete a byte,
  delete a run, duplicate a byte, swap two neighbors, truncate, substitute a byte the grammar knows, insert a byte
  the grammar knows, substitute an arbitrary byte.

The grammar then gives each document and each mutant its verdict. A `Build` keeps the cases that bring a coverage
signature nothing has brought yet: `Compact()` keeps up to {{% siteparam "metrics.json_per_signature" %}} per
signature, and `Full()` keeps every document. [Coverage](../../concepts/coverage/) explains signatures.

The generator draws only values whose meaning it knows, so two kinds of case are written by hand:

- `ValueShapes` writes the documents whose meaning RFC 8259 leaves open, each with its tag: duplicate names and
  lone surrogate escapes.
- `EncodingShapes` writes every byte order mark in every position it can occupy.

### The census

`TestTheGeneratedCorpusIsWiderThanTheSuite` checks that every construct a valid JSONTestSuite document holds, a
case of the compact corpus holds too. It counts {{% siteparam "metrics.json_constructs" %}} constructs, from a
top-level scalar to an escaped form feed, listed by `jsoncorpus.Constructs`. None is missing. Grammar coverage
cannot make this check: the suite and the corpus both enter every production.

## Normalization

`jsoncorpus.Normalize` removes a leading UTF-8 byte order mark, and nothing else. RFC 8259 section 8.1 lets a
parser ignore the mark, and the bytes after it are unchanged. A case's `wellFormed` is the grammar's verdict on
the normalized bytes.

Ill-formed UTF-8 and UTF-16 are not normalized. Reading them takes a decoder and a policy for what does not
decode, and the suite would be grading a parser against a policy of its own.

## Tags

The JSON vocabulary has {{% siteparam "metrics.json_tags" %}} tags. `jsoncorpus.Vocabulary` places each at a
stage:

| tag | stage | the open question |
|---|---|---|
| `encoding/bom`, `encoding/not-utf8`, `encoding/utf16`, `encoding/utf32` | parse | what the bytes denote |
| `string/lone-surrogate` | parse | what an unpaired `\uXXXX` escape means; RFC 8259 section 8.2 calls it unpredictable |
| `structure/deep-nesting` | parse | how deep a parser nests before it gives up |
| `object/duplicate-name` | compose | which member of a repeated name wins; RFC 8259 section 4 calls it unpredictable |
| `number/out-of-range` | construct | what a number past `float64` becomes; RFC 8259 section 6 lets a parser limit range and precision |

RFC 8259 settles none of them. [Tags](../../reference/tags/) gives the full description of each.

## DefaultLexer

`jsoncorpus.DefaultLexer` is the stance of the default lexer of `github.com/go-openapi/core/json`:

| tag | stand |
|---|---|
| `encoding/bom` | `Accepts` |
| `encoding/not-utf8`, `encoding/utf16`, `encoding/utf32` | `Refuses` |
| `string/lone-surrogate` | `Refuses` |
| `structure/deep-nesting` | `Accepts` |

Its `At` is `Parse`: the lexer returns a number's text and converts nothing, so it never reaches the stage where a
number can be out of range, and the table takes no position on `number/out-of-range`.

The positions were measured, by running the lexer over the suite's implementation-defined cases. Two of them
differed from what its authors expected:

- the lexer tolerates a byte order mark, though it still refuses a mark with no document after it;
- it refuses a lone surrogate escape, which is stricter than the grammar.

## What the suite leaves out

- **Resource limits.** `structure/deep-nesting` marks a document nested more than
  {{% siteparam "metrics.json_deep_nesting" %}} levels deep, the depth where nesting stops being a shape and becomes
  a question of a parser's limits. The generator stays well below it on purpose, so no case carries the tag: the
  suite tests what a parser reads, not the budget that stops it on hostile input.
- **Meanings.** A JSON case carries no `meaning`: the corpus says whether a parser should accept a document, not
  what value it should read.
- **Features.** A JSON case carries no `features`, so a subset cannot be selected by construct.
- **Other parsers.** The YAML suite is checked against five parsers in the `consensus` module. The JSON suite is
  checked against JSONTestSuite only.
