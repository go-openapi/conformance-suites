// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/jsonlines"
)

// process is one driver of a source: a `consensus drive` process the runner sends cases to, one at a time.
//
// It starts on the first case, and again on the next case after it crashed or timed out.
type process struct {
	child jsonlines.Child
}

// errTimedOut and errCrashed tell why a driver gave no answer. The process is stopped either way.
var (
	errTimedOut = errors.New("timed out")
	errCrashed  = jsonlines.ErrCrashed
)

func newProcess(argv, env []string) *process {
	return &process{child: jsonlines.Child{Argv: argv, Env: env}}
}

// ask sends one case and waits for its answer. On a timeout or a crash it stops the process and returns
// [errTimedOut], or [errCrashed] wrapped with what the driver wrote on stderr.
func (p *process) ask(q request, timeout time.Duration) (response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var r response
	err := p.child.Ask(ctx, q, &r)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return response{}, errTimedOut
	case err != nil:
		return response{}, err
	case r.SHA != q.SHA:
		return response{}, fmt.Errorf("%w: answered out of turn: %s", errCrashed, p.child.Stop())
	}

	return r, nil
}

// close ends the driver after its last case.
func (p *process) close() { p.child.Close() }
