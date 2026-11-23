// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
)

// TestTheNumberFormsMeanUnder11WhatTheLibraryReads holds yamlgen's table of
// YAML 1.1 answers to the library's own 1.1 reader.
//
// The table in reading.go is written out rather than derived, so it can be
// wrong, and being wrong there would mean stating a meaning no implementation
// holds -- the one failure the whole readings mechanism exists to avoid. This
// asks the library the same question for every form the emitter can write.
//
// The library is not the specification, so a disagreement is a question rather
// than a verdict. It has been the right question twice: libfyaml overturned a
// triage done without it, and the reference parser settled the key departure.
func TestTheNumberFormsMeanUnder11WhatTheLibraryReads(t *testing.T) {
	values := []yamlgen.Value{
		yamlgen.Int{V: 0},
		yamlgen.Int{V: 1},
		yamlgen.Int{V: 31},
		yamlgen.Int{V: 511},
		yamlgen.Int{V: 1000},
		yamlgen.Int{V: -31},
		// The leading zero's own corners: 9 writes "09", which 1.1 reads as a
		// string, and 7 writes "07", which both read as 7.
		yamlgen.Int{V: 9},
		yamlgen.Int{V: 7},
		yamlgen.Int{V: 777},
		yamlgen.Float{V: 1.5},
		yamlgen.Float{V: -1.5},
		yamlgen.Float{V: 0.5},
		yamlgen.Float{V: 1000},
		yamlgen.Float{V: 1e-320},
		yamlgen.Float{V: 0},
	}

	forms := []yamlgen.NumberForm{
		yamlgen.NumberPlain, yamlgen.NumberSigned,
		yamlgen.NumberHex, yamlgen.NumberOctal, yamlgen.NumberExponent,
		yamlgen.NumberLeadingZero,
	}

	for _, form := range forms {
		st := yamlgen.Style{NullSpelling: "null", NumberForm: form}

		for _, v := range values {
			w := yamlgen.Write(yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: v}}}, st)

			// The core answer first, so a form that changes the value at all
			// fails here rather than quietly in the 1.1 column.
			var core any
			require.NoError(t, yaml.Unmarshal([]byte(w.Text), &core), "%q", w.Text)
			assert.Equal(t, map[string]any{"k": v.Decoded()}, core,
				"%q does not read back as the value it was written from", w.Text)

			var legacy any
			require.NoError(t, yaml.Unmarshal([]byte("%YAML 1.1\n---\n"+w.Text), &legacy), "%q", w.Text)

			want := map[string]any{"k": v.Decoded()}
			if alt, differs := w.Readings[yamlgen.Reading11]; differs {
				want = alt.(map[string]any)
			}

			assert.Equal(t, want, legacy,
				"%q: yamlgen says %v under 1.1 and the library reads %v", w.Text, want, legacy)
		}
	}
}

// TestTheLegacyNumbersMeanUnderElevenWhatTheLibraryReads holds the other half
// of the table, the spellings YAML 1.1 reads as numbers and 1.2 reads as the
// text, to the library's own 1.1 reader.
//
// The same argument as the test above and the same risk: reading.go's
// legacyNumbers is written out from §10.3 and §10.4 of the 1.1 specification --
// the "_" separator, the "0b" prefix, base 60 -- and being wrong there would
// put a meaning in the corpus that no implementation holds.
//
// Each text is asked as a value and as a key, in block and in flow. A key is
// the harder half: "1:30" carries a ':' that is not the entry's, and the key
// moves with the reading, so "1:30: v" is keyed by the string "1:30" under core
// and by the integer 90 under 1.1.
//
// A text added to legacyNumbers without a row here is caught by
// TestASecondReadingOnlyArrivesWithALegacySpelling, which restates the whole
// table and fails on a spelling it does not know.
func TestTheLegacyNumbersMeanUnderElevenWhatTheLibraryReads(t *testing.T) {
	for _, tc := range []struct {
		text  string
		under any // what 1.1 reads the text as, and so the key it is
	}{
		{"1_000", uint64(1000)},
		{"-1_0", int64(-10)},
		{"0b1010", uint64(10)},
		{"+0b11", uint64(3)},
		{"0x_1F", uint64(31)},
		{"1:30", uint64(90)},
		{"190:20:30", uint64(685230)},
		{"685_230.15", 685230.15},
		{"12:00.5", 720.5},
	} {
		for _, flow := range []bool{false, true} {
			st := yamlgen.Style{NullSpelling: "null", Quoting: yamlgen.QuotePlain, Flow: flow}

			asValue := yamlgen.Map{Pairs: []yamlgen.Pair{
				{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Str{V: tc.text}},
			}}
			readsAs(t, yamlgen.Write(asValue, st),
				map[string]any{"k": tc.text}, map[string]any{"k": tc.under})

			asKey := yamlgen.Map{Pairs: []yamlgen.Pair{
				{Key: yamlgen.Str{V: tc.text}, Val: yamlgen.Str{V: "v"}},
			}}
			readsAs(t, yamlgen.Write(asKey, st),
				map[string]any{tc.text: "v"}, map[any]any{tc.under: "v"})
		}
	}
}

// readsAs checks a written document against both readings: the library reads
// core on its own and 1.1 under a directive, and yamlgen states the second one
// only where it differs.
func readsAs(t *testing.T, w yamlgen.Written, core, under11 any) {
	t.Helper()

	var got any
	require.NoError(t, yaml.Unmarshal([]byte(w.Text), &got), "%q", w.Text)
	assert.Equal(t, core, got, "%q does not read back as the value it was written from", w.Text)

	assert.Equal(t, under11, toCodec(w.Readings[yamlgen.Reading11]),
		"%q: yamlgen states %v under 1.1", w.Text, w.Readings[yamlgen.Reading11])

	var legacy any
	require.NoError(t, yaml.Unmarshal([]byte("%YAML 1.1\n---\n"+w.Text), &legacy), "%q", w.Text)
	assert.Equal(t, under11, legacy,
		"%q: yamlgen says %v under 1.1 and the library reads %v", w.Text, under11, legacy)
}

