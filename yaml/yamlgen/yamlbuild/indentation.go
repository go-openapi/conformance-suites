// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild

import "github.com/go-openapi/conformance-suites/stance"

// TagTabOpensContent is a root block scalar whose first content line opens with a tab at column 0.
//
// The grammar reads the tab as content. A bare document is s-l+block-node(-1, block-in), so a root block scalar's
// content stands at column 0 (FP8R and DK3J in the YAML Test Suite), s-indent(0) matches nothing, and the tab falls in
// nb-char: "|" over "\tx" is "\tx\n". 6.1 keeps a tab out of the indentation, and this tab stands after it.
//
// The spec holds no example of the shape, and to a reader the tab looks like the indentation 6.1 forbids, so a parser
// that refuses it as a typing mistake is defensible. libfyaml refuses "|" over "\tx" and reads "--- |" over "\tx";
// yaml.v3 refuses both. The tag leaves the stand to each consumer, and the shapes carry the grammar's value.
const TagTabOpensContent stance.Tag = "indentation/tab-opens-content"

// IndentationVocabulary places the question at parsing: the scanner reads the tab as content or refuses it.
func IndentationVocabulary() stance.Vocabulary {
	return stance.Vocabulary{
		TagTabOpensContent: stance.Parse,
	}
}

// IndentationShapes are root block scalars whose content opens with a tab, and two controls without one.
func IndentationShapes() []stance.Shape {
	return []stance.Shape{
		{
			Name:   "a literal scalar whose content opens with a tab",
			Src:    []byte("|\n\tx\n"),
			Intent: []stance.Tag{TagTabOpensContent},
			Means:  "\tx\n",
		},
		{
			Name:   "a folded scalar after the document marker whose content opens with a tab",
			Src:    []byte("--- >-\n\t.5\n"),
			Intent: []stance.Tag{TagTabOpensContent},
			Means:  "\t.5",
		},
		{
			Name:   "a tagged literal scalar whose content opens with a tab",
			Src:    []byte("--- !!str |\n\tx\n\ty\n"),
			Intent: []stance.Tag{TagTabOpensContent},
			Means:  "\tx\n\ty\n",
		},
		{
			// The control for the tab: after one space of indentation the tab is content for every parser.
			Name:  "a literal scalar whose content opens with a space and a tab",
			Src:   []byte("|\n \tx\n"),
			Means: "\tx\n",
		},
		{
			// The control for the column: FP8R, content at column 0 with no tab.
			Name:  "a folded scalar whose content stands at column 0",
			Src:   []byte("--- >\nline1\nline2\n"),
			Means: "line1 line2\n",
		},
	}
}
