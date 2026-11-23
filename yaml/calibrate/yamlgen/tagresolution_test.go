// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/codec"
)

// intText draws the spelling of a scalar that some schema might read as an integer.
//
// The parts are drawn separately rather than sampled whole, so the cross reaches
// combinations nobody wrote down: a "+" on a hex body, an underscore against a base
// prefix, a sexagesimal run with a leading zero. Those are where the two schemas stop
// agreeing.
func intText() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		sign := rapid.SampledFrom([]string{"", "", "-", "+"}).Draw(t, "sign")
		prefix := rapid.SampledFrom([]string{"", "", "", "0x", "0X", "0o", "0b", "0"}).Draw(t, "prefix")

		var digits string
		switch prefix {
		case "0x", "0X":
			digits = rapid.StringOfN(rapid.RuneFrom([]rune("0123456789abcdefABCDEF")), 1, 4, -1).Draw(t, "hex")
		case "0b":
			digits = rapid.StringOfN(rapid.RuneFrom([]rune("01")), 1, 6, -1).Draw(t, "bin")
		case "0o", "0":
			digits = rapid.StringOfN(rapid.RuneFrom([]rune("01234567")), 1, 4, -1).Draw(t, "oct")
		default:
			digits = rapid.StringOfN(rapid.RuneFrom([]rune("0123456789")), 1, 5, -1).Draw(t, "dec")
		}

		body := prefix + digits
		if rapid.Bool().Draw(t, "sexagesimal") && prefix == "" {
			// 1.1 reads "190:20:30" as base 60 and core reads it as a string.
			body += ":" + rapid.StringOfN(rapid.RuneFrom([]rune("012345")), 1, 2, -1).Draw(t, "sexa")
		}
		if rapid.Bool().Draw(t, "underscore") {
			body = insertUnderscore(t, body)
		}

		return sign + body
	})
}

// insertUnderscore puts a "_" somewhere in text, including the placements 1.1 refuses.
//
// 1.1 wants a digit before the first "_", so "_1_0" is a string at both versions while
// "1_0_" is ten under 1.1. Drawing the position rather than a well-formed one is the
// point: the refusals are as much of the answer as the acceptances.
func insertUnderscore(t *rapid.T, text string) string {
	if text == "" {
		return text
	}
	at := rapid.IntRange(0, len(text)).Draw(t, "underscoreat")

	return text[:at] + "_" + text[at:]
}

// atVersion writes doc under an explicit %YAML directive.
func atVersion(version, doc string) string {
	return "%YAML " + version + "\n---\n" + doc + "\n"
}

// resolution is what one scalar came to, in one position, at one version.
type resolution struct {
	// Refused says the document did not read at all.
	Refused bool
	// Value is what it resolved to, rendered so two readings can be compared
	// without caring which integer width carried them.
	Value string
}

func (r resolution) String() string {
	if r.Refused {
		return "refused"
	}

	return r.Value
}

// asValue reads the scalar as a mapping's value.
func asValue(version, scalar string) resolution {
	var got map[string]any
	if err := yaml.Unmarshal([]byte(atVersion(version, "v: "+scalar)), &got); err != nil {
		return resolution{Refused: true}
	}

	return resolution{Value: render(got["v"])}
}

// render writes a decoded scalar so two readings compare on the number rather than on the
// Go type that carried it.
func render(v any) string {
	switch n := v.(type) {
	case *big.Int:
		return "int:" + n.String()
	case big.Int:
		return "int:" + n.String()
	case int:
		return "int:" + strconv.Itoa(n)
	case int64:
		return "int:" + strconv.FormatInt(n, 10)
	case uint64:
		return "int:" + strconv.FormatUint(n, 10)
	default:
		return fmt.Sprintf("%T:%v", v, v)
	}
}

// isInt says the reading produced an integer.
func isInt(r resolution) bool {
	return !r.Refused && strings.HasPrefix(r.Value, "int:")
}

// isFloat says the reading produced a float.
func isFloat(r resolution) bool {
	return !r.Refused && strings.HasPrefix(r.Value, "float64:")
}

