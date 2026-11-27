// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
)

// TestACollectionKeyIsFoundBehindAnAliasOrEmpty separates the two questions about a collection key: does it read as
// a collection, and does Emit write it as a block collection. An alias to a mapping and an empty "{}" read as
// collections but write on one line.
func TestACollectionKeyIsFoundBehindAnAliasOrEmpty(t *testing.T) {
	inner := yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Str{V: "b"}}}}
	keyed := func(k yamlgen.Value) yamlgen.Value {
		return yamlgen.Map{Pairs: []yamlgen.Pair{{Key: k, Val: yamlgen.Str{V: "x"}}}}
	}

	for _, c := range []struct {
		name          string
		v             yamlgen.Value
		holds, writes bool
	}{
		{"a scalar key", keyed(yamlgen.Str{V: "k"}), false, false},
		{"a mapping key", keyed(inner), true, true},
		{"an anchored mapping key", keyed(yamlgen.Anchored{Name: "m", V: inner}), true, true},
		{"an alias to a mapping", keyed(yamlgen.Alias{Name: "m", V: inner}), true, false},
		{"an empty mapping key", keyed(yamlgen.Map{}), true, false},
		{"an empty sequence key", keyed(yamlgen.Seq{}), true, false},
		{"a mapping key inside a sequence", yamlgen.Seq{Items: []yamlgen.Value{keyed(inner)}}, true, true},
		{"a mapping as a value", yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: inner}}}, false, false},
	} {
		if got := yamlgen.HoldsACollectionKey(c.v); got != c.holds {
			t.Errorf("%s: HoldsACollectionKey = %v, want %v", c.name, got, c.holds)
		}
		if got := yamlgen.WritesACollectionKey(c.v); got != c.writes {
			t.Errorf("%s: WritesACollectionKey = %v, want %v", c.name, got, c.writes)
		}
	}
}
