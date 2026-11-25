---
title: Tags
weight: 20
description: |
  Every tag, its stage, and whether the specification settles it.
---

{{% notice style="info" title="Generated" %}}
This page is written by `go run hack/doc-site/gentags.go` from each grammar's `Vocabulary()`.
Edit the doc comment on the tag's constant, not this page.
{{% /notice %}}

A tag names an implementation-defined property a document exhibits. Every case in a corpus lists its tags, and the
file's header repeats each tag with its stage and, where the specification settles it, the outcome.

The stage is the earliest point at which the question can be asked: `parse`, `compose` or `construct`.
A consumer that answers it later answers the same question. "Settled" means the specification fixes the outcome,
so no stance can change it; "open" means each parser takes its own position.

## Encoding

Declared in package `stance` and placed by the JSON grammar. A question about what the bytes denote, settled before parsing starts.

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `encoding/bom` | parse | open | A document opening with a UTF-8 byte order mark. It declares no change of encoding, which is what makes ignoring it a defensible position and skipping the others not. |
| `encoding/not-utf8` | parse | open | Bytes that are not well-formed UTF-8 at all: lone continuation bytes, overlong sequences, truncated sequences, surrogates encoded directly, and code points beyond U+10FFFF. |
| `encoding/utf16` | parse | open | A document that announces or looks like UTF-16. |
| `encoding/utf32` | parse | open | A document that announces or looks like UTF-32. |

## JSON

Placed by `jsoncorpus.Vocabulary`. RFC 8259 settles none of them: each one is a position a parser takes.

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `number/out-of-range` | construct | open | A number the grammar admits and float64 cannot carry: it overflows, underflows to zero, or is an integer too large to survive as one. RFC 8259 section 6 lets an implementation limit range and precision, so a parser that converts is exposed where a parser that keeps the text is not. |
| `object/duplicate-name` | compose | open | An object holding two members of one name. RFC 8259 section 4 says names SHOULD be unique and calls the behavior of software receiving such an object unpredictable: it may keep the last, keep the first, or refuse. |
| `string/lone-surrogate` | parse | open | A `\uXXXX` escape naming a surrogate that is not part of a well-formed pair. RFC 8259 section 8.2 calls the behavior unpredictable. The escape itself is grammatical -- four hex digits -- so this is a question about the value, never about the syntax. |
| `structure/deep-nesting` | parse | open | A document nested more than 128 levels deep. No specification sets a limit and every parser has one, whether it says so or not. The generator stays well below that depth on purpose, so that no case tests the budget a parser keeps against hostile input. |

## YAML

Placed by `yamlbuild.Vocabulary`. A tag the specification settles carries the clause and the outcome it fixes, from `yamlbuild.AllRules`.

### Anchors and aliases

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `anchor/alias-across-documents` | compose | Reject, section 3.2.2.2: anchor names are local to a document, so an earlier document's anchor is out of scope | An alias naming an anchor from an earlier document in the same stream. |
| `anchor/alias-as-key` | compose | Accept, section 7.1: an alias is a node, and a node may be a mapping key | An alias used as a mapping key. Legal wherever a node is legal, which is a question about composing and not about the model underneath. |
| `anchor/alias-forward` | compose | Reject, section 7.1: the anchor must precede the alias, so a forward reference names nothing | An alias naming an anchor that occurs later. |
| `anchor/alias-recursive` | parse | Accept, section 3.2.1: an anchor identifies its node from where the node begins, so an alias within it resolves | An alias inside the node its own anchor names. |
| `anchor/alias-undefined` | compose | Reject, section 7.1: an alias refers to the most recent preceding node having the given anchor, and there is none | An alias naming an anchor that never occurs. |
| `anchor/cyclic-meaning` | construct | open | A document whose representation graph contains a cycle, which is a different question from whether it parses. |
| `anchor/on-empty-node` | compose | Accept, section 7.1: the empty node is a node, and may be anchored | An anchor on a node with no content. |
| `anchor/redefined` | compose | Accept, section 3.2.2.2: an anchor name may be reused, and the alias takes the most recent | The same anchor name given to two nodes. Legal, and the later one wins. |
| `anchor/unused` | compose | Accept, section 7.1: nothing requires an anchor to be referred to | An anchor nothing refers to. Legal, and worth having because it is the shape a parser is most likely to have simply dropped. |

