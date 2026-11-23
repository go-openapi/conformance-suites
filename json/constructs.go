// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Construct is one structural thing a document may contain.
type Construct struct {
	Name string
	has  func(*lexed) bool
}

// Has reports whether src contains the construct.
func (c Construct) Has(src []byte) bool { return c.has(lex(src)) }

// Constructs are the structural things a document may contain, recognized in its bytes.
//
// # Two callers, deliberately one list
//
// TestTheGeneratedCorpusIsWiderThanTheSuite compares the corpus with JSONTestSuite on them, and [Build.Write] keeps
// every document bringing a combination of them that nothing before it held. One list stops the census reporting a
// construct as covered while the minimizer throws away the only document that had it.
//
// # Why the grammar coverage cannot say this
//
// JSONTestSuite enters every one of the 33 reachable productions, and so does the corpus, so the coverage the
// minimizer keys on is saturated. A tab and a space between two tokens enter the same production, ws, and so do "\b"
// and "\n" inside a string. The difference is which characters, and a construct names it.
//
// # Reading the bytes
//
// Every predicate reads the result of one lexing pass, which tells the inside of a string from the outside. A
// number construct never fires on digits inside a string, and an escape construct never fires outside one. The
// pass tolerates broken documents: it reads what it can and stops at nothing.
//
// The list is hand-maintained, so it has the gap it exists to find: a construct nobody names here is invisible to
// both callers. The dangerous direction is a predicate too BROAD, which reports a construct as covered when it is not.
func Constructs() []Construct { return constructs }

// ConstructSet names every construct a document holds, as one string.
func ConstructSet(src []byte) string {
	l := lex(src)

	var b strings.Builder

	for _, c := range constructs {
		if c.has(l) {
			b.WriteString(c.Name)
			b.WriteByte(0x1f)
		}
	}

	return b.String()
}

var constructs = []Construct{
	{"a top-level scalar", func(l *lexed) bool { return l.topScalar }},
	{"an empty array", func(l *lexed) bool { return l.emptyArray }},
	{"an empty object", func(l *lexed) bool { return l.emptyObject }},
	{"an array inside an array", func(l *lexed) bool { return l.arrayInArray }},
	{"an object inside an object", func(l *lexed) bool { return l.objectInObject }},
	{"nesting deeper than 8", func(l *lexed) bool { return l.maxDepth > 8 }},
	{"a duplicate name in one object", func(l *lexed) bool { return l.duplicateName }},
	{"an empty name", func(l *lexed) bool { return l.emptyName }},
	{"an empty string value", func(l *lexed) bool { return l.emptyValue }},
	{"true", func(l *lexed) bool { return l.literals["true"] }},
	{"false", func(l *lexed) bool { return l.literals["false"] }},
	{"null", func(l *lexed) bool { return l.literals["null"] }},
	{"a negative number", func(l *lexed) bool { return l.numbers.negative }},
	{"a negative zero", func(l *lexed) bool { return l.numbers.negativeZero }},
	{"a fraction", func(l *lexed) bool { return l.numbers.fraction }},
	{"an exponent written e", func(l *lexed) bool { return l.numbers.lowerE }},
	{"an exponent written E", func(l *lexed) bool { return l.numbers.upperE }},
	{"an exponent with a plus sign", func(l *lexed) bool { return l.numbers.plusExponent }},
	{"an exponent with a minus sign", func(l *lexed) bool { return l.numbers.minusExponent }},
	{"a number of more than 17 digits", func(l *lexed) bool { return l.numbers.longDigits }},
	{`an escaped quote \"`, func(l *lexed) bool { return l.escapes['"'] }},
	{`an escaped backslash \\`, func(l *lexed) bool { return l.escapes['\\'] }},
	{`an escaped solidus \/`, func(l *lexed) bool { return l.escapes['/'] }},
	{`an escaped backspace \b`, func(l *lexed) bool { return l.escapes['b'] }},
	{`an escaped form feed \f`, func(l *lexed) bool { return l.escapes['f'] }},
	{`an escaped line feed \n`, func(l *lexed) bool { return l.escapes['n'] }},
	{`an escaped carriage return \r`, func(l *lexed) bool { return l.escapes['r'] }},
	{`an escaped tab \t`, func(l *lexed) bool { return l.escapes['t'] }},
	{`a \u escape`, func(l *lexed) bool { return l.escapes['u'] }},
	{`an escaped NUL \u0000`, func(l *lexed) bool { return l.escapedNUL }},
	{"an escaped surrogate pair", func(l *lexed) bool { return l.surrogatePair }},
	{"an escaped lone surrogate", func(l *lexed) bool { return l.loneSurrogate }},
	{"a raw character outside ASCII", func(l *lexed) bool { return l.rawNonASCII }},
	{"a space between tokens", func(l *lexed) bool { return l.whitespace[' '] }},
	{"a tab between tokens", func(l *lexed) bool { return l.whitespace['\t'] }},
	{"a line feed between tokens", func(l *lexed) bool { return l.whitespace['\n'] }},
	{"a carriage return between tokens", func(l *lexed) bool { return l.whitespace['\r'] }},
	{"whitespace before the value", func(l *lexed) bool { return l.leadingSpace }},
	{"whitespace after the value", func(l *lexed) bool { return l.trailingSpace }},
	{"a byte order mark", func(l *lexed) bool { return l.bom }},
}

