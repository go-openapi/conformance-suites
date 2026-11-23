// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsoncorpus_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"

	jsoncorpus "github.com/go-openapi/conformance-suites/json"
)

// TestEachConstructFiresOnlyWhereItShould pins every predicate on a document that holds the construct and on one
// that does not. The second column matters most: a predicate too broad reports a construct as covered when it is not.
func TestEachConstructFiresOnlyWhereItShould(t *testing.T) {
	tests := map[string]struct{ has, hasNot string }{
		"a top-level scalar":               {`1`, `[1]`},
		"an empty array":                   {`[ ]`, `[1]`},
		"an empty object":                  {`{}`, `{"a":1}`},
		"an array inside an array":         {`[[1]]`, `{"a":[1]}`},
		"an object inside an object":       {`{"a":{}}`, `[{}]`},
		"nesting deeper than 8":            {`[[[[[[[[[1]]]]]]]]]`, `[[[[[[[[1]]]]]]]]`},
		"a duplicate name in one object":   {`{"a":1,"a":2}`, `{"a":{"a":1}}`},
		"an empty name":                    {`{"":1}`, `{"a":""}`},
		"an empty string value":            {`{"a":""}`, `{"":1}`},
		"true":                             {`[true]`, `["true"]`},
		"false":                            {`[false]`, `["false"]`},
		"null":                             {`[null]`, `["null"]`},
		"a negative number":                {`[-1]`, `["-1"]`},
		"a negative zero":                  {`-0.0`, `-0.1`},
		"a fraction":                       {`1.5`, `15`},
		"an exponent written e":            {`1e5`, `1E5`},
		"an exponent written E":            {`1E5`, `1e5`},
		"an exponent with a plus sign":     {`1e+5`, `1e5`},
		"an exponent with a minus sign":    {`1e-5`, `-1e5`},
		"a number of more than 17 digits":  {`123456789012345678`, `12345678901234567`},
		`an escaped quote \"`:              {`"\""`, `"'"`},
		`an escaped backslash \\`:          {`"\\"`, `"/"`},
		`an escaped solidus \/`:            {`"\/"`, `"/"`},
		`an escaped backspace \b`:          {`"\b"`, `"\\b"`},
		`an escaped form feed \f`:          {`"\f"`, `"\\f"`},
		`an escaped line feed \n`:          {`"\n"`, `"\\n"`},
		`an escaped carriage return \r`:    {`"\r"`, `"\\r"`},
		`an escaped tab \t`:                {`"\t"`, `"\\t"`},
		`a \u escape`:                      {`"\u0041"`, `"\\u0041"`},
		`an escaped NUL \u0000`:            {`"\u0000"`, `"\u0001"`},
		"an escaped surrogate pair":        {`"\ud834\udd1e"`, `"\ud834"`},
		"an escaped lone surrogate":        {`"\udd1e"`, `"\ud834\udd1e"`},
		"a raw character outside ASCII":    {`"é"`, `"\u00e9"`},
		"a space between tokens":           {`[1, 2]`, `[1,2] `},
		"a tab between tokens":             {"[1,\t2]", `[1,2]`},
		"a line feed between tokens":       {"[1,\n2]", `"\n"`},
		"a carriage return between tokens": {"[1,\r2]", `"\r"`},
		"whitespace before the value":      {` 1`, `1 `},
		"whitespace after the value":       {`1 `, ` 1`},
		"a byte order mark":                {"\xef\xbb\xbf1", "\"\\ufeff\""},
	}

	byName := map[string]jsoncorpus.Construct{}
	for _, c := range jsoncorpus.Constructs() {
		byName[c.Name] = c
	}

	assert.Len(t, tests, len(byName), "every construct has a case")

	for name, tc := range tests {
		c, ok := byName[name]
		if !assert.Truef(t, ok, "no construct named %q", name) {
			continue
		}

		assert.Truef(t, c.Has([]byte(tc.has)), "%s: should hold it: %s", name, tc.has)
		assert.Falsef(t, c.Has([]byte(tc.hasNot)), "%s: should not hold it: %s", name, tc.hasNot)
	}

	assert.True(t, strings.HasSuffix(jsoncorpus.ConstructSet([]byte(`[1, 2]`)), "\x1f"))
}
