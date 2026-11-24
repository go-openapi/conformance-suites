// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlstar_test

import (
	"context"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/consensus/perlref"
	"github.com/go-openapi/conformance-suites/yaml/consensus/yamlstar"
)

// TestTheEventsMatchTheReferenceParser holds the conversion from "yaml -D parse" to test.event against the Perl
// reference parser, on documents both read alike: every style, tag, anchor, alias and document marker the
// conversion writes.
func TestTheEventsMatchTheReferenceParser(t *testing.T) {
	if !yamlstar.Available() {
		t.Skip("yamlstar is not installed; set YAMLSTAR or put yaml on PATH")
	}
	if !perlref.Available() {
		t.Skipf("the reference parser is not installed at %s", perlref.Home())
	}

	for _, src := range []string{
		"a: 1\n",
		"%YAML 1.2\n---\n- 'sq'\n- \"dq\"\n- !!str plain\n- !local x\n- |\n  lit\n- >-\n  fold\n- {a: 1}\n- ? k\n  : v\n-\n...\n",
		"a: &x [1, 2]\nb: *x\n",
		"--- a\n--- b\n",
		"\"tab\\there\\nline\"\n",
		"? [a, b]\n: c\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, wantOK, err := perlref.Events([]byte(src))
			require.NoError(t, err)

			got, ok, msg, err := yamlstar.Events(context.Background(), []byte(src))
			require.NoError(t, err)
			require.Equalf(t, wantOK, ok, "verdict: %s", msg)
			assert.Equal(t, want, got)
		})
	}
}

// TestARefusalIsTheParsers holds that a document the parser refuses comes back refused, with what it said.
func TestARefusalIsTheParsers(t *testing.T) {
	if !yamlstar.Available() {
		t.Skip("yamlstar is not installed; set YAMLSTAR or put yaml on PATH")
	}

	_, ok, msg, err := yamlstar.Events(context.Background(), []byte("[a"))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, msg, "Parser finished before end of input", "the reference parser, not the go_yaml plugin")
}

// TestAStreamComesBackOneValueADocument holds Load's reading of YAMLStar's one array for a stream.
func TestAStreamComesBackOneValueADocument(t *testing.T) {
	if !yamlstar.Available() {
		t.Skip("yamlstar is not installed; set YAMLSTAR or put yaml on PATH")
	}

	docs, msg, err := yamlstar.Load(context.Background(), []byte("--- a\n--- [b]\n"), 2)
	require.NoError(t, err)
	require.Empty(t, msg)
	assert.Equal(t, []string{`"a"`, `["b"]`}, docs)

	one, _, err := yamlstar.Load(context.Background(), []byte("[a, b]\n"), 1)
	require.NoError(t, err)
	assert.Equal(t, []string{`["a","b"]`}, one)
}