// numberKind pairs a tag with the spellings to draw for it and the reading it names.
type numberKind struct {
	tag  string
	draw func(*rapid.T) string
	is   func(resolution) bool
}

// numberKinds are the two numeric tags whose base the schema decides.
//
//nolint:gochecknoglobals // ok as an immutable table
var numberKinds = []numberKind{
	{tag: "!!int", draw: func(t *rapid.T) string { return intText().Draw(t, "int") }, is: isInt},
	{tag: "!!float", draw: func(t *rapid.T) string { return floatText().Draw(t, "float") }, is: isFloat},
}

// floatText draws the spelling of a scalar that some schema might read as a float.
//
// YAML 1.1 makes the "." mandatory and the exponent's sign mandatory with it, so "1e3"
// and "1.0e3" are both strings there while "1.0e+3" is a number. YAML 1.2 core wants
// neither. Drawing the point and the sign separately crosses that line in both
// directions.
//
// "0x1p-2" is drawn because it is Go's hex float literal and no YAML schema's. It read
// 0.25 at both versions until 7e20582, through strconv.ParseFloat, which is the defect
// this axis was written to catch.
func floatText() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		if rapid.Bool().Draw(t, "special") {
			return rapid.SampledFrom([]string{
				".inf", "-.inf", ".nan", ".Inf", ".NaN", "0x1p-2", "0x1P+3",
			}).Draw(t, "specialtext")
		}

		sign := rapid.SampledFrom([]string{"", "", "-", "+"}).Draw(t, "fsign")
		whole := rapid.StringOfN(rapid.RuneFrom([]rune("0123456789")), 1, 3, -1).Draw(t, "whole")

		text := sign + whole
		if rapid.Bool().Draw(t, "point") {
			text += "." + rapid.StringOfN(rapid.RuneFrom([]rune("0123456789")), 0, 3, -1).Draw(t, "frac")
		}
		if rapid.Bool().Draw(t, "exponent") {
			text += rapid.SampledFrom([]string{"e", "E"}).Draw(t, "e") +
				rapid.SampledFrom([]string{"", "+", "-"}).Draw(t, "esign") +
				rapid.StringOfN(rapid.RuneFrom([]rune("0123456789")), 1, 2, -1).Draw(t, "exp")
		}

		return text
	})
}

// versions are the two schemas a document can declare.
//
// "1.2" is the control: it declares what the corpus already assumes, so a scalar under it
// must resolve exactly as the same scalar resolves with no directive at all.
var versions = []string{"1.1", "1.2"}

// asJSON reads the scalar in a value position through codec.ToJSON.
//
// ToJSON walks the tree and writes the scalar itself, so it is a third reader beside the
// decoder and the tree. It carried its own copy of the 1.2 re-sniff in codec.taggedInteger
// and codec.taggedFloat until 7e20582, which is how it came to write 0 for a document the
// decoder read as 5.
func asJSON(version, scalar string) resolution {
	out, err := codec.ToJSON([]byte(atVersion(version, "v: "+scalar)))
	if err != nil {
		return resolution{Refused: true}
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		return resolution{Refused: true}
	}

	return resolution{Value: renderJSON(got["v"])}
}

// renderJSON writes what encoding/json built, which is a float64 for every number.
//
// The decoder answers with Go's integer types and ToJSON's number comes back through
// encoding/json as a float64, so the two cannot be compared as Go values. Comparing the
// number is the point, and a float64 carries every integer these documents spell.
func renderJSON(v any) string {
	if n, ok := v.(float64); ok {
		// A signed zero survives the round to an integer here. int64(-0.0) is 0,
		// and formatting that reported ToJSON as writing 0 for a document where it
		// wrote -0.0 -- the two readers were made to disagree by the harness, and
		// the ledger carried the direction backwards for a day because of it.
		// numberOf spells a zero the same way, so the two sides stay comparable.
		if n == 0 {
			if math.Signbit(n) {
				return "num:-0"
			}

			return "num:0"
		}
		if n == math.Trunc(n) && math.Abs(n) < 1<<53 {
			return "num:" + strconv.FormatInt(int64(n), 10)
		}

		return "num:" + strconv.FormatFloat(n, 'g', -1, 64)
	}

	return fmt.Sprintf("%T:%v", v, v)
}

