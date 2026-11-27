---
title: "go-openapi conformance suites"
type: home
description: Conformance test suites for JSON and YAML parsers, generated from the published grammars
weight: 1
---

Conformance test suites for JSON and YAML parsers.

This library generates test suites.

Each document is generated, and its verdict comes from the grammar published by its specification.

### Status

{{% button href="https://github.com/go-openapi/conformance-suites/fork" hint="fork me on github" style=primary icon=code-fork %}}Fork me{{% /button %}}

Early days: the API is still in flux. Documentation and code are still being honed...

### Getting started

Each suite is published as a gzipped JSON Lines file.

You can read it from any language with a JSON reader and a base64 decoder.

You don't need Go or this library to work with the suite. See [Read a corpus](./getting-started/read-a-corpus/).

**Go**

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
