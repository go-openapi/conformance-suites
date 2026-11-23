// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlembed embeds the stored YAML corpus, so a Go test can read it with one call.
//
// [github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild] writes the corpus to testdata/yaml-smoke.jsonl.gz
// in this directory. The file is a released artifact: any JSON reader with base64 can read it, and this package spares
// a Go test the generator, the oracle and their dependencies.
package yamlembed

import (
	_ "embed"

	"github.com/go-openapi/conformance-suites/suite"
)

//go:embed testdata/yaml-smoke.jsonl.gz
var smokeArtifact []byte

// SmokeSuite returns the header and the cases of the stored corpus.
func SmokeSuite() (suite.Header, []suite.Case, error) {
	return suite.FromBytes(smokeArtifact)
}

// Bytes returns the stored corpus as written: a gzipped JSONL file.
//
// Use it to keep a copy of the file in step with this module.
func Bytes() []byte {
	return smokeArtifact
}
