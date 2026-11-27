---
title: The oracle
weight: 10
description: |
  A recognizer compiled from the grammar each specification publishes, used to label every generated document.
---

Every case in a corpus carries `wellFormed`, the verdict of an oracle: a recognizer compiled from the grammar the
specification publishes. This page explains why the corpus needs one, where the grammars come from, and what the
oracle returns.

## Why a generated corpus needs an oracle

The generator writes a valid document, then breaks it with a mutation: it deletes a byte, swaps two, truncates
the text. The generator cannot tell whether the result is still valid. In JSON, most mutations break the
document. In YAML, most do not: `a: b` with a byte deleted is `a:b`, a plain scalar, and still valid YAML. So
each document needs a label from something that knows the language.

## Why not an existing parser

A corpus labelled by a parser stores that parser's answers. Every other parser would then be scored against
its bugs, and the parser itself would always score perfectly. The oracle instead reads the grammar that the
specification publishes, so a disagreement between a parser and the corpus points at one of two documents: the
parser or the grammar.

The oracle also separates two faults that a round trip cannot. When `parse(render(value))` fails, the renderer
may have written invalid text, or the parser may have refused valid text. Asking the oracle about
`render(value)` tells which.

## Where the grammars come from

| language | grammar file | source | productions |
|---|---|---|---|
| YAML 1.2 | `yaml/testdata/yaml-spec-1.2.json` | published by [`yaml/yaml-grammar`](https://github.com/yaml/yaml-grammar) | {{% siteparam "metrics.yaml_rules" %}} |
| JSON | `json/testdata/rfc8259.json` | transcribed from RFC 8259 into the same format | {{% siteparam "metrics.json_rules" %}} |

The YAML grammar is a parameterized PEG. Its rules take up to four arguments: the indentation `n`, the extra
indentation `m`, the context `c` (such as `block-in` or `flow-key`) and the chomping `t`. Its notation adds
ordered choice, lookahead in both directions, lookbehind and character-set subtraction. JSON needs none of the
parameters, but uses the same format, so one compiler serves both.

## What the oracle returns

```go
res := yamlcorpus.Stream(src) // or jsoncorpus.Text(src)
res.OK                        // whether the whole input matches the grammar
```

`Result` holds the verdict and two counters, `Steps` and `Hits`, which measure the work done. The oracle builds
no tree, emits no events and returns no error message. It only has to be right, so it skips everything a parser
spends its time on.

| function | question |
|---|---|
| `yamlcorpus.Stream` | is `src` a valid YAML 1.2 stream (`l-yaml-stream`, from `block-in`)? |
| `yamlcorpus.FlowNode` | is `src` exactly one flow node (`ns-flow-node`, from `flow-out`)? |
| `jsoncorpus.Text` | is `src` exactly one JSON text? |
| `NewRecognizer(hint)` | a reusable oracle, for a caller that checks documents in a loop |

The verdict covers syntax only. YAML sets rules the grammar cannot express, such as an alias naming an anchor
that occurs earlier, and a parser that composes the document must check them too. The corpus records how far the
verdict reaches in `verdictAt`: see [Tags and stances](../tags-and-stances/#how-far-the-verdict-reaches).

## Patches to the YAML grammar

The YAML specification states some rules in its prose and leaves them out of the grammar file. Compiling the file
as published would accept documents YAML forbids, or refuse documents it allows. So `yamlcorpus` applies
{{% siteparam "metrics.yaml_patches" %}} patches before compiling. Each names the rule it rewrites and the reason:

| rule | the prose the grammar file leaves out |
|---|---|
| `s-l+block-indented` | the extra indentation a compact collection detects is not written down |
| `c-b-block-header` | the two block header indicators may be written in either order |
| `c-b-block-header` | a block scalar's header ends at the end of its line |
| `c-indentation-indicator` | zero is not a legal indentation indicator |
| `ns-flow-map-entry`, `ns-flow-pair`, `c-l-block-map-explicit-key` | a `?` with no space after it opens a plain scalar, not an explicit key |
| `c-directives-end` | a directives-end marker is a line holding only the three dashes |
| `ns-flow-yaml-node` | a flow collection carrying properties is still a flow node |
| `ns-flow-map-implicit-entry` | a JSON-like key carrying properties may take a value adjacent to its `:` |
| `s-l+block-collection` | properties belong to the collection only if the line ends after them |
| `ns-reserved-directive` | `YAML` and `TAG` name the two directives the spec defines, so neither names a reserved one |

Most patches add the same missing check: the specification writes an indicator character into the grammar, then
constrains what may follow it in the text around the production.

None of the patches follows what a parser does. Each one rests on the specification's text, and each one checks
the shape of the rule it rewrites: if a newer grammar file fixes or moves the rule, the compile fails and names
the patch.

JSON needs no patch. RFC 8259's grammar runs as written.

## Calibration

A corpus stores the oracle's verdicts, so an oracle wrong on one document puts a false accusation into every
corpus built from it. Two tests score the oracles against the hand-written suites before any corpus is built:

| suite | must accept | must reject | the oracle |
|---|---|---|---|
| YAML Test Suite (`TestRecognizerAgreesWithTheYAMLTestSuite`) | {{% siteparam "metrics.yaml_yts_accept" %}} | {{% siteparam "metrics.yaml_yts_reject" %}} | agrees on all but {{% siteparam "metrics.yaml_yts_departures" %}} |
| JSONTestSuite (`TestRecognizerAgreesWithJSONTestSuite`) | {{% siteparam "metrics.json_jts_accept" %}} | {{% siteparam "metrics.json_jts_reject" %}} | agrees on all |

### The two YAML departures

The oracle accepts two fixtures that the YAML Test Suite marks as errors:

```yaml
# duplicate-yaml-directive
%YAML 1.2
%YAML 1.2
---
```

```yaml
# tag-shorthand-used-in-documents-but-only-defined-in-the-first
%TAG !prefix! tag:example.com,2011:
--- !prefix!A
a: b
--- !prefix!B
c: d
--- !prefix!C
e: f
```

The suite is right about both. Section 6.8.1 makes a second `%YAML` directive for the same document an error.
Section 6.8.2.2 scopes a `%TAG` directive to the document that follows it, so `!prefix!` is undefined in the second
and third documents.

The grammar cannot express either rule:

- `l-directive-document` takes one or more `l-directive`, and each matches one directive alone. Nothing in a
  production counts how many `%YAML` lines came before.
- `c-ns-shorthand-tag` is a `c-tag-handle` followed by tag characters. It matches the shape of `!prefix!` and
  never asks whether a `%TAG` directive declared it. Checking that takes a table of the handles each document declares, and a grammar keeps no table.

So the oracle accepts both documents, as the grammar does. A patch cannot fix this, because each rule needs state
that lives outside any production. `TestRecognizerAgreesWithTheYAMLTestSuite` lists the two fixtures by name, with
the reason. The test fails if either one starts to agree, so the list cannot go stale.

The corpus states both rules as tags with a settled outcome, instead of as verdicts:

| tag | stage | settled |
|---|---|---|
| `directive/yaml-repeated` | compose | reject, section 6.8.1 |
| `directive/handle-out-of-scope` | compose | reject, section 6.8.2.2 |

A case carrying either tag keeps the grammar's `wellFormed: true`, and `Table.Expect` turns the settled rule into
an expected rejection. See [Settled rules](../tags-and-stances/#settled-rules).

The YAML Test Suite has one flaw here: its `error` file marks a broken syntax and a broken rule the same way. A
parser that checks only syntax cannot tell which of its failures on the suite are real defects. The corpus keeps
the two apart: `wellFormed` holds the grammar's verdict, and the tag holds the rule and the stage it belongs to.

## How it runs

`grammar.Compile` walks the grammar file once and builds a tree of Go closures. Running closures costs two orders
of magnitude less than interpreting the decoded file at every step, and a corpus build calls the oracle hundreds
of thousands of times.

The recognizer memoizes the result of a rule at a position. Without it, nested flow collections take exponential time:
twenty nested sequences take 115 million steps and five seconds. With it, the time is linear, at about twice the
cost on documents that never backtrack.

The [package documentation of `internal/grammar`](https://pkg.go.dev/github.com/go-openapi/conformance-suites/internal/grammar)
describes the rest:

- how a block scalar's header passes `m` and `t` to the rule after it;
- how a block collection's indentation is measured from its first line;
- why a failed alternative costs nothing to undo.
