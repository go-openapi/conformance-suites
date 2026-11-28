// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/consensus"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/goopenapi"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/libfyaml"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/perlref"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/yamlstar"
	"github.com/go-openapi/conformance-suites/yaml/consensus/internal/parsers/yamlv3"
)

// source is one implementation the runner consults.
type source struct {
	// id names the source in the results and the report.
	id string
	// what says which implementation it is, for the report.
	what string
	// events and values say which answers the source can give: an event stream, a value.
	events, values bool
	// roundTrip says the source writes each value back as YAML and reads it again.
	roundTrip bool
	// procs is how many drivers of this source run side by side, each taking every procs-th case. Zero means one.
	// The Perl reference parser takes about 97 ms of CPU per document, and YAMLStar starts its native command once
	// per document; the other sources answer in a millisecond or two.
	procs     int
	available func() bool
	version   func() string
	answer    func(ctx context.Context, src []byte) consensus.Answer
}

func allSources() []source {
	return []source{
		{
			id: "perlref", what: "YAML 1.2 reference parser (Perl), yaml/yaml-reference-parser", events: true, procs: 4,
			available: perlref.Available, version: perlref.Commit, answer: perlrefAnswer,
		},
		{
			id: "libfyaml", what: "libfyaml (C): fy-tool for events, the Python binding for values",
			events: libfyaml.Tool() != "", values: true,
			available: func() bool { return libfyaml.Available() || libfyaml.Tool() != "" },
			version: func() string {
				return "binding " + orNone(libfyaml.Version()) + ", " + orNone(libfyaml.ToolVersion())
			},
			answer: libfyamlAnswer,
		},
		{
			id: "yamlstar", what: "YAMLStar (Clojure), its own reference parser", events: true, values: true, procs: 2,
			available: yamlstar.Available, version: yamlstar.Version, answer: yamlstarAnswer,
		},
		{
			id: "yaml.v3", what: "go.yaml.in/yaml/v3 (Go)", values: true, roundTrip: true,
			available: func() bool { return true }, version: func() string { return yamlv3.Version }, answer: yamlv3Answer,
		},
		{
			id: "go-yaml", what: "github.com/go-openapi/go-yaml (Go)", values: true, roundTrip: true,
			available: goopenapi.Available, version: goopenapi.Version, answer: goopenapiAnswer,
		},
	}
}

// bindingAvailable asks once whether libfyaml's Python binding can be run.
var bindingAvailable = sync.OnceValue(libfyaml.Available)

func orNone(s string) string {
	if s == "" {
		return "none"
	}

	return s
}

func failed(err error) consensus.Answer {
	verdict := consensus.Error
	if errors.Is(err, context.DeadlineExceeded) {
		return consensus.Answer{Verdict: verdict, Message: "timed out"}
	}

	return consensus.Answer{Verdict: verdict, Message: consensus.FirstLine(err.Error())}
}

// libfyamlLoader is the drive process's one Python process, started on the first document.
var libfyamlLoader = sync.OnceValue(func() *libfyaml.Loader { return libfyaml.New(libfyaml.Home()) })

// goopenapiReader is the drive process's one goyaml-json process, started on the first document.
var goopenapiReader = sync.OnceValue(func() *goopenapi.Reader { return goopenapi.New(goopenapi.Command()) })

// perlrefParser is the drive process's one Perl process, started on the first document.
var perlrefParser = sync.OnceValue(func() *perlref.Parser { return perlref.New(perlref.Home()) })

func perlrefAnswer(ctx context.Context, src []byte) consensus.Answer {
	events, ok, err := perlrefParser().Events(ctx, src)
	if err != nil {
		return failed(err)
	}
	if !ok {
		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Parser, Events: events}
	}

	return consensus.Answer{Verdict: consensus.Accept, Docs: countDocs(events), Events: events}
}

func libfyamlAnswer(ctx context.Context, src []byte) consensus.Answer {
	var a consensus.Answer
	parsed := true

	if libfyaml.Tool() != "" {
		events, ok, msg, err := libfyaml.Events(ctx, src)
		if err != nil {
			return failed(err)
		}
		if !ok {
			return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Parser, Message: consensus.FirstLine(msg)}
		}
		a.Events, a.Docs = events, countDocs(events)
	} else {
		parsed = false
	}

	if !bindingAvailable() {
		a.Verdict = consensus.Accept

		return a
	}

	docs, err := libfyamlLoader().Load(ctx, src, string(libfyaml.YAML12))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			return failed(context.DeadlineExceeded)
		}
		// Once fy-tool has parsed the document, a failure here is the binding's. Without fy-tool, the binding raises
		// "ValueError: Failed to parse YAML/JSON" for a syntax error, and a TypeError for a value it cannot build.
		said := lastLine(err.Error())
		a.Verdict, a.Message = consensus.Refuse, consensus.FirstLine(said)
		a.RefusedBy = consensus.Loader
		if !parsed && strings.HasPrefix(said, "ValueError: Failed to parse") {
			a.RefusedBy = consensus.Parser
		}

		return a
	}

	a.Verdict, a.JSON = consensus.Accept, canonical(docs)
	if a.Docs == 0 {
		a.Docs = len(docs)
	}

	return a
}

