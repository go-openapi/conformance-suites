// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package consensus_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
)

// TestEqualValuesHaveOneSpelling holds CanonicalJSON to the spellings the sources write for one value.
func TestEqualValuesHaveOneSpelling(t *testing.T) {
	for want, spellings := range map[string][]string{
		`{"a":1,"b":[true,null]}`:        {`{"b": [true, null], "a": 1}`, `{"a":1.0,"b":[true,null]}`, `{"a":1e0,"b":[true,null]}`},
		`1000`:                           {`1000`, `1000.0`, `1e3`, `10E2`},
		`"é"`:                            {`"é"`, `"\u00e9"`},
		`123456789012345678901234567890`: {`123456789012345678901234567890`, `1.2345678901234567890123456789e29`},
		`-0.5`:                           {`-0.5`, `-5e-1`, `-0.50`},
		`0.125`:                          {`0.125`, `1.25e-1`, `125e-3`},
	} {
		for _, s := range spellings {
			got, err := consensus.CanonicalJSON(s)
			require.NoErrorf(t, err, "%s", s)
			assert.Equalf(t, want, got, "%s", s)
		}
	}
}

// TestTextThatIsNotJSONIsRefused holds that a value with no JSON form is told apart, as YAMLStar's "Infinity" is.
func TestTextThatIsNotJSONIsRefused(t *testing.T) {
	for _, s := range []string{`Infinity`, `NaN`, `{"a":1} x`, ``} {
		_, err := consensus.CanonicalJSON(s)
		assert.Errorf(t, err, "%q", s)
	}
}

// TestGroupsPartitionTheSourcesByAnswer holds that Groups elects nothing: it partitions, whatever the sizes.
func TestGroupsPartitionTheSourcesByAnswer(t *testing.T) {
	groups := consensus.Groups(map[string]string{"perlref": "accept", "libfyaml": "refuse", "yaml.v3": "accept"})
	assert.Equal(t, []consensus.Group{
		{Answer: "refuse", Sources: []string{"libfyaml"}},
		{Answer: "accept", Sources: []string{"perlref", "yaml.v3"}},
	}, groups)

	assert.Len(t, consensus.Groups(map[string]string{"a": "x", "b": "x"}), 1, "unanimous")
	assert.Empty(t, consensus.Groups(nil), "nobody answered")
}
