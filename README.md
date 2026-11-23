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

<!--
## Announcements
-->

## Status

Early days. The API may still change.

What ships today:

- the YAML 1.2 and JSON (RFC 8259) oracles
- the JSON corpus: its generator, and a small corpus embedded in the module
- the corpus file format, and the types to read it and score a parser against it

What comes next:

- the YAML corpus and its generator, now being moved here from [`go-openapi/go-yaml`][go-yaml]
- full-size corpora, published as release assets

## Motivation

Hand-written test suites such as the [YAML Test Suite][yaml-test-suite] are precise but small, and they
only cover the cases somebody thought of. We want to test parsers on more than a hundred thousand documents,
most of them broken in ways nobody would write by hand.

Generating documents is easy. Labelling them is not: every generated document needs a verdict, valid or
invalid, and the generator cannot supply it. A slightly broken YAML document is usually still valid YAML,
just a different one. And no existing parser can supply the verdict either, because the corpus would then
score every other parser against that one parser's bugs.

So this project compiles the grammar each specification publishes into a recognizer, and uses it as the
oracle. The recognizer builds no tree and returns no error messages: it only says whether a document
matches the grammar.

A grammar does not settle everything. RFC 8259 lets a parser limit the range of numbers, and leaves an
unpaired surrogate escape undefined. YAML 1.2 allows three encodings, where a library may support one.
A corpus that stored "this document must be accepted" would fail a parser that conforms but chose
differently. So each case records the grammar's verdict plus a list of tags, one per
implementation-defined property the document has. You declare your parser's position on each tag, and
`Table.Expect` derives the expected outcome from both.

## Features

- **Oracles.** `yamlcorpus.Stream` checks a YAML 1.2 stream, `jsoncorpus.Text` checks a JSON text. Both
  are compiled at init from machine-readable grammars: the 211 productions published by
  [`yaml/yaml-grammar`][yaml-grammar], and RFC 8259 transcribed into the same format. RFC 8259 compiles
  with no patches. YAML 1.2 needs a few patches, for rules the spec states only in prose.
- **Coverage.** The oracle records which productions a document entered, and for YAML in which
  context. The corpus builder uses it to drop valid documents that add no coverage.
- **JSON corpus.** `jsoncorpus.Generate` draws documents from a seed and mutates each of them.
  `jsoncorpus.Compact` builds a small corpus: valid documents that add coverage, up to 16 refused
  documents per failure signature, and every encoding shape. `jsoncorpus.Full` keeps every document.
  `jsoncorpus.CompactSuite` returns the compact corpus embedded in the module.
- **A portable file format.** A corpus is a gzipped JSON Lines file: a header, then one case per line.
  Any language with a JSON reader and base64 can read it. In Go, use `suite.ReadAll`.
- **Positions.** A `stance.Table` declares your parser's position on each tag, and `Table.Expect` turns
  a case into `Accept`, `Reject` or `Undecided`. To test a strict mode and a lenient mode, declare two
  tables over the same corpus.

## Usage

```cmd
go get github.com/go-openapi/conformance-suites
```

Ask the oracles directly:

```go
ok := yamlcorpus.Stream([]byte("a: [1, 2]\n")).OK // YAML 1.2
ok = jsoncorpus.Text([]byte(`{"a":1}`)).OK         // RFC 8259
```

Score a JSON parser against the embedded corpus:

```go
header, cases, err := jsoncorpus.CompactSuite()
if err != nil {
	return err
}

table := jsoncorpus.DefaultLexer // declare your parser's own positions here

// Check first that your table answers every question the corpus asks.
if unknown := header.Unknown(table.Speaks.Tags()); len(unknown) > 0 {
	return fmt.Errorf("undeclared tags: %v", unknown)
}

for _, c := range cases {
	tags := make([]stance.Tag, len(c.Tags))
	for i, tag := range c.Tags {
		tags[i] = stance.Tag(tag)
	}

	want, why := table.Expect(stance.Doc{
		Name:       c.Name,
		Src:        c.Src,
		WellFormed: c.WellFormed,
		Opaque:     c.Opaque,
		Tags:       tags,
		VerdictAt:  suite.StageOf(c.VerdictAt),
	})
	if want == stance.Undecided {
		continue // this case tells nothing about this parser
	}

	accepted := myParser(c.Src) == nil
	if accepted != (want == stance.Accept) {
		fmt.Printf("%s: want %v (%s)\n", c.Name, want, why)
	}
}
```

## Consume released conformance suites

No full-size corpus is released yet. Until then, build the JSON one yourself with
`jsoncorpus.Full().Write(w)`. Released corpora will ship as release assets, in the same format as the
embedded one:

- line 1 is the header: grammar, grammar digest, generator, seed, tier, case count, and the tags the
  corpus uses
- every other line is a case: name, document (base64 in `src`), the grammar's verdict in `wellFormed`,
  tags, and where it came from

The header's seed and generator reproduce the corpus exactly. Its digest identifies the grammar file
behind the verdicts, so a corpus built from an older grammar is easy to detect.

## Packages

| package | contents |
|---|---|
| [`json`](./json) (`jsoncorpus`) | the JSON oracle, generator and corpus, and the JSON tags |
| [`yaml`](./yaml) (`yamlcorpus`) | the YAML 1.2 oracle and its coverage; the YAML corpus is moving in from go-yaml |
| [`suite`](./suite) | the corpus file format: `Reader`, `Writer`, `Header`, `Case` |
| [`stance`](./stance) | tags, positions and expected outcomes: `Tag`, `Stand`, `Table`, `Outcome` |

`suite` and `stance` import only the standard library.

This module imports no YAML or JSON parser, not even in tests. A corpus that depended on a parser could
not fairly score another one.

## Known limitations

- The recognizer stores offsets as `int32`: documents are limited to 2 GiB.
- The oracle reads UTF-8. Documents in other encodings are tagged, and a case the oracle cannot decode
  is marked `opaque`: its verdict tells nothing about a parser that decodes it differently.
- Verdicts are YAML 1.2 only: there is no YAML 1.1 grammar. The file format can record what a scalar
  means under each schema where the schemas disagree (`0777` is 511 under YAML 1.1 and 777 under the
  1.2 core schema).

## Credits

- The [YAML Test Suite][yaml-test-suite] by [`@ingydotnet`][ingydotnet]. We vendor it under
  `internal/yamltestsuite` to calibrate the YAML recognizer.
- [`yaml/yaml-grammar`][yaml-grammar], which publishes the YAML 1.2 productions as data. Our YAML
  recognizer is compiled from it.

## FAQ

## Change log

See <https://github.com/go-openapi/conformance-suites/releases>

## References

- [YAML 1.2.2 specification][yaml-spec]
- [RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format][rfc8259]

## Licensing

This library ships under the [SPDX-License-Identifier: Apache-2.0](./LICENSE).

The vendored YAML Test Suite fixtures ship under the MIT License. See [NOTICE](./NOTICE).

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
<!-- Reference material -->
[yaml-spec]: https://yaml.org/spec/1.2.2/
[rfc8259]: https://www.rfc-editor.org/rfc/rfc8259
[yaml-grammar]: https://github.com/yaml/yaml-grammar
[yaml-test-suite]: https://github.com/yaml/yaml-test-suite
[go-yaml]: https://github.com/go-openapi/go-yaml
<!-- Credits & mentions -->
[ingydotnet]: https://github.com/ingydotnet
