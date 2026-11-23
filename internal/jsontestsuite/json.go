// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package jsontestsuite reads the vendored copy of Nicolas Seriot's JSONTestSuite.
//
// The fixtures sit in testdata/test_parsing, unmodified. testdata/SOURCE.md names the upstream revision.
package jsontestsuite

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Expect is what the suite requires of a parser, read from the first letter of a fixture's name.
type Expect byte

const (
	// Accept is a "y_" fixture: a conforming parser accepts it.
	Accept Expect = 'y'
	// Reject is an "n_" fixture: a conforming parser rejects it.
	Reject Expect = 'n'
	// Either is an "i_" fixture: the outcome is implementation-defined.
	Either Expect = 'i'
)

// Case is one fixture of the suite.
type Case struct {
	// Name is the file name without ".json", as in "n_number_0.1.2".
	Name string
	// Src is the file's bytes, unmodified.
	Src []byte
	// Expect is what the suite requires of a parser.
	Expect Expect
}

// fixturesDir is the vendored copy, held in this package's testdata directory so that the fixtures sit beside the
// loader that reads them.
func fixturesDir() string {
	_, file, _, _ := runtime.Caller(0) //nolint:dogsled

	return filepath.Join(filepath.Dir(file), "testdata", "test_parsing")
}

// Cases reads every fixture of the vendored suite, in name order.
func Cases() ([]Case, error) {
	return casesIn(fixturesDir())
}

func casesIn(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading the JSON test suite: %w", err)
	}

	out := make([]Case, 0, len(entries))

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		expect, ok := expectOf(name)
		if !ok {
			return nil, fmt.Errorf("fixture %s: the name does not start with y_, n_ or i_", name)
		}

		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading the JSON test suite: %w", err)
		}

		out = append(out, Case{Name: strings.TrimSuffix(name, ".json"), Src: src, Expect: expect})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}

func expectOf(name string) (Expect, bool) {
	if len(name) < 2 || name[1] != '_' {
		return 0, false
	}

	switch e := Expect(name[0]); e {
	case Accept, Reject, Either:
		return e, true
	default:
		return 0, false
	}
}
