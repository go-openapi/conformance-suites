// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlcorpus compiles the YAML 1.2 grammar into an oracle for conformance testing.
//
// [YAML] is the YAML 1.2 grammar published by github.com/yaml/yaml-grammar,
// compiled with a few patches for the rules the spec states only in prose.
// [Stream] checks a whole YAML stream and [FlowNode] checks a single flow node.
// [NewRecognizer] returns a reusable oracle for a caller that checks documents in a loop.
//
// The YAML corpus and its generator are being moved into this package from github.com/go-openapi/go-yaml.
// Until then, this package provides the oracle only.
//
// # Using it
//
// Check one document:
//
//	ok := yamlcorpus.Stream([]byte("a: [1, 2]\n")).OK
//
// Check many, and record which productions they enter:
//
//	r := yamlcorpus.NewRecognizer(1024)
//	cov := yamlcorpus.NewCoverage()
//	r.Cover(cov)
//
//	for _, doc := range docs {
//		if !r.Stream(doc).OK {
//			// ...
//		}
//	}
//
// # Calibration
//
// The tests in this package run the oracle over the YAML Test Suite, vendored under internal/yamltestsuite.
// The oracle agrees with every case that states an expectation, except two.
// Both state a rule over the directives of one document,
// which a grammar cannot express: at most one %YAML directive per document,
// and a tag handle used in a document must be defined in that document.
package yamlcorpus
