// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
)

// placementFloor is how many drawn documents carrying a meaning each placement needs in the compact corpus.
//
// Fred, 2026-11-23. Fifty is enough that a placement is drawn against most of the styles and readings, so a defect
// on one decode path meets it more than once.
const placementFloor = 50

// TestEveryPlacementIsDrawnWithAMeaning holds the compact corpus to [yamlgen.Placements].
//
// A drawn document with a meaning is one a decoder is scored on: the corpus states what it denotes. The compact
// build keeps every such document, so counting the documents the build draws counts what the corpus ships.
func TestEveryPlacementIsDrawnWithAMeaning(t *testing.T) {
	b := yamlbuild.Compact()

	counts := map[string]int{}
	documents := 0

	for _, e := range yamlbuild.Generate(b.Seed, b.Documents, 0) {
		if e.Value == nil || e.Means == nil {
			continue
		}

		documents++

		for _, p := range yamlgen.Placements() {
			if p.Has(e.Value) {
				counts[p.Name]++
			}
		}
	}

	for _, p := range yamlgen.Placements() {
		t.Logf("%-50s %5d", p.Name, counts[p.Name])

		if counts[p.Name] < placementFloor {
			t.Errorf("%q: %d drawn documents with a meaning hold it, under the floor of %d",
				p.Name, counts[p.Name], placementFloor)
		}
	}

	t.Logf("%d placements over %d drawn documents with a meaning", len(yamlgen.Placements()), documents)
}
