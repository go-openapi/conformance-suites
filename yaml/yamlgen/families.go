// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"fmt"

	"pgregory.net/rapid"
)

// Families are documents built around the placements a drawn tree rarely reaches, about one document in seven.
//
// # Why build them
//
// A drawn tree holds a mapping in 438 documents of 3,000, and the merge, the "!!omap" and the typed scalars sit on
// mappings. Raising those odds everywhere would reshape the whole corpus to reach a few combinations. A family is a
// whole document made for them: a mapping anchored at the top and merged from below, an "!!omap" anchored and
// aliased, the 1.1 scalar types side by side. See [Placements] for what each is held to.
//
// # The anchors they write
//
// A family names its own anchors "m1", "m2" and so on, which the aliaser's "a1", "a2" never collide with, and
// writes each [Alias] holding the very Value its [Anchored] holds. The tagger runs over a family document and leaves
// an anchored or aliased node alone, so that invariant holds. The aliaser does not run over one: it replaces whole
// nodes with aliases, and replacing "item" took the anchor "m2" out from under "again: *m2" on the first build.
//
// # The shares, measured
//
// rapid draws an integer toward the small end: over 3,000 examples, IntRange(0, 99) gave a value under 8 in 1,163,
// not 240. The first cut, "n < 8" for the merge family, made half the corpus family documents. The upper end is
// close to flat, about 1.4% a value, so each family takes a band there: 87 and up is about 9% of documents, 79 to
// 86 about 4%, 74 to 78 about 2.5%.
const (
	familyMerge = 87
	familyOMap  = 79
	familyTyped = 74
)

// drawFamily returns a family document, or nil for an ordinary draw.
func drawFamily(t *rapid.T) Value {
	switch n := rapid.IntRange(0, 99).Draw(t, "family"); {
	case n >= familyMerge:
		return drawMergeFamily(t)
	case n >= familyOMap:
		return drawOMapFamily(t)
	case n >= familyTyped:
		return drawTypedFamily(t)
	default:
		return nil
	}
}

// drawMergeFamily writes an anchored mapping and a mapping that merges from it.
//
//	base: &m1 {b0: 1, b1: 2}
//	item:
//	  !!merge &k1 <<: [*m1, {b2: 3}]
//	  b1: 9
//
// The "<<" value is an alias, a sequence holding one, or a mapping written in place, tagged "!!map" or anchored. The
// key is bare, tagged "!!merge", anchored, or both. An own key shares its name with a base key half the time, so the
// precedence rule decides a value.
func drawMergeFamily(t *rapid.T) Value {
	base := smallMap(t, "b", 1, 3)
	baseAnchor := Anchored{Name: "m1", V: base}
	baseAlias := Alias(baseAnchor)

	var key Value = MergeKey{}

	if rapid.Bool().Draw(t, "familymergetag") {
		key = Tagged{Tag: TagMerge, V: key}
	}

	if rapid.IntRange(0, 2).Draw(t, "familymergekeyanchor") == 0 {
		key = Anchored{Name: "k1", V: key}
	}

	inline := smallMap(t, "c", 1, 2)

	var (
		val   Value
		later []Pair
	)

	switch rapid.IntRange(0, 4).Draw(t, "familymergevalue") {
	case 0:
		val = baseAlias
	case 1:
		val = Seq{Items: []Value{baseAlias, inline}}
	case 2:
		val = Tagged{Tag: TagSeq, V: Seq{Items: []Value{inline, baseAlias}}}
	case 3:
		val = Tagged{Tag: TagMap, V: inline}
	default:
		// Anchored where it stands, and aliased below: the mapping a merge
		// brought in is a node like any other.
		anchored := Anchored{Name: "m2", V: inline}
		val = anchored
		later = append(later, Pair{Key: Str{V: "again"}, Val: Alias(anchored)})
	}

	own := smallMap(t, "d", 0, 2)
	if rapid.Bool().Draw(t, "familymergeshares") {
		// An own key named as a base key: own keys win over merged ones.
		own.Pairs = append(own.Pairs, Pair{Key: base.Pairs[0].Key, Val: Int{V: 99}})
	}

	item := Map{Pairs: append([]Pair{{Key: key, Val: val}}, own.Pairs...)}

	return Map{Pairs: append([]Pair{
		{Key: Str{V: "base"}, Val: baseAnchor},
		{Key: Str{V: "item"}, Val: item},
	}, later...)}
}

