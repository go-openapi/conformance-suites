// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package libfyaml

import (
	"context"
	_ "embed"
	"errors"
	"os"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/jsonlines"
)

//go:embed batch.py
var script string

// Loader is one Python process that loads documents one after another.
//
// [Load] starts a process per document, which suits a test that asks about a few documents. Starting Python and
// importing the binding takes about 18 ms, so the consensus driver uses a Loader.
type Loader struct {
	child jsonlines.Child
}

// New returns a loader that imports the binding installed under home. The process starts on the first document.
func New(home string) *Loader {
	return &Loader{child: jsonlines.Child{
		Argv: []string{"python3", "-c", script},
		Env:  append(os.Environ(), "PYTHONPATH="+home),
	}}
}

type answer struct {
	Docs  []string `json:"docs"`
	Error *string  `json:"error"`
}

// Load reads src under mode ("1.2" or "1.1") and returns each document rendered as JSON by the binding.
//
// When the binding raises, the error is the last line of the traceback, as Python prints it: "ValueError: Failed to
// parse YAML/JSON". When ctx ends first, or the process dies, Load returns that error and the next call starts a new
// process.
func (l *Loader) Load(ctx context.Context, src []byte, mode string) ([]string, error) {
	var a answer
	if err := l.child.Ask(ctx, struct {
		Src  []byte `json:"src"`
		Mode string `json:"mode"`
	}{src, mode}, &a); err != nil {
		return nil, err
	}
	if a.Error != nil {
		return nil, errors.New(*a.Error)
	}
	if a.Docs == nil {
		a.Docs = []string{}
	}

	return a.Docs, nil
}

// Close ends the process.
func (l *Loader) Close() { l.child.Close() }
