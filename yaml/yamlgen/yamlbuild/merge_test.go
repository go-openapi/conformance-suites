// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"testing"

	yamloracle "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
)

// TestTheGrammarHasNoOpinionAboutMerge is why the family is enumerated.
//
// Every merge shape is a document the grammar accepts, the ones a merging
// parser must refuse included. "<<" is an ordinary plain scalar to a grammar --
// there is no production for it and there could not be, since 1.2 does not
// define the type at all.
func TestTheGrammarHasNoOpinionAboutMerge(t *testing.T) {
	rec := yamloracle.NewRecognizer(1024)

	for _, s := range yamlbuild.MergeShapes() {
		if got := rec.Stream(s.Src); !got.OK {
			t.Errorf("%s: the grammar refuses it, so it is testing syntax: %q", s.Name, string(s.Src))
		}
	}
}

// TestNoMergeQuestionIsSettled is the assertion that keeps this family from
// becoming a judgement.
//
// No version of the specification we test against defines "<<", so a rule here
// would be the corpus picking a version of the language and calling every other
// implementation defective. libfyaml reads "<<" as an ordinary key and this
// library merges; both are right.
func TestNoMergeQuestionIsSettled(t *testing.T) {
	rules := yamlbuild.KeyRules()
	rules = append(rules, yamlbuild.AnchorRules()...)
	rules = append(rules, yamlbuild.TagRules()...)

	for tag := range yamlbuild.MergeVocabulary() {
		if _, settled := rules.Of(tag); settled {
			t.Errorf("%s is settled by a rule, and merge is not ours to settle", tag)
		}
	}
}
