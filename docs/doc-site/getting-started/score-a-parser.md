---
title: Score a parser in Go
weight: 20
description: |
  Score a Go parser against the embedded JSON corpus with `suite`, `stance` and `jsoncorpus`.
---

This page scores the standard library's `json.Valid` against the JSON compact corpus embedded in the module.
Replace `json.Valid` with a call to your own parser.

## Install

```cmd
go get github.com/go-openapi/conformance-suites
```

The module requires {{% goversion "go1.26" %}} or newer.

## Load a corpus

`jsoncorpus.CompactSuite` returns the header and the cases of the compact corpus embedded in the module:

```go
header, cases, err := jsoncorpus.CompactSuite()
```

To use a corpus file you downloaded or built, open it and call `suite.ReadAll`:

```go
f, err := os.Open("json-full.jsonl.gz")
if err != nil {
	return err
}
defer f.Close()

header, cases, err := suite.ReadAll(f)
```

You can build the full JSON corpus with `jsoncorpus.Full().Write(w)`.

## Declare your parser's positions

The JSON grammar does not settle everything: a parser may refuse a byte order mark, or accept an unpaired
surrogate escape, and still conform to RFC 8259. Each such property has a tag. A `stance.Table` records your
parser's position on every tag:

```go
var stdlib = stance.Table{
	Name:    "encoding/json Valid",
	Because: "checks syntax only, and requires UTF-8 with no byte order mark",
	At:      stance.Parse,
	Speaks:  jsoncorpus.Vocabulary(),
	Stands: map[stance.Tag]stance.Stand{
		stance.TagBOM:               stance.Refuses,
		stance.TagNotUTF8:           stance.Accepts,
		stance.TagUTF16:             stance.Refuses,
		stance.TagUTF32:             stance.Refuses,
		jsoncorpus.TagLoneSurrogate: stance.Accepts,
		jsoncorpus.TagDeepNesting:   stance.Refuses,
	},
}
```

- `Name` and `Because` appear in the reasons that `Table.Expect` returns.
- `At` is the last stage your parser reaches. `json.Valid` only checks syntax, so it stops at `stance.Parse`,
  and the corpus does not ask it about `number/out-of-range`, which arises at `stance.Construct`.
- `Speaks` gives the stage of each tag. Use the vocabulary of the grammar under test.
- `Stands` gives your position on each tag:

  | stand | meaning | effect on a case carrying the tag |
  |---|---|---|
  | `stance.Accepts` | your parser reads such documents | the grammar's verdict decides |
  | `stance.Refuses` | your parser rejects them on purpose | the case expects a rejection |
  | `stance.Either` | your parser does not guarantee either answer | the case is not scored |
  | `stance.Undeclared` | no position (the zero value) | the case is not scored |

Find the positions by measuring your parser, not by reading its documentation. `jsoncorpus.DefaultLexer` was
built that way, and two of its positions differed from what its authors expected.

## Check the vocabulary first

Before you score anything, check that your table covers every tag the corpus uses:

```go
if unknown := header.Unknown(stdlib.Speaks.Tags()); len(unknown) > 0 {
	return fmt.Errorf("the corpus uses tags this table does not know: %v", unknown)
}
```

A tag missing from your table counts as `Undeclared`, so every case carrying it becomes `Undecided`. Without
this check, a newer corpus with a new tag would score fewer of your cases and give no warning.

## Score every case

Convert each `suite.Case` to a `stance.Doc`, ask the table for the expected outcome, and compare it with what
your parser does:

```go
for _, c := range cases {
	tags := make([]stance.Tag, len(c.Tags))
	for i, tag := range c.Tags {
		tags[i] = stance.Tag(tag)
	}

	want, why := stdlib.Expect(stance.Doc{
		Name:       c.Name,
		Src:        c.Src,
		WellFormed: c.WellFormed,
		Opaque:     c.Opaque,
		Tags:       tags,
		VerdictAt:  suite.StageOf(c.VerdictAt),
	})
	if want == stance.Undecided {
		continue
	}

	accepted := json.Valid(c.Src)
	if accepted != (want == stance.Accept) {
		fmt.Printf("%s: want %v, got accepted=%v (%s)\n", c.Name, want, accepted, why)
	}
}
```

With the table above, `json.Valid` has no disagreement with the compact corpus.

## Read the outcome

`Table.Expect` returns one of three outcomes, and a reason:

- `stance.Accept`: a conforming parser with your positions reads the document.
- `stance.Reject`: it refuses the document.
- `stance.Undecided`: the case tells nothing about your parser. Skip it, and do not count it as a pass.

A case is `Undecided` when it carries a tag you did not declare or declared as `Either`, when the grammar's
acceptance does not reach the stage your parser stops at, or when the bytes are opaque to the oracle.
With the table above, every `Undecided` case is opaque: its bytes are not valid UTF-8, `json.Valid` accepts
some such documents, and the oracle cannot say what they denote. The reason string names the cause:

```
encoding/not-utf8: encoding/json Valid tolerates this, and nothing here can say what the bytes denote
without the decoder that would read them
```

## Score several modes

A parser with a strict mode and a lenient mode needs two tables, and one corpus. Copy the table, change the
positions that differ, and run the loop once per mode.

## Select a subset

Each case lists the constructs it contains in `Features`. To run only part of a corpus, filter the
`stance.Doc` values with `stance.With` or `stance.Without`. Features never change the expected outcome.

## Keep your results in your repository

This module does not ship a test harness or a record of known failures. Keep the scoring loop, your tables
and your list of known disagreements in your own repository, next to your parser.
