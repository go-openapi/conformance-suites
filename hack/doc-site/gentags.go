// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// gentags writes docs/doc-site/reference/tags.md from each grammar's vocabulary.
//
// Usage, from the repository root, with go.work in place:
//
//	go run hack/doc-site/gentags.go
//
// The page lists every tag JSON and YAML place: its stage, whether the specification settles it, and the first
// paragraph of the doc comment on its constant. It reads the stages from jsoncorpus.Vocabulary and
// yamlbuild.Vocabulary, the settled outcomes from yamlbuild.AllRules, and the comments from the Go sources declaring
// the tags.
//
// The yamlbuild import is why this is a script and not a package of the root module: the root module does not
// require yaml/yamlgen, so only the workspace resolves it.
//
// It refuses to write the page when a placed tag has no doc comment, and it reports a declared tag that no
// vocabulary places.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	jsoncorpus "github.com/go-openapi/conformance-suites/json"
	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/conformance-suites/yaml/yamlgen/yamlbuild"
)

// sources are the packages declaring tags, relative to the repository root.
var sources = []string{"stance", "json", "yaml/yamlgen/yamlbuild"}

const page = "docs/doc-site/reference/tags.md"

// familyTitles names the sections of the YAML table, by the namespace a tag opens with.
var familyTitles = map[string]string{
	"anchor":         "Anchors and aliases",
	"directive":      "Directives",
	"json":           "JSON representation",
	"key":            "Mapping keys",
	"merge":          "Merge keys",
	"representation": "Representation in a model",
	"schema":         "Schema resolution",
	"separation":     "Separation",
	"tag":            "Tags on nodes",
}

// declared is a tag constant read from the sources.
type declared struct {
	ident string
	doc   string
}

