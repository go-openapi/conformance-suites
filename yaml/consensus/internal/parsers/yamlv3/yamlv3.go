// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlv3 reads a document with go.yaml.in/yaml/v3, the third reading the corpus can consult.
//
// The public package github.com/go-openapi/conformance-suites/yaml/consensus/goyaml documents the source and
// re-exports what other modules use.
package yamlv3

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	yaml "go.yaml.in/yaml/v3"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
)

// Version names what is being consulted, so a corpus entry that rests on this
// reading records which one gave it.
const Version = "go.yaml.in/yaml/v3 v3.0.6"

// Load reads src and returns what yaml.v3 makes of each document in it.
//
// The Go values rather than JSON, because JSON is a lossy medium for this
// question and the loss is sometimes the finding. yaml.v3 keeps a mapping key's
// type -- float64(1) for "1.0", int(1) for "1", nil for "~" -- so it reads
// "1.0: a" into a map JSON cannot name at all, where this library stringifies
// the key to "1" and libfyaml stringifies it to "1.0". A JSON-only reading
// would have reported that as an error and hidden the third answer.
func Load(src []byte) ([]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))

	var out []any

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out, nil
		}

		if err != nil {
			return nil, fmt.Errorf("yaml.v3: %w", err)
		}

		out = append(out, v)
	}
}

// LoadJSON is [Load] rendered as JSON, one string per document, which is what
// makes it comparable with libfyaml.Load.
//
// ErrNotJSON says the document was read and JSON has no spelling for what came
// back: a non-string mapping key, a cycle, a NaN. That is an answer about the
// document rather than a failure of this function, so it is a named error and
// not a generic one.
func LoadJSON(src []byte) ([]string, error) {
	docs, err := Load(src)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(docs))

	for _, v := range docs {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNotJSON, err)
		}

		out = append(out, string(encoded))
	}

	return out, nil
}

// ErrNotJSON says a document was read and JSON cannot write what came back.
var ErrNotJSON = errors.New("yaml.v3: read it, and JSON cannot write it")

// RoundTrip writes each document [Load] read back as YAML with yaml.Marshal, reads that with yaml.Unmarshal, and
// compares the two values with [consensus.SameValue]. It returns nil when src does not load.
func RoundTrip(src []byte) *consensus.RoundTrip {
	docs, err := Load(src)
	if err != nil {
		return nil
	}

	for i, v := range docs {
		if consensus.HasCycle(v) {
			return &consensus.RoundTrip{Result: consensus.Cyclic, Doc: i}
		}
		out, err := marshal(v)
		if err != nil {
			return &consensus.RoundTrip{Result: consensus.Unwritable, Doc: i, Message: consensus.FirstLine(err.Error())}
		}
		var again any
		if err := yaml.Unmarshal(out, &again); err != nil {
			return &consensus.RoundTrip{
				Result: consensus.Unreadable, Doc: i, YAML: cut(out), Message: consensus.FirstLine(err.Error()),
			}
		}
		if !consensus.SameValue(v, again) {
			return &consensus.RoundTrip{Result: consensus.Different, Doc: i, YAML: cut(out), Again: render(again)}
		}
	}

	return &consensus.RoundTrip{Result: consensus.Same}
}

// marshal is yaml.Marshal with its panics turned into errors: it panics on a value it has no encoding for.
func marshal(v any) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("yaml.v3: Marshal panicked: %v", r)
		}
	}()

	return yaml.Marshal(v)
}

func cut(out []byte) string {
	if len(out) > consensus.MaxRoundTripYAML {
		return string(out[:consensus.MaxRoundTripYAML]) + "\n..."
	}

	return string(out)
}

// render writes a value as JSON where it has a JSON form, and as Go syntax where it does not.
func render(v any) string {
	if encoded, err := json.Marshal(v); err == nil {
		return string(encoded)
	}

	return fmt.Sprintf("%#v", v)
}
