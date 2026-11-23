// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"testing"

	"github.com/go-openapi/conformance-suites/stance"
	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
)

// TestEverySchemaShapeIsAValidDocument keeps the family honest about what it is
// testing.
//
// None of these raises a question about syntax: every one is a document YAML
// accepts under every schema, and the whole disagreement is about what the
// scalar denotes. A shape the grammar refused would be a syntax test wearing a
// schema's clothes.
func TestEverySchemaShapeIsAValidDocument(t *testing.T) {
	rec := yamloracle.NewRecognizer(256)

	for _, s := range yamlbuild.SchemaShapes() {
		if got := rec.Stream(s.Src); !got.OK {
			t.Errorf("%s: the grammar refuses %q", s.Name, string(s.Src))
		}
	}
}

// TestEverySchemaTagIsPlaced holds the family to the vocabulary, the way the
// anchor patterns are held to theirs.
func TestEverySchemaTagIsPlaced(t *testing.T) {
	vocabulary := yamlbuild.Vocabulary()

	var docs []stance.Doc
	for _, s := range yamlbuild.SchemaShapes() {
		docs = append(docs, stance.Doc{Name: s.Name, Tags: s.Intent})
	}

	if unplaced := vocabulary.Unplaced(docs); len(unplaced) > 0 {
		t.Errorf("the schema shapes use tags no stage places: %v", unplaced)
	}
}

// TestNoSchemaQuestionIsSettled is the assertion that keeps this family from
// quietly becoming a judgement.
//
// Not one of these tags may be a rule. A parser implementing the JSON schema
// and a parser implementing 1.1 disagree about nearly every scalar here and
// both are conformant, so a rule would be this corpus asserting one schema and
// calling every other implementation defective.
func TestNoSchemaQuestionIsSettled(t *testing.T) {
	rules := yamlbuild.AnchorRules()

	for tag := range yamlbuild.SchemaVocabulary() {
		if _, settled := rules.Of(tag); settled {
			t.Errorf("%s is settled by a rule, and which schema a parser implements is not ours to settle", tag)
		}
	}
}