// drawOMapFamily writes an "!!omap", with half the time a "!!pairs" and a "!!set" beside it.
//
//	base: &m4 {o9: 1}
//	o: &m3 !!omap
//	- o0: 1
//	- <<: *m4
//	- o1: *m4
//	again: *m3
//
// Each of the anchor on the "!!omap", an entry merging a mapping, and an alias as an entry's value is drawn half the
// time. The merged mapping holds one key, o9, which no entry holds, so the entry it fills is one entry of one key
// under YAML 1.1 as the "!!omap" type requires.
func drawOMapFamily(t *rapid.T) Value {
	entries := smallMap(t, "o", 1, 3).Pairs

	base := Anchored{Name: "m4", V: Map{Pairs: []Pair{{Key: Str{V: "o9"}, Val: Int{V: rapid.IntRange(-9, 9).Draw(t, "o9")}}}}}
	usesBase := false

	items := make([]Value, 0, len(entries)+2)
	for _, p := range entries {
		items = append(items, Map{Pairs: []Pair{p}})
	}

	if rapid.Bool().Draw(t, "omapmerge") {
		var from Value = Alias(base)
		if rapid.IntRange(0, 2).Draw(t, "omapmergeinline") == 0 {
			from = Map{Pairs: []Pair{{Key: Str{V: "o9"}, Val: Int{V: 7}}}}
		} else {
			usesBase = true
		}

		at := rapid.IntRange(0, len(items)).Draw(t, "omapmergeat")
		items = append(items[:at], append([]Value{Map{Pairs: []Pair{{Key: MergeKey{}, Val: from}}}}, items[at:]...)...)
	}

	if rapid.Bool().Draw(t, "omapalias") {
		usesBase = true
		items = append(items, Map{Pairs: []Pair{{Key: Str{V: "o8"}, Val: Alias(base)}}})
	}

	var omap Value = Tagged{Tag: TagOMap, V: Seq{Items: items}}

	var later []Pair

	if rapid.Bool().Draw(t, "omapanchor") {
		anchored := Anchored{Name: "m3", V: omap}
		omap = anchored
		later = append(later, Pair{Key: Str{V: "again"}, Val: Alias(anchored)})
	}

	pairs := []Pair{{Key: Str{V: "o"}, Val: omap}}
	if usesBase {
		pairs = append([]Pair{{Key: Str{V: "base"}, Val: base}}, pairs...)
	}

	pairs = append(pairs, later...)

	if rapid.Bool().Draw(t, "omappairs") {
		// "!!pairs" admits a key twice, which is what sets it apart from "!!omap".
		k := Str{V: "p0"}
		pairs = append(pairs, Pair{Key: Str{V: "p"}, Val: Tagged{Tag: TagPairs, V: Seq{Items: []Value{
			Map{Pairs: []Pair{{Key: k, Val: Int{V: 1}}}},
			Map{Pairs: []Pair{{Key: k, Val: Int{V: 2}}}},
		}}}})
	}

	if rapid.Bool().Draw(t, "omapset") {
		set := smallMap(t, "s", 1, 3)
		for i := range set.Pairs {
			set.Pairs[i].Val = Null{}
		}

		pairs = append(pairs, Pair{Key: Str{V: "s"}, Val: Tagged{Tag: TagSet, V: set}})
	}

	return Map{Pairs: pairs}
}

// drawTypedFamily writes the 1.1 scalar types side by side, each three draws in four: "!!null" on a null,
// "!!timestamp" on an instant with a time of day, and "!!binary".
//
// go-yaml fixed "!!null" decoded into a struct (5516a57b) and two timestamp spellings (b7c254c0, 0f8e39f9) in one
// week, and the drawn trees held each in under fifty documents with a meaning.
func drawTypedFamily(t *rapid.T) Value {
	var pairs []Pair

	if rapid.IntRange(0, 3).Draw(t, "typednull") != 0 {
		pairs = append(pairs, Pair{Key: Str{V: "n"}, Val: Tagged{Tag: TagNull, V: Null{}}})
	}

	if rapid.IntRange(0, 3).Draw(t, "typedtime") != 0 {
		pairs = append(pairs, Pair{Key: Str{V: "t"}, Val: Tagged{Tag: TagTimestamp, V: Timestamp{V: drawTimeOfDay(t)}}})
	}

	if rapid.IntRange(0, 3).Draw(t, "typedbinary") != 0 {
		bytes := rapid.SliceOfN(rapid.Byte(), 1, 12).Draw(t, "typedbytes")
		pairs = append(pairs, Pair{Key: Str{V: "b"}, Val: Tagged{Tag: TagBinary, V: Binary{V: bytes}}})
	}

	if len(pairs) == 0 {
		pairs = append(pairs, Pair{Key: Str{V: "n"}, Val: Tagged{Tag: TagNull, V: Null{}}})
	}

	return Map{Pairs: pairs}
}

// smallMap draws a mapping of lo to hi entries, keyed prefix0, prefix1 and so on, holding small integers.
func smallMap(t *rapid.T, prefix string, lo, hi int) Map {
	n := rapid.IntRange(lo, hi).Draw(t, prefix+"entries")

	pairs := make([]Pair, 0, n)
	for i := range n {
		pairs = append(pairs, Pair{
			Key: Str{V: fmt.Sprintf("%s%d", prefix, i)},
			Val: Int{V: rapid.IntRange(-9, 9).Draw(t, prefix+"value")},
		})
	}

	return Map{Pairs: pairs}
}
