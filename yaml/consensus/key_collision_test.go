// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package consensus_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/consensus/goyaml"
	"github.com/go-openapi/conformance-suites/yaml/consensus/libfyaml"
)

// TestTheOtherSourcesOnACollidingKey is the corroboration, and it is split.
//
// Pinned so that an upgrade changing either one is visible, and pinned as a
// disagreement rather than as support: only libfyaml holds both keys.
func TestTheOtherSourcesOnACollidingKey(t *testing.T) {
	const src = "1: a\n\"1\": b\n"

	// yaml.v3 refuses it, so it names keys the way this library does and
	// merges the two the way this library used to. It keeps int and float
	// apart -- see TestFixedTwoTypesAreTwoKeys -- and not int and string, so
	// its rule is the rendered name rather than the type.
	_, err := goyaml.Load([]byte(src))
	require.Error(t, err, "yaml.v3 refuses it")
	assert.Contains(t, err.Error(), `mapping key "1" already defined`)

	if !libfyaml.Available() {
		t.Skipf("libfyaml is not installed at %s", libfyaml.Home())
	}

	// Only libfyaml holds both, so the reading here rests on 3.2.1.1 rather
	// than on a majority.
	theirs, err := libfyaml.Load([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, []string{`{"1": "a", "1": "b"}`}, theirs,
		"libfyaml keeps both nodes and renders a repeated member name")
}
