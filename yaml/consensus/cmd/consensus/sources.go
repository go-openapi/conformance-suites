// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-openapi/conformance-suites/yaml/consensus"
	"github.com/go-openapi/conformance-suites/yaml/consensus/goopenapi"
	"github.com/go-openapi/conformance-suites/yaml/consensus/goyaml"
	"github.com/go-openapi/conformance-suites/yaml/consensus/libfyaml"
	"github.com/go-openapi/conformance-suites/yaml/consensus/perlref"
	"github.com/go-openapi/conformance-suites/yaml/consensus/yamlstar"
)

// source is one implementation the runner consults.
type source struct {
	// id names the source in the results and the report.
	id string
	// what says which implementation it is, for the report.
	what string
	// underTest marks the parser the report scores, as against the references it is compared with.
	underTest bool
	// events and values say which answers the source can give: an event stream, a value.
	events, values bool
	// roundTrip says the source writes each value back as YAML and reads it again.
	roundTrip bool
	available func() bool
	version   func() string
	answer    func(ctx context.Context, src []byte) consensus.Answer
}

func allSources() []source {
	return []source{
		{
			id: "perlref", what: "YAML 1.2 reference parser (Perl), yaml/yaml-reference-parser", events: true,
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
			id: "yamlstar", what: "YAMLStar (Clojure), its own reference parser", events: true, values: true,
			available: yamlstar.Available, version: yamlstar.Version, answer: yamlstarAnswer,
		},
		{
			id: "yaml.v3", what: "go.yaml.in/yaml/v3 (Go)", values: true, roundTrip: true,
			available: func() bool { return true }, version: func() string { return goyaml.Version }, answer: yamlv3Answer,
		},
		{
			id: "go-yaml", what: "github.com/go-openapi/go-yaml (Go), the parser under test", underTest: true, values: true,
			roundTrip: true,
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

func perlrefAnswer(ctx context.Context, src []byte) consensus.Answer {
	events, ok, err := perlref.EventsContext(ctx, src)
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

	docs, err := libfyaml.LoadContext(ctx, src, libfyaml.YAML12)
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

func yamlstarAnswer(ctx context.Context, src []byte) consensus.Answer {
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
	docs, err := goyaml.Load(src)
	if err != nil {
		stage := consensus.Parser
		// yaml.v3 names a loader error "unmarshal errors", and its parser errors carry a line. An alias naming no
		// anchor is its composer's.
		switch msg := err.Error(); {
		case strings.Contains(msg, "unmarshal errors") || strings.Contains(msg, "invalid map key"):
			stage = consensus.Loader
		case strings.Contains(msg, "unknown anchor"):
			stage = consensus.Composer
		}

		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: stage, Message: consensus.FirstLine(err.Error())}
	}

	a := consensus.Answer{Verdict: consensus.Accept, Docs: len(docs), RoundTrip: goyaml.RoundTrip(src)}
	rendered, err := goyaml.LoadJSON(src)
	if err != nil {
		// Read, and JSON has no spelling for a value: every document is left without one.
		a.JSON = make([]string, len(docs))

		return a
	}
	a.JSON = canonical(rendered)

	return a
}

func goopenapiAnswer(ctx context.Context, src []byte) consensus.Answer {
	got, err := goopenapi.Read(ctx, src)
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
