// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlembed_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/conformance-suites/suite"
	yamlembed "github.com/go-openapi/conformance-suites/yaml/corpus"
)

func TestBytesIsTheFileSmokeSuiteReads(t *testing.T) {
	header, cases, err := yamlembed.SmokeSuite()
	require.NoError(t, err)

	fromBytes, casesFromBytes, err := suite.FromBytes(yamlembed.Bytes())
	require.NoError(t, err)

	require.Equal(t, header, fromBytes)
	require.Len(t, casesFromBytes, len(cases))
}
