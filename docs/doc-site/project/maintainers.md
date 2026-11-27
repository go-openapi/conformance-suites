---
title: Maintainers
weight: 40
description: |
  Package layout, where new code goes, and how a release is cut.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is up for review; the prose is not written.
{{% /notice %}}

## What the page must answer

- Where does a new grammar, generator or helper go?
- Which packages are public, and why?
- When does a second module appear?
- How is a corpus regenerated, and how is a release cut?

## Sections

- One directory per grammar (`json`, `yaml`); `suite` and `stance` beside them; the engine in `internal/grammar`
- Public: `suite` and `stance` (a consumer reads a file and declares a table); the oracle's result types
- Each grammar package re-exports the engine types it returns: `type Result = grammar.Result`
- Package names carry the grammar: `jsoncorpus`, `yamlcorpus`, `yamlgen`
- No parser imported in the root module, tests included; `consensus` is a module of its own
- One module until a dependency or an embed forces a split (`yaml/corpus`)
- Regenerating the compact corpus, and the regenerate-and-diff test
- Updating `metrics.yaml`
- Releasing: `bump-release.yml` or a signed semver tag; corpus assets once released

## The consensus report

The consensus image (`yaml/consensus/docker/Dockerfile`) pins every parser, go-openapi/go-yaml included. Bump the
go-yaml pin by hand, together with the stance files it needs. Each release runs the image on the compact corpus and
attaches the results file, `consensus-results.jsonl.gz`, to the release. The doc site renders the
[consensus report](../../reference/consensus/) from the results file of the latest release.

To produce a report yourself, build the image and run it on a corpus file, as the Dockerfile's header describes.
Then render it into this site, under `/consensus/`, or into any directory:

```sh
hack/doc-site/consensus-report.sh results.jsonl.gz
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" conformance-consensus \
  report -o /work/report /work/results.jsonl
```

The script also writes the results header to `hack/doc-site/hugo/data/consensus.json`, which the reference page
reads. Both outputs are gitignored.
