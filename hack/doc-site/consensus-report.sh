#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# consensus-report.sh renders the consensus report into the doc site.
#
# Usage, from anywhere in the repository:
#
#   hack/doc-site/consensus-report.sh results.jsonl[.gz]
#
# It writes the report, unchanged, to hack/doc-site/hugo/consensus/, which Hugo serves at /consensus/.
# It writes the results header, without the tag vocabulary, to hack/doc-site/hugo/data/consensus.json,
# which the reference/consensus page reads. Both are gitignored.
#
# The results file comes from "consensus run" in the consensus image (yaml/consensus/docker/Dockerfile).
# Rendering needs Go and this checkout only: no parser is run.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 results.jsonl[.gz]" >&2
  exit 2
fi

root=$(git rev-parse --show-toplevel)
hugo="${root}/hack/doc-site/hugo"
results=$(realpath "$1")

work=$(mktemp -d)
trap 'rm -rf -- "${work}"' EXIT

# consensus report reads plain JSON Lines.
gzip -cdf -- "${results}" > "${work}/results.jsonl"

out="${hugo}/consensus"
rm -rf -- "${out}"
(cd "${root}" && go run ./yaml/consensus/cmd/consensus report -o "${out}" "${work}/results.jsonl")

mkdir -p "${hugo}/data"
head -n 1 "${work}/results.jsonl" | jq 'del(.corpus.vocabulary)' > "${hugo}/data/consensus.json"
