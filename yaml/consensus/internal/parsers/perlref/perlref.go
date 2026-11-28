// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package perlref runs the YAML 1.2 reference parser and returns its event stream.
//
// The public package github.com/go-openapi/conformance-suites/yaml/consensus/perlref documents the source and
// re-exports what other modules use.
package perlref

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotInstalled says the reference parser could not be found.
var ErrNotInstalled = errors.New(
	"perlref: not installed, run hack/conformance/install-reference-parser.sh")

// Home is where the install script puts it.
//
// REFPARSER_HOME overrides it, and the script reads the same variable, so the
// two cannot drift apart.
func Home() string {
	if set := os.Getenv("REFPARSER_HOME"); set != "" {
		return set
	}

	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		cache = filepath.Join(home, ".cache")
	}

	return filepath.Join(cache, "go-openapi", "yaml-reference-parser")
}

// Available reports whether the reference parser can be run.
func Available() bool {
	_, err := os.Stat(filepath.Join(Home(), "bin", "yaml-parser"))

	return err == nil
}

// Commit is the pinned revision, so a corpus entry that rests on this reading
// records which one gave it.
func Commit() string {
	out, err := os.ReadFile(filepath.Join(Home(), "COMMIT"))
	if err != nil {
		return ""
	}

	first, _, _ := strings.Cut(string(out), "\n")

	return strings.TrimSpace(first)
}

// Events runs src through the reference parser and returns the event stream,
// one line per event, and whether the parse succeeded.
//
// A failed parse still returns the events it managed, which is the useful part:
// where a document stops being one is more informative than the fact that it
// did.
func Events(src []byte) (events []string, ok bool, err error) {
	return EventsContext(context.Background(), src)
}

// EventsContext is [Events], stopped when ctx is done.
func EventsContext(ctx context.Context, src []byte) (events []string, ok bool, err error) {
	if !Available() {
		return nil, false, ErrNotInstalled
	}

	cmd := exec.CommandContext(ctx, "perl", filepath.Join(Home(), "bin", "yaml-parser")) //nolint:gosec // the reference parser the install script put under Home
	cmd.Stdin = bytes.NewReader(src)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	// A refused document exits non-zero on some builds and zero on others, and
	// the PASS/FAIL line is the answer either way, so the exit status is not
	// consulted. Only a failure to run at all is an error.
	if runErr := cmd.Run(); runErr != nil {
		if _, ok := errors.AsType[*exec.ExitError](runErr); !ok {
			return nil, false, fmt.Errorf("perlref: %w: %s", runErr, strings.TrimSpace(errs.String()))
		}
	}

	return parse(out.String(), string(src))
}

// parse reads the parser's report on src: a PASS or FAIL line that also echoes the document, then the events, then
// a timing line this package has no use for.
//
// The echo runs from the "'" after "PASS - " to a "'" at its end, and src lines starting with "-", "+" or "=" look
// like events. Read as events, a document holding "- a: 1" gave the event "- a: 1", and two documents differing only
// in such a line had different streams. The echo is skipped by its length when it holds src exactly, and otherwise
// only the lines after its closing "'" are read.
func parse(out, src string) ([]string, bool, error) {
	var ok bool

	switch {
	case strings.HasPrefix(out, "PASS - '"):
		ok, out = true, out[len("PASS - '"):]
	case strings.HasPrefix(out, "FAIL - '"):
		out = out[len("FAIL - '"):]
	default:
		// A message may stand before the verdict, as "Parser finished before end of input" does on a refusal.
		pass, fail := strings.Index(out, "\nPASS - '"), strings.Index(out, "\nFAIL - '")
		switch {
		case pass >= 0 && (fail < 0 || pass < fail):
			ok, out = true, out[pass+len("\nPASS - '"):]
		case fail >= 0:
			out = out[fail+len("\nFAIL - '"):]
		default:
			return nil, false, fmt.Errorf("perlref: no PASS or FAIL in its report: %q", out)
		}
	}

	if strings.HasPrefix(out, src+"'") {
		out = out[len(src)+1:]
	} else if i := strings.Index(out, "'\n+STR"); i >= 0 {
		out = out[i+1:]
	}

	var events []string

	for line := range strings.SplitSeq(out, "\n") {
		if isEvent(line) {
			events = append(events, line)
		}
	}

	return events, ok, nil
}

// isEvent reports whether line is one of the YAML Test Suite's events: "+STR", "-DOC ...", "=VAL :a", "=ALI *a".
func isEvent(line string) bool {
	for _, kind := range []string{"+STR", "-STR", "+DOC", "-DOC", "+MAP", "-MAP", "+SEQ", "-SEQ", "=VAL", "=ALI"} {
		if line == kind || strings.HasPrefix(line, kind+" ") {
			return true
		}
	}

	return false
}
