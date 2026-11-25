// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yamlstar reads a document with YAMLStar's own parser, through its native yaml command.
//
// YAMLStar is Ingy döt Net's YAML 1.2 loader, written in Clojure and built into a native binary with Glojure, so
// it needs no JVM. Its default parser is org.yamlstar/yaml-parser (yaml/yaml-reference-parser-clj), a Clojure
// parser generated from the specification's grammar.
//
// # The parser has to be asked for
//
// The released binary also carries a "go_yaml" parser plugin, and the native build uses it by default: a plain
// "yaml" call refuses "[a" with "go-yaml load error in parser (while parsing a flow sequence)". Every call here
// passes "--plugin yaml-parser=reference", which selects YAMLStar's own parser. Without it, this source would repeat
// a Go library the runner already consults.
//
// # Two calls, two answers
//
// [Events] runs "yaml -D parse", which prints the parser's own events and nothing that comes after. "yaml -e" is
// not the parser's output: it resolves tags first, and refuses "!local x" with "Unknown tag", where the parser
// reads it.
//
// [Load] runs "yaml -j" and returns the value of each document. A refusal there after a clean parse is the loader's.
//
// # Two defects of 0.1.28
//
// "yaml -D parse" writes every character as one byte, so "é" comes out as the byte 0xE9 and a character past U+00FF
// is cut to its low byte. The event text of a document holding anything but ASCII cannot be read back, and
// [EventsReadable] says so; the verdict still can.
//
// The reference parser reads a double-quoted escape as the decimal digits of its code point: "\u00e9" is "233", and
// "\x41\u263A" is "659786". That is in the value "yaml -j" writes too, so it is an answer of YAMLStar's, and the
// report shows it as one.
package yamlstar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ErrNotInstalled says the yaml command could not be found.
var ErrNotInstalled = errors.New("yamlstar: the yaml command is not installed; set YAMLSTAR or put yaml on PATH")

// referenceParser selects YAMLStar's own parser instead of the native build's go_yaml default.
var referenceParser = []string{"--plugin", "yaml-parser=reference"}

// Command is the yaml command this package runs: YAMLSTAR when set, else yaml on PATH.
func Command() string {
	if set := os.Getenv("YAMLSTAR"); set != "" {
		return set
	}
	found, err := exec.LookPath("yaml")
	if err != nil {
		return ""
	}

	return found
}

// Available reports whether the yaml command can be run.
func Available() bool { return Version() != "" }

// Version is what "yaml --version" prints, as "yaml v0.1.28", or empty when there is no command.
func Version() string {
	if Command() == "" {
		return ""
	}
	out, _, err := run(context.Background(), nil, "--version")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// EventsReadable reports whether the events [Events] returns for src are the parser's: false when src holds a byte
// past ASCII, which "yaml -D parse" cannot print.
func EventsReadable(src []byte) bool {
	for _, b := range src {
		if b >= 0x80 {
			return false
		}
	}

	return true
}

// Events runs src through YAMLStar's parser and returns the event stream in test.event form.
//
// ok is false when the parser refused the document, and msg is what it said. err is a failure to run at all.
func Events(ctx context.Context, src []byte) (events []string, ok bool, msg string, err error) {
	args := append(append([]string{}, referenceParser...), "-D", "parse")
	out, errs, runErr := run(ctx, src, args...)
	if runErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](runErr); !exited {
			return nil, false, "", runErr
		}

		return nil, false, strings.TrimSpace(errs), nil
	}

	if !EventsReadable(src) {
		// The parser read it; its events cannot be read back. See EventsReadable.
		return nil, true, "", nil
	}

	events, err = convert(out)
	if err != nil {
		return nil, false, "", err
	}

	return events, true, "", nil
}

// Load runs src through YAMLStar and returns the JSON of each document, as YAMLStar writes it.
//
// YAMLStar writes a stream of several documents as one JSON array, which reads exactly like one document holding a
// sequence, so docs says how many documents the stream holds -- count them with [Events]. A value with no JSON form
// comes back as YAMLStar spells it: ".inf" is "Infinity", which is not JSON.
func Load(ctx context.Context, src []byte, docs int) ([]string, string, error) {
	args := append(append([]string{}, referenceParser...), "-j")
	out, errs, runErr := run(ctx, src, args...)
	if runErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](runErr); !exited {
			return nil, "", runErr
		}

		return nil, strings.TrimSpace(errs), nil
	}

	text := strings.TrimSpace(out)
	if docs <= 1 {
		return []string{text}, "", nil
	}

	var each []json.RawMessage
	if err := json.Unmarshal([]byte(text), &each); err != nil {
		// A document whose value has no JSON form, as ".inf" written Infinity, spoils the whole array: no document of
		// the stream can be told apart, so none has a value.
		return make([]string, docs), "", nil //nolint:nilerr // the answer is "no JSON form", not a failure
	}
	if len(each) != docs {
		return nil, "", fmt.Errorf("yamlstar: %d documents and an array of %d", docs, len(each))
	}
	out2 := make([]string, len(each))
	for i, v := range each {
		out2[i] = string(v)
	}

	return out2, "", nil
}