// TestAnOrderedMapKeyIsWhatItResolvesTo holds the "!!omap" model to the library
// under both readings: a codec.MapItem holds the value its key resolves to, so a
// plain null key is nil, and a plain "yes" key is the string under core and the
// boolean true under 1.1.
//
// TestAStreamReadsBackAsItsDocuments drew a null key inside an "!!omap" on
// 2026-09-11 and read nil where yamlgen said "null": orderedMapDecoded named
// the key with KeyText. A draw reaches it only when an "!!omap" entry gets a
// key that is not a string, so this holds it where the property rarely goes.
func TestAnOrderedMapKeyIsWhatItResolvesTo(t *testing.T) {
	omap := yamlgen.Tagged{Tag: yamlgen.TagOMap, V: yamlgen.Seq{Items: []yamlgen.Value{
		yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Null{}, Val: yamlgen.Bool{V: false}}}},
		yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "yes"}, Val: yamlgen.Str{V: "v"}}}},
	}}}
	doc := yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: omap}}}

	ordered := func(items ...codec.MapItem) codec.MapSliceSeq {
		seq, err := codec.NewMapSliceSeq(items...)
		require.NoError(t, err)

		return seq
	}

	// Indent 2: a zero Style writes the entries' mappings flush under "-", which
	// no reader takes for the entries' content.
	w := yamlgen.Write(doc, yamlgen.Style{NullSpelling: "null", Quoting: yamlgen.QuotePlain, Indent: 2})
	core := map[string]any{"k": ordered(codec.MapItem{Key: nil, Value: false}, codec.MapItem{Key: "yes", Value: "v"})}

	// readsAs holds the library to core and the 1.1 reading to the library, and
	// never the core meaning yamlgen states, which is the half the draw caught.
	assert.Equal(t, core, toCodec(w.Means), "%q: yamlgen states %v under core", w.Text, w.Means)
	readsAs(t, w, core,
		map[string]any{"k": ordered(codec.MapItem{Key: nil, Value: false}, codec.MapItem{Key: true, Value: "v"})})
}

// TestAMergedKeyIsBeatenByTheOwnKeyThatResolvesToIt guards the accepting side
// of the unique-key work against the merge rule.
//
// A mapping's own keys beat the ones a "<<" brings in, and naming a key by
// [ast.KeyIdentity] rather than by its spelling lets `? [ 1 ]` beat a merged
// `? [1]`: the two resolve to one node, so one overrides the other instead of
// standing beside it.
//
// It is the direction that fails loudly. Had the parser recorded a merged
// mapping's keys in the scope of the mapping holding the "<<", every override
// would be refused as a repeat -- a valid document turned into an error. The
// merged mapping is a node of its own with its own scope, so it is not, and
// this says so rather than leaving it to be inferred.
//
// The corpus draws merges that share a key with what they merge, 34 of 51 as of
// yamlgen/44, but it names those keys with scalars. A collection key and an
// alias key are the shapes this work introduced identity comparison for, and
// neither is drawn there.
//
// "%YAML 1.1" because a bare "<<" is an ordinary key under 1.2, where the merge
// type is not in the schema.
func TestAMergedKeyIsBeatenByTheOwnKeyThatResolvesToIt(t *testing.T) {
	// ⚠️ The three cases here keyed on a sequence, which reached the decoder
	// only while a collection key was named with Go's printing. A collection
	// cannot be a key in a Go map, so the vehicle is a scalar written two ways:
	// the question is whether an own key beats a merged one that *resolves* to
	// it, not whether the two are spelled alike.
	for _, tc := range []struct{ name, src string }{
		{"the same key respelled", "%YAML 1.1\n---\na: &m\n  ? 1\n  : from_merge\n  extra: kept\nb:\n  <<: *m\n  ? !!int 1\n  : own\n"},
		{"quoted against plain", "%YAML 1.1\n---\na: &m\n  ? k\n  : from_merge\n  extra: kept\nb:\n  <<: *m\n  ? \"k\"\n  : own\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(tc.src), &got), "%q", tc.src)

			// A map[any]any where a key is not a string, as "? 1" is not.
			var b map[any]any
			switch m := got["b"].(type) {
			case map[any]any:
				b = m
			case map[string]any:
				b = make(map[any]any, len(m))
				for key, v := range m {
					b[key] = v
				}
			default:
				require.Failf(t, "b is not a mapping", "%q read %#v", tc.src, got["b"])
			}

			assert.Lenf(t, b, 2, "%q read %#v", tc.src, b)
			assert.Equalf(t, "kept", b["extra"], "the merge-only key is kept: %q", tc.src)
			for key, v := range b {
				if key != "extra" {
					assert.Equalf(t, "own", v, "the own entry wins: %q", tc.src)
				}
			}
		})
	}

	t.Run("and a collection key does not reach the question", func(t *testing.T) {
		const src = "%YAML 1.1\n---\na: &m\n  ? [1]\n  : from_merge\nb:\n  <<: *m\n  ? [1]\n  : own\n"

		_, err := parser.ParseBytes([]byte(src))
		require.NoError(t, err)

		var got any
		assert.Error(t, codec.Unmarshal([]byte(src), &got), "read %v", got)
	})
}