### Directives

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `directive/handle-out-of-scope` | compose | Reject, section 6.8.2.2: TAG directives apply to the document that follows them, so a later document has no such handle | A handle declared for one document and used in a later one. |
| `directive/multiple` | compose | Accept, section 6.8: nothing limits a document to one directive, and a %YAML with a %TAG is the ordinary prelude | A document with more than one directive, which is ordinary: a %YAML and a %TAG together are the commonest prelude there is. |
| `directive/reserved` | compose | Accept, section 6.8: a directive a processor does not recognize is ignored with a warning, not refused | A directive that is neither %YAML nor %TAG, which the spec says to ignore with a warning rather than refuse. |
| `directive/tag-handle-repeated` | compose | Reject, section 6.8.2.2: it is an error to specify more than one TAG directive for the same handle in the same document | The same %TAG handle declared twice for one document. |
| `directive/yaml-major-version` | compose | Reject, section 6.8.1: a processor must reject a directive whose major version differs from its own | A %YAML whose major version is not 1, which a 1.x processor must refuse rather than guess at. |
| `directive/yaml-minor-version` | compose | open | A %YAML whose minor version is beyond what the processor knows -- 1.9 today. |
| `directive/yaml-repeated` | compose | Reject, section 6.8.1: it is an error to specify more than one YAML directive for the same document | Two %YAML directives for one document. |
| `directive/yaml-version` | parse | open | A %YAML directive stating the version. |

### escape

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `escape/past-unicode` | compose | Reject, section 5.7: \U escapes a 32-bit Unicode character, and no character lies past U+10FFFF | A double-quoted scalar holding a `\U` escape past U+10FFFF. |

### indentation

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `indentation/tab-opens-content` | parse | open | A root block scalar whose first content line opens with a tab at column 0. |

### JSON representation

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `json/not-representable` | construct | open | A document whose meaning has no JSON form: a non-finite float, or an entry keyed on a collection. |

### Mapping keys

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `key/distinct-after-resolution` | compose | Accept, section 3.2.1.1: keys resolving to different nodes are different keys, however alike they look | Two keys that look nearly alike and are not equal, because resolution gives them different types. |
| `key/duplicate` | parse | Reject, section 3.2.1.1: it is an error for two equal keys to appear in the same mapping node | Two keys with the same spelling. |
| `key/duplicate-after-resolution` | compose | Reject, section 3.2.1.1: keys are equal when they resolve to the same node, however they are written | Two keys that are equal only once they are resolved: quoting removed, tags applied, aliases followed. |
| `key/integral-float` | construct | open | A mapping key that resolves to a float whose value is a whole number: `1.0`, `1e3`, `-0.0`. |
| `key/not-a-scalar` | construct | open | A mapping key that resolves to a collection. |
| `key/not-a-string` | construct | open | A mapping key that resolves to something other than a string: a number, a boolean, a null. |

### Merge keys

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `merge/into-non-mapping` | construct | open | `<<` given something that is not a mapping. |
| `merge/key` | construct | open | A `<<` key whose value is an alias to a mapping -- the construct working as intended. |
| `merge/key-alone` | construct | open | A `<<` written as a flow entry's key with no `:`. |
| `merge/quoted` | construct | open | A quoted `<<`, which suppresses the merge in every reading because quoting resolves it to a string. |
| `merge/sequence` | construct | open | `<<` given a sequence of mappings, which 1.1 merges in order with the earlier winning. |
| `merge/written-in-place` | construct | open | `<<` given a mapping written where it stands, rather than an alias to one written elsewhere. |

### Representation in a model

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `representation/binary` | construct | open | A `!!binary`, whose value a model holds as raw bytes or as the base64 text. |
| `representation/float-inexact` | construct | open | A float written with more digits than the model's float type keeps, so a reader into one rounds and the document's digits are not what it holds. |
| `representation/float-non-finite` | construct | open | `.inf`, `-.inf` or `.nan`: a float a model may hold, refuse, or never resolve to a float in the first place. |
| `representation/float-out-of-range` | construct | open | A float whose magnitude is past what the model's float type holds, in either direction: it overflows to an infinity or underflows to zero. |
| `representation/integer-wide` | construct | open | An integer past what a machine word holds, which a model may hold whole, round to a float, or refuse. |
| `representation/ordered-map` | construct | open | An `!!omap`, whose entries a model holds as ordered pairs or as a plain mapping. |
| `representation/timestamp` | construct | open | A `!!timestamp`, whose value a model holds as a moment, as a zoned text, or as the text the document wrote. |

