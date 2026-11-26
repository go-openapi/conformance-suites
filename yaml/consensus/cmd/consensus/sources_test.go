// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

func TestYAMLv3StageTellsTheDecoderFromTheParser(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want consensus.Stage
	}{
		{"a: [1", consensus.Parser},
		{"%YAML 1.2\n---\na: 1\n", consensus.Parser},
		{"a: *nowhere\n", consensus.Composer},
		{"a: !!bool maybe\n", consensus.Loader},
		{"a: !!timestamp '1913-09-11 13:00:Z'\n", consensus.Loader},
		{"a: !!binary '1w='\n", consensus.Loader},
		{"<<: 1\n", consensus.Loader},
		{"? [1]\n: 2\n", consensus.Loader},
		{"a: 1\na: 2\n", consensus.Loader},
	} {
		a := yamlv3Answer(t.Context(), []byte(tc.src))
		if a.Verdict != consensus.Refuse {
			t.Errorf("%q: yaml.v3 accepts it", tc.src)
			continue
		}
		if a.RefusedBy != tc.want {
			t.Errorf("%q: refused by the %s (%s), want the %s", tc.src, a.RefusedBy, a.Message, tc.want)
		}
	}
}
