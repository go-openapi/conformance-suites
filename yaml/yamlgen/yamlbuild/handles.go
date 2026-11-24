// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlbuild

import (
	"bytes"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-openapi/conformance-suites/stance"
)

// namedHandle matches the handle of a tag shorthand written with a named handle: "!e!" in "!e!str".
var namedHandle = regexp.MustCompile(`^![0-9A-Za-z-]+!`)

// DirectiveTags reports the tags the directives of the stream src give it, sorted.
//
// A mutant needs them. The generator tags what it wrote, and a mutation that breaks a directive line changes what
// the stream holds: "%TAG! x! ..." is a reserved directive and declares nothing, so the "!x!str" below it has a
// handle no directive declares; "%YAML 1.22" names a minor version past 1.2. The grammar cannot see either, since
// both need the directives already read for the document, and no production keeps them. Untagged, such a mutant
// claimed that a parser must read a document every implementation refuses.
//
// The tags found are [TagUndeclaredHandle], [TagHandleOutOfScope], [TagYAMLMajorVersion], [TagYAMLMinorVersion],
// [TagYAMLRepeated], [TagTagHandleRepeated] and [TagDirectiveReserved].
//
// It reads the stream as text, which is enough for a stream the grammar accepts. It tracks the directives of each
// document, and steps over the text a tag cannot stand in: quoted, plain and block scalars, and comments. The
// primary handle "!" and the secondary "!!" need no declaration.
func DirectiveTags(src []byte) []stance.Tag {
	s := directiveScan{
		found: map[stance.Tag]bool{}, declared: map[string]bool{}, earlier: map[string]bool{},
		inDirectives: true, blockParent: -1,
	}
	for _, line := range splitLines(src) {
		s.line(line)
	}

	out := make([]stance.Tag, 0, len(s.found))
	for tag := range s.found {
		out = append(out, tag)
	}
	slices.Sort(out)

	return out
}

// directiveScan is the state DirectiveTags carries from one line to the next.
type directiveScan struct {
	found map[stance.Tag]bool
	// declared holds the handles the directives above the next "---" declare; current, those of the document
	// being read; earlier, those an earlier document of the stream declared.
	declared, current, earlier map[string]bool
	// yamls counts the %YAML directives above the next "---".
	yamls int
	// inDirectives is true where directives may be written: at the start of the stream or after "...".
	inDirectives bool
	// flow is how deep the scan stands in flow collections.
	flow int
	// quote is the quote character of a quoted scalar running on from an earlier line, or 0.
	quote byte
	// blockParent is the indentation of the line holding a block scalar's header while the scan is inside its
	// content, and -1 otherwise.
	blockParent int
}

// line reads one line.
func (s *directiveScan) line(line string) {
	// A byte order mark may open any document of a stream, in front of its directives or its content.
	line = strings.TrimPrefix(line, "\ufeff")
	indent := len(line) - len(strings.TrimLeft(line, " "))
	blank := strings.TrimSpace(line) == ""

	if s.blockParent >= 0 {
		if blank || indent > s.blockParent {
			return
		}
		s.blockParent = -1
	}

	if s.quote == 0 && s.flow == 0 {
		switch {
		case isMarker(line, "---"):
			s.open(s.declared)
			s.content(line[3:], indent)

			return
		case isMarker(line, "..."):
			s.open(nil)
			s.inDirectives = true

			return
		case s.inDirectives && strings.HasPrefix(line, "%"):
			s.directive(line)

			return
		case s.inDirectives && !blank && !strings.HasPrefix(strings.TrimSpace(line), "#"):
			// A document with no "---": the directives above it, if any, declare nothing for it.
			s.open(map[string]bool{})
		}
	}

	s.content(line, indent)
}

// open starts a document whose shorthands resolve through handles, and keeps the handles of the one before.
func (s *directiveScan) open(handles map[string]bool) {
	for h := range s.current {
		s.earlier[h] = true
	}
	s.current, s.declared, s.inDirectives, s.yamls = handles, map[string]bool{}, false, 0
}

// directive reads one directive line.
func (s *directiveScan) directive(line string) {
	if i := strings.Index(line, " #"); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(line)
	switch name := strings.TrimPrefix(fields[0], "%"); name {
	case "":
	case "YAML":
		s.yamls++
		if s.yamls > 1 {
			s.found[TagYAMLRepeated] = true
		}
		if len(fields) > 1 {
			s.version(fields[1])
		}
	case "TAG":
		if len(fields) >= 3 {
			if s.declared[fields[1]] {
				s.found[TagTagHandleRepeated] = true
			}
			s.declared[fields[1]] = true
		}
	default:
		s.found[TagDirectiveReserved] = true
	}
}

