// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser"
)

// renderChangesValue reports whether reading a document, writing it back and
// reading it again gives a different value.
//
// It takes only the document, which is what lets it double as the predicate for
// reduction: the expected value comes from reading the document itself rather
// than from the generator, so it stays meaningful on text the generator never
// produced.
func renderChangesValue(src []byte) bool {
	var before any
	if err := yaml.Unmarshal(src, &before); err != nil {
		return false
	}

	file, err := parser.ParseBytes(src, parser.WithComments())
	if err != nil {
		return false
	}

	var after any
	if err := yaml.Unmarshal([]byte(file.String()), &after); err != nil {
		// Rendering produced something unreadable, which is a change of value
		// by any measure.
		return true
	}

	return !sameValue(before, after)
}
