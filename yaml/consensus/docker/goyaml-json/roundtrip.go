// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

// This file copies SameValue and HasCycle from github.com/go-openapi/conformance-suites/yaml/consensus, which this
// module does not require: it is built with go-openapi/go-yaml alone. Keep the two copies the same.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"time"

	"github.com/go-openapi/go-yaml/codec"
)

// roundTrip is consensus.RoundTrip, in the same JSON form.
type roundTrip struct {
	Result  string `json:"result"`
	Doc     int    `json:"doc,omitempty"`
	YAML    string `json:"yaml,omitempty"`
	Again   string `json:"again,omitempty"`
	Message string `json:"message,omitempty"`
}

const maxRoundTripYAML = 2048

// checkRoundTrip decodes each document of src into any with codec.NewDecoder, writes it back with codec.Marshal,
// reads that with codec.Unmarshal, and compares the two values. It returns nil when the first reading fails.
func checkRoundTrip(src []byte) *roundTrip {
	dec := codec.NewDecoder(bytes.NewReader(src))
	for i := 0; ; i++ {
		var v any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return &roundTrip{Result: "same"}
		}
		if err != nil {
			return nil
		}
		if holdsCycle(v) {
			return &roundTrip{Result: "cyclic", Doc: i}
		}
		out, err := marshal(v)
		if err != nil {
			return &roundTrip{Result: "unwritable", Doc: i, Message: firstLine(err)}
		}
		var again any
		if err := codec.Unmarshal(out, &again); err != nil {
			return &roundTrip{Result: "unreadable", Doc: i, YAML: cut(out), Message: firstLine(err)}
		}
		if !sameDecoded(v, again) {
			return &roundTrip{Result: "different", Doc: i, YAML: cut(out), Again: render(again)}
		}
	}
}

// marshal is codec.Marshal with a panic turned into an error.
func marshal(v any) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("codec.Marshal panicked: %v", r)
		}
	}()

	return codec.Marshal(v)
}

func cut(out []byte) string {
	if len(out) > maxRoundTripYAML {
		return string(out[:maxRoundTripYAML]) + "\n..."
	}

	return string(out)
}

func render(v any) string {
	if encoded, err := json.Marshal(v); err == nil {
		return string(encoded)
	}

	return fmt.Sprintf("%#v", v)
}

// sameDecoded says whether two decoded values are equal: the same types and the same contents, with a NaN equal to a
// NaN, which reflect.DeepEqual denies.
func sameDecoded(a, b any) bool {
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

// holdsCycle says whether a decoded value holds a map or a slice inside itself.
func holdsCycle(v any) bool {
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
