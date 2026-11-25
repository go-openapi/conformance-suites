// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"math"
	"testing"
	"time"
)

func TestSameValueComparesTypesAndContents(t *testing.T) {
	at := time.Date(2001, 12, 14, 21, 59, 43, 0, time.FixedZone("", -5*3600))
	for _, tc := range []struct {
		name string
		a, b any
		want bool
	}{
		{"equal maps", map[string]any{"a": 1, "b": []any{"x"}}, map[string]any{"a": 1, "b": []any{"x"}}, true},
		{"an int and a float", 1, 1.0, false},
		{"a NaN and a NaN", math.NaN(), math.NaN(), true},
		{"a string and a nil", "", nil, false},
		{"two nils", nil, nil, true},
		{"one more entry", map[string]any{"a": 1}, map[string]any{"a": 1, "b": 2}, false},
		{"a key of another type", map[any]any{1: "a"}, map[any]any{"1": "a"}, false},
		{"the same instant in two locations", at, at.In(time.FixedZone("EST", -5*3600)), true},
		{"the same instant in another offset", at, at.UTC(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameValue(tc.a, tc.b); got != tc.want {
				t.Errorf("SameValue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasCycleFindsAValueInsideItself(t *testing.T) {
	m := map[string]any{}
	m["self"] = m
	s := []any{nil}
	s[0] = s
	shared := []any{1}

	for _, tc := range []struct {
		name string
		v    any
		want bool
	}{
		{"a map holding itself", m, true},
		{"a slice holding itself", s, true},
		{"one value twice, side by side", []any{shared, shared}, false},
		{"a plain value", map[string]any{"a": []any{1, 2}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasCycle(tc.v); got != tc.want {
				t.Errorf("HasCycle = %v, want %v", got, tc.want)
			}
		})
	}
}
