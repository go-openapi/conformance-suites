// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"regexp"
	"slices"
	"strings"
)

// Divergence is a shape of document where this library disagrees with YAML 1.2.
//
// The generator keeps producing these rather than steering around them. Steering
// around a defect makes the harness quieter and blinder: the shape stops being
// generated, and nobody notices when it is fixed or when it spreads.
//
// Match describes the shape rather than naming a document, because there is no
// document to name -- every run draws different ones.
type Divergence struct {
	// Name is short and stable, so a count can be reported against it.
	Name string
	// Reason says what the library does and what YAML 1.2 says instead.
	Reason string
	// Property is which question this shape fails to answer. A shape that
	// reads back correctly but renders wrongly excuses one property and not
	// the other, and conflating them would let a decode defect hide behind a
	// render defect.
	Property Property
	// Pin names the library's test that reproduces this entry with one
	// document, deterministically.
	//
	// Required: the library's tests hold it to a test that exists. It is what the tally's staleness check sends the reader to: an
	// entry drawn past suspectAfter times with no divergence is either fixed or
	// matching a family it is not in, and the count alone cannot say which --
	// the pin can, because it runs the one document the entry was written for.
	Pin string
	// Match reports whether this pairing has the shape.
	Match func(Value, Style) bool
}

// Property names the questions the generated documents are put to. It is a set,
// because one root cause can fail more than one: a value that is already wrong
// when read is still wrong after being written out again.
type Property int

const (
	// Decode: reading the emitted document gives back the value.
	Decode Property = 1 << iota
	// Render: parsing the emitted document and writing it out again gives a
	// document that still means the same thing.
	Render
	// Settle: rendering reaches a fixed point after one cycle.
	//
	// Separate from Render because the two fail independently. A comment can
	// move about without any value changing, and an entry that excused both
	// would be tolerating documents that are fine on one of them.
	Settle
	// CommentsKept: rendering keeps every comment the document had.
	//
	// Nothing else notices a lost comment. Comments carry no meaning, so the
	// value is unaffected and rendering still settles -- the document is simply
	// poorer than the one that went in, which for a library that offers to
	// preserve them is the whole failure.
	CommentsKept
	// Parses: the emitted document is one the library will read at all.
	//
	// Weaker than Decode and worth separating: a document that is rejected
	// outright is a different failure from one that is read as the wrong
	// value, and it is the only one where the library and the grammar can be
	// asked the same question.
	Parses
	// RenderValid: the text the renderer wrote is a YAML 1.2 document.
	//
	// Only the grammar can answer this. Re-reading the rendering, which is what
	// Render and Settle do, cannot: a renderer and a parser that make the same
	// mistake agree with each other while the file on disk is one no other tool
	// will read.
	RenderValid
	// DecodeTyped: reading the emitted document into a Go type built from the
	// value gives what reading it into an `any` gives.
	//
	// A separate property because it is a separate path through codec. Reading
	// into an `any` builds the value with one builder; reading into a Go type
	// fills fields by reflection with another, or through a tree where the walk
	// cannot serve, and the two can disagree silently. The comparison is
	// against the `any` answer rather than against Value.Decoded, so a defect on
	// both paths cancels out and only the destination is left as the variable.
	// See [TargetFor].
	DecodeTyped
	// StreamDecode: the documents of a stream read back as the values they
	// were written from.
	//
	// A property of its own because a stream is not a document: the separator,
	// the scope of what a document declares, and the count are questions a
	// single document cannot ask. An entry claiming it is consulted by
	// TestAStreamReadsBackAsItsDocuments and by nothing else, so a shape that
	// only goes wrong in a stream excuses nothing anywhere else.
	StreamDecode
	// StreamDecodeTyped: a Decoder reading a stream document by document into
	// Go types, a different type for each, gives what a Decoder reading it
	// through a tree gives.
	//
	// A property of its own for the reason StreamDecode is: the pause between
	// documents, a destination that changes type, and a switch to the tree in
	// the middle of a stream are questions neither a single typed document nor
	// a stream into `any` asks. Consulted by
	// TestAStreamDecodesIntoAGoTypePerDocument and by nothing else.
	StreamDecodeTyped
)

func (p Property) String() string {
	names := []string{}
	if p&Decode != 0 {
		names = append(names, "decode")
	}
	if p&Render != 0 {
		names = append(names, "render")
	}
	if p&Settle != 0 {
		names = append(names, "settle")
	}
	if p&CommentsKept != 0 {
		names = append(names, "comments")
	}
	if p&Parses != 0 {
		names = append(names, "parses")
	}
	if p&RenderValid != 0 {
		names = append(names, "render-valid")
	}
	if p&DecodeTyped != 0 {
		names = append(names, "decode-typed")
	}
	if p&StreamDecode != 0 {
		names = append(names, "stream-decode")
	}
	if p&StreamDecodeTyped != 0 {
		names = append(names, "stream-decode-typed")
	}

	return strings.Join(names, "|")
}

// KeyCarriesAProperty reports whether a key is written with an anchor or a tag
// in front of it.
func KeyCarriesAProperty(k Value) bool {
	switch k.(type) {
	case Anchored, Tagged:
		return true
	}

	return false
}

// WritesAsABlockScalar reports whether a value reaches the document as a block
// scalar, looking through the properties that may stand in front of it.
func WritesAsABlockScalar(v Value, st Style) bool {
	switch n := v.(type) {
	case Str:
		return blockScalarIn(n.V, st)
	case Anchored:
		return WritesAsABlockScalar(n.V, st)
	case Tagged:
		return WritesAsABlockScalar(n.V, st)
	}

	return false
}

