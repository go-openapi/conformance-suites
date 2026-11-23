// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
	"github.com/go-openapi/go-yaml/codec"
)

// statedShapes are the enumerated families whose shapes may state a meaning.
func statedShapes() map[string][]stance.Shape {
	return map[string][]stance.Shape{
		"key":        yamlbuild.KeyShapes(),
		"merge":      yamlbuild.MergeShapes(),
		"tag":        yamlbuild.TagShapes(),
		"directive":  yamlbuild.DirectiveShapes(),
		"separation": yamlbuild.SeparationShapes(),
		"reach":      yamlbuild.ReachShapes(),
	}
}

// TestEveryStatedMeaningIsWhatTheLibraryReads holds the library to what the
// corpus says these documents denote.
//
// [stance.Shape.Means] is the specification's answer, so this is not a
// round-trip: it is the corpus telling the library what a document means and
// the library agreeing or not. A shape naming a [stance.Shape.Pin] is one we do
// not agree with yet, and the pin says what we do instead.
//
// # Which way each direction fails
//
// A shape with no pin must agree, and a disagreement fails: the corpus states
// what the specification settles, so the library is what moved.
//
// A shape with a pin must disagree, and agreement is *reported* rather than
// failed -- the same shape [yamlgen.Laxity] uses. A defect being fixed should not
// turn a tree red; it should say "drop the pin" and let somebody drop it.
func TestEveryStatedMeaningIsWhatTheLibraryReads(t *testing.T) {
	var stated, pinned int

	for family, shapes := range statedShapes() {
		for _, s := range shapes {
			if s.Means == nil {
				continue
			}

			t.Run(family+"/"+s.Name, func(t *testing.T) {
				want, err := json.Marshal(s.Means)
				require.NoError(t, err, "the shape's own Means does not marshal")

				var got any
				readErr := codec.Unmarshal(s.Src, &got)

				if s.Pin != "" {
					pinned++

					if readErr != nil {
						t.Logf("still refused -- %q -- %s", s.Src, firstLine(readErr.Error()))

						return
					}

					encoded, merr := yamlgen.MeaningJSON(got)
					if merr != nil || string(encoded) != string(want) {
						t.Logf("still differs -- %q reads %s where the corpus says %s",
							s.Src, encoded, want)

						return
					}

					t.Logf("NOW AGREES -- %q -- drop Pin %q from the shape", s.Src, s.Pin)

					return
				}

				stated++

				require.NoErrorf(t, readErr, "the corpus states a meaning for %q and the library refuses it", s.Src)

				encoded, merr := yamlgen.MeaningJSON(got)
				require.NoError(t, merr)
				assert.Equalf(t, string(want), string(encoded),
					"%q: the corpus states what the specification settles, so the library is what moved", s.Src)
			})
		}
	}

	t.Logf("%d shapes state a meaning the library produces, %d state one it does not yet", stated, pinned)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")

	return line
}
