---
title: Fuzz and property tests in Go
weight: 30
description: |
  Seed a Go fuzz target with a corpus, and draw YAML documents for property tests with rapid.
---

Scoring a parser against a corpus checks the cases the corpus holds. A fuzz target and a property test go past
them. The fuzzer mutates the corpus documents, and `yamlgen` draws new documents for
[rapid](https://github.com/flyingmutant/rapid). In both, the oracle gives the verdict on any document,
so a test can check documents nobody wrote down.

The examples below test [go-openapi/go-yaml](https://github.com/go-openapi/go-yaml). Replace `yaml.Unmarshal`
with your parser's entry point.

## Install

```cmd
go get github.com/go-openapi/conformance-suites
go get github.com/go-openapi/conformance-suites/yaml/corpus   # the embedded YAML corpus, 1.6 MB
go get github.com/go-openapi/conformance-suites/yaml/yamlgen  # the YAML generator, which requires rapid
```

Each is a module of its own, so a test requires only what it uses.

## Seed a fuzz target

Add every corpus document as a seed. The corpus already covers the grammar, so the fuzzer starts there and spends
its time on combinations nobody generated.

```go
import (
	"testing"

	yamlcorpus "github.com/go-openapi/conformance-suites/yaml"
	yamlembed "github.com/go-openapi/conformance-suites/yaml/corpus"
	"github.com/go-openapi/go-yaml"
)

func FuzzUnmarshal(f *testing.F) {
	_, cases, err := yamlembed.CompactSuite()
	if err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		f.Add(c.Src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		var v any
		err := yaml.Unmarshal(src, &v)
		if err == nil && !yamlcorpus.Stream(src).OK {
			t.Errorf("accepted a document the YAML 1.2 grammar rejects:\n%q", src)
		}
	})
}
```

The check runs in one direction only. A parser that accepts a document the grammar rejects has a defect, unless
you declare that leniency. A parser that refuses a document the grammar accepts may be right: the specification
sets rules beyond the grammar, such as unique keys, and your parser takes positions on the questions it leaves open.
To check that direction too, give each document a verdict with your `stance.Table`, as
[Score a parser](../score-a-parser/) does for the corpus cases.

`go test` without `-fuzz` runs each seed once: 25,175 distinct documents, about 6 seconds with go-yaml.
With `-fuzz`, Go first measures the coverage of every seed, which takes about a minute before the fuzzer starts
mutating. Seed a subset, such as the refused cases, to start sooner.

go-yaml's own [`fuzz_test.go`](https://github.com/go-openapi/go-yaml/blob/master/fuzz_test.go) adds the YAML Test
Suite and the inputs reduced from past bug reports to the same seeds.

### JSON

RFC 8259 leaves little to the implementation, so a JSON fuzz target can compare verdicts in both directions:

```go
import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	jsoncorpus "github.com/go-openapi/conformance-suites/json"
)

func FuzzValid(f *testing.F) {
	_, cases, err := jsoncorpus.CompactSuite()
	if err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		f.Add(c.Src)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		if !utf8.Valid(src) {
			return // json.Valid does not check UTF-8: the corpus tags this encoding/not-utf8
		}
		if got, want := json.Valid(src), jsoncorpus.Text(src).OK; got != want {
			t.Errorf("json.Valid says %v, RFC 8259 says %v:\n%q", got, want, src)
		}
	})
}
```

The JSON corpus is embedded in the root module, so this needs no other module.

## Draw documents with rapid

`yamlgen.Values` draws what a document means and `yamlgen.Styles` draws how to write it. `yamlgen.Emit` writes a
value in a style, and every document it writes is valid YAML 1.2. Two properties follow, and neither needs an
expected value.

### Every style reads the same

Write one value in two styles. A parser must read both documents as the same value.

```go
import (
	"bytes"
	"testing"

	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/go-yaml"
	"pgregory.net/rapid"
)

func TestEveryStyleReadsTheSame(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		sa, sb := yamlgen.Styles().Draw(rt, "style a"), yamlgen.Styles().Draw(rt, "style b")
		if sa.Version != "" || sb.Version != "" {
			return // "%YAML 1.1" changes what some spellings mean
		}
		a, b := yamlgen.Emit(value, sa), yamlgen.Emit(value, sb)

		var va, vb any
		ea, eb := yaml.Unmarshal([]byte(a), &va), yaml.Unmarshal([]byte(b), &vb)
		switch {
		case ea != nil && eb != nil:
			return // the parser refuses this value in any style: a position, not a presentation defect
		case ea != nil || eb != nil:
			rt.Fatalf("one style refused:\n%s\nerror: %v\n\nthe other:\n%s\nerror: %v", a, ea, b, eb)
		}

		// Compare what the parser writes back: one value has one canonical form.
		ca, _ := yaml.Marshal(va)
		cb, _ := yaml.Marshal(vb)
		if !bytes.Equal(ca, cb) {
			rt.Fatalf("one value, two readings:\n%s\nreads as\n%s\n%s\nreads as\n%s", a, ca, b, cb)
		}
	})
}
```

Three choices keep this property free of false failures:

- A style that declares `%YAML 1.1` is skipped. Under YAML 1.1, `<<` is a merge key and some plain spellings
  resolve to other types, so the two documents can mean different things.
- A value that both styles fail to read is skipped. go-yaml, for example, refuses a mapping as a key when it reads
  into `any`. That is a position the parser takes, and it does not depend on the style.
- The two readings are compared through `yaml.Marshal`. Comparing Go values directly fails on NaN, and printing
  them shows the addresses of pointers such as `*big.Float`.

### No broken document is accepted

`yamlgen.Mutate` breaks a valid document. Many mutations leave a document that is still valid YAML, so ask the
oracle first, and keep only the documents it rejects.

```go
import (
	"testing"

	yamlcorpus "github.com/go-openapi/conformance-suites/yaml"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen"
	"github.com/go-openapi/go-yaml"
	"pgregory.net/rapid"
)

func TestNoMutantIsAccepted(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		src := yamlgen.Emit(yamlgen.Values().Draw(rt, "value"), yamlgen.Styles().Draw(rt, "style"))
		mutant, how := yamlgen.Mutate(rt, src)
		if yamlcorpus.Stream([]byte(mutant)).OK {
			return // still YAML: the mutation broke nothing
		}

		var v any
		if err := yaml.Unmarshal([]byte(mutant), &v); err == nil {
			rt.Fatalf("accepted a document the YAML 1.2 grammar rejects (%s):\n%q", how, mutant)
		}
	})
}
```

`how` names the mutation, such as `a-document-marker-inside`, so a failure tells you what kind of break the parser
missed.

### When a property fails

rapid shrinks the drawn value and style to a small failing case, and prints a `-rapid.failfile` flag that replays
it. Raise the number of draws with `-rapid.checks`: the default is 100, and the rarer shapes appear only after
thousands of draws.

go-yaml's tests go further. They compare each document with the value the generator meant (`yamlgen.Write`
returns it as `Written.Means`), and they keep a ledger of known divergences (`yamlgen.Divergence`), so that a known
defect does not stop the run and a fixed one is noticed.
