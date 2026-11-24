// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

//go:embed templates/*.html
var templates embed.FS

// The statuses a source gets against the corpus.
const (
	statusOK        = "ok"
	statusDiff      = "diff"
	statusDiffTag   = "diff, tagged"
	statusError     = "error"
	statusUnscored  = "unscored"
	statusRefusedOK = "refused-correctly"
	statusAccepted  = "accepted-wrongly"
	statusRefused   = "refused"
	statusNoAnswer  = "n/i"
	statusUnanimous = "unanimous"
	statusSplit     = "split"
)

// question is one of the three things the report compares.
type question struct {
	ID, Title string
	// Statuses are the columns of the summary table, in order.
	Statuses []string
}

var questions = []question{
	{ID: "verdict", Title: "Verdict, against the corpus", Statuses: []string{
		statusOK, statusDiff, statusDiffTag, statusRefusedOK, statusAccepted, statusError,
	}},
	{ID: "value", Title: "Value, against the meaning the corpus states", Statuses: []string{
		statusOK, statusDiff, statusDiffTag, statusRefused, statusUnscored, statusNoAnswer, statusError,
	}},
	{ID: "events", Title: "Events, between the sources (the corpus holds none)", Statuses: []string{
		statusUnanimous, statusSplit, statusNoAnswer,
	}},
}

// judged is a record with the comparisons made.
type judged struct {
	record

	// Status maps a question to each source's status, by source index.
	Status map[string][]string
	// Groups maps a question to the answer groups the sources fall into.
	Groups map[string][]consensus.Group
	// Split names the questions the sources answer differently.
	Split map[string]bool
}

// Short is the case's file name in the report.
func (j judged) Short() string { return j.SHA[:16] }

func (j judged) notable() bool {
	if len(j.Split) > 0 {
		return true
	}
	for _, statuses := range j.Status {
		for _, s := range statuses {
			if s == statusDiff || s == statusDiffTag || s == statusAccepted || s == statusError {
				return true
			}
		}
	}

	return false
}

func judge(rec record, sources []sourceInfo) judged {
	j := judged{record: rec, Status: map[string][]string{}, Groups: map[string][]consensus.Group{}, Split: map[string]bool{}}

	verdicts, events, values := map[string]string{}, map[string]string{}, map[string]string{}
	j.Status["verdict"] = make([]string, len(rec.Answers))
	j.Status["value"] = make([]string, len(rec.Answers))
	j.Status["events"] = make([]string, len(rec.Answers))

	tagged := len(rec.Tags) > 0
	meaning := ""
	if rec.Meaning != nil && !rec.Meaning.Cyclic && len(rec.Meaning.JSON) > 0 {
		meaning, _ = consensus.CanonicalJSON(string(rec.Meaning.JSON))
	}

	for i, a := range rec.Answers {
		if a.Verdict == consensus.Error {
			j.Status["verdict"][i], j.Status["value"][i], j.Status["events"][i] = statusError, statusError, statusNoAnswer

			continue
		}

		// The verdict is the parser's: a refusal after a clean parse is the parser accepting.
		parsed := a.Verdict == consensus.Accept || a.RefusedBy == consensus.Loader
		verdicts[a.Source] = map[bool]string{true: "accept", false: "refuse"}[parsed]
		switch {
		case rec.WellFormed && parsed:
			j.Status["verdict"][i] = statusOK
		case rec.WellFormed && tagged:
			j.Status["verdict"][i] = statusDiffTag
		case rec.WellFormed:
			j.Status["verdict"][i] = statusDiff
		case parsed:
			j.Status["verdict"][i] = statusAccepted
		default:
			j.Status["verdict"][i] = statusRefusedOK
		}

		if parsed && len(a.Events) > 0 {
			events[a.Source] = strings.Join(a.Events, "\n")
		}

		switch {
		case !sources[i].Values:
			j.Status["value"][i] = statusNoAnswer
		case a.Verdict != consensus.Accept:
			j.Status["value"][i] = statusRefused
			if !rec.WellFormed {
				j.Status["value"][i] = statusUnscored
			}
		case a.JSON == nil:
			j.Status["value"][i] = statusNoAnswer
		default:
			values[a.Source] = strings.Join(a.JSON, "\n")
			j.Status["value"][i] = valueStatus(rec, a, meaning, tagged)
		}
	}

	for q, answers := range map[string]map[string]string{"verdict": verdicts, "events": events, "value": values} {
		groups := consensus.Groups(answers)
		j.Groups[q] = groups
		if len(groups) > 1 {
			j.Split[q] = true
		}
	}

	for i, a := range rec.Answers {
		if _, answered := events[a.Source]; !answered {
			j.Status["events"][i] = statusNoAnswer

			continue
		}
		j.Status["events"][i] = statusUnanimous
		if j.Split["events"] {
			j.Status["events"][i] = statusSplit
		}
	}

	return j
}