// version reads a %YAML version, and tags a major version other than 1 or a minor version past 2.
func (s *directiveScan) version(v string) {
	major, minor, ok := strings.Cut(v, ".")
	if !ok {
		return
	}
	m, errMajor := strconv.Atoi(major)
	n, errMinor := strconv.Atoi(minor)
	switch {
	case errMajor != nil || errMinor != nil:
	case m != 1:
		s.found[TagYAMLMajorVersion] = true
	case n > 2:
		s.found[TagYAMLMinorVersion] = true
	}
}

// content reads the content of one line, starting at text.
func (s *directiveScan) content(text string, indent int) {
	i := 0
	if s.quote != 0 {
		i = s.closeQuote(text, 0)
		if i < 0 {
			return
		}
	}

	for i < len(text) {
		c := text[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '#':
			return
		case c == '\'' || c == '"':
			s.quote = c
			i = s.closeQuote(text, i+1)
			if i < 0 {
				return
			}
		case c == '!':
			end := tokenEnd(text, i, s.flow > 0)
			s.shorthand(text[i:end])
			i = end
		case c == '&' || c == '*':
			i = tokenEnd(text, i, s.flow > 0)
		case (c == '|' || c == '>') && s.flow == 0:
			s.blockParent = indent

			return
		case c == '[' || c == '{':
			s.flow++
			i++
		case c == ']' || c == '}':
			if s.flow > 0 {
				s.flow--
			}
			i++
		case c == ',' && s.flow > 0:
			i++
		case (c == '-' || c == '?' || c == ':') && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\t' ||
			(s.flow > 0 && strings.IndexByte(",[]{}", text[i+1]) >= 0)):
			i++
		default:
			i = plainEnd(text, i, s.flow > 0)
		}
	}
}

// shorthand tags a tag token whose named handle the current document does not declare: out of scope when an
// earlier document declared it, undeclared otherwise.
func (s *directiveScan) shorthand(tag string) {
	handle := namedHandle.FindString(tag)
	switch {
	case handle == "" || handle == "!!" || s.current[handle]:
	case s.earlier[handle]:
		s.found[TagHandleOutOfScope] = true
	default:
		s.found[TagUndeclaredHandle] = true
	}
}

// closeQuote steps over the rest of a quoted scalar from i, and returns where it ends, or -1 when it runs on past
// the line. A single quote escapes itself; a double-quoted scalar escapes with a backslash.
func (s *directiveScan) closeQuote(text string, i int) int {
	for i < len(text) {
		switch c := text[i]; {
		case s.quote == '"' && c == '\\':
			i += 2
		case c == s.quote && s.quote == '\'' && i+1 < len(text) && text[i+1] == '\'':
			i += 2
		case c == s.quote:
			s.quote = 0

			return i + 1
		default:
			i++
		}
	}

	return -1
}

// tokenEnd is where a tag, an anchor or an alias starting at i ends.
func tokenEnd(text string, i int, flow bool) int {
	for i < len(text) && text[i] != ' ' && text[i] != '\t' && (!flow || strings.IndexByte(",[]{}", text[i]) < 0) {
		i++
	}

	return i
}

// plainEnd is where a plain scalar starting at i ends on its line: at ": ", at " #", at the line's end, and in a
// flow collection at a flow indicator.
func plainEnd(text string, i int, flow bool) int {
	for i < len(text) {
		c := text[i]
		switch {
		case c == ':' && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\t' ||
			(flow && strings.IndexByte(",[]{}", text[i+1]) >= 0)):
			return i
		case c == '#' && i > 0 && (text[i-1] == ' ' || text[i-1] == '\t'):
			return i
		case flow && strings.IndexByte(",[]{}", c) >= 0:
			return i
		}
		i++
	}

	return i
}

// isMarker reports whether line opens with a document marker: "---" or "..." followed by a blank or nothing.
func isMarker(line, marker string) bool {
	return strings.HasPrefix(line, marker) && (len(line) == 3 || line[3] == ' ' || line[3] == '\t')
}

// splitLines splits a stream at every line break YAML admits: a line feed, a carriage return and line feed, or a
// carriage return alone.
func splitLines(src []byte) []string {
	normalized := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))

	return strings.Split(string(normalized), "\n")
}
