// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"sort"
	"testing"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"

	"github.com/go-openapi/conformance-suites/yaml/consensus/goyaml"
	"github.com/go-openapi/conformance-suites/yaml/consensus/libfyaml"
)

// uncorroborated names the shapes whose stated meaning no outside
// implementation can confirm, with the production that settles it.
//
// One, and it is a question about what a key is. Adding another means claiming
// a meaning on a reading alone, and the test below makes that a deliberate act
// rather than a quiet one.
//
// "two collection keys in one mapping" left on 2026-09-10: it stated a meaning
// this library alone read, and only because a collection key was named with
// Go's printing of the value the decoder built. A mapping cannot be a key in a
// Go map, so the shape now states none, which is what libfyaml and yaml/v3
// already said.
var uncorroborated = map[string]string{
	"an explicit key whose own key is explicit": "8.2.2 puts an explicit entry's key at " +
		"s-l+block-indented(n, block-out), which is any block node -- a mapping written the long way " +
		"included, and a '?' of its own with it. The key it builds is a mapping, which libfyaml " +
		"cannot hash and yaml/v3 refuses, so neither can say what the document holds",
}

// TestEveryUncorroboratedMeaningIsNamed holds the corpus to naming the places
// where careful reading is the only instrument.
//
// The failure it exists to stop is quiet: a meaning added to a shape no
// implementation will read looks exactly like a meaning three of them agree on,
// and the corpus then asserts one person's reading of the specification with
// the authority of a measurement.
func TestEveryUncorroboratedMeaningIsNamed(t *testing.T) {
	if !libfyaml.Available() {
		t.Skipf("libfyaml is not installed at %s", libfyaml.Home())
	}

	rec := yamloracle.NewRecognizer(1024)

	var silent []string

	claimed := map[string]bool{}

	for family, shapes := range statedShapes() {
		for _, s := range shapes {
			if _, fromV3 := goyaml.Load(s.Src); fromV3 == nil {
				continue
			}

			if _, fromLibfy := libfyaml.Load(s.Src); fromLibfy == nil {
				continue
			}

			if !rec.Stream(s.Src).OK {
				// A document the grammar refuses is one every source answers,
				// by refusing it.
				continue
			}

			if s.Means == nil {
				silent = append(silent, family+": "+s.Name)

				continue
			}

			claimed[s.Name] = true

			if _, named := uncorroborated[s.Name]; !named {
				t.Errorf("%s states a meaning no implementation can confirm and is not named in "+
					"uncorroborated: %q\n  document: %q", s.Name, s.Name, string(s.Src))
			}
		}
	}

	for name := range uncorroborated {
		if !claimed[name] {
			t.Errorf("%q is named as uncorroborated and either states no meaning now or an "+
				"implementation reads it, so the entry can go", name)
		}
	}

	sort.Strings(silent)

	t.Logf("%d shapes the grammar accepts and no implementation reads; %d of them state a meaning",
		len(silent)+len(claimed), len(claimed))

	for _, name := range silent {
		t.Logf("  no meaning stated: %s", name)
	}
}
