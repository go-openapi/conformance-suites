// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"pgregory.net/rapid"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser"
)

// TestEmitParses is the weaker half of the same idea, kept separate because it
// fails for a different reason: whatever the value, every style has to produce
// something the parser will read.
//
// With the grammar to hand, a failure here says which side is wrong rather than
// only that the two disagree. The emitter is gated above, so a document that
// reaches this point is valid YAML 1.2 and the refusal to read it is the
// library's -- which is what makes an entry in the ledger a parser defect
// rather than a note that something, somewhere, does not line up.
func TestEmitParses(t *testing.T) {
	tally := newTally()
	oracle := yamloracle.NewRecognizer(1024)

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")

		src := yamlgen.Emit(value, style)

		// The parse and not a decode: a document may parse and still hold no Go
		// value, which a collection standing as a mapping key does. Reading
		// through Unmarshal reported that refusal here as a parser defect.
		_, err := parser.ParseBytes([]byte(src))

		if known := yamlgen.Known(ledger, yamlgen.Parses, value, style); known != nil {
			tally.record(known.Name, err != nil)

			return
		}

		if err != nil {
			rt.Fatalf("style %s produced a document that does not parse, and %s:\n%s\n---\nerror: %v",
				style, verdict(oracle, src), indent(src), err)
		}
	})

	tally.report(t, yamlgen.Parses)
}

// verdict names which side of a disagreement to look at first.
func verdict(oracle *yamloracle.Recognizer, src string) string {
	if oracle.Stream([]byte(src)).OK {
		return "the YAML 1.2 grammar accepts it, so the fault is the parser's"
	}

	return "the YAML 1.2 grammar rejects it too, so the fault is the emitter's"
}

// TestEmitterAgreesOnKnownDocuments guards the emitter itself.
//
// The property tests above are only worth what the emitter is worth: if it
// wrote a value down wrongly, the failure would be blamed on the library. These
// are the cases where the expected text is obvious enough to write out by hand.
func TestEmitterAgreesOnKnownDocuments(t *testing.T) {
	block := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	flow := yamlgen.Style{Flow: true, Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	literal := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, NullSpelling: "null"}

	tests := []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "block mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Int{V: 1}}}},
			style: block,
			want:  "a: 1\n",
		},
		{
			name:  "nested block mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "b"}, Val: yamlgen.Int{V: 2}}}}}}},
			style: block,
			want:  "a:\n  b: 2\n",
		},
		{
			name:  "block sequence",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}, yamlgen.Int{V: 2}}},
			style: block,
			want:  "- 1\n- 2\n",
		},
		{
			name:  "empty collections have no block spelling",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Seq{}}}},
			style: block,
			want:  "a: []\n",
		},
		{
			name:  "flow mapping",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}}}},
			style: flow,
			want:  "{a: [1]}\n",
		},
		{
			name:  "literal block scalar clips one trailing newline",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Str{V: "one\ntwo\n"}}}},
			style: literal,
			want:  "a: |\n  one\n  two\n",
		},
		{
			name:  "literal block scalar strips when there is none",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Str{V: "one\ntwo"}}}},
			style: literal,
			want:  "a: |-\n  one\n  two\n",
		},
		{
			name:  "literal block scalar keeps the extra ones",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Str{V: "one\n\n"}}}},
			style: literal,
			want:  "a: |+\n  one\n\n",
		},
	}

	tests = append(tests, commentCases()...)
	tests = append(tests, anchorCases()...)
	tests = append(tests, blockScalarCases()...)
	tests = append(tests, foldedCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, yamlgen.Emit(tc.value, tc.style))

			// A hand-written expectation is as capable of not being YAML as a
			// generated one, and rather more likely to be believed.
			assert.Truef(t, yamloracle.Stream([]byte(tc.want)).OK,
				"the expected document is not valid YAML 1.2:\n%s", indent(tc.want))

			var got any
			require.NoError(t, yaml.Unmarshal([]byte(tc.want), &got))
			assert.Equal(t, tc.value.Decoded(), got)
		})
	}
}

