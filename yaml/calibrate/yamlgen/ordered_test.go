// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/go-yaml/codec"
)

// toCodec converts the generator's own value types to the codec types they replace.
func toCodec(v any) any {
	switch n := v.(type) {
	case yamlgen.Base64:
		return codec.Base64(n)
	case yamlgen.OrderedMap:
		var items []codec.MapItem
		for key, value := range n.All() {
			items = append(items, codec.MapItem{Key: toCodec(key), Value: toCodec(value)})
		}
		out, err := codec.NewMapSliceSeq(items...)
		if err != nil {
			panic(err)
		}

		return out
	case []any:
		out := make([]any, 0, len(n))
		for _, item := range n {
			out = append(out, toCodec(item))
		}

		return out
	case map[string]any:
		out := make(map[string]any, len(n))
		for key, value := range n {
			out[key] = toCodec(value)
		}

		return out
	case map[any]any:
		out := make(map[any]any, len(n))
		for key, value := range n {
			out[toCodec(key)] = toCodec(value)
		}

		return out
	}

	return v
}
