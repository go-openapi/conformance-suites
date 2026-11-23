// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-openapi/conformance-suites/internal/jsontestsuite"
	jsoncorpus "github.com/go-openapi/conformance-suites/json"
)

// TestRecognizerAgreesWithJSONTestSuite scores the recognizer against every
// document in Nicolas Seriot's JSONTestSuite.
//
// This runs before anything is generated, and it is the one step of the spike
// that must come out clean. A corpus is a cache of the oracle's verdicts, so an
// oracle that is wrong anywhere produces fixtures that accuse a parser of
// defects it does not have -- and unlike a live oracle, a frozen one cannot
// retroactively correct itself.
//
// The naming convention is the suite's own:
//
//   - y_*  must be accepted
//   - n_*  must be rejected
//   - i_*  implementation-defined, so recorded and never asserted
func TestRecognizerAgreesWithJSONTestSuite(t *testing.T) {
	var (
		yes, no, impl int
		disagreements []string
		implAccepted  []string
	)

	for _, c := range suiteCases(t) {
		got := jsoncorpus.Text(c.Src).OK

		switch c.Expect {
		case jsontestsuite.Accept:
			yes++
			if !got {
				disagreements = append(disagreements, "refused a valid document: "+c.Name+" -- "+excerpt(c.Src))
			}
		case jsontestsuite.Reject:
			no++
			if got {
				disagreements = append(disagreements, "accepted an invalid document: "+c.Name+" -- "+excerpt(c.Src))
			}
		default: // jsontestsuite.Either
			impl++
			if got {
				implAccepted = append(implAccepted, c.Name)
			}
		}
	}

	t.Logf("scored %d must-accept, %d must-reject, %d implementation-defined", yes, no, impl)
	t.Logf("of the implementation-defined cases, %d of %d are accepted", len(implAccepted), impl)

	if len(disagreements) == 0 {
		t.Logf("no disagreements")

		return
	}

	sort.Strings(disagreements)
	for _, d := range disagreements {
		t.Errorf("%s", d)
	}
	t.Errorf("%d disagreements over %d asserted documents", len(disagreements), yes+no)
}

// excerpt renders the start of a fixture so a disagreement is readable without
// opening the file. The suite carries deliberately hostile bytes, so it is
// quoted rather than printed.
func excerpt(src []byte) string {
	const most = 48

	s := string(src)
	if len(s) > most {
		return strings.TrimSuffix(strconv.Quote(s[:most]), `"`) + `..."`
	}

	return strconv.Quote(s)
}

// suiteCases reads the vendored JSONTestSuite, in name order.
func suiteCases(t *testing.T) []jsontestsuite.Case {
	t.Helper()

	cases, err := jsontestsuite.Cases()
	if err != nil {
		t.Fatalf("cannot read the suite: %v", err)
	}

	return cases
}
