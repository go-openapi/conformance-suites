// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
)

// TestTheDirectiveTagsAreFoundInTheText holds DirectiveTags to the mutants the consensus runner found untagged,
// where every implementation refuses the document, and to the places a "!x!" is text and not a tag.
func TestTheDirectiveTagsAreFoundInTheText(t *testing.T) {
	const (
		undeclared = yamlbuild.TagUndeclaredHandle
		reserved   = yamlbuild.TagDirectiveReserved
	)

	for src, want := range map[string][]stance.Tag{
		// Declared, and so read.
		"%TAG !e! tag:example.com,2011:\n---\nk: !e!x v\n":                nil,
		"%TAG !x! tag:yaml.org,2002:\r---\r!x!str \"Null\"\r":             nil,
		"\ufeff%TAG !n-1! tag:yaml.org,2002:\r---\r!n-1!bool &a1 false\r": nil,
		"!!str a\n":                     nil,
		"! a\n":                         nil,
		"!local a\n":                    nil,
		"!<tag:example.com,2000:x> a\n": nil,
		"%TAG !! tag:yaml.org,2002:\n---\n!!int 1\n":                                 nil,
		"%YAML 1.2\n%TAG !x! tag:yaml.org,2002:\n---\n- !x!int 1\n- {k: !x!str v}\n": nil,

		// Undeclared: the shape the corpus writes on purpose, and the mutants.
		"k: !e!x v\n":      {undeclared},
		"\ufeff!x!str a\n": {undeclared},
		"TAG !x! tag:yaml.org,2002:\r---\r!x!str \"Null\"\r":    {undeclared},
		"%TAG !! tag:yaml.org,2002:\r---\r!x!pairs []\r":        {undeclared},
		"%TAG !TAG2! tag:yaml.org,2002:\r---\r!TG2!int 0o513\r": {undeclared},
		"!i!nt 4\n": {undeclared},
		"- &a1 null # c1\r\n- !i!nt -735 # c2\r\n":                                 {undeclared},
		"%TAG !x! tag:yaml.org,2002:\r---\r{\"s\": !xx!set {\"s0\": null}} # c1\r": {undeclared},
		"%TAG! TAG2! tag:yaml.org,2002:\r---\r!TAG2!int 149\r":                     {reserved, undeclared},
		"%TCG !TAG2! tag:yaml.org,2002:\r---\r!TAG2!int 149\r":                     {reserved, undeclared},
		"%TAG !x! tag:yaml.org,2002:\n---\na\n---\n!x!str b\n":                     {yamlbuild.TagHandleOutOfScope},

		// The other directive tags.
		"%YAML 2.0\n---\na\n":                         {yamlbuild.TagYAMLMajorVersion},
		"%YAML 1.22\n---\na\n":                        {yamlbuild.TagYAMLMinorVersion},
		"%YAML 1.1\n---\na\n":                         nil,
		"%YAML 1.2\n%YAML 1.2\n---\na\n":              {yamlbuild.TagYAMLRepeated},
		"%YAML 1.2\n---\na\n...\n%YAML 1.2\n---\nb\n": nil,
		"%TAG !x! a:\n%TAG !x! b:\n---\n!x!c d\n":     {yamlbuild.TagTagHandleRepeated},
		"%%YAML 1.2\n---\na\n":                        {reserved},
		"%FOO bar # c\n---\na\n":                      {reserved},

		// Text, not a tag: quoted, plain, a comment, a block scalar.
		"k: '!x!str'\n":                nil,
		"k: \"a !x!str\"\n":            nil,
		"k: a !x!str\n":                nil,
		"k: v # !x!str\n":              nil,
		"k: |\n  !x!str\n  more\n":     nil,
		"- >-\n  !x!str\n- b\n":        nil,
		"k: 'two\n  lines !x! here'\n": nil,
	} {
		got := yamlbuild.DirectiveTags([]byte(src))
		if len(want) == 0 {
			assert.Emptyf(t, got, "%q", src)

			continue
		}
		assert.Equalf(t, want, got, "%q", src)
	}
}