// commentCases pins where comments are written. Comments carry no meaning, so
// nothing else in the suite would notice if they landed somewhere legal but
// unintended -- or somewhere illegal, which would then be blamed on the parser.
func commentCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	base := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	head := base
	head.Comments = yamlgen.HeadComments
	line := base
	line.Comments = yamlgen.LineComments
	both := base
	both.Comments = yamlgen.AllComments

	pair := func(k string, v yamlgen.Value) yamlgen.Value {
		return yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: k}, Val: v}}}
	}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "a head comment sits above its entry",
			value: pair("a", yamlgen.Int{V: 1}),
			style: head,
			want:  "# c1\na: 1\n",
		},
		{
			name:  "a line comment sits after the value",
			value: pair("a", yamlgen.Int{V: 1}),
			style: line,
			want:  "a: 1 # c1\n",
		},
		{
			name:  "both, numbered in the order they are written",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "a"}, Val: yamlgen.Int{V: 1}}, {Key: yamlgen.Str{V: "b"}, Val: yamlgen.Int{V: 2}}}},
			style: both,
			want:  "# c1\na: 1 # c2\n# c3\nb: 2 # c4\n",
		},
		{
			name:  "a comment introduces a nested block",
			value: pair("a", pair("b", yamlgen.Int{V: 2})),
			style: line,
			want:  "a: # c1\n  b: 2 # c2\n",
		},
		{
			name:  "head comments indent with their entry",
			value: pair("a", pair("b", yamlgen.Int{V: 2})),
			style: head,
			want:  "# c1\na:\n  # c2\n  b: 2\n",
		},
		{
			name:  "sequence entries take comments too",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}},
			style: both,
			want:  "# c1\n- 1 # c2\n",
		},
	}
}

// anchorCases pins where an anchor is written and what an alias looks like.
//
// This is the first axis that changes the value rather than its presentation,
// so a mistake here would not merely write a document oddly -- it would write a
// different document and blame the library for reading it as one. Each case
// also asserts that the text decodes to the value, which is what catches an
// anchor written somewhere the parser attaches to the wrong node.
func anchorCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	block := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	flow := yamlgen.Style{Flow: true, Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: "null"}
	literal := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, NullSpelling: "null"}

	one := yamlgen.Anchored{Name: "a1", V: yamlgen.Int{V: 1}}
	seq := yamlgen.Anchored{Name: "a1", V: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "an anchored scalar keeps the anchor on its line",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: one}}},
			style: block,
			want:  "k: &a1 1\n",
		},
		{
			name: "an alias refers back to it",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{
				{Key: yamlgen.Str{V: "a"}, Val: one},
				{Key: yamlgen.Str{V: "b"}, Val: yamlgen.Alias{Name: "a1", V: yamlgen.Int{V: 1}}},
			}},
			style: block,
			want:  "a: &a1 1\nb: *a1\n",
		},
		{
			name:  "an anchored block collection takes the line above it",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: seq}}},
			style: block,
			want:  "k: &a1\n  - 1\n",
		},
		{
			name:  "an anchor in flow style sits inside the brackets",
			value: yamlgen.Seq{Items: []yamlgen.Value{one}},
			style: flow,
			want:  "[&a1 1]\n",
		},
		{
			name:  "an anchored block scalar keeps its header on the line",
			value: yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: yamlgen.Anchored{Name: "a1", V: yamlgen.Str{V: "x\n"}}}}},
			style: literal,
			want:  "k: &a1 |\n  x\n",
		},
		{
			name:  "an anchored empty node is the anchor alone",
			value: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Anchored{Name: "a1", V: yamlgen.Null{}}}},
			style: yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, NullSpelling: ""},
			want:  "- &a1\n",
		},
		{
			name:  "an anchor at the root of a block collection",
			value: seq,
			style: block,
			want:  "&a1\n- 1\n",
		},
		{
			name: "a sequence entry can be an alias",
			value: yamlgen.Seq{Items: []yamlgen.Value{
				seq,
				yamlgen.Alias{Name: "a1", V: yamlgen.Seq{Items: []yamlgen.Value{yamlgen.Int{V: 1}}}},
			}},
			style: block,
			want:  "- &a1\n  - 1\n- *a1\n",
		},
	}
}

