// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import "time"

// Placement is one combination of a node property, an alias or a 1.1 type with the place it stands, recognized in a
// [Value] tree.
type Placement struct {
	Name string
	has  func(*propertyFacts) bool
}

// Has reports whether v holds the combination.
func (p Placement) Has(v Value) bool { return p.has(factsOf(v)) }

// Placements are the combinations the generator is held to draw, each in enough documents that carry a meaning.
//
// # Why these, and why not the YAML Test Suite
//
// The construct census holds the corpus to what the YAML Test Suite contains. These are mostly YAML 1.1 types --
// "!!merge", "!!omap", "!!pairs", "!!set", "!!timestamp", "!!binary" -- which the 1.2 suite barely uses, so that census
// sets no bar for them. Between 2026-11-16 and 2026-11-23 go-yaml fixed about twenty defects in exactly these
// combinations: a property on a "<<" key or value, an "!!omap" holding a "<<" or an alias, a timestamp's spelling.
// Each entry names a combination one of those defects lived in.
//
// They are read from the Value tree, not from the bytes, so a predicate names the node's role exactly: a tag on a
// merge value is a [Tagged] standing as the value of a [MergeKey] entry, whatever the emitter made of it.
func Placements() []Placement { return placements }

var placements = []Placement{
	{"a merge key", func(f *propertyFacts) bool { return f.mergeKey }},
	{"a merge key tagged !!merge", func(f *propertyFacts) bool { return f.taggedMergeKey }},
	{"an anchored merge key", func(f *propertyFacts) bool { return f.anchoredMergeKey }},
	{"a merge value that is an alias", func(f *propertyFacts) bool { return f.mergeValue["alias"] }},
	{"a merge value that is a mapping written in place", func(f *propertyFacts) bool { return f.mergeValue["mapping"] }},
	{"a merge value that is a sequence", func(f *propertyFacts) bool { return f.mergeValue["sequence"] }},
	{"a tagged merge value", func(f *propertyFacts) bool { return f.mergeValue["tagged"] }},
	{"an anchored merge value", func(f *propertyFacts) bool { return f.mergeValue["anchored"] }},
	{"an !!omap", func(f *propertyFacts) bool { return f.omap }},
	{"an anchored !!omap", func(f *propertyFacts) bool { return f.anchoredOMap }},
	{"an alias to an !!omap", func(f *propertyFacts) bool { return f.aliasToOMap }},
	{"an !!omap entry holding a merge", func(f *propertyFacts) bool { return f.omapEntryMerge }},
	{"an alias inside an !!omap", func(f *propertyFacts) bool { return f.aliasInOMap }},
	{"a !!pairs", func(f *propertyFacts) bool { return f.tags[TagPairs] }},
	{"a !!set", func(f *propertyFacts) bool { return f.tags[TagSet] }},
	{"a tagged key", func(f *propertyFacts) bool { return f.taggedKey }},
	{"a tagged and anchored key", func(f *propertyFacts) bool { return f.taggedAnchoredKey }},
	{"a !!timestamp", func(f *propertyFacts) bool { return f.tags[TagTimestamp] }},
	{"a !!timestamp with a time of day", func(f *propertyFacts) bool { return f.timeOfDay }},
	{"a !!binary", func(f *propertyFacts) bool { return f.tags[TagBinary] }},
	{"a !!null", func(f *propertyFacts) bool { return f.tags[TagNull] }},
	{"a tag and an anchor on one node", func(f *propertyFacts) bool { return f.taggedAnchored }},
}

// propertyFacts is what one walk over a tree found.
type propertyFacts struct {
	mergeKey, taggedMergeKey, anchoredMergeKey bool
	mergeValue                                 map[string]bool
	omap, anchoredOMap, aliasToOMap            bool
	omapEntryMerge, aliasInOMap                bool
	taggedKey, taggedAnchoredKey               bool
	timeOfDay, taggedAnchored                  bool
	tags                                       map[string]bool
}