// yamlstarAnswer runs YAMLStar's command once per document, with -j, and a second time with -D parse only to count
// the documents of an answer written as a JSON array, which one document holding a sequence also is. With
// collectEvents, it runs -D parse on every document, for the events.
func yamlstarAnswer(ctx context.Context, src []byte) consensus.Answer {
	if collectEvents {
		return yamlstarAnswerWithEvents(ctx, src)
	}

	text, msg, err := yamlstar.Values(ctx, src)
	switch {
	case err != nil:
		return failed(err)
	case msg != "":
		stage := consensus.Loader
		if strings.HasPrefix(msg, "Error: parse") {
			stage = consensus.Parser
		}

		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: stage, Message: consensus.FirstLine(msg)}
	}

	a := consensus.Answer{Verdict: consensus.Accept, Docs: 1}
	if strings.HasPrefix(text, "[") {
		events, ok, msg, err := yamlstar.Events(ctx, src)
		switch {
		case err != nil:
			return failed(err)
		case !ok:
			return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Parser, Message: consensus.FirstLine(msg)}
		}
		a.Docs = countDocs(events)
	}

	docs, err := yamlstar.Split(text, a.Docs)
	if err != nil {
		return failed(err)
	}
	a.JSON = canonical(docs)

	return a
}

func yamlstarAnswerWithEvents(ctx context.Context, src []byte) consensus.Answer {
	events, ok, msg, err := yamlstar.Events(ctx, src)
	if err != nil {
		return failed(err)
	}
	if !ok {
		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Parser, Message: consensus.FirstLine(msg)}
	}

	a := consensus.Answer{Verdict: consensus.Accept, Docs: countDocs(events)}
	if yamlstar.EventsReadable(src) {
		a.Events = events
	}

	docs, msg, err := yamlstar.Load(ctx, src, a.Docs)
	switch {
	case err != nil:
		return failed(err)
	case msg != "":
		a.Verdict, a.RefusedBy, a.Message = consensus.Refuse, consensus.Loader, consensus.FirstLine(msg)
	default:
		a.JSON = canonical(docs)
	}

	return a
}

func yamlv3Answer(_ context.Context, src []byte) consensus.Answer {
	docs, err := yamlv3.Load(src)
	if err != nil {
		msg := err.Error()

		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: yamlv3Stage(msg), Message: consensus.FirstLine(msg)}
	}

	a := consensus.Answer{Verdict: consensus.Accept, Docs: len(docs), RoundTrip: yamlv3.RoundTrip(src)}
	rendered, err := yamlv3.LoadJSON(src)
	if err != nil {
		// Read, and JSON has no spelling for a value: every document is left without one.
		a.JSON = make([]string, len(docs))

		return a
	}
	a.JSON = canonical(rendered)

	return a
}

func goopenapiAnswer(ctx context.Context, src []byte) consensus.Answer {
	if !goopenapi.Available() {
		return failed(goopenapi.ErrNotInstalled)
	}
	got, err := goopenapiReader().Read(ctx, src)
	if err != nil {
		return failed(err)
	}

	return consensus.Answer{
		Verdict:   consensus.Verdict(got.Verdict),
		RefusedBy: consensus.Stage(got.RefusedBy),
		Docs:      got.Docs,
		JSON:      canonical(got.JSON),
		Message:   got.Message,
		RoundTrip: got.RoundTrip,
	}
}

// canonical rewrites each document's JSON in canonical form, leaving the empty string for one with no JSON form.
func canonical(docs []string) []string {
	if docs == nil {
		return nil
	}
	out := make([]string, len(docs))
	for i, d := range docs {
		if c, err := consensus.CanonicalJSON(d); err == nil {
			out[i] = c
		}
	}

	return out
}

func countDocs(events []string) int {
	n := 0
	for _, e := range events {
		if strings.HasPrefix(e, "+DOC") {
			n++
		}
	}

	return n
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}

	return s
}

// timed runs a source on src under a timeout and records how long it took.
func timed(s source, src []byte, timeout time.Duration) consensus.Answer {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	a := s.answer(ctx, src)
	if ctx.Err() != nil && a.Verdict != consensus.Accept && a.Verdict != consensus.Refuse {
		a = failed(context.DeadlineExceeded)
	}
	a.Source = s.id
	a.ElapsedMS = float64(time.Since(start).Microseconds()) / 1000

	return a
}

// yamlv3Loader holds the messages of yaml.v3's decoder, which builds the value after a clean parse: the type errors
// it gathers as "unmarshal errors", and the failures of decode.go and resolve.go.
var yamlv3Loader = []string{
	"unmarshal errors",
	"invalid map key",
	"cannot decode ",
	"!!binary value contains invalid base64 data",
	"map merge requires map or sequence of maps",
	"value contains itself",
	"document contains excessive aliasing",
}

// yamlv3Stage returns the stage of yaml.v3 that wrote an error message. An alias naming no anchor is the
// composer's, and every other message without a loader's wording is the parser's.
func yamlv3Stage(msg string) consensus.Stage {
	if strings.Contains(msg, "unknown anchor") {
		return consensus.Composer
	}
	for _, m := range yamlv3Loader {
		if strings.Contains(msg, m) {
			return consensus.Loader
		}
	}

	return consensus.Parser
}
