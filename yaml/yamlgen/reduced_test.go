// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"strings"
)

func indent(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