// blockScalarCases pins the block scalar header, which is where the emitter
// makes the most decisions per character written.
//
// The indicator cases matter most: they are the only way the generator can
// write content whose first line is empty or whose lines begin with a space,
// so if they are wrong those strings are silently mis-tested rather than
// untested.
func blockScalarCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	plain := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Literal: true, NullSpelling: "null"}
	stated := plain
	stated.BlockIndicator = true
	deep := stated
	deep.Indent = 3

	pair := func(v yamlgen.Value) yamlgen.Value {
		return yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: v}}}
	}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "the indicator goes before the chomping indicator",
			value: pair(yamlgen.Str{V: "one\ntwo"}),
			style: stated,
			want:  "k: |2-\n  one\n  two\n",
		},
		{
			name:  "and states the indent actually used",
			value: pair(yamlgen.Str{V: "one\n"}),
			style: deep,
			want:  "k: |3\n   one\n",
		},
		{
			name:  "content whose first line is empty needs it",
			value: pair(yamlgen.Str{V: "\ntwo\n"}),
			style: stated,
			want:  "k: |2\n\n  two\n",
		},
		{
			name:  "so does content whose lines begin with a space",
			value: pair(yamlgen.Str{V: "  indented\n"}),
			style: stated,
			want:  "k: |2\n    indented\n",
		},
		{
			name:  "a line of nothing but spaces is content too",
			value: pair(yamlgen.Str{V: "a\n \nb\n"}),
			style: stated,
			want:  "k: |2\n  a\n   \n  b\n",
		},
		{
			name:  "without it those strings are not written as block scalars",
			value: pair(yamlgen.Str{V: "  indented\n"}),
			style: plain,
			want:  "k: \"  indented\\n\"\n",
		},
	}
}

// foldedCases pin the folded block scalar, which is the one presentation that
// rewrites the text it is given.
//
// Folding joins two lines with a space and turns n+1 breaks into n, so the
// emitter is solving the inverse of what the parser does. Get that inverse
// wrong and the document still looks perfectly reasonable while meaning
// something else -- so these are written out by hand, and every one of them
// asserts the text reads back as the value as well as being the text expected.
func foldedCases() []struct {
	name  string
	value yamlgen.Value
	style yamlgen.Style
	want  string
} {
	folded := yamlgen.Style{Indent: 2, Quoting: yamlgen.QuotePlain, Folded: true, NullSpelling: "null"}
	stated := folded
	stated.BlockIndicator = true

	pair := func(v yamlgen.Value) yamlgen.Value {
		return yamlgen.Map{Pairs: []yamlgen.Pair{{Key: yamlgen.Str{V: "k"}, Val: v}}}
	}

	return []struct {
		name  string
		value yamlgen.Value
		style yamlgen.Style
		want  string
	}{
		{
			name:  "one line needs no fold at all",
			value: pair(yamlgen.Str{V: "one\n"}),
			style: folded,
			want:  "k: >\n  one\n",
		},
		{
			name:  "a break in the value is written as a blank line",
			value: pair(yamlgen.Str{V: "one\ntwo\n"}),
			style: folded,
			want:  "k: >\n  one\n\n  two\n",
		},
		{
			name:  "three lines, two blank lines",
			value: pair(yamlgen.Str{V: "one\ntwo\nthree\n"}),
			style: folded,
			want:  "k: >\n  one\n\n  two\n\n  three\n",
		},
		{
			name:  "no trailing break strips",
			value: pair(yamlgen.Str{V: "one\ntwo"}),
			style: folded,
			want:  "k: >-\n  one\n\n  two\n",
		},
		{
			name:  "more than one trailing break keeps",
			value: pair(yamlgen.Str{V: "one\n\n"}),
			style: folded,
			want:  "k: >+\n  one\n\n",
		},
		{
			name:  "the indicator comes before the chomping",
			value: pair(yamlgen.Str{V: "one\ntwo"}),
			style: stated,
			want:  "k: >2-\n  one\n\n  two\n",
		},
		{
			name:  "at the root it states one more than its column",
			value: yamlgen.Str{V: "one\ntwo\n"},
			style: stated,
			want:  ">3\n  one\n\n  two\n",
		},
	}
}