// Known returns the ledger entry describing this pairing for the given
// property, or nil.
func Known(ledger []Divergence, p Property, v Value, st Style) *Divergence {
	for i := range ledger {
		if ledger[i].Property&p != 0 && ledger[i].Match(v, st) {
			return &ledger[i]
		}
	}

	return nil
}

// Entries returns the ledger entries for one property.
func Entries(ledger []Divergence, p Property) []Divergence {
	var out []Divergence
	for _, d := range ledger {
		if d.Property&p != 0 {
			out = append(out, d)
		}
	}

	return out
}

// CommentsIn returns the comment markers a generated document carries, in the
// order they appear.
//
// Emit numbers its comments, so a lost one is identifiable rather than merely
// countable, and a moved one can be told from a dropped one.
func CommentsIn(src string) []string {
	return commentMarker.FindAllString(src, -1)
}

var commentMarker = regexp.MustCompile(`#\s*c\d+`)

// WritesACollectionKey reports whether v holds, anywhere, a mapping key that
// [Emit] writes as a block collection.
//
// Such a key takes the "?" form whatever the style asks. An alias to a
// collection and an empty collection write on one line, so they do not count:
// use [HoldsACollectionKey] to ask what the key means.
func WritesACollectionKey(v Value) bool {
	return anyKey(v, isCollection)
}

// HoldsACollectionKey reports whether v holds, anywhere, a mapping key that
// reads as a collection: a sequence or a mapping, empty or not, behind any
// anchor, tag or alias.
//
// A parser that refuses collection keys refuses every document of v.
func HoldsACollectionKey(v Value) bool {
	return anyKey(v, readsAsCollection)
}

// anyKey reports whether is holds for a mapping key anywhere in v, aliased
// nodes included.
func anyKey(v Value, is func(Value) bool) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if is(p.Key) || anyKey(p.Key, is) || anyKey(p.Val, is) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, func(item Value) bool { return anyKey(item, is) })
	case Anchored:
		return anyKey(n.V, is)
	case Alias:
		return anyKey(n.V, is)
	case Tagged:
		return anyKey(n.V, is)
	}

	return false
}

// readsAsCollection reports whether v reads as a sequence or a mapping, looking
// through anchors, tags and aliases.
func readsAsCollection(v Value) bool {
	switch n := v.(type) {
	case Seq, Map:
		return true
	case Anchored:
		return readsAsCollection(n.V)
	case Tagged:
		return readsAsCollection(n.V)
	case Alias:
		return readsAsCollection(n.V)
	}

	return false
}

// WritesTwoBareColonLinesInARow reports whether the document can put one bare
// ":" line under another.
//
// Two things make a bare ":" line: the empty null spelling, which writes nothing
// for a Null, and an explicit key, which puts the ":" on a line of its own. The
// third is a Null standing as a key or a value for the spelling to swallow.
//
// ⚠️ Wider than the defect, and deliberately so after three tries at narrowing
// it. The two ":" lines reach each other in more arrangements than a predicate
// over the value can enumerate: as sibling entries, as an entry's value nested
// under it, and inside a collection key. Each narrowing matched the arrangement
// it was written for and missed the next, so this asks for the ingredients
// rather than the recipe. It reports more divergences than draws; the pin
// carries the precision.
func WritesTwoBareColonLinesInARow(v Value, st Style) bool {
	if st.NullSpelling != "" || !WritesAnExplicitKey(v, st) {
		return false
	}

	return HoldsANullKeyOrValue(v)
}

// HoldsANullKeyOrValue reports whether a Null stands as a mapping's key or
// value anywhere in v.
func HoldsANullKeyOrValue(v Value) bool {
	switch n := v.(type) {
	case Map:
		for _, p := range n.Pairs {
			if isNull(p.Key) || isNull(p.Val) {
				return true
			}

			if HoldsANullKeyOrValue(p.Key) || HoldsANullKeyOrValue(p.Val) {
				return true
			}
		}
	case Seq:
		return slices.ContainsFunc(n.Items, HoldsANullKeyOrValue)
	case Anchored:
		return HoldsANullKeyOrValue(n.V)
	case Alias:
		return HoldsANullKeyOrValue(n.V)
	case Tagged:
		return HoldsANullKeyOrValue(n.V)
	}

	return false
}

// isNull reports whether a value writes as the empty node, looking through the
// properties that may stand in front of one.
func isNull(v Value) bool {
	switch n := v.(type) {
	case Null:
		return true
	case Anchored:
		return isNull(n.V)
	case Alias:
		return isNull(n.V)
	case Tagged:
		return isNull(n.V)
	}

	return false
}

// WritesACommentAboveABlankLine reports whether the style writes both a comment
// on an entry's line and the blank lines that padding leaves.
func WritesACommentAboveABlankLine(_ Value, st Style) bool {
	return st.Comments.line() && st.Chomping == ChompPadded
}

// WritesAnExplicitKey reports whether the document writes a "?" at all.
//
// Two sources, and the second was added on 2026-09-08 when Keys began drawing a
// collection: Style.ExplicitKeys asks for every entry the long way, and a
// collection key takes the long form whatever the style says, because 8.2.2
// makes an implicit key one line and a block collection is not one line.
//
// Three predicates here asked only the style and missed every "?" the value
// forced. Each was found the same way -- a property failing on a document whose
// style label carried no "?key" at all.
func WritesAnExplicitKey(v Value, st Style) bool {
	return st.ExplicitKeys || WritesACollectionKey(v)
}
