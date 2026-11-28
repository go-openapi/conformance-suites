// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package perlref runs the YAML 1.2 reference parser and returns its event
// stream.
//
// # A different question from the other two
//
// [libfyaml] and [goyaml] say what a document *denotes*, and the corpus asks
// them where a meaning is in doubt. This one says what a document *is*, and it
// resolves nothing at all: a plain "1.0" comes back as "=VAL :1.0", the text
// that was written, with no schema applied and no type decided.
//
// That is the question underneath the other two. When libfyaml reads "1.0: a"
// as the key "1.0" and this library reads it as "1", both are naming a node
// that neither of them disagrees about -- and the reference parser is what
// shows the node. It settles structure and identity, which is exactly what JSON
// cannot express and where the other two sources are weakest.
//
// yaml/yaml-reference-parser generates a parser into four languages from the
// specification's own grammar. This is the Perl one, chosen because it needs
// nothing but perl.
//
// # It is not a value oracle
//
// It has no schema, builds no values and answers no question about what a
// scalar means. Asking it whether "1.0" is a float is asking the wrong source.
//
// [libfyaml]: https://pkg.go.dev/github.com/go-openapi/conformance-suites/yaml/consensus/libfyaml
// [goyaml]: https://pkg.go.dev/github.com/go-openapi/conformance-suites/yaml/consensus/goyaml
package perlref

import "github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/perlref"

// Available reports whether the reference parser can be run.
func Available() bool { return perlref.Available() }

// Events runs src through the reference parser and returns the event stream,
// one line per event, and whether the parse succeeded.
//
// A failed parse still returns the events it managed, which is the useful part:
// where a document stops being one is more informative than the fact that it
// did.
func Events(src []byte) (events []string, ok bool, err error) { return perlref.Events(src) }

// Home is where the install script puts it.
//
// REFPARSER_HOME overrides it, and the script reads the same variable, so the
// two cannot drift apart.
func Home() string { return perlref.Home() }