func factsOf(v Value) *propertyFacts {
	f := &propertyFacts{mergeValue: map[string]bool{}, tags: map[string]bool{}}
	f.walk(v, false)

	return f
}

// walk records what v holds. inOMap is true below an "!!omap".
func (f *propertyFacts) walk(v Value, inOMap bool) {
	switch n := v.(type) {
	case Anchored:
		if isOMap(n.V) {
			f.anchoredOMap = true
		}

		if _, tagged := n.V.(Tagged); tagged {
			f.taggedAnchored = true
		}

		f.walk(n.V, inOMap)
	case Alias:
		if inOMap {
			f.aliasInOMap = true
		}

		if isOMap(bareOfAnchor(n.V)) {
			f.aliasToOMap = true
		}
		// The aliased node was walked where it was anchored.
	case Tagged:
		f.tags[n.Tag] = true

		if _, anchored := n.V.(Anchored); anchored {
			f.taggedAnchored = true
		}

		if ts, isTime := bare(n.V).(Timestamp); isTime && !ts.V.Equal(ts.V.Truncate(24*time.Hour)) {
			f.timeOfDay = true
		}

		if n.Tag == TagOMap && isSequenceOfSinglePairs(n.V) {
			f.omap = true
			inOMap = true
		}

		f.walk(n.V, inOMap)
	case Seq:
		for _, item := range n.Items {
			f.walk(item, inOMap)
		}
	case Map:
		for _, p := range n.Pairs {
			f.key(p, inOMap)
			f.walk(p.Key, inOMap)
			f.walk(p.Val, inOMap)
		}
	}
}

// key records what one entry's key and, for a merge, its value hold.
func (f *propertyFacts) key(p Pair, inOMap bool) {
	if IsMergeKey(p.Key) {
		f.mergeKey = true
		f.mergeValueKinds(p.Val)

		if inOMap {
			f.omapEntryMerge = true
		}
	}

	switch k := p.Key.(type) {
	case Tagged:
		if _, merge := k.V.(MergeKey); merge && k.Tag == TagMerge {
			f.taggedMergeKey = true
		} else {
			f.taggedKey = true
		}

		if _, anchored := k.V.(Anchored); anchored {
			f.taggedAnchoredKey = true
		}
	case Anchored:
		if _, merge := k.V.(MergeKey); merge {
			f.anchoredMergeKey = true
		}

		if _, tagged := k.V.(Tagged); tagged {
			f.taggedKey = true
			f.taggedAnchoredKey = true
		}
	}
}

func (f *propertyFacts) mergeValueKinds(v Value) {
	switch n := v.(type) {
	case Alias:
		f.mergeValue["alias"] = true
	case Anchored:
		f.mergeValue["anchored"] = true
		f.mergeValueKinds(n.V)
	case Tagged:
		f.mergeValue["tagged"] = true
		f.mergeValueKinds(n.V)
	case Map:
		f.mergeValue["mapping"] = true
	case Seq:
		f.mergeValue["sequence"] = true

		for _, item := range n.Items {
			f.mergeValueKinds(item)
		}
	}
}

// IsMergeKey reports whether k is a "<<" key, under any anchor or tag standing on it.
func IsMergeKey(k Value) bool {
	switch n := k.(type) {
	case MergeKey:
		return true
	case Tagged:
		return IsMergeKey(n.V)
	case Anchored:
		return IsMergeKey(n.V)
	default:
		return false
	}
}

func isOMap(v Value) bool {
	t, tagged := v.(Tagged)

	return tagged && t.Tag == TagOMap && isSequenceOfSinglePairs(t.V)
}

// bareOfAnchor looks through the anchors in front of v, and stops at a tag.
func bareOfAnchor(v Value) Value {
	for {
		a, anchored := v.(Anchored)
		if !anchored {
			return v
		}

		v = a.V
	}
}
