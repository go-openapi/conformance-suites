// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package goopenapi

import (
	"context"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/jsonlines"
)

// Reader is one goyaml-json process that reads documents one after another, through goyaml-json -lines.
//
// [Read] starts goyaml-json once per document, which suits a test that asks about a few documents. Over a corpus the
// process starts take most of go-yaml's time, so the consensus driver uses a Reader.
type Reader struct {
	child jsonlines.Child
}

// New returns a reader that runs the goyaml-json program at command. The process starts on the first document.
func New(command string) *Reader {
	return &Reader{child: jsonlines.Child{Argv: []string{command, "-lines"}}}
}

// Read returns goyaml-json's answer for src.
//
// When ctx ends first, or the process dies, Read returns an error and the next call starts a new process.
func (r *Reader) Read(ctx context.Context, src []byte) (Answer, error) {
	var a Answer
	if err := r.child.Ask(ctx, struct {
		Src []byte `json:"src"`
	}{src}, &a); err != nil {
		return Answer{}, err
	}

	return a, nil
}

// Close ends the process.
func (r *Reader) Close() { r.child.Close() }
