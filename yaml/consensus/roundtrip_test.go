// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"math"
	"math/big"
	"strings"
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

func TestSameValueComparesBigNumbersByValue(t *testing.T) {
	wide := func() *big.Int { n, _ := new(big.Int).SetString("89156680234109371544", 10); return n }
	huge := func(prec uint) *big.Float {
		f, _, _ := big.ParseFloat("2.0e+717", 10, prec, big.ToNearestEven)
		return f
	}

	for _, tc := range []struct {
		name string
		a, b any
		want bool
	}{
		{"two equal big ints", wide(), wide(), true},
		{"two big floats", huge(64), huge(64), true},
		{"one value at two precisions", new(big.Float).SetPrec(64).SetInt64(3), new(big.Float).SetPrec(128).SetInt64(3), true},
		{"a big int as a key", map[any]any{wide(): "a"}, map[any]any{wide(): "a"}, true},
		{"a big int key with another value", map[any]any{wide(): "a"}, map[any]any{wide(): "b"}, false},
		{"a big int and a big float", wide(), huge(64), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameValue(tc.a, tc.b); got != tc.want {
				t.Errorf("SameValue = %v, want %v", got, tc.want)
			}
		})
	}
}

// base64Text stands in for go-yaml's codec.Base64: a text whose blanks and line breaks carry nothing.
type base64Text string

func (b base64Text) Canonical() string { return strings.Join(strings.Fields(string(b)), "") }

func TestSameValueComparesACanonicalTextByItsCanonicalForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b any
		want bool
	}{
		{"a folded break and none", base64Text("Iwc= "), base64Text("Iwc="), true},
		{"two lines and one", base64Text("SGVs\nbG8=\n"), base64Text("SGVsbG8="), true},
		{"other bytes", base64Text("SGVs"), base64Text("bG8="), false},
		{"a canonical text and a string", base64Text("Iwc="), "Iwc=", false},
		{"in a map", map[string]any{"b": base64Text("aGk=\n")}, map[string]any{"b": base64Text("aGk=")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SameValue(tc.a, tc.b); got != tc.want {
				t.Errorf("SameValue = %v, want %v", got, tc.want)
			}
		})
	}
}
