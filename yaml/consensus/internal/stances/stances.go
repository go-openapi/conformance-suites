// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package stances embeds the stance file of each parser the consensus report scores, one <id>.yaml per source.
//
// A stance file records the parser's positions on features of the corpus and the departures that explain its
// differences from the corpus. `consensus report` reads these unless given -stances DIR.
package stances

import "embed"

// Files holds every stance file, at the root.
//
//go:embed *.yaml
var Files embed.FS
