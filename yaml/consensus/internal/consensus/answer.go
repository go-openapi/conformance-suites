// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package consensus holds the answer every YAML implementation the runner consults gives for one document, and the
// comparisons the report makes between those answers.
//
// The sources are not votes. The report observes how different implementations take different routes through the
// same document, so this package never elects a winning answer: [Groups] partitions the sources by the answer they
// give, and each source is scored against the corpus alone.
//
// Each source lives in a package of its own -- perlref, libfyaml, yamlstar, goyaml for go.yaml.in/yaml/v3 -- and
// goopenapi runs go-openapi/go-yaml as a separate program, so that this module does not require it.
package consensus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"
)

// Verdict is what a source made of a document.
type Verdict string

const (
	// Accept says the source read the document.
	Accept Verdict = "accept"
	// Refuse says the source refused the document. See [Answer.RefusedBy] for which stage refused it.
	Refuse Verdict = "refuse"
	// Error says the source crashed, timed out, or wrote output its adapter cannot read. It says nothing about the
	// document.
	Error Verdict = "error"
	// NotImplemented says the source cannot take this input at all.
	NotImplemented Verdict = "n/i"
)

// Stage names the step of a source that refused a document.
type Stage string

const (
	// Parser is a refusal by the parser: the document is not YAML, by this source's reading.
	Parser Stage = "parser"
	// Composer is a refusal by the step that resolves the representation after a clean parse: an alias naming no
	// anchor, a key written twice. A document the corpus claims for parsing only may be refused here.
	Composer Stage = "composer"
	// Loader is a refusal by the step that builds a value, after a clean parse: a key Python cannot hash, a boolean
	// built from an empty node, a tag the loader has no constructor for. It is not a syntax verdict.
	Loader Stage = "loader"
)

// Answer is one source's answer for one document.
type Answer struct {
	// Source is the source's id: "perlref", "libfyaml", "yamlstar", "yaml.v3", "go-yaml".
	Source string `json:"source"`
	// Verdict is the source's verdict on the document.
	Verdict Verdict `json:"verdict"`
	// RefusedBy is the stage that refused, when Verdict is [Refuse] and the source tells the stages apart.
	RefusedBy Stage `json:"refusedBy,omitempty"`
	// Docs is the number of documents the source read, when it says.
	Docs int `json:"docs,omitempty"`
	// Events is the event stream in yaml-test-suite test.event form, one event per line, absent when the source has
	// no event output.
	Events []string `json:"events,omitempty"`
	// JSON holds the value of each document as canonical JSON (see [CanonicalJSON]), absent when the source builds
	// no value. A document whose value has no JSON form is the empty string.
	JSON []string `json:"json,omitempty"`
	// RoundTrip is the check of decode(encode(decode(document))) against decode(document), absent when the source
	// has no encoder the runner drives, or did not accept the document.
	RoundTrip *RoundTrip `json:"roundTrip,omitempty"`
	// Message is what the source said when it refused or failed, trimmed to its first line.
	Message string `json:"message,omitempty"`
	// ElapsedMS is how long the source took, in milliseconds to the microsecond. A source run as a program counts
	// its start-up, and one run in the runner's process does not.
	ElapsedMS float64 `json:"elapsedMs"`
}

// FirstLine trims a message to the line a report shows.
func FirstLine(msg string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(msg), "\n")

	return strings.TrimSpace(first)
}

// CanonicalJSON rewrites a JSON value so that two equal values have one spelling: no insignificant whitespace,
// object members sorted by name, and every number written as its exact value in decimal.
//
// Numbers are compared by value because the sources spell them differently: 1000, 1000.0 and 1e3 are one number,
// written 1000, and -0.5 and -5e-1 are written -0.5. A JSON number always has a finite decimal expansion, so the
// spelling is exact, and a value too wide for a float64 keeps its digits.
//
// It returns an error when text is not JSON, as YAMLStar's "Infinity" for ".inf" is not.
func CanonicalJSON(text string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return "", fmt.Errorf("not JSON: %w", err)
	}
	if dec.More() {
		return "", errors.New("not JSON: text after the value")
	}

	var buf bytes.Buffer
	if err := writeCanonical(&buf, v); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			name, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(name)
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case json.Number:
		buf.WriteString(decimal(string(t)))
	default:
		out, err := json.Marshal(t)
		if err != nil {
			return err
		}
		buf.Write(out)
	}

	return nil
}

// decimal writes a JSON number as its exact value: an integer without a fraction, a fraction without trailing zeros.
func decimal(number string) string {
	r, ok := new(big.Rat).SetString(number)
	if !ok {
		return number
	}
	if r.IsInt() {
		return r.Num().String()
	}

	// The denominator of a decimal is 2^a 5^b, and max(a, b) digits write it exactly.
	d := new(big.Int).Set(r.Denom())
	digits := 0
	for _, p := range []int64{2, 5} {
		n, q, m := 0, big.NewInt(p), new(big.Int)
		for x := new(big.Int).Set(d); ; n++ {
			x.QuoRem(x, q, m)
			if m.Sign() != 0 {
				break
			}
			d.Quo(d, q)
		}
		digits = max(digits, n)
	}

	return strings.TrimRight(strings.TrimRight(r.FloatString(digits), "0"), ".")
}

// Group is the sources giving one answer to one question.
type Group struct {
	// Answer is the answer, in the form the question compares: a verdict, an event stream joined by newlines, or the
	// canonical JSON of every document joined by newlines.
	Answer string `json:"answer"`
	// Sources are the ids of the sources giving it, sorted.
	Sources []string `json:"sources"`
}

// Groups partitions the sources by the answer each gives, in the order of the first source giving each answer.
//
// answers maps a source id to its answer; a source with nothing to say on the question is left out of the map. The
// result is one group for a unanimous question, and more for a split one.
func Groups(answers map[string]string) []Group {
	ids := make([]string, 0, len(answers))
	for id := range answers {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var out []Group
	for _, id := range ids {
		answer := answers[id]
		i := slices.IndexFunc(out, func(g Group) bool { return g.Answer == answer })
		if i < 0 {
			out = append(out, Group{Answer: answer})
			i = len(out) - 1
		}
		out[i].Sources = append(out[i].Sources, id)
	}

	return out
}
