// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"pgregory.net/rapid"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
)

// TestEveryMutationBreaksSomething guards the mutator the way
// TestEveryBlockScalarStyleIsActuallyReached guards the style axes.
//
// A mutation that never produces an invalid document makes the hunt quieter
// and no better, and nothing else would notice: the hunt passes, the ledger
// holds, and the axis reads as covered. One of them was exactly that -- it left
// an alias pointing at no anchor, which the grammar has no opinion about -- and
// it was found by counting rather than by reasoning.
func TestEveryMutationBreaksSomething(t *testing.T) {
	oracle := yamloracle.NewRecognizer(1024)
	broke := map[string]int{}

	// enough is how many refusals a mutation has to earn, and draws how many
	// documents each check offers it.
	//
	// a-document-marker-inside sets the budget on its own. Inserting `---` or
	// `...` into a document usually leaves another good one -- the marker opens
	// a document rather than breaking one -- so it earns a refusal perhaps once
	// in forty draws, where a-stray-indicator earns one in three. At forty
	// draws the test failed about one run in twelve before the Break and flow
	// axes went in and one in four after, because those axes spend the same
	// budget on more shapes. Raising the draws costs nothing once every
	// mutation is covered, since covered() returns before any of them are made.
	const (
		enough = 5
		draws  = 200
	)

	covered := func() bool {
		for _, name := range yamlgen.MutationNames() {
			if broke[name] < enough {
				return false
			}
		}

		return true
	}

	rapid.Check(t, func(rt *rapid.T) {
		if covered() {
			return
		}

		for range draws {
			src := yamlgen.Emit(
				yamlgen.Values().Draw(rt, "value"),
				yamlgen.Styles().Draw(rt, "style"),
			)

			mutant, how := yamlgen.Mutate(rt, src)
			if !oracle.Stream([]byte(mutant)).OK {
				broke[how]++
			}
		}
	})

	for _, name := range yamlgen.MutationNames() {
		assert.Positivef(t, broke[name], "%s never made a document the grammar refuses", name)
		t.Logf("%-32s broke %d", name, broke[name])
	}
}
