// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/testify/v2/require"
)

// TestInvariantsAreStillOutstanding reports which invariants fail today,
// without failing itself.
//
// Each document listed is one the generator found and reduced. It names the
// invariant it breaks and says so on every run until somebody fixes it, and
// says the opposite the moment somebody does. Nothing else in the suite states
// an outstanding defect in a form a fixer can act on.
func TestInvariantsAreStillOutstanding(t *testing.T) {
	outstanding := []struct {
		invariant string
		src       string
		fails     func([]byte) bool
	}{
		// Empty. The last entry to leave was "k: >+\n  trail\n\n" against
		// "rendering preserves the value"; it is pinned the other way round now,
		// in TestFixedKeepChompingKeepsItsBlankLinesWhenFolded.
	}

	var open int
	for _, o := range outstanding {
		// Every document here is an accusation, so it had better be YAML. One
		// that is not would be this suite's own defect, filed under the
		// library's name and left there.
		require.Truef(t, yamloracle.Stream([]byte(o.src)).OK,
			"%q is not valid YAML 1.2, so it says nothing about the library", o.src)

		if o.fails([]byte(o.src)) {
			open++
			t.Logf("still failing -- %s: %q", o.invariant, o.src)

			continue
		}
		t.Logf("NOW HOLDS  -- %s: %q -- drop it from this list and from the ledger",
			o.invariant, o.src)
	}

	t.Logf("%d of %d known documents still violate an invariant", open, len(outstanding))
}