// lexed is what one pass over a document found.
type lexed struct {
	topScalar, emptyArray, emptyObject   bool
	arrayInArray, objectInObject         bool
	maxDepth                             int
	duplicateName, emptyName, emptyValue bool
	literals                             map[string]bool
	numbers                              numberForms
	escapes                              map[byte]bool
	escapedNUL, surrogatePair            bool
	loneSurrogate, rawNonASCII           bool
	whitespace                           map[byte]bool
	leadingSpace, trailingSpace, bom     bool
}

type numberForms struct {
	negative, negativeZero, fraction bool
	lowerE, upperE                   bool
	plusExponent, minusExponent      bool
	longDigits                       bool
}

// frame is one open array or object.
type frame struct {
	object bool
	empty  bool
	names  map[string]bool
	// expectName is true in an object where the next string is a member name.
	expectName bool
}

const bomBytes = "\xef\xbb\xbf"

func lex(src []byte) *lexed {
	l := &lexed{literals: map[string]bool{}, escapes: map[byte]bool{}, whitespace: map[byte]bool{}}

	s := string(src)
	if strings.HasPrefix(s, bomBytes) {
		l.bom = true
		s = s[len(bomBytes):]
	}

	x := &lexer{l: l, s: s}
	lastToken := 0 // offset just past the last token

	for i := 0; i < len(s); {
		c := s[i]

		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			x.space(c)
			i++

			continue
		case c == '[' || c == '{':
			x.open(c == '{')
			i++
		case c == ']' || c == '}':
			x.close()
			i++
		case c == ',':
			x.comma()
			i++
		case c == '"':
			i = x.str(i)
		case c == '-' || (c >= '0' && c <= '9'):
			i = x.scalar(l.number(s, i))
		case c >= 'a' && c <= 'z':
			i = x.word(i)
		default: // ':' and any byte that is not JSON
			x.tokens++
			i++
		}

		lastToken = i
	}

	if x.tokens > 0 && lastToken < len(s) {
		l.trailingSpace = true
	}

	return l
}

// lexer is the state of one pass: the open collections and how many tokens came before.
type lexer struct {
	l      *lexed
	s      string
	stack  []*frame
	tokens int
}

func (x *lexer) top() *frame {
	if len(x.stack) == 0 {
		return nil
	}

	return x.stack[len(x.stack)-1]
}

func (x *lexer) space(c byte) {
	switch {
	case x.tokens == 0:
		x.l.leadingSpace = true
	case len(x.stack) > 0:
		x.l.whitespace[c] = true
	}
}

func (x *lexer) open(object bool) {
	if f := x.top(); f != nil {
		f.empty = false
		x.l.arrayInArray = x.l.arrayInArray || (!object && !f.object)
		x.l.objectInObject = x.l.objectInObject || (object && f.object)
	}

	x.tokens++
	x.stack = append(x.stack, &frame{object: object, empty: true, names: map[string]bool{}, expectName: object})
	x.l.maxDepth = max(x.l.maxDepth, len(x.stack))
}

func (x *lexer) close() {
	if f := x.top(); f != nil {
		x.l.emptyObject = x.l.emptyObject || (f.empty && f.object)
		x.l.emptyArray = x.l.emptyArray || (f.empty && !f.object)
		x.stack = x.stack[:len(x.stack)-1]
	}

	x.tokens++
}

func (x *lexer) comma() {
	if f := x.top(); f != nil && f.object {
		f.expectName = true
	}

	x.tokens++
}

// scalar records a value that is not a collection, and returns end.
func (x *lexer) scalar(end int) int {
	if len(x.stack) == 0 && x.tokens == 0 {
		x.l.topScalar = true
	}

	if f := x.top(); f != nil {
		f.empty = false
	}

	x.tokens++

	return end
}

// str reads a string at i, as a member name or as a value, and returns the offset past it.
func (x *lexer) str(i int) int {
	text, end := x.l.str(x.s, i)

	f := x.top()
	if f == nil || !f.object || !f.expectName {
		x.l.emptyValue = x.l.emptyValue || text == ""

		return x.scalar(end)
	}

	x.l.emptyName = x.l.emptyName || text == ""
	x.l.duplicateName = x.l.duplicateName || f.names[text]
	f.names[text] = true
	f.expectName = false
	f.empty = false
	x.tokens++

	return end
}

