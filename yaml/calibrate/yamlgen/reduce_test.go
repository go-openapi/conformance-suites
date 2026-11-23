// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// interestingHere stands in for a defect while none is outstanding.
//
// The reducer and its report do not care what makes a document interesting,
// only that something does -- and wiring them to whichever defect happens to be
// open would leave them untested the moment it is fixed, which is exactly when
// the next one needs them.
func interestingHere(src []byte) bool {
	return bytes.Contains(src, []byte("BANG"))
}

// TestReducedReportIsUseful exercises the whole failure path, because the
// report is only ever produced when something has already gone wrong -- which
// is precisely when nobody wants to discover that the reporting itself is
// broken.
func TestReducedReportIsUseful(t *testing.T) {
	report := reduced("Defect", "a: 1\nb: BANG\nc: 3\n", interestingHere)

	assert.Contains(t, report, "as generated")
	assert.Contains(t, report, "reduced to")
	assert.Contains(t, report, "renders to:")
	assert.Contains(t, report, "reads as:")
	assert.Contains(t, report, "func TestDefect(t *testing.T) {")
	t.Log(report)
}

// TestAValueChangedBeforeRenderingIsNotReportedAsRendering holds the document
// apart from the message that reports it.
//
// TestRenderPreservesValue compares Written.Means against a reading of the
// rendered text, so it fails when either the writing or the rendering moved the
// value. It reported both through reduced("RenderChangedTheValue", ...), whose
// predicate reads the document and reads the rendering of it -- a different
// comparison, and one that holds only for the rendering half.
//
// On the document below the predicate is false: the rendering preserves it
// exactly, and the value moved when the document was written. So the report named the
// renderer, printed "reads as" and "then as" byte-identical because both come
// after the step that moved the value, and emitted a reproducer asserting the
// value its own "// today:" line gave. yaml-transform read it on 2026-09-10 and
// could not tell what differed, which is the whole complaint.
//
// The document is the one the failfile held, kept verbatim. Reduction cannot
// shrink it -- Reduce keeps its predicate true, and this predicate is false at
// every step.
func TestAValueChangedBeforeRenderingIsNotReportedAsRendering(t *testing.T) {
	const src = "\ufeff&a3\n- &a2\n ?\tnull\n : &a1 !!binary\t\"AA==\"\n ?\t*a1\n :\t\"aliased\"\n"

	assert.False(t, renderChangesValue([]byte(src)),
		"rendering preserves this document, so a message naming the renderer names the wrong stage")

	// The "!!binary" value reads as a codec.Base64, the text the document
	// carries, since 752f09c. It was []byte{0} before that, which Go hashes
	// nowhere and which nothing marked as binary. The alias names it as a key
	// too, and the null key beside it widens the mapping to a map[any]any.
	var got any
	require.NoError(t, codec.Unmarshal([]byte(src), &got))
	assert.Equal(t, []any{map[any]any{nil: codec.Base64("AA=="), codec.Base64("AA=="): "aliased"}}, got)
}
