// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package perlref

import (
	"context"
	_ "embed"
	"path/filepath"
	"strings"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/jsonlines"
)

//go:embed batch.pl
var script string

// Parser is one Perl process that parses documents one after another.
//
// [Events] starts a process per document, which suits a test that asks about a few documents. Loading the parser
// takes about 65 ms, more than parsing most documents, so the consensus driver uses a Parser.
type Parser struct {
	child jsonlines.Child
}

// New returns a parser that runs the reference parser checked out at home. The process starts on the first
// document.
func New(home string) *Parser {
	return &Parser{child: jsonlines.Child{Argv: []string{
		"perl",
		"-I" + filepath.Join(home, "lib"),
		"-I" + filepath.Join(home, "ext", "perl", "lib", "perl5"),
		"-e", script,
	}}}
}

type answer struct {
	OK     bool   `json:"ok"`
	Events string `json:"events"`
	Error  string `json:"error"`
}

// Events parses src and returns its event stream in yaml-test-suite test.event form, and whether the parser accepted
// it. A refused document returns the events up to the refusal.
//
// When ctx ends first, or the process dies, Events returns an error and the next call starts a new process.
func (p *Parser) Events(ctx context.Context, src []byte) (events []string, ok bool, err error) {
	var a answer
	if err := p.child.Ask(ctx, struct {
		Src []byte `json:"src"`
	}{src}, &a); err != nil {
		return nil, false, err
	}

	for line := range strings.SplitSeq(a.Events, "\n") {
		if line != "" {
			events = append(events, line)
		}
	}

	return events, a.OK, nil
}

// Close ends the process.
func (p *Parser) Close() { p.child.Close() }