func run(ctx context.Context, stdin []byte, args ...string) (string, string, error) {
	command := Command()
	if command == "" {
		return "", "", ErrNotInstalled
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = bytes.NewReader(stdin)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	err := cmd.Run()

	return out.String(), errs.String(), err
}

// convert turns the parser's events, one EDN map each after a timing header, into test.event lines.
//
//	{:event "scalar", :value "sq", :style "single"}   =VAL 'sq
//	{:event "sequence_start", :flow true}             +SEQ []
//	{:event "document_start", :explicit true}         +DOC ---
//
// The maps are read as a stream and not line by line: the parser prints a scalar's line breaks as they are, so one
// event may span lines.
func convert(out string) ([]string, error) {
	var events []string
	for rest := out; ; {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			return events, nil
		}
		m, after, err := readMap(rest[start:])
		if err != nil {
			return nil, fmt.Errorf("yamlstar: reading the events: %w", err)
		}
		events = append(events, testEvent(m))
		rest = after
	}
}

func testEvent(m map[string]string) string {
	props := func() string {
		var b strings.Builder
		if a := m["anchor"]; a != "" {
			b.WriteString(" &" + a)
		}
		if t := m["tag"]; t != "" {
			b.WriteString(" <" + t + ">")
		}

		return b.String()
	}

	switch m["event"] {
	case "stream_start":
		return "+STR"
	case "stream_end":
		return "-STR"
	case "document_start":
		if m["explicit"] == "true" {
			return "+DOC ---"
		}

		return "+DOC"
	case "document_end":
		if m["explicit"] == "true" {
			return "-DOC ..."
		}

		return "-DOC"
	case "mapping_start":
		flow := ""
		if m["flow"] == "true" {
			flow = " {}"
		}

		return "+MAP" + flow + props()
	case "mapping_end":
		return "-MAP"
	case "sequence_start":
		flow := ""
		if m["flow"] == "true" {
			flow = " []"
		}

		return "+SEQ" + flow + props()
	case "sequence_end":
		return "-SEQ"
	case "alias":
		return "=ALI *" + m["name"]
	case "scalar":
		indicator := ":"
		switch m["style"] {
		case "single":
			indicator = "'"
		case "double":
			indicator = `"`
		case "literal":
			indicator = "|"
		case "folded":
			indicator = ">"
		}

		return "=VAL" + props() + " " + indicator + escapeValue(m["value"])
	default:
		return "?" + m["event"]
	}
}

// escapeValue writes a scalar's value as test.event does: a backslash, a line break, a tab and the other control
// characters escaped.
func escapeValue(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\b':
			b.WriteString(`\b`)
		case 0:
			b.WriteString(`\0`)
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// readMap reads one EDN map of keyword keys to strings, booleans and numbers at the start of s, as the parser prints
// its events -- {:event "scalar", :value "a\nb", :flow true} -- and returns it with what follows.
func readMap(s string) (map[string]string, string, error) {
	if !strings.HasPrefix(s, "{") {
		return nil, "", errors.New("not a map")
	}
	s = s[1:]

	m := map[string]string{}
	for {
		s = strings.TrimLeft(s, " ,\n\r\t")
		switch {
		case s == "":
			return nil, "", errors.New("a map with no end")
		case s[0] == '}':
			return m, s[1:], nil
		case s[0] != ':':
			return nil, "", fmt.Errorf("a key that is not a keyword at %q", head(s))
		}
		end := strings.IndexAny(s, " ,}")
		if end < 0 {
			return nil, "", fmt.Errorf("a key with no value at %q", head(s))
		}
		key := s[1:end]
		s = strings.TrimLeft(s[end:], " ")

		if strings.HasPrefix(s, `"`) {
			value, rest, err := readString(s)
			if err != nil {
				return nil, "", err
			}
			m[key], s = value, rest

			continue
		}
		end = strings.IndexAny(s, " ,}")
		if end < 0 {
			end = len(s)
		}
		m[key], s = s[:end], s[end:]
	}
}

func head(s string) string {
	if len(s) > 20 {
		return s[:20]
	}

	return s
}

// readString reads a Clojure string literal at the start of s, and returns its value and what follows it.
func readString(s string) (string, string, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			return b.String(), s[i+1:], nil
		case '\\':
			if i+1 >= len(s) {
				return "", "", errors.New("a string ending on a backslash")
			}
			i++
			switch e := s[i]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '"', '\\':
				b.WriteByte(e)
			case 'u':
				if i+4 >= len(s) {
					return "", "", errors.New(`a short \u escape`)
				}
				code, err := strconv.ParseUint(s[i+1:i+5], 16, 16)
				if err != nil {
					return "", "", fmt.Errorf(`a \u escape: %w`, err)
				}
				b.WriteRune(rune(code))
				i += 4
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
		default:
			b.WriteByte(c)
		}
	}

	return "", "", errors.New("an unterminated string")
}