// word reads a run of lower-case letters at i, and returns the offset past it.
func (x *lexer) word(i int) int {
	end := i
	for end < len(x.s) && x.s[end] >= 'a' && x.s[end] <= 'z' {
		end++
	}

	switch w := x.s[i:end]; w {
	case "true", "false", "null":
		x.l.literals[w] = true
	}

	return x.scalar(end)
}

// str reads the string starting at the quote at i, and returns a key for its value and the offset past its closing
// quote. Two strings have equal keys exactly when they denote the same code units; an empty string has an empty key.
func (l *lexed) str(s string, i int) (string, int) {
	j := i + 1

	// The string's value, as UTF-16 code units, so that two spellings of one
	// name compare equal: "a" and "a" are the same name. Code units and
	// not runes, because a lone surrogate has no rune, and two different ones
	// would both decode to U+FFFD.
	var units []uint16

	for j < len(s) {
		c := s[j]

		switch {
		case c == '"':
			return unitsKey(units), j + 1

		case c == '\\' && j+1 < len(s):
			e := s[j+1]
			l.escapes[e] = true

			if e == 'u' && j+6 <= len(s) {
				hex := s[j+2 : j+6]
				l.unicodeEscape(s, j, hex)

				if v, ok := hexValue(hex); ok {
					units = append(units, uint16(v)) // four hex digits fit in 16 bits
				}

				j += 6

				continue
			}

			units = append(units, uint16(shortEscape(e)))
			j += 2

		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRuneInString(s[j:])
			if r != utf8.RuneError || size > 1 {
				l.rawNonASCII = true
			}

			units = utf16.AppendRune(units, r)
			j += size

		default:
			units = append(units, uint16(c))
			j++
		}
	}

	return unitsKey(units), j
}

// shortEscape returns the character a two-character escape stands for, or the
// escaped byte itself where the escape is not JSON.
func shortEscape(e byte) byte {
	switch e {
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	default: // '"', '\\', '/', and bytes no escape names
		return e
	}
}

// unitsKey writes code units as a string that compares equal exactly when the
// units do.
func unitsKey(units []uint16) string {
	var b strings.Builder

	for _, u := range units {
		b.WriteByte(byte(u >> 8))
		b.WriteByte(byte(u))
	}

	return b.String()
}

// unicodeEscape classifies the \u escape at j, whose four hex digits are hex.
func (l *lexed) unicodeEscape(s string, j int, hex string) {
	v, ok := hexValue(hex)
	if !ok {
		return
	}

	switch {
	case v == 0:
		l.escapedNUL = true
	case v >= 0xD800 && v <= 0xDBFF:
		next := j + 6
		if next+6 <= len(s) && s[next] == '\\' && s[next+1] == 'u' {
			if low, ok := hexValue(s[next+2 : next+6]); ok && low >= 0xDC00 && low <= 0xDFFF {
				l.surrogatePair = true

				return
			}
		}

		l.loneSurrogate = true
	case v >= 0xDC00 && v <= 0xDFFF:
		// A low surrogate after a high one was counted with the pair.
		if prev := j - 6; prev >= 0 && s[prev] == '\\' && s[prev+1] == 'u' {
			if high, ok := hexValue(s[prev+2 : prev+6]); ok && high >= 0xD800 && high <= 0xDBFF {
				return
			}
		}

		l.loneSurrogate = true
	}
}

func hexValue(hex string) (int, bool) {
	v := 0

	for i := range len(hex) {
		c := hex[i]

		switch {
		case c >= '0' && c <= '9':
			v = v<<4 | int(c-'0')
		case c >= 'a' && c <= 'f':
			v = v<<4 | int(c-'a'+10)
		case c >= 'A' && c <= 'F':
			v = v<<4 | int(c-'A'+10)
		default:
			return 0, false
		}
	}

	return v, true
}

// number reads the number starting at i and returns the offset past it.
func (l *lexed) number(s string, i int) int {
	j := i
	for j < len(s) && strings.IndexByte("+-.0123456789eE", s[j]) >= 0 {
		j++
	}

	text := s[i:j]
	mantissa, exponent, hasExponent := strings.Cut(strings.ReplaceAll(text, "E", "e"), "e")

	if strings.HasPrefix(mantissa, "-") {
		l.numbers.negative = true

		if strings.Trim(mantissa, "-0.") == "" && strings.Contains(mantissa, "0") {
			l.numbers.negativeZero = true
		}
	}

	if strings.Contains(mantissa, ".") {
		l.numbers.fraction = true
	}

	if hasExponent {
		if strings.Contains(text, "e") {
			l.numbers.lowerE = true
		}

		if strings.Contains(text, "E") {
			l.numbers.upperE = true
		}

		switch {
		case strings.HasPrefix(exponent, "+"):
			l.numbers.plusExponent = true
		case strings.HasPrefix(exponent, "-"):
			l.numbers.minusExponent = true
		}
	}

	if digits := strings.Count(mantissa, "") - 1 - strings.Count(mantissa, "-") - strings.Count(mantissa, "."); digits > 17 {
		l.numbers.longDigits = true
	}

	return j
}
