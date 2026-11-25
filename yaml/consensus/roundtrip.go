// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"math"
	"math/big"
	"reflect"
	"time"
)

// RoundTripResult says what came back when a source wrote the value it read as YAML and read that YAML again.
type RoundTripResult string

const (
	// Same says every document came back as the value first read.
	Same RoundTripResult = "same"
	// Different says a document came back as another value.
	Different RoundTripResult = "different"
	// Unwritable says the source's encoder failed on a value its decoder built.
	Unwritable RoundTripResult = "unwritable"
	// Unreadable says the source's decoder rejected what its encoder wrote.
	Unreadable RoundTripResult = "unreadable"
	// Cyclic says a value holds a cycle, which the runner does not hand to an encoder: an encoder that follows a
	// cycle never returns.
	Cyclic RoundTripResult = "cyclic"
)

// RoundTrip is the check of decode(encode(decode(document))) against decode(document), document by document, in
// the source's own types: a value that turns from an int into a float differs even where JSON would write both the
// same way.
type RoundTrip struct {
	Result RoundTripResult `json:"result"`
	// Doc is the index of the first document that did not come back the same, from zero.
	Doc int `json:"doc,omitempty"`
	// YAML is what the encoder wrote for that document, cut to [MaxRoundTripYAML] bytes.
	YAML string `json:"yaml,omitempty"`
	// Again is the second reading of that document, as JSON where it has a JSON form, else as Go syntax.
	Again string `json:"again,omitempty"`
	// Message is the encoder's or the decoder's error.
	Message string `json:"message,omitempty"`
}

// MaxRoundTripYAML caps the encoder output a [RoundTrip] keeps.
const MaxRoundTripYAML = 2048