### Schema resolution

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `schema/bool-legacy` | construct | open | A scalar YAML 1.1 read as a boolean and 1.2 does not: yes, no, on, off, y, n and their capitalizations. |
| `schema/float-exponent-only` | construct | open | `1e3`: digits and an exponent with no decimal point. A float under 1.2's core schema and under JSON, and a string under 1.1, whose float required a point. |
| `schema/float-special` | construct | open | `.inf` and `.nan`. Floats under core, strings under failsafe and under JSON, which has no way to write either. |
| `schema/int-binary` | construct | open | `0b1010`. A YAML 1.1 integer, and nothing in 1.2. |
| `schema/int-non-decimal` | construct | open | `0x1A` and `0o17`: core integers, and strings under the JSON schema, which writes neither. |
| `schema/int-octal-legacy` | construct | open | A leading zero. YAML 1.1 read `0777` as octal 511; 1.2 spells octal `0o777` and leaves the leading-zero form as a decimal or a string. It is the disagreement most likely to be load-bearing, because file modes are written this way and both readings are plausible numbers. |
| `schema/int-sexagesimal` | construct | open | `1:30`. A YAML 1.1 integer meaning 90, and a string in 1.2 -- which is why `12:34:56` is a time in some documents and a string in others. |
| `schema/int-underscores` | construct | open | `1_000`. A YAML 1.1 integer, and nothing in 1.2. |
| `schema/timestamp-legacy` | construct | open | `2001-12-14`. A timestamp in YAML 1.1, which had the type, and a string in 1.2, which does not. |

### Separation

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `separation/tab` | parse | open | A document separating with a tab where a space would do. |

### Tags on nodes

| Tag | Stage | Specification | What it marks |
|---|---|---|---|
| `tag/binary-not-base64` | construct | open | `!!binary not base64!`. |
| `tag/bool-not-a-boolean` | construct | open | `!!bool 7`. |
| `tag/float-not-a-number` | construct | open | `!!float xyz`. |
| `tag/int-not-an-integer` | construct | open | `!!int abc`. |
| `tag/kind-mismatch` | construct | open | A tag over a node of the wrong kind: a collection tag on a scalar or on the other collection, or a scalar tag on a collection. |
| `tag/local` | construct | open | A `!` shorthand, whose meaning is the application's. |
| `tag/named-handle` | parse | open | A shorthand using a handle a %TAG directive declared. |
| `tag/non-specific` | compose | Accept, section 6.9.1: a non-specific tag is legal, and asks that resolution be suppressed | A bare `!`, which suppresses resolution rather than naming a type. |
| `tag/null-not-null` | construct | open | `!!null 5`. |
| `tag/omap-not-a-sequence-of-pairs` | construct | open | `!!omap` over anything but a sequence of one-entry mappings: an element that is not a mapping, an element holding two entries, the tag on a mapping, or one key written twice. |
| `tag/percent-escape` | parse | open | A tag URI carrying a percent escape, which is the only route to a hex digit outside a quoted scalar. |
| `tag/secondary` | compose | Accept, section 6.8.2.2: the secondary handle is declared by default as tag:yaml.org,2002: | A `!!` shorthand, which resolves through the secondary handle to tag:yaml.org,2002: unless a %TAG says otherwise. |
| `tag/timestamp-not-a-date` | construct | open | `!!timestamp not-a-date`. |
| `tag/undeclared-handle` | parse | Reject, section 6.8.2.2: a tag shorthand must use a handle a %TAG directive declared, and this one does not | A shorthand whose handle no %TAG declares. |
| `tag/verbatim` | compose | Accept, section 6.8.2.1: a verbatim tag is used as presented and resolves through nothing | A tag written out in full between angle brackets, which resolves through nothing at all. |

