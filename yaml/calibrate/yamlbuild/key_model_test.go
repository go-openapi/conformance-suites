// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/consensus/goyaml"
	"github.com/go-openapi/go-yaml/codec"
)

func decodeInto(t *testing.T, src string, opts ...codec.DecodeOption) any {
	t.Helper()

	var v any
	require.NoError(t, codec.NewDecoder(bytes.NewReader([]byte(src)), opts...).Decode(&v), "%q", src)

	return v
}

// TestYAMLv3KeepsTheKeyType is the corroboration, pinned so that an upgrade
// changing its mind is visible rather than silently re-scoring the departures.
func TestYAMLv3KeepsTheKeyType(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want any
	}{
		{src: "true: a\n", want: map[any]any{true: "a"}},
		{src: "~: a\n", want: map[any]any{nil: "a"}},
		{src: "0.5: a\n", want: map[any]any{0.5: "a"}},
		{src: "1.0: a\n", want: map[any]any{float64(1): "a"}},
		{src: "1: a\n", want: map[any]any{1: "a"}},
	} {
		docs, err := goyaml.Load([]byte(tc.src))
		require.NoError(t, err, "%q", tc.src)
		require.Len(t, docs, 1)
		assert.Equal(t, tc.want, docs[0], "%q", tc.src)
	}

	t.Run("so a float and an integer of the same value are two keys", func(t *testing.T) {
		docs, err := goyaml.Load([]byte("1.0: a\n1: b\n"))
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Len(t, docs[0], 2, "yaml.v3 keeps both")

		// ✅ And so does this library: it named the float "1.0" and the
		// integer "1" from 2026-09-10, and holds them as float64(1) and
		// uint64(1) now.
		assert.Len(t, decodeInto(t, "1.0: a\n1: b\n"), 2)
	})
}
