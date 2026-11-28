// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package libfyaml_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/libfyaml"
)

// TestOneProcessAnswersAsAProcessPerDocument checks the batch script against a Python process per document.
//
// The public package's error wraps the whole traceback; the driver keeps its last line, so that is what is compared.
func TestOneProcessAnswersAsAProcessPerDocument(t *testing.T) {
	if !libfyaml.Available() {
		t.Skip("libfyaml's binding is not installed: run hack/conformance/install-libfyaml.sh")
	}

	l := libfyaml.New(libfyaml.Home())
	defer l.Close()

	for _, src := range []string{"a: 1\n", "", "a: [1", "{a: 1}\n--- 2\n", "? [a]\n: b\n"} {
		wantDocs, wantErr := libfyaml.Load([]byte(src))
		docs, err := l.Load(context.Background(), []byte(src), "1.2")
		switch {
		case (err == nil) != (wantErr == nil):
			t.Errorf("%q: got error %v, want %v", src, err, wantErr)
		case err != nil && lastLine(wantErr.Error()) != err.Error():
			t.Errorf("%q: got %q, want the last line of %q", src, err, wantErr)
		case !slices.Equal(docs, wantDocs):
			t.Errorf("%q: got %q, want %q", src, docs, wantDocs)
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")

	return strings.TrimSpace(lines[len(lines)-1])
}
