---
title: Tags and stances
weight: 20
description: |
  Why a case stores the grammar's verdict and a list of tags, and how a stance turns them into an expected outcome.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Why can a corpus not store "must accept" or "must reject"?
- What is a tag, and how is it different from a feature?
- What are the stages, and what does `verdictAt` bound?
- What does `Opaque` mean for a parser that decodes other encodings?
- Which tags are settled by the specification, and can a stance override them?

## Sections

- Implementation-defined properties: the RFC 8259 and YAML 1.2 examples
- `Tag`: a question the language leaves open; gates scoring
- `Feature`: a construct the document contains; selects, never decides
- `Stand`: `Undeclared`, `Accepts`, `Refuses`, `Either`
- `Table` and `Table.Expect`; `Outcome`: `Accept`, `Reject`, `Undecided`
- Stages: `Parse`, `Compose`, `Construct`; `Table.At` as a floor; `TagSpec.Stage`
- `VerdictAt`: why acceptance needs a bound and refusal does not
- Settled rules: `Rules`, `TagSpec.Settled`, `Rules.Contradicted`
- `Opaque`: the limit of a UTF-8 oracle
