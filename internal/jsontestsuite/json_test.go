// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsontestsuite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"
)

// TestTheVendoredSuiteIsWhole counts the fixtures by expectation, as upstream revision 1ef36fa holds them.
func TestTheVendoredSuiteIsWhole(t *testing.T) {
	t.Parallel()

	cases, err := Cases()
	require.NoError(t, err)

	counts := map[Expect]int{}
	for _, c := range cases {
		counts[c.Expect]++
	}

	assert.Equal(t, map[Expect]int{Accept: 95, Reject: 188, Either: 35}, counts)
}

func TestCasesInAMissingDirectoryReturnsTheError(t *testing.T) {
	t.Parallel()

	cases, err := casesIn(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	require.Nil(t, cases)
}

func TestAFixtureNamedOutsideTheConventionIsAnError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x_odd.json"), []byte("1"), 0o600))

	_, err := casesIn(dir)
	require.Error(t, err)
}
