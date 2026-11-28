// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package goopenapi_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/goopenapi"
)

// TestOneProcessAnswersAsAProcessPerDocument checks goyaml-json -lines against goyaml-json run once per document.
func TestOneProcessAnswersAsAProcessPerDocument(t *testing.T) {
	if !goopenapi.Available() {
		t.Skip("goyaml-json is not installed: set GOYAML_JSON or put goyaml-json on PATH")
	}

	r := goopenapi.New(goopenapi.Command())
	defer r.Close()

	for _, src := range []string{
		"a: 1\n", "", "a: [1", "{a: 1}\n--- 2\n", "? [a]\n: b\n", "a: *x\n", "a: 1\na: 2\n", "x: 1.5e400\n",
	} {
		want, err := goopenapi.Read(context.Background(), []byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		got, err := r.Read(context.Background(), []byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v, want %+v", src, got, want)
		}
	}
}
