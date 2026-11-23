// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild_test

import (
	"testing"

	"github.com/go-openapi/conformance-suites/suite"

	yamlembed "github.com/go-openapi/conformance-suites/yaml/corpus"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
)

func storedCases(t *testing.T) (suite.Header, []suite.Case) {
	t.Helper()

	header, cases, err := yamlembed.CompactSuite()
	if err != nil {
		t.Fatalf("reading the stored corpus: %v", err)
	}

	return header, cases
}

// TestEachReadingGetsItsOwnAnswer is the case the field exists for.
func TestEachReadingGetsItsOwnAnswer(t *testing.T) {
	want := map[string][2]string{
		"shape/schema/0777":  {`{"k":777}`, `{"k":511}`},
		"shape/schema/1_000": {`{"k":"1_000"}`, `{"k":1000}`},
		"shape/schema/1:30":  {`{"k":"1:30"}`, `{"k":90}`},
		"shape/schema/yes":   {`{"k":"yes"}`, `{"k":true}`},
		"shape/schema/1e3":   {`{"k":1000}`, `{"k":"1e3"}`},
		"shape/schema/0o17":  {`{"k":15}`, `{"k":"0o17"}`},
	}

	found := 0

	header, cases := storedCases(t)

	for _, c := range cases {
		answers, named := want[c.Name]
		if !named {
			continue
		}

		found++

		core, ok := header.MeaningFor(c, yamlgen.ReadingCore)
		if !ok || string(core.JSON) != answers[0] {
			t.Errorf("%s under the core schema: %q, want %q", c.Name, core.JSON, answers[0])
		}

		legacy, ok := header.MeaningFor(c, yamlgen.Reading11)
		if !ok || string(legacy.JSON) != answers[1] {
			t.Errorf("%s under YAML 1.1: %q, want %q", c.Name, legacy.JSON, answers[1])
		}
	}

	if found != len(want) {
		t.Errorf("found %d of the %d named scalars in the stored corpus", found, len(want))
	}
}
