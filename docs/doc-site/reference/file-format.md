---
title: File format
weight: 10
description: |
  Format 4: every field of the header and of a case, and the compatibility rules.
---

A corpus file holds one conformance suite: a header describing the suite, followed by its test cases. This
page lists every field, with its type and when it may be absent. The reasons behind the fields are explained
in [Concepts](../../concepts/).

## Container

A corpus file is a gzip-compressed [JSON Lines](https://jsonlines.org/) file, encoded in UTF-8.

- Line 1 holds the header object.
- Each following line holds one case object.
- The test document in `src` is encoded in standard base64 with padding, because it often holds bytes that
  are not valid UTF-8 and cannot appear in a JSON string. Every other field is plain JSON.

The gzip header carries no modification time and no operating system, so two builds of the same corpus with
the same Go toolchain produce identical files.

The Go reader accepts lines up to 8 MiB.

## Header

The header of the embedded JSON compact corpus:

```json
{
  "format": 4,
  "grammar": "RFC 8259",
  "digest": "sha256:ae0322c3aeb0db880dc20a2a5984f4318d59469efe4c5e3b80de5f1314ac584c",
  "generator": "jsoncorpus/1",
  "seed": 1,
  "tier": "compact",
  "cases": 1642,
  "vocabulary": [
    { "tag": "encoding/bom", "stage": "parse" },
    { "tag": "encoding/not-utf8", "stage": "parse" },
    { "tag": "encoding/utf16", "stage": "parse" },
    { "tag": "encoding/utf32", "stage": "parse" },
    { "tag": "number/out-of-range", "stage": "construct" }
  ]
}
```

| field | type | present | content |
|---|---|---|---|
| `format` | integer | always | The layout version, `4` for this page. |
| `grammar` | string | always | The name of the grammar behind the verdicts: `RFC 8259`, `YAML 1.2`. |
| `digest` | string | always | `sha256:` and the hex SHA-256 of the grammar file behind the verdicts. |
| `generator` | string | always | The name and version of the generator that produced the documents: `jsoncorpus/1`. |
| `seed` | integer | always | The seed given to the generator. |
| `tier` | string | always | `compact` for the small corpus embedded in the module, `full` for the complete one. |
| `cases` | integer | always | The number of case lines after the header. |
| `vocabulary` | array of tag specs | always | Every tag used by at least one case, sorted by name. See below. |
| `readings` | array of strings | optional | Every reading the corpus considered when it recorded meanings, sorted: `yaml-1.2-core`, `yaml-1.1`. |
| `features` | array of strings | optional | Every feature name used by at least one case, sorted. |

### Tag spec

Each entry of `vocabulary` describes one tag, so that a consumer can score a case from the file alone.

| field | type | present | content |
|---|---|---|---|
| `tag` | string | always | The tag name, as it appears in a case's `tags`. |
| `stage` | string | always | The earliest stage at which the question can arise: `parse`, `compose` or `construct`. |
| `settled` | string | optional | `accept` or `reject` when the specification decides the question. Absent when each implementation chooses. |
| `because` | string | optional | The rule from the specification, and where it is stated. |

## Case

A refused mutant and the valid document it came from:

```json
{"name":"generated/0000","src":"IiI=","wellFormed":true,"origin":{"document":0,"signature":"c7ae50c0…"}}
{"name":"generated/0000/mutant/01","src":"IiIi","wellFormed":false,"origin":{"document":0,"mutation":"duplicate a byte","signature":"c7ae50c0…"}}
```

An encoding shape, which carries a tag:

```json
{"name":"shape/encoding/a utf-8 mark opening-a-document","src":"77u/eyJhIjoxfQ==","wellFormed":true,"tags":["encoding/bom"],"verdictAt":"construct","origin":{"document":0,"mutation":"enumerated"}}
```

| field | type | present | content |
|---|---|---|---|
| `name` | string | always | A name for the case, unique within the file. |
| `src` | base64 string | always | The test document, as raw bytes. |
| `wellFormed` | boolean | always | The grammar's verdict, after removing the encoding properties a parser may normalize without loss. For JSON, a leading byte order mark is removed. |
| `opaque` | boolean | optional, default `false` | `true` when the bytes are not valid UTF-8 even after normalization. The oracle could not read them, so `wellFormed` tells nothing about a parser that decodes them some other way. |
| `tags` | array of strings | optional | The implementation-defined properties of the document. Each one is listed in the header's `vocabulary`. |
| `features` | array of strings | optional | The constructs the document contains, such as a flow collection or a CRLF line break. They never change the expected outcome. |
| `verdictAt` | string | optional, default `parse` | The last stage at which an acceptance by the grammar is evidence: `parse`, `compose` or `construct`. A refusal is evidence at every stage. |
| `meaning` | meaning | optional | What the document denotes under the specification's default reading. |
| `meanings` | array of meanings | optional | What the document denotes under every reading that has an answer. Present only when the readings disagree; it repeats the default reading. |
| `origin` | origin | always | Where the case came from. See below. |

### Origin

| field | type | present | content |
|---|---|---|---|
| `document` | integer | always | The index of the generated document the case comes from. |
| `mutation` | string | optional | How the document was changed: a mutation name such as `delete a byte`, or `enumerated` for an encoding shape. Absent for a document generated whole. |
| `signature` | string | optional | The hex SHA-256 of the coverage fingerprint the builder used to select the case. |

### Meaning

| field | type | present | content |
|---|---|---|---|
| `under` | string | always | The reading: `rfc8259`, `yaml-1.2-core`, `yaml-1.1`. |
| `json` | any JSON value | optional | The value, as JSON. `null` is a value; an absent field means there is none. Absent when `cyclic` is `true`. |
| `cyclic` | boolean | optional, default `false` | `true` when the value is a graph with a cycle, which has no JSON form. |

## Checks before scoring

Run these checks before you use a corpus file:

1. **The format.** Reject a file whose `format` you do not support. Read `format` on its own first: a newer
   format may change the shape of other fields.
2. **The length.** Count the case lines and compare the count with `cases`. A shorter file is truncated.
3. **The vocabulary.** Every tag in `vocabulary` must be one your parser has a position on, or one the
   specification settles. A reader that ignores an unknown tag scores the case as if the tag were absent.
4. **The grammar,** if you also run the oracle: compare `digest` with the digest of your grammar. A
   different digest means the verdicts came from another version of the grammar file.

The Go reader runs the first two checks. Call `Header.Unknown` for the third.

## Compatibility

`format` changes only when a reader of the previous version would misunderstand a newer file.

Adding an optional field does not change `format`. A reader must ignore the fields it does not know.
`features`, `meanings` and `readings` were all added this way: a reader that skips them still derives the
same expected outcomes, and still finds the default reading in `meaning`.

Format 4 changed `meaning.json` from a base64 string to a JSON value. A format-3 reader cannot decode it.

Compare meanings as JSON values, not as text: the writer removes insignificant whitespace, and a reader that
decodes numbers into 64-bit floats rounds the ones it cannot hold.

Do not rely on case names, or on the seed reproducing the corpus, to stay stable between releases.

## Go

The `suite` package reads and writes this format:

- `suite.NewReader` reads the header and returns a `Reader`. `Reader.Next` returns one case at a time and
  fails on a truncated file.
- `suite.ReadAll` reads a whole file into memory. `suite.FromBytes` does the same for a file held in a byte
  slice, such as an embedded corpus.
- `suite.Content` removes the gzip container and returns the JSON Lines.
- `suite.NewWriter` and `Writer.Add` write a file. `Writer.Close` fails if the number of cases written
  differs from `cases`.
- `Header.Spec` returns the tag spec for one tag. `Header.MeaningFor` returns the meaning of a case under one
  reading.
