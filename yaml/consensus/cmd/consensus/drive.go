// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/jsonlines"
)

// request is one case the runner sends a driver, as one JSON line on the driver's stdin.
type request struct {
	SHA string `json:"sha"`
	Src []byte `json:"src"`
}

// response is a driver's answer to one request, as one JSON line on its stdout, in the order of the requests.
type response struct {
	SHA    string           `json:"sha"`
	Answer consensus.Answer `json:"answer"`
	// Seconds is how long the driver took to answer, from reading the request to writing the answer.
	Seconds float64 `json:"seconds"`
}

func driveCommand(args []string) error {
	fs := flag.NewFlagSet("drive", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 10*time.Second, "how long the source may take on one document")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("drive takes one source id")
	}

	for _, s := range allSources() {
		if s.id == fs.Arg(0) {
			return drive(s, *timeout, os.Stdin, os.Stdout)
		}
	}

	return fmt.Errorf("drive: no source %q", fs.Arg(0))
}

// drive answers every request read from r with source s, writing one response per request to w, until r ends.
//
// A panic while answering one document becomes an error answer for that document, and the loop goes on. A failure
// the process cannot survive, such as a stack overflow, ends it; the runner then restarts the driver at the next
// case.
func drive(s source, timeout time.Duration, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), jsonlines.MaxLine)
	out := bufio.NewWriter(w)
	enc := json.NewEncoder(out)

	for sc.Scan() {
		var q request
		if err := json.Unmarshal(sc.Bytes(), &q); err != nil {
			return fmt.Errorf("drive %s: reading a request: %w", s.id, err)
		}

		start := time.Now()
		a := answerSafely(s, q.Src, timeout)
		if err := enc.Encode(response{SHA: q.SHA, Answer: a, Seconds: time.Since(start).Seconds()}); err != nil {
			return err
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}

	return sc.Err()
}

// answerSafely is [timed], with a panic turned into an error answer.
func answerSafely(s source, src []byte, timeout time.Duration) (a consensus.Answer) {
	defer func() {
		if r := recover(); r != nil {
			a = consensus.Answer{Source: s.id, Verdict: consensus.Error, Message: consensus.FirstLine(fmt.Sprintf("panic: %v", r))}
		}
	}()

	return timed(s, src, timeout)
}
