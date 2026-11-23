// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import "github.com/go-openapi/conformance-suites/yaml/yamlgen"

// ledger copies go-yaml's open divergences at the version this module requires.
//
// go-yaml keeps the list beside its own tests, and its pins (TestDefect*) live there. Update this copy when the go-yaml
// requirement moves: the tally fails the run on an entry that no longer diverges, which is how a fix shows up here.
var ledger = []yamlgen.Divergence{
	{
		Name: "render/a-comment-above-a-blank-line-adds-a-leading-break",
		Pin:  "TestDefectACommentAboveABlankLineAddsALeadingBreak",
		Reason: "A sequence entry carrying a comment, with a blank line before its content, renders " +
			"with a blank line at the head of the document: `-\t#` over an empty line over ` e` " +
			"comes back as `\n- e #`. Rendering that again drops the leading break, so the first " +
			"pass does not settle and the second does.\n\n" +
			"The value survives every pass -- [\"e\"] throughout -- so this claims Settle alone. The " +
			"tab is not part of it: `- #` over a blank line over ` e` does the same, and " +
			"`-\t#` over ` e` with no blank line settles on the first pass.\n\n" +
			"The predicate is wider than the defect. It asks for a line comment and the padding that " +
			"writes blank lines, which is where the pair comes from in a generated document; a blank " +
			"line arriving another way would fail the property rather than be excused. Widen it then.",
		Property: yamlgen.Settle,
		Match:    yamlgen.WritesACommentAboveABlankLine,
	},
	{
		Name: "parse/two-bare-colon-lines-in-a-row-are-refused",
		Pin:  "TestDefectTwoBareColonLinesInARowAreRefused",
		Reason: "`? a` over `:` over `: v` is refused with `found an invalid key for this map`. The " +
			"first `:` is the explicit entry's empty value and the second opens an entry whose key " +
			"is the empty node, which 8.2.2 allows in both places.\n\n" +
			"Three neighbors say it is the pair of bare `:` lines and nothing else, and each reads: " +
			"`a:` over `: v` writes the same two entries with an implicit key; `? a` over `: 1` over " +
			"`: v` gives the explicit entry a value; and the collection key that first reached this " +
			"is not needed at all -- a scalar key does it.\n\n" +
			"grammar.NewRecognizer accepts the document, the reference parser passes it, and " +
			"libfyaml 1.0.0b1 reads {a: null, null: \"v\"}. go.yaml.in/yaml/v3 v3.0.5 refuses it, " +
			"which is its own refusal of a bare `:` as an empty key rather than agreement with us -- " +
			"see yamlcorpus's key shape `a block scalar under an entry with no key`.\n\n" +
			"Reached on 2026-09-08 by Keys drawing a collection, which forces the explicit form and " +
			"so put a bare `:` where nothing had put one before. It claims every property, because a " +
			"document that does not parse answers none.",
		Property: yamlgen.Parses | yamlgen.Decode | yamlgen.Render | yamlgen.Settle | yamlgen.CommentsKept | yamlgen.RenderValid | yamlgen.DecodeTyped,
		Match:    yamlgen.WritesTwoBareColonLinesInARow,
	},
}
