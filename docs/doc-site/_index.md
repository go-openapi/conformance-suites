---
title: "go-openapi conformance suites"
type: home
description: Extended conformance test suites for YAML and JSON
weight: 1
---

### Status

{{% button href="https://github.com/go-openapi/conformance-test-suites/fork" hint="fork me on github" style=primary icon=code-fork %}}Fork me{{% /button %}}
Early days. API may still change.

<!-- See our [ROADMAP](./project/maintainers/ROADMAP.md). -->

### Getting started

```cmd
go get github.com/go-openapi/conformance-suites
```

Using only the dependency-free release artifacts:

```cmd
...
```

### Where to go next

{{< cards >}}
{{% card title="Features" %}}
Features supported by our client and server, with normative references.
→ [usage/features](./usage/features/)
{{% /card %}}
{{% card title="Core" %}}
The five interfaces (`Consumer`, `Producer`, `Authenticator`, `Authorizer`,
`OperationHandler`) every other piece is built on, plus content-type and
validation plumbing.

→ [usage/core](./usage/core/)
{{% /card %}}

{{% card title="Client" %}}
Configuring `client.Runtime` for TLS, auth, OpenTelemetry tracing and
context-aware request submission.

→ [usage/client](./usage/client/)
{{% /card %}}

{{% card title="Server" %}}
The Router → Binder → Validator → Security → OperationExecutor → Responder
pipeline that turns a spec into a handler.

→ [usage/server](./usage/server/)
{{% /card %}}

{{% card title="Standalone" %}}
Use the media-type, content-negotiation and doc-UI helpers from any plain
`net/http` server, with no transitive OpenAPI dependencies.

→ [usage/standalone](./usage/standalone/)
{{% /card %}}
{{< /cards >}}

Looking for runnable code? See [examples](./usage/examples/).

## Licensing

`SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers`

This library ships under the [Apache-2.0 license](./project/LICENSE.md).

## Contributing

Issues and pull requests welcome.

See the shared [go-openapi contributing guidelines][contributing-doc-site] and
the per-repo notes in [project/](./project/).

---

{{< children type="card" description="true" >}}

[go-yaml]: https://github.com/go-openapi/go-yaml
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