func valueStatus(rec record, a consensus.Answer, meaning string, tagged bool) string {
	switch {
	case rec.Meaning == nil || len(a.JSON) != 1:
		return statusUnscored
	case rec.Meaning.Cyclic:
		if a.JSON[0] == "" {
			return statusOK
		}
	case a.JSON[0] == meaning:
		return statusOK
	}
	if tagged {
		return statusDiffTag
	}

	return statusDiff
}

// summary counts, per question, each source's statuses, and the split cases.
type summary struct {
	Counts map[string][]map[string]int
	Splits map[string]int
}

func summarize(cases []judged, n int) summary {
	s := summary{Counts: map[string][]map[string]int{}, Splits: map[string]int{}}
	for _, q := range questions {
		s.Counts[q.ID] = make([]map[string]int, n)
		for i := range n {
			s.Counts[q.ID][i] = map[string]int{}
		}
	}
	for _, c := range cases {
		for q, statuses := range c.Status {
			for i, st := range statuses {
				s.Counts[q][i][st]++
			}
		}
		for q := range c.Split {
			s.Splits[q]++
		}
	}

	return s
}

// grid is a table of differences: a row per tag or feature, a column per source.
type grid struct {
	Title, Kind string
	Families    []family
}

type family struct {
	Name string
	Rows []gridRow
}

type gridRow struct {
	Name  string
	Page  string
	Cases int
	// Verdict and Value are each source's difference count against the corpus.
	Verdict, Value []int
	Splits         int
}

func buildGrid(title, kind string, cases []judged, n int, keys func(judged) []string, all []string) grid {
	rows := map[string]*gridRow{}
	row := func(name string) *gridRow {
		r, ok := rows[name]
		if !ok {
			r = &gridRow{Name: name, Page: kind + "/" + pageName(name) + ".html", Verdict: make([]int, n), Value: make([]int, n)}
			rows[name] = r
		}

		return r
	}
	for _, name := range all {
		row(name)
	}
	for _, c := range cases {
		names := keys(c)
		if len(names) == 0 {
			names = []string{"(none)"}
		}
		for _, name := range names {
			r := row(name)
			r.Cases++
			for i := range n {
				if isDiff(c.Status["verdict"][i]) {
					r.Verdict[i]++
				}
				if isDiff(c.Status["value"][i]) {
					r.Value[i]++
				}
			}
			if len(c.Split) > 0 {
				r.Splits++
			}
		}
	}

	byFamily := map[string][]gridRow{}
	for name, r := range rows {
		f, _, _ := strings.Cut(name, "/")
		if !strings.Contains(name, "/") {
			f = ""
		}
		byFamily[f] = append(byFamily[f], *r)
	}
	names := make([]string, 0, len(byFamily))
	for f := range byFamily {
		names = append(names, f)
	}
	sort.Strings(names)

	g := grid{Title: title, Kind: kind}
	for _, f := range names {
		r := byFamily[f]
		sort.Slice(r, func(a, b int) bool { return r[a].Name < r[b].Name })
		g.Families = append(g.Families, family{Name: f, Rows: r})
	}

	return g
}

func isDiff(status string) bool {
	return status == statusDiff || status == statusDiffTag || status == statusAccepted
}

func pageName(name string) string {
	return unsafeName.ReplaceAllString(strings.ReplaceAll(name, "/", "--"), "_")
}

// view is what every page template receives.
type view struct {
	Head      head
	Questions []question
	Summary   summary
	Grid      grid
	Title     string
	Cases     []judged
	Case      judged
	Source    sourceInfo
	Index     int
	Root      string
	Truncated int
}

