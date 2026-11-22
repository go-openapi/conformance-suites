---
title: Score a parser in Go
weight: 20
description: |
  Score a Go parser against the embedded JSON corpus with `suite`, `stance` and `jsoncorpus`.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I load a corpus in Go?
- How do I declare my parser's positions?
- What does `Undecided` mean, and why is it not a pass?
- How do I score two modes of the same parser?

## Sections

- Install, and the minimum Go version (the `goversion` shortcode)
- Load: `jsoncorpus.SmokeSuite`, or `suite.ReadAll` over a downloaded file
- Declare a `stance.Table`: `Name`, `Because`, `Stands`, `Speaks`, `At`; start from `jsoncorpus.DefaultLexer`
- Check first: `Header.Unknown`, and `Rules.Contradicted` for a table that accepts a settled rule
- The loop: build a `stance.Doc` from a `suite.Case`, call `Table.Expect`, compare with your parser
- Reading the result: `Accept`, `Reject`, `Undecided`, and the reason string
- Two tables over one corpus: strict and lenient modes
- Selecting a subset with features: `stance.With`, `stance.Without`
- What this module does not ship: a harness or a ledger. Keep yours in your own repository
