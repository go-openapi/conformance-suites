// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"regexp"
	"testing"

	"pgregory.net/rapid"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
)

// TestEveryEmittedDocumentIsValidYAML gates the emitter against the grammar
// rather than against the library.
//
// This is the one test here whose failures are always ours. The emitter decides
// how to write a value down; the YAML 1.2 grammar decides whether that is a
// document at all. Neither of them is the code under test, so a disagreement is
// an emitter defect and there is no ledger entry to hide behind.
//
// It has already earned its place. The emitter refused control characters in
// unquoted scalars but let the byte order mark through, because U+FEFF is not a
// control character -- and nb-char excludes it, so those documents were not
// YAML. Every property test was green throughout: the library reads them, and
// reading them back gives the value.
func TestEveryEmittedDocumentIsValidYAML(t *testing.T) {
	oracle := yamloracle.NewRecognizer(1024)

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")

		src := yamlgen.Emit(value, style)

		if !oracle.Stream([]byte(src)).OK {
			rt.Fatalf("style %s wrote something that is not YAML 1.2:\n%s", style, indent(src))
		}
	})
}

// TestEveryBlockScalarStyleIsActuallyReached guards against an axis that is
// wired up everywhere except where it is switched on.
//
// A presentation the generator never produces costs nothing to keep and proves
// nothing: the properties pass, the ledger stays empty, and the axis reads as
// covered. Folded scalars were drawn zero times out of twenty thousand
// documents before the style generator was taught the field, and every property
// was green throughout.
var blockScalarStyles = []string{"literal", "folded", "stated indent"}

func TestEveryBlockScalarStyleIsActuallyReached(t *testing.T) {
	seen := map[string]int{}

	// Counted over its own documents rather than over -rapid.checks, because
	// this asks about coverage and not about a property, and the answer does
	// not get better with depth. The rarest of the three is a literal scalar,
	// which needs a style that asks for one and a string that folding cannot
	// express: about two documents in a hundred, so a default run of a hundred
	// would draw none about one time in eight and report a live axis dead.
	const (
		perCheck = 25
		enough   = 20
	)

	covered := func() bool {
		for _, name := range blockScalarStyles {
			if seen[name] < enough {
				return false
			}
		}

		return true
	}

	rapid.Check(t, func(rt *rapid.T) {
		if covered() {
			return
		}

		for range perCheck {
			src := yamlgen.Emit(
				yamlgen.Values().Draw(rt, "value"),
				yamlgen.Styles().Draw(rt, "style"),
			)

			for name, marker := range map[string]*regexp.Regexp{
				"literal":       regexp.MustCompile(`\|[0-9]?[-+]?\n`),
				"folded":        regexp.MustCompile(`>[0-9]?[-+]?\n`),
				"stated indent": regexp.MustCompile(`[|>][0-9][-+]?\n`),
			} {
				if marker.MatchString(src) {
					seen[name]++
				}
			}
		}
	})

	for _, name := range blockScalarStyles {
		assert.Positivef(t, seen[name], "no document used a %s block scalar", name)
		t.Logf("%-13s reached %d times", name, seen[name])
	}
}
