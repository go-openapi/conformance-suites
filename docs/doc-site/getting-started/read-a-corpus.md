---
title: Read a corpus
weight: 10
description: |
  Read a corpus file from any language: the header, the cases, and what to check before scoring.
---

A corpus is one file. This page reads it with `jq` and with Python, and turns a case into an expected outcome
without the Go packages. [File format](../../reference/file-format/) lists every field.

## Get a corpus file

The repository holds the compact corpus of each suite:

| suite | file |
|---|---|
| JSON | `json/testdata/json-compact.jsonl.gz` |
| YAML | `yaml/corpus/testdata/yaml-compact.jsonl.gz` |

Releases do not attach corpus files yet. To get the full JSON corpus, build it with Go:

```go
f, err := os.Create("json-full.jsonl.gz")
if err != nil {
	return err
}
defer f.Close()

return jsoncorpus.Full().Write(f)
```

## Choose a tier

| tier | JSON cases | use it to |
|---|---|---|
| `compact` | {{% siteparam "metrics.json_compact_cases" %}} | run on every change: it keeps up to {{% siteparam "metrics.json_per_signature" %}} cases per coverage signature |
| `full` | {{% siteparam "metrics.json_full_cases" %}} ({{% siteparam "metrics.json_full_size" %}}) | run before a release: it keeps every generated document |

The header's `tier` field says which one a file holds. [Coverage](../../concepts/coverage/) explains what the
compact tier leaves out.

## The layout

- The file is gzip-compressed JSON Lines.
- Line 1 is the header.
- Every following line is one case.
- A case's `src` holds the test document in base64, because many documents are not valid UTF-8.

## Read it with jq

The header:

```cmd
gunzip -c json-compact.jsonl.gz | head -1 | jq .
```

Every tag the corpus uses, with its stage and whether the specification settles it:

```cmd
gunzip -c json-compact.jsonl.gz | head -1 | jq -r '.vocabulary[] | "\(.tag) \(.stage) \(.settled // "open")"'
```

How many cases carry each tag:

```cmd
gunzip -c json-compact.jsonl.gz | tail -n +2 | jq -r '.tags[]?' | sort | uniq -c
```

The documents carrying one tag, decoded:

```cmd
gunzip -c json-compact.jsonl.gz | tail -n +2 \
  | jq -r 'select(.tags | index("string/lone-surrogate")) | .src | @base64d'
```

`@base64d` writes text, so use it only on documents that are valid UTF-8. For the bytes of any case, pipe `.src`
to `base64 -d`.

## Read it in another language

Any language with gzip, JSON and base64 can read a corpus. In Python:

```python
import base64
import gzip
import json

with gzip.open("json-compact.jsonl.gz", "rt", encoding="utf-8") as f:
    header = json.loads(f.readline())
    if header["format"] != 4:
        raise SystemExit(f"unsupported format {header['format']}")

    cases = [json.loads(line) for line in f]

if len(cases) != header["cases"]:
    raise SystemExit(f"truncated: {len(cases)} of {header['cases']} cases")

for case in cases:
    src = base64.b64decode(case["src"])
    tags = case.get("tags", [])
    # score src here
```

The example checks the format first and the length last, as [File format](../../reference/file-format/#checks-before-scoring)
asks.

## Check the vocabulary

Before you score anything, read `header.vocabulary` and decide your parser's position on every tag in it that
is not `settled`. A tag you have no position on leaves every case carrying it unscored. A newer corpus can add a
tag, so run this check on every new file.

## Turn a case into an expected outcome

You need three things about your parser:

- **Its stage:** how far it takes a document. `parse` for a parser that only checks syntax, `compose` for one that
  builds a tree and resolves aliases, `construct` for one that builds native values.
- **Its stand on each open tag:** it accepts such documents, refuses them on purpose, or does not guarantee
  either answer.
- **Which settled rules it declines.** A parser that never resolves aliases cannot check a rule about them, and
  declines it.

Then work through these steps in order. The first step that gives an answer decides.

1. **Leave out the tags past your stage.** A tag whose `stage` in the header comes later than your parser's stage
   asks nothing of it.
2. **Settled tags.** Go through the case's tags in the order it lists them. For a tag whose spec has `settled`:
   - if you decline its rule, the case is **not scored**;
   - if `settled` is `reject`, **expect a rejection**;
   - if `settled` is `accept`, the construct is legal and the grammar still decides, so go on.
3. **Open tags.** Go through the remaining tags in order:
   - you refuse it: **expect a rejection**;
   - you have no position, or you do not guarantee either answer: the case is **not scored**;
   - you accept it, and the case is `opaque`: the case is **not scored**, because nothing can say what the bytes
     denote without the decoder your parser uses;
   - you accept it: go on.
4. **The grammar refuses it.** If `wellFormed` is `false`, **expect a rejection**. A document that does not parse
   does not compose or construct either.
5. **The verdict's reach.** `verdictAt` is the last stage at which the grammar's acceptance is evidence. If it is
   earlier than your parser's stage, the case is **not scored**. An absent `verdictAt` means `parse`.
6. Otherwise, **expect acceptance**.

A case that is not scored tells nothing about your parser. Skip it, and do not count it as a pass. The Go
package `stance` runs the same steps in `Table.Expect`; see [Score a parser in Go](../score-a-parser/).

{{% notice style="info" %}}
The YAML Test Suite view, a jq filter that writes `in.yaml`, `in.json` and `error` for each case, comes with the
YAML pages.
{{% /notice %}}
