// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild

import "github.com/go-openapi/conformance-suites/stance"

// TagEscapePastUnicode is a double-quoted scalar holding a "\U" escape past U+10FFFF.
//
// ns-esc-32-bit is "U" followed by any eight hex digits, so the grammar accepts "\U0B000023". 5.7 describes the
// escape as a 32-bit Unicode character, and Unicode ends at U+10FFFF: the escape names no character. A scalar's
// content is a series of Unicode characters, so the document has no representation, and every implementation that
// builds a value refuses it. The reference parsers check the grammar only, and read it.
const TagEscapePastUnicode stance.Tag = "escape/past-unicode"

// lastCodePoint is U+10FFFF, the last code point Unicode defines.
const lastCodePoint = 0x10FFFF

// EscapeRules settles the escape: a document holding one is refused.
func EscapeRules() stance.Rules {
	return stance.Rules{
		{
			Tag:     TagEscapePastUnicode,
			Because: "5.7: \\U escapes a 32-bit Unicode character, and no character lies past U+10FFFF",
			Then:    stance.Reject,
		},
	}
}

// EscapeVocabulary places the question at composing: the stream parses, and the scalar it names has no content.
func EscapeVocabulary() stance.Vocabulary {
	return stance.Vocabulary{
		TagEscapePastUnicode: stance.Compose,
	}
}

// EscapeShapes are the escape one past the last code point, and the last code point itself as the control.
func EscapeShapes() []stance.Shape {
	return []stance.Shape{
		{
			Name:   "an escape one past the last code point",
			Src:    []byte("k: \"\\U00110000\"\n"),
			Intent: []stance.Tag{TagEscapePastUnicode},
		},
		{
			// The control: U+10FFFF is a noncharacter, and still a code point, so the escape names it.
			Name:  "an escape of the last code point",
			Src:   []byte("k: \"\\U0010FFFF\"\n"),
			Means: map[string]any{"k": "\U0010FFFF"},
		},
	}
}
