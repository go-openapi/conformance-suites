// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus

import (
	"github.com/go-openapi/conformance-suites/stance"
)

// around is what JSON contributes to the shared encoding shapes: a small valid
// document, a way to put bytes inside a string, and its whitespace.
//
// The document is deliberately the smallest thing that is unambiguously a
// document. A larger one would exercise the same encoding question and make
// every failure harder to read.
func around() stance.Around {
	return stance.Around{
		Document:   []byte(`{"a":1}`),
		Whitespace: []byte(" \t\r\n"),
		InString: func(b []byte) []byte {
			out := make([]byte, 0, len(b)+10)
			out = append(out, `{"a":"`...)
			out = append(out, b...)

			return append(out, `"}`...)
		},
	}
}

// EncodingShapes is the cross-product of every byte order mark against every
// position it can occupy, as JSON documents.
func EncodingShapes() []stance.Shape { return stance.EncodingShapes(around()) }

// ValueShapes are the documents whose meaning RFC 8259 leaves open, written by
// hand because the generator draws only values it knows the meaning of.
//
// A Go string cannot hold a lone surrogate, so a drawn Str never writes one,
// and RandomValue drops repeated names on purpose. Each shape carries the tag
// that makes its outcome a parser's declared position.
func ValueShapes() []stance.Shape {
	dup := []stance.Tag{TagDuplicateName}
	lone := []stance.Tag{TagLoneSurrogate}

	return []stance.Shape{
		{Name: "duplicate name", Src: []byte(`{"a":1,"a":2}`), Intent: dup},
		{Name: "duplicate name and value", Src: []byte(`{"a":1,"a":1}`), Intent: dup},
		{Name: "duplicate name spelled two ways", Src: []byte(`{"a":1,"a":2}`), Intent: dup},
		{Name: "duplicate empty name", Src: []byte(`{"":1,"":2}`), Intent: dup},
		{Name: "duplicate name in a nested object", Src: []byte(`[{"b":[],"b":{}}]`), Intent: dup},
		{Name: "lone high surrogate", Src: []byte(`["\uD834"]`), Intent: lone},
		{Name: "lone low surrogate", Src: []byte(`["\uDD1E"]`), Intent: lone},
		{Name: "surrogates in reverse order", Src: []byte(`["\uDD1E\uD834"]`), Intent: lone},
		{Name: "high surrogate before text", Src: []byte(`["\uD834x"]`), Intent: lone},
		{Name: "two high surrogates", Src: []byte(`["\uD834\uD834"]`), Intent: lone},
		{Name: "lone surrogate in a name", Src: []byte(`{"\uDFFF":1}`), Intent: lone},
	}
}
