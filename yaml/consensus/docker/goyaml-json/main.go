// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Command goyaml-json reads one YAML stream on stdin with github.com/go-openapi/go-yaml and writes its answer as one
// JSON object on stdout, in the form the consensus runner reads:
//
//	{"verdict":"accept","docs":1,"json":["{\"a\":1}"]}
//	{"verdict":"refuse","refusedBy":"parser","message":"[1:2] ..."}
//
// It is a program of its own, built into the consensus image, so that the yaml/consensus module does not require
// go-openapi/go-yaml: the parser under test changes far more often than the references it is compared with.
//
// The verdict is parser.ParseBytes's, with the refusals that are its composer's told apart. A stream of one document is converted with codec.ToJSON, go-yaml's own JSON
// path; a longer one is decoded a document at a time with codec.UseStringKeys, because ToJSON converts only the first.
// An error after a clean parse is the loader's.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser"
)

type answer struct {
	Verdict   string   `json:"verdict"`
	RefusedBy string   `json:"refusedBy,omitempty"`
	Docs      int      `json:"docs,omitempty"`
	JSON      []string `json:"json,omitempty"`
	Message   string   `json:"message,omitempty"`
}

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goyaml-json: %v\n", err)
		os.Exit(2)
	}

	out, err := json.Marshal(read(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "goyaml-json: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, string(out))
}

func read(src []byte) answer {
	file, err := parser.ParseBytes(src)
	if err != nil {
		// go-yaml's parser resolves aliases and checks keys as it reads, and those refusals are the composer's: an
		// alias naming no anchor or standing inside its own, a key written twice.
		stage := "parser"
		if errors.Is(err, yamlerrors.ErrUnknownAnchor) || errors.Is(err, yamlerrors.ErrRecursiveAlias) ||
			errors.Is(err, yamlerrors.ErrDuplicateKey) {
			stage = "composer"
		}

		return answer{Verdict: "refuse", RefusedBy: stage, Message: firstLine(err)}
	}

	docs := len(file.Docs)
	if docs <= 1 {
		converted, err := codec.ToJSON(src)
		if err != nil {
			return answer{Verdict: "refuse", RefusedBy: "loader", Docs: docs, Message: firstLine(err)}
		}

		return answer{Verdict: "accept", Docs: docs, JSON: []string{strings.TrimSpace(string(converted))}}
	}

	dec := codec.NewDecoder(bytes.NewReader(src), codec.UseStringKeys())
	values := make([]string, 0, docs)
	for {
		var v any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return answer{Verdict: "refuse", RefusedBy: "loader", Docs: docs, Message: firstLine(err)}
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			// The document was read, and JSON has no spelling for its value.
			values = append(values, "")

			continue
		}
		values = append(values, string(encoded))
	}

	return answer{Verdict: "accept", Docs: docs, JSON: values}
}

func firstLine(err error) string {
	first, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")

	return first
}