// writeReport reads a results file and writes the report into dir.
func writeReport(results, dir string) error {
	h, cases, err := readResults(results)
	if err != nil {
		return err
	}
	n := len(h.Sources)

	judgedCases := make([]judged, len(cases))
	for i, c := range cases {
		judgedCases[i] = judge(c, h.Sources)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"status":   func(c judged, q string, i int) string { return c.Status[q][i] },
		"count":    func(s summary, q string, i int, st string) int { return s.Counts[q][i][st] },
		"split":    func(c judged, q string) bool { return c.Split[q] },
		"groups":   func(c judged, q string) []consensus.Group { return c.Groups[q] },
		"answerOf": func(c judged, i int) consensus.Answer { return c.Answers[i] },
		"lines":    func(s []string) string { return strings.Join(s, "\n") },
		"doc":      document,
		"pagename": pageName,
		"meaning": func(c judged) string {
			if c.Meaning == nil {
				return ""
			}
			if c.Meaning.Cyclic {
				return "(cyclic: no JSON form)"
			}

			return string(c.Meaning.JSON)
		},
		"cls": func(s string) string { return "s-" + strings.NewReplacer(" ", "", ",", "", "/", "").Replace(s) },
	}).ParseFS(templates, "templates/*.html")
	if err != nil {
		return err
	}

	sum := summarize(judgedCases, n)
	base := view{Head: h, Questions: questions, Summary: sum}

	var notable []judged
	for _, c := range judgedCases {
		if c.notable() {
			notable = append(notable, c)
		}
	}

	pages := buildPages(h, base, judgedCases, notable)

	for name, v := range pages {
		page := "list.html"
		switch {
		case name == "index.html":
			page = "index.html"
		case name == "tags.html" || name == "features.html":
			page = "grid.html"
		case strings.HasPrefix(name, "case/"):
			page = "case.html"
		}
		if err := writePage(tmpl, page, filepath.Join(dir, name), v); err != nil {
			return err
		}
	}

	return nil
}

// buildPages lays out every page of the report but the templates' own work: the summary, the two grids with a
// list page per row, a page per source, and a page per notable case.
func buildPages(h head, base view, judgedCases, notable []judged) map[string]view {
	n := len(h.Sources)
	pages := map[string]view{"index.html": base}

	vocabulary := make([]string, 0, len(h.Corpus.Vocabulary))
	for _, spec := range h.Corpus.Vocabulary {
		vocabulary = append(vocabulary, spec.Tag)
	}
	tags := buildGrid("Differences by tag", "tag", judgedCases, n, func(c judged) []string { return c.Tags }, vocabulary)
	features := buildGrid("Differences by feature", "feature", judgedCases, n,
		func(c judged) []string { return c.Features }, nil)
	for _, g := range []grid{tags, features} {
		v := base
		v.Grid = g
		pages[g.Kind+"s.html"] = v
		for _, f := range g.Families {
			for _, r := range f.Rows {
				pages[r.Page] = listPage(base, g.Kind, r.Name, notable)
			}
		}
	}

	for i, s := range h.Sources {
		v := base
		v.Root, v.Source, v.Index, v.Title = "../", s, i, s.ID
		for _, c := range notable {
			if isDiff(c.Status["verdict"][i]) || isDiff(c.Status["value"][i]) || c.Status["verdict"][i] == statusError {
				v.Cases = append(v.Cases, c)
			}
		}
		v.Cases, v.Truncated = capped(v.Cases)
		pages["source/"+pageName(s.ID)+".html"] = v
	}

	for _, c := range notable {
		v := base
		v.Root, v.Case, v.Title = "../", c, c.Name
		pages["case/"+c.Short()+".html"] = v
	}

	return pages
}

// listPage is the page listing the notable cases carrying one tag or one feature.
func listPage(base view, kind, name string, notable []judged) view {
	lv := base
	lv.Root, lv.Title = "../", name
	for _, c := range notable {
		keys := keysOf(kind, c)
		if slices.Contains(keys, name) || (name == "(none)" && len(keys) == 0) {
			lv.Cases = append(lv.Cases, c)
		}
	}
	lv.Cases, lv.Truncated = capped(lv.Cases)

	return lv
}

func keysOf(kind string, c judged) []string {
	if kind == "tag" {
		return c.Tags
	}

	return c.Features
}

// capped keeps a list page readable: the first cases, and how many more there are.
func capped(cases []judged) ([]judged, int) {
	const most = 500
	if len(cases) <= most {
		return cases, 0
	}

	return cases[:most], len(cases) - most
}

func writePage(tmpl *template.Template, name, path string, v view) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	if err := tmpl.ExecuteTemplate(w, name, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return w.Flush()
}

// document writes a document for a page: as text when it is UTF-8, quoted byte by byte when it is not.
func document(src []byte) string {
	if utf8.Valid(src) {
		return string(src)
	}

	return fmt.Sprintf("%q\n(not UTF-8: shown as a Go string literal)", src)
}

func readResults(path string) (head, []record, error) {
	f, err := os.Open(path)
	if err != nil {
		return head{}, nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(bufio.NewReader(f))
	var h head
	if err := dec.Decode(&h); err != nil {
		return head{}, nil, fmt.Errorf("%s: reading the first line: %w", path, err)
	}
	var out []record
	for {
		var r record
		if err := dec.Decode(&r); err != nil {
			if err == io.EOF {
				break
			}

			return head{}, nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, r)
	}

	return h, out, nil
}
