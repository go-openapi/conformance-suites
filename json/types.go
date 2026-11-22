// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus

import "github.com/go-openapi/conformance-suites/internal/grammar"

// The types the JSON oracle returns. The engine that compiles a grammar is
// internal to this module, so these aliases let a caller name what it receives.
type (
	// Grammar is a compiled grammar, such as [JSON].
	Grammar = grammar.Grammar
	// Result is the oracle's answer for one document.
	Result = grammar.Result
	// Coverage records which productions a recognizer entered, and in which
	// context.
	Coverage = grammar.Coverage
)
