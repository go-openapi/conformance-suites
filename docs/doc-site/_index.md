---
title: "go-openapi conformance suites"
type: home
description: Conformance test suites for JSON and YAML parsers, generated from the published grammars
weight: 1
---

Conformance test suites for JSON and YAML parsers. Each document is generated, and its verdict comes from
the grammar the specification publishes, not from another parser.

### Status

{{% button href="https://github.com/go-openapi/conformance-suites/fork" hint="fork me on github" style=primary icon=code-fork %}}Fork me{{% /button %}}
Early days. The file format and the API may still change.

The JSON suite is usable today. The YAML suite is moving here from [go-openapi/go-yaml][go-yaml].

### Getting started

Each suite is published as a gzipped JSON Lines file. You can read it from any language with a JSON reader
and a base64 decoder: you need neither Go nor this module. See [Read a corpus](./getting-started/read-a-corpus/).

To use the oracles, the generators, or the embedded corpus from Go:

```cmd
go get github.com/go-openapi/conformance-suites
```

Then see [Score a parser](./getting-started/score-a-parser/).

## Licensing

`SPDX-FileCopyrightText: Copyright 2025-2026 go-swagger maintainers`

This library ships under the [Apache-2.0 license](./project/LICENSE.md). The vendored YAML Test Suite
and JSONTestSuite fixtures ship under the MIT License.

## Contributing

Issues and pull requests welcome.

See the shared [go-openapi contributing guidelines][contributing-doc-site] and the per-repo notes in
[project/](./project/).

---

{{< children type="card" description="true" >}}

[go-yaml]: https://github.com/go-openapi/go-yaml
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
