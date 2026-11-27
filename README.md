# conformance-suites

<!-- Badges: status  -->
[![Tests][test-badge]][test-url] [![Coverage][cov-badge]][cov-url] [![CI vuln scan][vuln-scan-badge]][vuln-scan-url] [![CodeQL][codeql-badge]][codeql-url]
<!-- Badges: release & docker images  -->
<!-- Badges: code quality  -->
<!-- Badges: license & compliance -->
[![Release][release-badge]][release-url] [![CodeFactor Grade][codefactor-badge]][codefactor-url] [![License][license-badge]][license-url]
<!-- Badges: documentation & support -->
<!-- Badges: others & stats -->
[![GoDoc][godoc-badge]][godoc-url] [![Discord Channel][discord-badge]][discord-url] [![go version][goversion-badge]][goversion-url] ![Top language][top-badge] ![Commits since latest release][commits-badge]

---

**Conformance test suites for JSON and YAML parsers, generated from the published grammars.**

## Announcements

* **2026-11-2 7** :
  * v0.0.4 landed, first YAML test corpus released (compact)

## Status

Early days: the API is still in flux. Documentation and code are still being honed...

A first corpus is released (that's the compact YAML compact, with ~ 28 000 documents).
Other will follow suit!

Test suites for YAML 1.2 and JSON (RFC 8259) ship today.
Each comes with an oracle, a suite generator and a compact corpus embedded in Go.

Full-size corpora ship as release assets.

## Motivation

Hand-written test suites such as the [YAML Test Suite][yaml-test-suite] are precise but small.

We want to test parsers on thousands of documents, most of them broken in ways nobody would write by hand.
We also want tools to test our YAML and JSON parsers using property-based and fuzz tests.

For that we need corpus generators, based on grammars and that are smart enough to capture extra-grammar rules.

Each document produced needs a verdict, and a classification to compare and explain the outcome of different parsers,
which may come with different implementation-specific stances.

This project compiles the grammar each specification publishes into a recognizer, and uses it as the oracle.

Where a specification leaves a choice to the implementation, a case carries a tag,
and we may declare a parser's position on each tag.

## Documentation

The [documentation site][doc-site] explains the [file format][doc-format], the [tags and positions][doc-tags],
and how to [score a parser][doc-score].

## Usage

```cmd
go get github.com/go-openapi/conformance-suites
```

Ask the oracles directly:

```go
import (
    jsoncorpus "github.com/go-openapi/conformance-suites/json"
    yamlcorpus "github.com/go-openapi/conformance-suites/yaml"
)

ok := yamlcorpus.Stream([]byte("a: [1, 2]\n")).OK // YAML 1.2
ok = jsoncorpus.Text([]byte(`{"a":1}`)).OK         // RFC 8259
```

Read an embedded corpus, then score your parser case by case with a `stance.Table`:

```go
import yamlembed "github.com/go-openapi/conformance-suites/yaml/corpus" // a module of its own: 1.6 MB

header, cases, err := jsoncorpus.CompactSuite() // in the root module
header, cases, err = yamlembed.CompactSuite()
```

To go past the corpus, seed a Go fuzz target with it, or draw YAML documents for property tests with
`yamlgen` and [rapid][rapid]: see [Fuzz and property tests][doc-fuzz].


## Corpora

A corpus is a gzipped JSON Lines file: a parser written in any language can be scored against it.

## Consensus

For YAML specifically, it is very difficult to compare parsers.

We have taken inspiration from the work by `@perlpunk` there [`YAML test matrix`][yaml-test-matrix].
and produce our own comparison of YAML parsers.

[Our consensus][doc-consensus] report compares fewer parsers, and tries to provide a deeper insight on how their differ.
We introduce our new YAML parser [`go-openapi/go-yaml`][openapi-go-yaml] to the bunch.

## Modules

| module | packages | contents |
|---|---|---|
| [root](./go.mod) | `suite`, `stance`, [`json`](./json) (`jsoncorpus`), [`yaml`](./yaml) (`yamlcorpus`) | the file format, positions and outcomes, the JSON suite, the YAML oracle |
| [`yaml/yamlgen`](./yaml/yamlgen) | `yamlgen`, `yamlbuild` | the YAML generator and corpus builder |
| [`yaml/corpus`](./yaml/corpus) | `yamlembed` | the compact YAML corpus, embedded |
| [`yaml/consensus`](./yaml/consensus) | `consensus`, `goopenapi`, `goyaml`, `libfyaml`, `perlref`, `yamlstar`, `cmd/consensus` | adapters for five YAML implementations, and the command that writes the [consensus report][doc-consensus] |
| [`yaml/calibrate`](./yaml/calibrate) | tests only | checks the generator against go-openapi/go-yaml |

The root module imports only the standard library. Only `yaml/consensus` and `yaml/calibrate` import a parser.

## Known limitations

- The recognizer stores offsets as `int32`: documents are limited to 2 GiB.
- The oracle reads UTF-8. A case in another encoding is tagged, or marked `opaque` when the oracle cannot decode it.
- Verdicts are YAML 1.2 only: there is no YAML 1.1 grammar.

## Credits

- The [YAML Test Suite][yaml-test-suite] by [`@ingydotnet`][ingydotnet]. We vendor it under
  `internal/yamltestsuite` to calibrate the YAML recognizer.
- [JSONTestSuite][json-test-suite] by Nicolas Seriot. We vendor it under `internal/jsontestsuite` to
  calibrate the JSON recognizer, and check that the generated JSON corpus holds every construct it holds.
- [`yaml/yaml-grammar`][yaml-grammar], which publishes the YAML 1.2 productions as data. Our YAML
  recognizer is compiled from it.
- The [YAML test matrix][yaml-test-matrix] by [`@perlpunk`][perlpunk] has been a valuable source of inspiration for our work.

<!--
## FAQ
-->

## Change log

See <https://github.com/go-openapi/conformance-suites/releases>

## References

- [YAML 1.2.2 specification][yaml-spec]
- [RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format][rfc8259]

Academic work on the topic of grammar-based test generation

Prior art

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

The vendored YAML Test Suite and JSONTestSuite fixtures ship under the MIT License. See [NOTICE](./NOTICE).

## Other documentation

* [All-time contributors](./CONTRIBUTORS.md)
* [Contributing guidelines][contributing-doc-site]
* [Maintainers documentation][maintainers-doc-site]
* [Code style][style-doc-site]

## Cutting a new release

Maintainers can cut a new release by either:

* running [this workflow](https://github.com/go-openapi/conformance-suites/actions/workflows/bump-release.yml)
* or pushing a semver tag
  * signed tags are preferred
  * The tag message is prepended to release notes

<!-- Badges: status  -->
[test-badge]: https://github.com/go-openapi/conformance-suites/actions/workflows/go-test.yml/badge.svg
[test-url]: https://github.com/go-openapi/conformance-suites/actions/workflows/go-test.yml
[cov-badge]: https://codecov.io/gh/go-openapi/conformance-suites/branch/master/graph/badge.svg
[cov-url]: https://codecov.io/gh/go-openapi/conformance-suites
[vuln-scan-badge]: https://github.com/go-openapi/conformance-suites/actions/workflows/scanner.yml/badge.svg
[vuln-scan-url]: https://github.com/go-openapi/conformance-suites/actions/workflows/scanner.yml
[codeql-badge]: https://github.com/go-openapi/conformance-suites/actions/workflows/codeql.yml/badge.svg
[codeql-url]: https://github.com/go-openapi/conformance-suites/actions/workflows/codeql.yml
<!-- Badges: release & docker images  -->
[release-badge]: https://badge.fury.io/gh/go-openapi%2Fconformance-suites.svg
[release-url]: https://badge.fury.io/gh/go-openapi%2Fconformance-suites
<!-- Badges: code quality  -->
[codefactor-badge]: https://img.shields.io/codefactor/grade/github/go-openapi/conformance-suites
[codefactor-url]: https://www.codefactor.io/repository/github/go-openapi/conformance-suites
<!-- Badges: documentation & support -->
[godoc-badge]: https://pkg.go.dev/badge/github.com/go-openapi/conformance-suites
[godoc-url]: http://pkg.go.dev/github.com/go-openapi/conformance-suites
[discord-badge]: https://img.shields.io/discord/1446918742398341256?logo=discord&label=discord&color=blue
[discord-url]: https://discord.gg/FfnFYaC3k5

<!-- Badges: license & compliance -->
[license-badge]: http://img.shields.io/badge/license-Apache%20v2-orange.svg
[license-url]: https://github.com/go-openapi/conformance-suites/?tab=Apache-2.0-1-ov-file#readme
<!-- Badges: others & stats -->
[goversion-badge]: https://img.shields.io/github/go-mod/go-version/go-openapi/conformance-suites
[goversion-url]: https://github.com/go-openapi/conformance-suites/blob/master/go.mod
[top-badge]: https://img.shields.io/github/languages/top/go-openapi/conformance-suites
[commits-badge]: https://img.shields.io/github/commits-since/go-openapi/conformance-suites/latest
<!-- Organization docs -->
[contributing-doc-site]: https://go-openapi.github.io/doc-site/contributing/contributing/index.html
[maintainers-doc-site]: https://go-openapi.github.io/doc-site/maintainers/index.html
[style-doc-site]: https://go-openapi.github.io/doc-site/contributing/style/index.html
<!-- Documentation -->
[doc-site]: https://go-openapi.github.io/conformance-suites/
[doc-format]: https://go-openapi.github.io/conformance-suites/reference/file-format/
[doc-tags]: https://go-openapi.github.io/conformance-suites/concepts/tags-and-stances/
[doc-score]: https://go-openapi.github.io/conformance-suites/getting-started/score-a-parser/
[doc-fuzz]: https://go-openapi.github.io/conformance-suites/getting-started/fuzz-and-properties/
[doc-consensus]: https://go-openapi.github.io/conformance-suites/consensus/index.html
<!-- Reference material -->
[yaml-spec]: https://yaml.org/spec/1.2.2/
[rfc8259]: https://www.rfc-editor.org/rfc/rfc8259
[yaml-grammar]: https://github.com/yaml/yaml-grammar
[yaml-test-suite]: https://github.com/yaml/yaml-test-suite
[json-test-suite]: https://github.com/nst/JSONTestSuite
<!-- Credits & mentions -->
[ingydotnet]: https://github.com/ingydotnet
[perlpunk]: https://github.com/perlpunk
[yaml-test-matrix]: https://github.com/perlpunk/yaml-test-matrix
[openapi-go-yaml]: https://github.com/go-openapi/go-yaml
[rapid]: https://github.com/flyingmutant/rapid
