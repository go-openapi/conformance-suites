---
title: Meanings and readings
weight: 40
description: |
  What a document denotes, under each reading that has an answer.
---

A verdict says whether a parser must read a document. A **meaning** says what the parser must read it as. The YAML
corpus carries a meaning for {{% siteparam "metrics.yaml_compact_meanings" %}} of its
{{% siteparam "metrics.yaml_compact_cases" %}} compact cases. The JSON corpus carries
none.

## When a case carries a meaning

The generator draws a value first and writes it as YAML second, so it knows what each document it writes means.
The hand-written families, such as the anchor and schema shapes, state theirs too. A case carries no meaning
when:

- the grammar refuses the document: a refused document denotes nothing;
- the document is a mutant: nobody knows what a broken document denotes, even when it is still valid;
- the generator declines to say. One shape does this, under `%YAML 1.1`: the same text written plain in one place
  and quoted in another, where the generator cannot tell which occurrence resolved to a boolean.

## The fields

A meaning is stored in the case, as a JSON object:

| field | holds |
|---|---|
| `under` | the reading that gives this answer, such as `yaml-1.2-core` |
| `json` | the value, written as JSON |
| `cyclic` | `true` when the value holds a cycle, and then `json` is absent |

Compare `json` as a JSON value, not as text: the writer compacts the whitespace, and a reader that decodes
numbers into float64 rounds those it cannot hold.

A cyclic value, such as `recursive: &x [ *x ]`, has no JSON form. The compact corpus holds
{{% siteparam "metrics.yaml_compact_cyclic" %}} such cases. A parser that returns a value it can write as JSON for
one of them has replaced the cycle with something else.

## Readings

A plain scalar carries no tag, so a resolution step outside the grammar decides what it denotes. YAML defines
several. A **reading** names the one used:

| reading | resolution |
|---|---|
| `yaml-1.2-core` | YAML 1.2's core schema, the specification's default |
| `yaml-1.1` | YAML 1.1's types, still common: `yes` is a boolean and `0777` is octal |

The header lists the readings the corpus weighed in `readings`. The JSON and failsafe schemas of YAML 1.2 are not
weighed. The failsafe schema reads every scalar as a string. The JSON schema may refuse a scalar that matches none
of its own productions, which would be a verdict and not a value, and the corpus leaves that question open.

## One meaning, or one per reading

Most documents mean the same under every reading. Such a case carries `meaning` alone.

Where the readings disagree, the case also carries `meanings`, with one entry per reading that has an answer. The
core answer appears in both fields, so a consumer can look in one place. The compact corpus holds
{{% siteparam "metrics.yaml_compact_meanings_split" %}} such cases:

```json
{
  "name": "shape/schema/0777",
  "src": "azogMDc3Nwo=",
  "wellFormed": true,
  "tags": ["schema/int-octal-legacy"],
  "meaning": {"under": "yaml-1.2-core", "json": {"k": 777}},
  "meanings": [
    {"under": "yaml-1.2-core", "json": {"k": 777}},
    {"under": "yaml-1.1", "json": {"k": 511}}
  ]
}
```

`src` is `k: 0777` in base64. When `meanings` is present and has no entry for a reading, that reading has no
answer for the document: YAML 1.1, for example, has no reading of a `0o513` integer under a `!!int` tag.

## Pick the meaning for your parser

Declare the reading your parser implements in `stance.Table.Reads`, then ask the header:

```go
table := stance.Table{Reads: "yaml-1.1" /* ... */}

if m, ok := header.MeaningFor(c, table.Reads); ok {
	// compare your parser's value with m.JSON, or check m.Cyclic
}
```

`Header.MeaningFor` returns one of three answers:

- the case lists an answer under the reading: it returns that answer;
- the case lists none, and the corpus weighed the reading: the readings agree, so it returns `meaning`;
- the corpus never weighed the reading: it returns `false`, and the case says nothing about your values.

An empty `Reads` asks for the corpus default, the answer in `meaning`.

Without the header's list, the second and third answers would look the same. A parser reading the failsafe schema
would be handed the integer for `k: 1`, and marked wrong for reading the string `"1"`.

Without Go, the same steps take a few lines: look for your reading in `meanings`; if `meanings` is absent and your
reading is in the header's `readings`, use `meaning`; otherwise skip the value.

## A meaning never changes a verdict

`Table.Expect` does not read `Table.Reads`. `k: 0777` is a valid document under every reading; the readings
disagree only on what it denotes. So a parser on another reading is scored on the verdict as usual, and on the
value only where the corpus states an answer for its reading.