// SameValue says whether two decoded values are equal: the same types and the same contents, with a NaN equal to a
// NaN, which reflect.DeepEqual denies.
func SameValue(a, b any) bool {
	return sameValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

func sameValue(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	if same, ok := sameCanonical(a, b); ok {
		return same
	}

	switch a.Kind() {
	case reflect.Interface, reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		if same, ok := sameNumber(a, b); ok {
			return same
		}

		return sameValue(a.Elem(), b.Elem())
	case reflect.Float32, reflect.Float64:
		x, y := a.Float(), b.Float()

		return x == y || (math.IsNaN(x) && math.IsNaN(y))
	case reflect.Slice, reflect.Array:
		return sameElements(a, b)
	case reflect.Map:
		return sameEntries(a, b)
	case reflect.Struct:
		return sameFields(a, b)
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
}

func sameElements(a, b reflect.Value) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := range a.Len() {
		if !sameValue(a.Index(i), b.Index(i)) {
			return false
		}
	}

	return true
}

// sameEntries matches each entry of a with an entry of b. A key found by MapIndex is the same key; a key MapIndex
// misses, such as a *big.Int that is equal in value and is another pointer, is matched with sameValue.
func sameEntries(a, b reflect.Value) bool {
	if a.Len() != b.Len() {
		return false
	}
	var missed []reflect.Value
	iter := a.MapRange()
	for iter.Next() {
		other := b.MapIndex(iter.Key())
		if !other.IsValid() {
			missed = append(missed, iter.Key())

			continue
		}
		if !sameValue(iter.Value(), other) {
			return false
		}
	}

	return matchKeys(a, b, missed)
}

// matchKeys pairs each key of a that b's index missed with a key of b equal in value and not paired already.
func matchKeys(a, b reflect.Value, missed []reflect.Value) bool {
	used := map[int]bool{}
	keys := b.MapKeys()
	for _, k := range missed {
		found := false
		for i, other := range keys {
			if used[i] || a.MapIndex(other).IsValid() || !sameValue(k, other) {
				continue
			}
			if !sameValue(a.MapIndex(k), b.MapIndex(other)) {
				return false
			}
			used[i], found = true, true

			break
		}
		if !found {
			return false
		}
	}

	return true
}

// canonical is a value with a canonical text: go-yaml's codec.Base64 holds the text a "!!binary" wrote, and
// Canonical drops the blanks and line breaks base64 tolerates.
type canonical interface{ Canonical() string }

// sameCanonical compares two values of one type by their canonical text, when the type has one: a "!!binary" written
// on two lines and read back from one holds the same bytes. ok is false for a type with no Canonical method.
func sameCanonical(a, b reflect.Value) (same, ok bool) {
	if !a.CanInterface() || !b.CanInterface() {
		return false, false
	}
	x, isCanonical := reflect.TypeAssert[canonical](a)
	if !isCanonical {
		return false, false
	}
	y, _ := reflect.TypeAssert[canonical](b)

	return x.Canonical() == y.Canonical(), true
}

// sameNumber compares a *big.Int or a *big.Float by value, which a field-by-field walk cannot: two equal big.Float
// values may differ in precision and accuracy. ok is false for any other type.
func sameNumber(a, b reflect.Value) (same, ok bool) {
	switch x := a.Interface().(type) {
	case *big.Int:
		y, isInt := reflect.TypeAssert[*big.Int](b)

		return isInt && x.Cmp(y) == 0, true
	case *big.Float:
		y, isFloat := reflect.TypeAssert[*big.Float](b)

		return isFloat && x.Cmp(y) == 0, true
	default:
		return false, false
	}
}

// sameFields compares two structs field by field. A time.Time is the same instant in the same zone offset, because
// its fields are unexported and its location pointer differs between two readings of one timestamp. Any other struct
// with unexported fields is compared whole with reflect.DeepEqual.
func sameFields(a, b reflect.Value) bool {
	if a.Type() == reflect.TypeFor[time.Time]() && a.CanInterface() && b.CanInterface() {
		x, xok := reflect.TypeAssert[time.Time](a)
		y, yok := reflect.TypeAssert[time.Time](b)
		if !xok || !yok {
			return false
		}
		_, xo := x.Zone()
		_, yo := y.Zone()

		return x.Equal(y) && xo == yo
	}
	for i := range a.NumField() {
		if !a.Type().Field(i).IsExported() {
			return a.CanInterface() && b.CanInterface() && reflect.DeepEqual(a.Interface(), b.Interface())
		}
		if !sameValue(a.Field(i), b.Field(i)) {
			return false
		}
	}

	return true
}

// HasCycle says whether a decoded value holds a map or a slice inside itself.
func HasCycle(v any) bool {
	return hasCycle(reflect.ValueOf(v), map[uintptr]bool{})
}

func hasCycle(v reflect.Value, onPath map[uintptr]bool) bool {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return false
		}
		if v.Kind() == reflect.Pointer {
			p := v.Pointer()
			if onPath[p] {
				return true
			}
			onPath[p] = true
			defer delete(onPath, p)
		}

		return hasCycle(v.Elem(), onPath)
	case reflect.Map, reflect.Slice:
		if v.IsNil() || (v.Kind() == reflect.Slice && v.Len() == 0) {
			return false
		}
		p := v.Pointer()
		if onPath[p] {
			return true
		}
		onPath[p] = true
		defer delete(onPath, p)

		return containsCycle(v, onPath)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() && hasCycle(v.Field(i), onPath) {
				return true
			}
		}

		return false
	default:
		return false
	}
}

// containsCycle walks the entries of a map or the elements of a slice already on the path.
func containsCycle(v reflect.Value, onPath map[uintptr]bool) bool {
	if v.Kind() == reflect.Map {
		iter := v.MapRange()
		for iter.Next() {
			if hasCycle(iter.Key(), onPath) || hasCycle(iter.Value(), onPath) {
				return true
			}
		}

		return false
	}
	for i := range v.Len() {
		if hasCycle(v.Index(i), onPath) {
			return true
		}
	}

	return false
}
