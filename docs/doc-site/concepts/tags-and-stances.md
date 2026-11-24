---
title: Tags and stances
weight: 20
description: |
  Why a case stores the grammar's verdict and a list of tags, and how a stance turns them into an expected outcome.
---

A case does not say "must accept" or "must reject". It stores the grammar's verdict and the list of questions
the document raises that the specification leaves open. A stance, declared by each parser, turns the two into
an expected outcome when the corpus is replayed.

## Why a corpus cannot store "must accept"

Every specification worth testing against leaves some questions to the implementation:

- RFC 8259 lets a parser limit the range and precision of numbers, and calls the meaning of an unpaired surrogate
  escape unpredictable.
- YAML 1.2 admits UTF-8, UTF-16 and UTF-32, and a library may support only one of them.

A corpus that stored "this document must be accepted" would store one parser's choice as if it were the language,
and would mark a conforming parser wrong for making another. So a case stores what the grammar says, and names
the open questions with tags.

## Tags and features

A **tag** (`stance.Tag`) names a question the language leaves open: `number/out-of-range`, `encoding/bom`,
`anchor/cyclic-meaning`. A case lists the tags its document carries. A tag gates scoring: a parser that has no
position on a tag is not scored on the cases carrying it.

A **feature** (`stance.Feature`) names a construct the document contains: a flow collection, a CRLF line break,
an anchor. Features never change the expected outcome. Use them to select a subset of the corpus with
`stance.With` and `stance.Without`, and to report what a failing document holds.

To decide which one a new label is, ask whether a conforming parser could refuse the construct, or read it
differently. If it could, the label is a tag. If it could not, it is a feature.

[Tags](../../reference/tags/) lists every tag.

## Stands

A `stance.Table` records a parser's stand on each tag:

| stand | meaning |
|---|---|
| `Accepts` | the parser reads such documents, and the grammar's verdict decides |
| `Refuses` | the parser rejects them on purpose, so the case expects a rejection |
| `Either` | the parser does not guarantee either answer, so the case is not scored |
| `Undeclared` | no position, the zero value: the case is not scored |

`Undeclared` is not scored on purpose. A table that leaves a tag out gives no free pass on the cases carrying
it. Call `Header.Unknown` to find the tags a corpus uses that your table does not know.

## Outcomes

`Table.Expect` returns an outcome and the reason for it:

- `Accept`: a conforming parser with these stands reads the document.
- `Reject`: it refuses the document.
- `Undecided`: the case tells nothing about this parser. Skip it, and do not count it as a pass.

[Read a corpus](../../getting-started/read-a-corpus/#turn-a-case-into-an-expected-outcome) lists the steps
`Table.Expect` takes, for readers without Go.

## Stages

"Valid" is not one question. A document can be well formed and still not resolve, or resolve and still not fit
the model a program loads it into. The YAML specification names the steps, and they apply to JSON too:

| stage | what a parser does |
|---|---|
| `Parse` | reads the characters into a serialization: syntax |
| `Compose` | builds the representation: resolves aliases, collects mapping keys |
| `Construct` | builds native values: numbers, strings, maps |

Each tag has the stage at which its question first arises, in the grammar's vocabulary and in the header's
`vocabulary`. `number/out-of-range` arises at `Construct`, because nothing before construction has a numeric type.

`Table.At` is how far a parser takes a document, and it is a floor. A parser that stops at `Parse` is not asked
about a tag that arises at `Construct`. A parser that notices a parse question later, say while it builds a
string, still answers it.

Encoding is not a stage. What a run of bytes denotes is settled before parsing starts.

## How far the verdict reaches

`wellFormed` is the grammar's verdict at `Parse`. A refusal holds at every stage: a document that does not
parse does not compose or construct either. An acceptance does not: a document can parse and still fail to
compose, as when it uses an alias to an anchor that never occurs.

So a case records `verdictAt`, the last stage at which an acceptance by the grammar is evidence. A parser that
goes further than `verdictAt` is not scored on the case's acceptance. An absent `verdictAt` means `Parse`, the
least a case can claim.

## Settled rules

Some questions the grammar cannot express are settled by the specification all the same. YAML says an alias
must refer to an anchor that occurs earlier in the document: a parser that reads an undefined alias is wrong,
whatever its stance.

A `stance.Rule` records such a requirement: the tag, the clause that settles it, and the outcome it fixes. The
header's tag spec carries the same facts in `settled` and `because`. `Table.Requires` holds a language's rules,
and `Table.Expect` checks them before the stands:

- A rule that fixes `Reject` makes the case expect a rejection, whatever the table's stand.
- A rule that fixes `Accept` says the construct is legal. The grammar still decides whether the document is.
- A table can decline a rule with `Either`, when the parser cannot check it at all: a parser that never resolves
  aliases cannot check a rule about them. The case is then not scored. A table cannot turn a settled rejection
  into an acceptance.

`Rules.Contradicted` names the settled rules a table marks `Accepts`. Such an entry is a mistake, most likely a
tag copied into the wrong table, and the function reports it so you can delete it.

## Opaque bytes

The oracle reads UTF-8. A case is `opaque` when its bytes are not valid UTF-8, even after the normalization the
grammar allows. The grammar's verdict then says nothing about a parser that decodes those bytes in some other
way, as UTF-16 or with replacement characters.

So for a parser that `Accepts` the tag explaining the bytes, usually `encoding/not-utf8`, an opaque case is
`Undecided`. A parser that `Refuses` the tag still expects a rejection: refusing does not depend on what the
bytes denote.
