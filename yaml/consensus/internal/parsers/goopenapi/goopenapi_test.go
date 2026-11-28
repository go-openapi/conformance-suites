// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package goopenapi_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/goopenapi"
)

// TestTwoLocalBuildsHaveTwoVersions holds Version to the program's code for a build from a local checkout, whose
// version line is the same for every checkout. Two builds sharing a version shared the runner's cache.
func TestTwoLocalBuildsHaveTwoVersions(t *testing.T) {
	versionOf := func(program, line string) string {
		path := filepath.Join(t.TempDir(), "goyaml-json")
		require.NoError(t, os.WriteFile(path, []byte(program), 0o600))
		require.NoError(t, os.WriteFile(path+".version", []byte(line+"\n"), 0o600))
		t.Setenv("GOYAML_JSON", path)

		return goopenapi.Version()
	}

	const local = "go-openapi/go-yaml: local checkout of unknown revision"
	first, second := versionOf("one build", local), versionOf("another build", local)
	assert.NotEqual(t, first, second)
	assert.Contains(t, first, local+" (build ")
	assert.Equal(t, first, versionOf("one build", local), "the same code gives the same version")

	const pinned = "go-openapi/go-yaml: v0.0.0-20261123092944-569e0d675ee1"
	assert.Equal(t, pinned, versionOf("one build", pinned), "a pinned version names the code already")
}
