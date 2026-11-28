// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package perlref_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/perlref"
)

// TestOneProcessAnswersAsAProcessPerDocument checks the batch script against the reference parser's own command.
//
// Both answers are compared as the results file stores them, after JSON: a code point past U+10FFFF comes back from
// the command as Perl's own bytes, which are not UTF-8, and JSON writes each as U+FFFD.
func TestOneProcessAnswersAsAProcessPerDocument(t *testing.T) {
	if !perlref.Available() {
		t.Skip("the reference parser is not installed: run hack/conformance/install-reference-parser.sh")
	}

	p := perlref.New(perlref.Home())
	defer p.Close()

	for _, src := range []string{"a: 1\n", "", "[", "- a\n- b\n", "--- !!str\n...\n", "k: \"\\U00110023\"\n"} {
		wantEvents, wantOK, err := perlref.Events([]byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		events, ok, err := p.Events(context.Background(), []byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if wantEvents = viaJSON(t, wantEvents); ok != wantOK || !slices.Equal(events, wantEvents) {
			t.Errorf("%q: got %v %q, want %v %q", src, ok, events, wantOK, wantEvents)
		}
	}
}

func viaJSON(t *testing.T, events []string) []string {
	t.Helper()
	b, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	return out
}
