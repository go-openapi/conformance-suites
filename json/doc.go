// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package jsoncorpus builds a conformance corpus for JSON.
//
// [JSON] is RFC 8259 compiled by the grammar package, with no patches at all:
// every production the RFC publishes runs as written. [Generate] draws
// documents from a seed and breaks each of them a fixed number of times,
// [Build] keeps the ones entering a coverage bucket nothing has entered yet,
// and [Build.Write] produces the artifact. [Compact] is the corpus checked in
// beside this package, and [CompactSuite] reads it with one call.
//
// # JSON asks fewer questions than YAML
//
// The grammar compiles to 33 rules and has no propagated parameters, so a
// coverage bucket is a production and not a production crossed with a
// context. RFC 8259 also leaves little to the implementation, and these tags
// name what it does leave open, each placed by [Vocabulary] at the stage its
// question arises:
//
//   - [TagLoneSurrogate], "string/lone-surrogate", at parse: a \uXXXX escape
//     naming a surrogate that pairs with nothing. Section 8.2 calls its
//     meaning unpredictable.
//   - [TagDeepNesting], "structure/deep-nesting", at parse: a document nested
//     more than 128 levels deep. No specification sets a limit, and every
//     parser has one. The generator nests at most 24 levels on purpose: the
//     corpus tests what a parser reads, not the budget that stops it on
//     hostile input, so no case carries this tag.
//   - [TagDuplicateName], "object/duplicate-name", at compose: an object with
//     two members of one name. Section 4 says names SHOULD be unique and calls
//     the behavior of a receiver unpredictable.
//   - [TagNumberOutOfRange], "number/out-of-range", at construct: a number the
//     grammar admits and a float64 cannot hold. Section 6 lets a parser limit
//     range and precision.
//
// The four encoding tags of package stance -- [stance.TagBOM],
// [stance.TagNotUTF8], [stance.TagUTF16] and [stance.TagUTF32] -- complete the
// vocabulary, at parse. RFC 8259 settles none of the eight, so every one is a
// position a parser takes.
//
// So a consumer declares a short [stance.Table], as [DefaultLexer] does, and
// most cases score against the grammar's verdict alone.
//
// # Using it
//
// Read the artifact and score a parser against it:
//
//	header, cases, err := jsoncorpus.CompactSuite()
//
// [Tags] reports what a document exhibits and [Describe] builds the
// [stance.Doc] a table turns into an expectation. [Normalize] strips the
// encoding properties a parser is entitled to normalize away before the
// grammar sees the bytes.
//
// Nothing here depends on YAML, which is what keeps the grammar package honest:
// a recognizer that compiled only one language would have YAML built into it.
package jsoncorpus
