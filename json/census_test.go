// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	jsoncorpus "github.com/go-openapi/conformance-suites/json"
)

// knownGaps are the constructs JSONTestSuite has and the corpus cannot hold yet, with why, so that the census fails
// on a NEW gap and not on the standing ones. Closing one is logged, not failed.
//
// It is empty. The census found eight gaps on the run it was written, 2026-11-23, and the next change closed them:
// Style.Before and Style.After write whitespace around the value, Style.ShortEscapes writes \b and \f,
// randomNumber writes more than 17 digits, and RandomValue draws deep chains. Duplicate names and lone surrogates
// have meanings RFC 8259 leaves open, so ValueShapes writes them by hand, tagged.
var knownGaps = map[string]string{}

// TestTheGeneratedCorpusIsWiderThanTheSuite is the census.
//
// The stored corpus is meant to be strictly wider than JSONTestSuite: every construct a valid suite document holds,
// some valid corpus case should hold too. Where none does, the generator has a construct it cannot write, and no
// amount of drawing reaches it.
//
// Grammar coverage cannot say this: the suite and the corpus both enter all 33 reachable productions. See
// [jsoncorpus.Constructs].
//
// Only valid documents are compared. A refused suite document breaks something, and its constructs are the parts
// left standing around the break.
func TestTheGeneratedCorpusIsWiderThanTheSuite(t *testing.T) {
	inSuite := map[string]int{}
	documents := 0

	for _, c := range suiteCases(t) {
		if !jsoncorpus.Text(jsoncorpus.Normalize(c.Src)).OK {
			continue
		}

		documents++

		for _, k := range jsoncorpus.Constructs() {
			if k.Has(c.Src) {
				inSuite[k.Name]++
			}
		}
	}

	_, cases, err := jsoncorpus.CompactSuite()
	if err != nil {
		t.Fatal(err)
	}

	// Counted three ways, as for YAML. A construct the generator draws is one it can write. One only an enumerated
	// case holds was written by hand, which answers a gap but is a different claim. One only a byte mutant holds
	// is there by accident: the mutation happened to produce it.
	drawn, byHand, mutated := map[string]int{}, map[string]int{}, map[string]int{}

	for _, c := range cases {
		if !c.WellFormed {
			continue
		}

		into := drawn

		switch {
		case strings.HasPrefix(c.Name, "shape/") || c.Origin.Mutation == "enumerated":
			into = byHand
		case c.Origin.Mutation != "":
			into = mutated
		}

		for _, k := range jsoncorpus.Constructs() {
			if k.Has(c.Src) {
				into[k.Name]++
			}
		}
	}

	var missing []string

	for _, k := range jsoncorpus.Constructs() {
		t.Logf("%-40s suite %4d   drawn %6d   by hand %3d   mutated %5d",
			k.Name, inSuite[k.Name], drawn[k.Name], byHand[k.Name], mutated[k.Name])

		if inSuite[k.Name] == 0 || drawn[k.Name] > 0 || byHand[k.Name] > 0 {
			continue
		}

		if why, known := knownGaps[k.Name]; known {
			t.Logf("KNOWN GAP %q: %s", k.Name, why)

			continue
		}

		missing = append(missing, fmt.Sprintf(
			"%q: %d suite documents hold it, no generated or enumerated case does, and %d mutants hold it by accident",
			k.Name, inSuite[k.Name], mutated[k.Name]))
	}

	for name, why := range knownGaps {
		if drawn[name] > 0 || byHand[name] > 0 {
			t.Logf("CLOSED %q is reached now -- drop it from knownGaps (%s)", name, why)
		}
	}

	sort.Strings(missing)

	for _, line := range missing {
		t.Errorf("%s", line)
	}

	t.Logf("%d constructs, %d valid suite documents, %d corpus cases", len(jsoncorpus.Constructs()), documents, len(cases))
}