func main() {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	docs, err := readDeclarations(root)
	if err != nil {
		fail("%v", err)
	}

	jsonVocabulary := jsoncorpus.Vocabulary()
	yamlVocabulary := yamlbuild.Vocabulary()
	rules := yamlbuild.AllRules()

	var missing []string
	for _, v := range []stance.Vocabulary{jsonVocabulary, yamlVocabulary} {
		for _, tag := range v.Tags() {
			if docs[tag].doc == "" {
				missing = append(missing, tag)
			}
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		fail("these tags are placed and carry no doc comment: %s", strings.Join(missing, ", "))
	}
	for tag, d := range docs {
		if _, inJSON := jsonVocabulary.Of(stance.Tag(tag)); inJSON {
			continue
		}
		if _, inYAML := yamlVocabulary.Of(stance.Tag(tag)); inYAML {
			continue
		}
		fmt.Fprintf(os.Stderr, "gentags: %s (%s) is declared and no vocabulary places it\n", tag, d.ident)
	}

	var out bytes.Buffer
	writeHeader(&out)

	shared, jsonOnly := split(jsonVocabulary.Tags(), func(tag string) bool { return strings.HasPrefix(tag, "encoding/") })
	writeSection(&out, "## Encoding", "Declared in package `stance` and placed by the JSON grammar. A question about "+
		"what the bytes denote, settled before parsing starts.", shared, jsonVocabulary, nil, docs)
	writeSection(&out, "## JSON", "Placed by `jsoncorpus.Vocabulary`. RFC 8259 settles none of them: each one is a "+
		"position a parser takes.", jsonOnly, jsonVocabulary, nil, docs)

	out.WriteString("## YAML\n\nPlaced by `yamlbuild.Vocabulary`. A tag the specification settles carries the clause " +
		"and the outcome it fixes, from `yamlbuild.AllRules`.\n\n")
	for _, family := range families(yamlVocabulary.Tags()) {
		title, known := familyTitles[family]
		if !known {
			title = family
		}
		var tags []string
		for _, tag := range yamlVocabulary.Tags() {
			if namespace(tag) == family {
				tags = append(tags, tag)
			}
		}
		writeSection(&out, "### "+title, "", tags, yamlVocabulary, rules, docs)
	}

	if err := os.WriteFile(filepath.Join(root, page), out.Bytes(), 0o600); err != nil {
		fail("%v", err)
	}
	fmt.Printf("gentags: wrote %s\n", page)
}

func writeHeader(out *bytes.Buffer) {
	out.WriteString(`---
title: Tags
weight: 20
description: |
  Every tag, its stage, and whether the specification settles it.
---

{{% notice style="info" title="Generated" %}}
This page is written by ` + "`go run hack/doc-site/gentags.go`" + ` from each grammar's ` + "`Vocabulary()`" + `.
Edit the doc comment on the tag's constant, not this page.
{{% /notice %}}

A tag names an implementation-defined property a document exhibits. Every case in a corpus lists its tags, and the
file's header repeats each tag with its stage and, where the specification settles it, the outcome.

The stage is the earliest point at which the question can be asked: ` + "`parse`, `compose` or `construct`" + `.
A consumer that answers it later answers the same question. "Settled" means the specification fixes the outcome,
so no stance can change it; "open" means each parser takes its own position.

`)
}

// writeSection writes a table of tags. rules is nil for a grammar the specification settles nothing for.
func writeSection(
	out *bytes.Buffer, heading, intro string, tags []string, v stance.Vocabulary, rules stance.Rules,
	docs map[string]declared,
) {
	if len(tags) == 0 {
		return
	}
	out.WriteString(heading + "\n\n")
	if intro != "" {
		out.WriteString(intro + "\n\n")
	}
	out.WriteString("| Tag | Stage | Specification | What it marks |\n|---|---|---|---|\n")
	for _, tag := range tags {
		stage, _ := v.Of(stance.Tag(tag))
		settled := "open"
		if rule, ok := rules.Of(stance.Tag(tag)); ok {
			settled = settledBy(rule)
		}
		fmt.Fprintf(out, "| `%s` | %s | %s | %s |\n", tag, stage, cell(literals(settled)), cell(literals(description(docs[tag]))))
	}
	out.WriteString("\n")
}

// settledBy writes the outcome a rule fixes and the clause fixing it.
//
// A rule's Because opens on the clause it cites, as in "7.1: the anchor must precede the alias", which becomes
// "Reject, section 7.1: the anchor must precede the alias".
func settledBy(rule stance.Rule) string {
	outcome := rule.Then.String()
	outcome = strings.ToUpper(outcome[:1]) + strings.ToLower(outcome[1:])
	clause, reason, cited := strings.Cut(rule.Because, ": ")
	if !cited || strings.ContainsAny(clause, " ") {
		return outcome + ": " + rule.Because
	}

	return outcome + ", section " + clause + ": " + reason
}

// description turns a constant's doc comment into the sentence a table cell holds: the first paragraph, without
// the identifier it opens with.
//
// "TagAliasForward is an alias naming an anchor that occurs later." becomes "An alias naming an anchor that occurs
// later."
func description(d declared) string {
	text := d.doc
	for _, verb := range []string{" is ", " marks "} {
		if rest, ok := strings.CutPrefix(text, d.ident+verb); ok {
			text = rest
			break
		}
	}
	if text == "" {
		return text
	}

	return strings.ToUpper(text[:1]) + text[1:]
}

// quoted matches the YAML or JSON text a doc comment quotes: "<<", "!!int abc", "1e3".
var quoted = regexp.MustCompile(`"([^"\s][^"]*)"`)

// literals writes the text a comment quotes, and a \uXXXX escape, as inline code.
//
// Left as prose, the site's typographer reads them as quotations: "<<" came out as a pair of curly quotes around a
// guillemet.
func literals(s string) string {
	s = strings.ReplaceAll(s, `\uXXXX`, "`\\uXXXX`")

	return quoted.ReplaceAllString(s, "`$1`")
}

// cell escapes what would end a Markdown table cell.
func cell(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

func namespace(tag string) string {
	ns, _, _ := strings.Cut(tag, "/")

	return ns
}

func families(tags []string) []string {
	var out []string
	for _, tag := range tags {
		if ns := namespace(tag); !slices.Contains(out, ns) {
			out = append(out, ns)
		}
	}
	slices.Sort(out)

	return out
}

func split(tags []string, first func(string) bool) (yes, no []string) {
	for _, tag := range tags {
		if first(tag) {
			yes = append(yes, tag)
		} else {
			no = append(no, tag)
		}
	}

	return yes, no
}

// readDeclarations reads every constant of type stance.Tag in the source packages, by its value.
func readDeclarations(root string) (map[string]declared, error) {
	out := make(map[string]declared)
	for _, dir := range sources {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, filepath.Join(root, dir), func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				readFile(file, out)
			}
		}
	}

	return out, nil
}

func readFile(file *ast.File, out map[string]declared) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			if !isTagType(value.Type) {
				continue
			}
			doc := value.Doc.Text()
			if doc == "" && len(gen.Specs) == 1 {
				doc = gen.Doc.Text()
			}
			for i, expr := range value.Values {
				lit, isString := expr.(*ast.BasicLit)
				if !isString || lit.Kind != token.STRING {
					continue
				}
				tag, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				out[tag] = declared{ident: value.Names[i].Name, doc: firstParagraph(doc)}
			}
		}
	}
}

// isTagType reports whether a constant is declared as stance.Tag, or as Tag inside package stance.
func isTagType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Tag"
	case *ast.SelectorExpr:
		pkg, ok := t.X.(*ast.Ident)

		return ok && pkg.Name == "stance" && t.Sel.Name == "Tag"
	default:
		return false
	}
}

func firstParagraph(doc string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(doc), "\n\n")

	return strings.Join(strings.Fields(first), " ")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gentags: "+format+"\n", args...)
	os.Exit(1)
}
