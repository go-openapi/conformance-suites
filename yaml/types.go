// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "github.com/go-openapi/conformance-suites/internal/grammar"

// The types the YAML oracle returns. The engine that compiles a grammar is
// internal to this module, so these aliases let a caller name what it receives.
type (
	// Grammar is a compiled grammar, such as [YAML].
	Grammar = grammar.Grammar
	// Result is the oracle's answer for one document.
	Result = grammar.Result
	// Coverage records which productions a recognizer entered, and in which
	// context.
	Coverage = grammar.Coverage
	// Reach is the set of productions and contexts a start rule can enter,
	// against which a coverage vector is measured.
	Reach = grammar.Reach
)