// numberOf reduces a decoder reading to the same shape renderJSON writes, so the two
// readers are compared on the number and not on the Go type that carried it.
func numberOf(r resolution) (string, bool) {
	switch {
	case r.Refused:
		return "", false
	case strings.HasPrefix(r.Value, "int:"):
		return "num:" + strings.TrimPrefix(r.Value, "int:"), true
	case strings.HasPrefix(r.Value, "*big.Float:"), strings.HasPrefix(r.Value, "float64:"):
		_, digits, _ := strings.Cut(r.Value, ":")
		f, err := strconv.ParseFloat(digits, 64)
		if err != nil {
			return "", false
		}
		if f == 0 {
			// A signed zero is a value this library keeps deliberately -- see the
			// negative-zero fix in codec -- so the two zeros are not folded here.
			if math.Signbit(f) {
				return "num:-0", true
			}

			return "num:0", true
		}
		if f == math.Trunc(f) && math.Abs(f) < 1<<53 {
			return "num:" + strconv.FormatInt(int64(f), 10), true
		}

		return "num:" + strconv.FormatFloat(f, 'g', -1, 64), true
	default:
		return "", false
	}
}

// TestToJSONAgreesWithTheDecoderOnATaggedNumber holds the two readers together.
//
// A tagged scalar is resolved once and read by three paths: the decoder, the tree and
// codec.ToJSON. Each carried its own copy of the base-and-schema decision, so they could
// disagree about one document without any of them looking wrong on its own -- under
// %YAML 1.1 "!!int 0b101" decoded to 5 while ToJSON wrote 0, and neither number is
// obviously the mistaken one until they are put side by side.
//
// ast.Resolution.Type is the single answer now, so this compares the readers rather than
// pinning what either of them says. Nothing here states a value: the property is that the
// two agree, which survives every ruling still to come about what a tag may spell.
//
// Only a number is compared. JSON has one number type, so a document the decoder refuses
// and ToJSON refuses agree by refusing, and a scalar that reads as a string on either side
// is left alone -- ToJSON's quoting rules are not this axis's question.
func TestToJSONAgreesWithTheDecoderOnATaggedNumber(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom(numberKinds).Draw(rt, "kind")
		text := kind.tag + " " + kind.draw(rt)
		version := rapid.SampledFrom(versions).Draw(rt, "version")

		decoded, written := asValue(version, text), asJSON(version, text)

		if notJSON(decoded) {
			// JSON has no spelling for an infinity or a NaN, so ToJSON refuses a
			// document the decoder reads perfectly well. That is ErrNotJSON doing
			// its job and not a disagreement about the characters.
			return
		}

		if decoded.Refused != written.Refused {
			rt.Fatalf("%%YAML %s: %q is %s to the decoder and %s to ToJSON;"+
				" one document, two answers about whether it reads",
				version, text, decoded, written)
		}

		want, isNumber := numberOf(decoded)
		if !isNumber {
			return
		}
		if want != written.Value {
			rt.Fatalf("%%YAML %s: %q decodes to %s and ToJSON writes %s;"+
				" the two readers resolved the same characters differently",
				version, text, want, written.Value)
		}
	})
}

// notJSON says the decoder built a number JSON cannot write.
//
// ".inf", "-.inf" and ".nan" are floats YAML resolves and JSON has no spelling for, so
// codec.ToJSON refuses them by design. Comparing the two readers on such a document would
// measure that rule instead of the resolution the axis is about.
func notJSON(r resolution) bool {
	if !isFloat(r) {
		return false
	}
	f, err := strconv.ParseFloat(strings.TrimPrefix(r.Value, "float64:"), 64)

	return err != nil || math.IsInf(f, 0) || math.IsNaN(f)
}
