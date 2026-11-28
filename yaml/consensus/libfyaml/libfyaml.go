// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package libfyaml reads a document with libfyaml, so the corpus can settle a
// question against a second implementation instead of against itself.
//
// # Why a second implementation at all
//
// The recognizer answers whether a run of bytes is a document and nothing
// else. Alias resolution, key identity and what a tag means are outside the
// grammar, so for those the corpus has only what somebody wrote down by hand --
// and a hand-written answer is one library's opinion until something
// independent agrees with it. Fifty-seven of its documents are well formed and
// state no meaning at all for exactly that reason.
//
// libfyaml is a separate implementation of YAML 1.2, in C, by the author of the
// specification's own test suite. Where it and this library agree, a stated
// meaning rests on two readings rather than one. Where they disagree, one of
// them is wrong and the corpus says which rather than guessing.
//
// # It is a yardstick and not an oracle
//
// Two limits, both met in practice and both worth knowing before a
// disagreement is read as a verdict.
//
// [Load] goes through json_dumps, which is a lossy view: a mapping with two
// keys that render alike comes back as JSON with a repeated key, so the
// rendering is not always the key identity. Where the question is identity
// rather than value, compare event streams instead.
//
// And loads builds a value, so a refusal may be the binding declining to hold
// something rather than the parser refusing the document. Cycles are where that
// matters.
//
// # A merge needs YAML 1.1
//
// The merge key is a YAML 1.1 type (yaml.org/type/merge) and not part of 1.2,
// so libfyaml resolves "<<" only when it is asked for 1.1. [Load] asks for 1.2,
// where "a: &a {x: 1}" over "m:" over "  <<: *a" comes back as
// {"m": {"<<": {"x": 1}}} -- the handle written out as a member name. Read that
// as the mode and not as libfyaml lacking the feature.
//
// [Load] reads under 1.2, so it cannot answer a question about what a merge
// means. Under 1.1 libfyaml is still not an oracle for two of them: it resolves a merge by document order,
// so the mapping's own key loses to a "<<" written after it, where the merge
// type says "unless the key already exists in it" and go.yaml.in/yaml/v3 agrees
// with the type. And it refuses no duplicate key at all -- "{a: 1, a}" comes
// back as {"a": null} -- so it cannot corroborate a duplicate-key refusal.
//
// # Read the error, not the failure
//
// That second limit cuts both ways, and both directions have cost a register
// entry, so the discriminator is worth stating: **a position and a parser
// message is a syntax verdict; a Python traceback is not.**
//
//	{[a\nb]: 1}    missing comma in flow mapping at 2:3   -- the C parser
//	{[a, b]: 1}    TypeError: unhashable type: 'sequence' -- the binding, after a clean parse
//
// Taking the first for the binding put a wrong ruling in yamlgen.Strict; taking
// the second for the parser would have put a wrong defect in the ledger. When a
// refusal decides a claim, quote the message into the entry so the next reader
// can tell which one it was.
//
// The other tell is a tag whose type cannot be built from its content:
// "{a: !!bool &x}" is refused here because no boolean comes from an empty node,
// and "{a: !!str &x}" reads. Ask with `!!str` when the question is syntax, or
// ask [github.com/go-openapi/conformance-suites/yaml/consensus/perlref], which
// builds nothing and so cannot make either mistake.
package libfyaml

import "github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/libfyaml"

// Available reports whether libfyaml can be reached.
func Available() bool { return libfyaml.Available() }

// Home is where the install script puts the bindings.
//
// LIBFYAML_HOME overrides it, and the script reads the same variable, so the
// two cannot drift apart.
func Home() string { return libfyaml.Home() }

// Load reads src under YAML 1.2 and returns what libfyaml makes of each
// document in it, rendered as JSON.
//
// A document libfyaml refuses returns an error naming what it said. Under 1.2
// "<<" is an ordinary key, so Load does not answer what a merge means.
func Load(src []byte) ([]string, error) { return libfyaml.Load(src) }
