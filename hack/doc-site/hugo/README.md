# Hugo documentation site

Configuration for the conformance-suites documentation site, published to
https://go-openapi.github.io/conformance-suites/ by `.github/workflows/update-doc.yml`.

## Layout

```
hugo/
├── hugo.yaml               # static configuration: theme, mounts, parameters
├── conformance-suites.yaml.template   # version parameters, filled at build time
├── conformance-suites.yaml            # generated from the template; not committed
├── metrics.yaml            # corpus and grammar counts, referenced by siteparam
├── gendoc.go               # local dev server: go run gendoc.go
├── layouts/                # shortcodes and partials that override the theme
└── themes/
    ├── hugo-relearn/       # the Relearn theme, downloaded by CI
    ├── goyaml-assets/      # custom SCSS, logo
    └── goyaml-static/      # favicon and other static files
```

Content lives in `docs/doc-site/` at the repository root and is mounted as
Hugo's content directory. Every page there is hand-written; nothing on the site
is generated from source.

## Running it locally

```cmd
go run gendoc.go
```

The script reads the Go version from `go.mod` and the latest tag from git,
writes `conformance-suites.yaml` from the template, and starts Hugo with live reload on
http://localhost:1313/conformance-suites/.

It needs `hugo` (extended, v0.153.3 — CI pins the version because Hugo breaks
things between releases) and the Relearn theme unpacked into
`themes/hugo-relearn`.

## Configuration

Three config files are merged, in this order:

1. `hugo.yaml` — everything that does not change between builds.
2. `conformance-suites.yaml` — `params.conformancesuites.goVersion`, `latestRelease`,
   `versionMessage` and `buildTime`, generated from `conformance-suites.yaml.template`.
3. `metrics.yaml` — `params.metrics.*`, the corpus and grammar counts.

Pass all three to Hugo: `--config hugo.yaml,conformance-suites.yaml,metrics.yaml`. Hugo
takes no parameters on the command line, which is why the version numbers
arrive through a generated file.

Read a parameter from a page with the Relearn `siteparam` shortcode:

```markdown
The full JSON corpus holds {{% siteparam "metrics.json_full_cases" %}} cases.
```

`metrics.yaml` is maintained by hand for now. The corpus builder should emit it,
so that a page quoting a count cannot go stale.
