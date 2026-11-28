// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Command goyaml-json reads one YAML stream on stdin with github.com/go-openapi/go-yaml and writes its answer as one
// JSON object on stdout, in the form the consensus runner reads:
//
//	{"verdict":"accept","docs":1,"json":["{\"a\":1}"]}
//	{"verdict":"refuse","refusedBy":"parser","message":"[1:2] ..."}
//
// With -lines it reads one request per line, {"src":"<base64>"}, and writes one answer per line, until its input
// ends, so one process answers a whole corpus. A panic on one document becomes an answer with verdict "error".
//
// It is a program of its own, built into the consensus image, so that the yaml/consensus module does not require
// go-openapi/go-yaml: the parser under test changes far more often than the references it is compared with.
//
// The verdict is parser.ParseBytes's, with the refusals that are its composer's told apart. A stream of one document is converted with codec.ToJSON, go-yaml's own JSON
// path; a longer one is decoded a document at a time with codec.UseStringKeys, because ToJSON converts only the first.
// An error after a clean parse is the loader's.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/go-openapi/go-yaml/ast"
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
	// RoundTrip checks decode(encode(decode(document))) against decode(document), when the document was accepted.
	RoundTrip *roundTrip `json:"roundTrip,omitempty"`
}

func main() {
	lines := flag.Bool("lines", false, "read one JSON request per line and answer each on a line of its own")
	flag.Parse()

	if *lines {
		if err := serve(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "goyaml-json: %v\n", err)
			os.Exit(2)
		}

		return
	}

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

// maxLine bounds one request, as the runner bounds its own lines.
const maxLine = 64 << 20

// serve answers every request read from r, writing one answer per line to w, until r ends.
func serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), maxLine)
	out := bufio.NewWriter(w)
	enc := json.NewEncoder(out)

	for sc.Scan() {
		var q struct {
			Src []byte `json:"src"`
		}
		if err := json.Unmarshal(sc.Bytes(), &q); err != nil {
			return fmt.Errorf("reading a request: %w", err)
		}
		if err := enc.Encode(readSafely(q.Src)); err != nil {
			return err
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}

	return sc.Err()
}

// readSafely is [read], with a panic turned into an answer with verdict "error".
func readSafely(src []byte) (a answer) {
	defer func() {
		if r := recover(); r != nil {
			// The parser may hold the state of the parse that panicked; the next stream gets a new one.
			reuse = nil
			a = answer{Verdict: "error", Message: firstLine(fmt.Errorf("panic: %v", r))}
		}
	}()

	return read(src)
}

// reuse is the parser read uses for each stream, reset between streams: a new Parser allocates its arenas, about
// 36 KB, which costs more than parsing most documents of a corpus. The program reads one stream at a time.
var reuse *parser.Parser

func read(src []byte) answer {
	if reuse == nil {
		reuse = parser.New()
	} else {
		reuse.Reset()
	}
	file, err := reuse.Parse(src)
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

	docs := documents(file)
	if docs <= 1 {
		converted, err := codec.ToJSON(src)
		if err != nil {
			return answer{Verdict: "refuse", RefusedBy: "loader", Docs: docs, Message: firstLine(err)}
		}

		return answer{
			Verdict: "accept", Docs: docs, JSON: []string{strings.TrimSpace(string(converted))},
			RoundTrip: checkRoundTrip(src),
		}
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
		encoded, err := json.Marshal(numbersAsJSON(v))
		if err != nil {
			// The document was read, and JSON has no spelling for its value.
			values = append(values, "")

			continue
		}
		values = append(values, string(encoded))
	}

	return answer{Verdict: "accept", Docs: docs, JSON: values, RoundTrip: checkRoundTrip(src)}
}

// documents counts the documents of a stream. go-yaml's AST holds the directives in front of a document in a
// DocumentNode of their own, so "%YAML 1.1\n---\nx\n" parses as two DocumentNode values and holds one document.
func documents(file *ast.File) int {
	n := 0
	for _, doc := range file.Docs {
		if _, directive := doc.Body.(*ast.DirectiveNode); !directive {
			n++
		}
	}

	return n
}

// numbersAsJSON replaces each *big.Float in a decoded value with a json.Number holding its digits:
// encoding/json writes a *big.Float through MarshalText, as a string. A *big.Int has its own MarshalJSON and is
// written as a number already.
func numbersAsJSON(v any) any {
	switch n := v.(type) {
	case *big.Float:
		return json.Number(n.Text('g', -1))
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, e := range n {
			out[k] = numbersAsJSON(e)
		}

		return out
	case []any:
		out := make([]any, len(n))
		for i, e := range n {
			out[i] = numbersAsJSON(e)
		}

		return out
	default:
		return v
	}
}

func firstLine(err error) string {
	first, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")

	return first
}
