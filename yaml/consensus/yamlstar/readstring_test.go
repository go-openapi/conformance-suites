// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlstar

import "testing"

func TestReadStringDecodesAUnicodeEscape(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{
		{"a basic plane character", `"café" rest`, "café"},
		{"the highest code a four-digit escape writes", `"￿"`, "￿"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := readString(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("readString = %q, want %q", got, tc.want)
			}
		})
	}

	for _, src := range []string{`"\u12"`, `"\u12zz"`} {
		if _, _, err := readString(src); err == nil {
			t.Errorf("readString(%s) took a malformed escape", src)
		}
	}
}
