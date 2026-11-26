// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

//go:embed templates/*.html
var templates embed.FS

// The statuses a source gets on each question.
const (
	statusOK        = "ok"
	statusDiff      = "diff"
	statusDiffTag   = "diff, tagged"
	statusError     = "error"
	statusUnscored  = "unscored"
	statusRefusedOK = "refused-correctly"
	statusAccepted  = "accepted-wrongly"
	statusRefused   = "refused"
	statusNoValue   = "no value"
	statusNotScored = "not scored"
	statusNoAnswer  = "n/i"
	statusAgree     = "agree"
	statusOnly      = "only"
	statusMajority  = "majority"
	statusMinority  = "minority"
	statusNoMajor   = "no majority"
	statusNoEvents  = "no events"
	statusRTSame    = "same"
	statusRTDiff    = "different"
	statusRTUnwrite = "unwritable"
	statusRTUnread  = "unreadable"
	statusRTCyclic  = "cyclic"
)

// question is one of the three things the report compares, and the summary table it gets on the index.
type question struct {
	ID, Title string
	// Lead is the paragraphs above the table.
	Lead []string
	// Bands group the columns of the table under a heading each.
	Bands []band
	// Split is the sentence below the table, with a %d for the number of split cases.
	Split string
	// Sources are the indexes of the sources the table lists.
	Sources []int
}

// Columns are the columns of every band, in order.
func (q question) Columns() []column {
	var out []column
	for _, b := range q.Bands {
		out = append(out, b.Columns...)
	}

	return out
}

// band is a group of columns under one heading, which may be empty.
type band struct {
	Label   string
	Columns []column
}

// column is one status, with the label the table shows and the tooltip that explains it.
type column struct {
	Status, Label, Help string
}

// corpusStats counts the corpus's documents by what the corpus states about them.
type corpusStats struct {
	Documents, Valid, Invalid, WithValue, Tagged int
	Bytes                                        int64
}

func countCorpus(cases []record) corpusStats {
	var st corpusStats
	for _, c := range cases {
		st.Documents++
		st.Bytes += int64(len(c.Src))
		if c.WellFormed {
			st.Valid++
			if c.Meaning != nil {
				st.WithValue++
			}
		} else {
			st.Invalid++
		}
		if len(c.Tags) > 0 {
			st.Tagged++
		}
	}

	return st
}

// questionsFor writes the three questions' tables for a corpus and a set of sources.
func questionsFor(st corpusStats, sources []sourceInfo) []question {
	all := make([]int, 0, len(sources))
	var values, trips []int
	var noValues []string
	for i, s := range sources {
		all = append(all, i)
		if s.RoundTrip {
			trips = append(trips, i)
		}
		if s.Values {
			values = append(values, i)
		} else {
			noValues = append(noValues, s.ID)
		}
	}
	leftOut := func(ids []string, one, many string) string {
		switch len(ids) {
		case 0:
			return ""
		case 1:
			return fmt.Sprintf(" %s %s, so the table leaves it out.", ids[0], one)
		default:
			return fmt.Sprintf(" %s %s, so the table leaves them out.", joinList(ids), many)
		}
	}

	const stage = " A parser is answerable up to the stage its stance file names. A rejection past it, " +
		"such as libfyaml's Python binding failing after fy-tool parsed the document, counts as accepting."

	return []question{
		{
			ID:    "verdict",
			Title: "Verdict",
			Lead: []string{
				"Verdict on a YAML document being valid or invalid. " + fmt.Sprintf(
					"Each parser accepts or rejects each of the %d documents, and the corpus grammar states "+
						"which is correct.", st.Documents),
				"A rejection of a valid document, or an acceptance of an invalid one, is split by its reason: " +
					"a rule the specification settles beyond the grammar, a position the parser declares in its " +
					"stance file, a known departure the stance file explains, or a tag marking an open question. " +
					"What is left is a defect in the parser or in the corpus. " +
					"The table below the counts lists the reasons.",
			},
			Bands: []band{
				{Label: fmt.Sprintf("valid documents: %d", st.Valid), Columns: []column{
					{statusOK, "accepted", "The parser accepts a document the corpus holds valid." + stage},
					{statusAgainst, "accepted, against the rules", "The parser accepts a document that the grammar " +
						"allows and that a settled rule, or the parser's own declared stance, rejects."},
					{statusSettled, "rejected, settled", "The parser rejects a document the grammar allows " +
						"and a rule the specification settles forbids: an alias naming no anchor, a key written twice."},
					{statusDeclared, "rejected, as declared", "The parser rejects a document carrying a tag " +
						"its stance file declares it refuses."},
					{statusUnscored, "rejected after the parse", "The parser's composer or loader rejects " +
						"a mutant. The corpus vouches for a mutant's syntax only, so the report does not score this."},
					{statusExplained, "rejected, explained", "The parser rejects a valid document, " +
						"and a departure in its stance file explains why."},
					{statusDiffTag, "rejected, tagged", "The parser rejects a valid document carrying a tag " +
						"its stance file says nothing about. The tag marks a question the specification leaves open."},
					{statusDiff, "rejected", "The parser rejects a valid document, and nothing explains it. " +
						"Each of these is a defect in the parser or in the corpus."},
				}},
				{Label: fmt.Sprintf("invalid documents: %d", st.Invalid), Columns: []column{
					{statusRefusedOK, "rejected", "The parser rejects a document the corpus holds invalid."},
					{statusAccExplained, "accepted, explained", "The parser accepts an invalid document, " +
						"and a departure in its stance file explains why."},
					{statusAccTagged, "accepted, tagged", "The parser accepts an invalid document carrying a tag " +
						"its stance file says nothing about, or bytes the corpus cannot decode."},
					{statusAccepted, "accepted", "The parser accepts an invalid document, and nothing explains it. " +
						"Each of these is a defect in the parser or in the corpus." + stage},
				}},
				{Columns: []column{
					{statusError, "error", "The parser crashed, timed out, or wrote output the runner cannot read."},
				}},
			},
			Split:   "The parsers do not all give the same verdict on %d documents.",
			Sources: all,
		},
		{
			ID:    "value",
			Title: "Value",
			Lead: []string{
				fmt.Sprintf("The value a parser builds from a valid document, written as JSON, against the value "+
					"the corpus states. The corpus states a value for %d of its %d valid documents, "+
					"and the table counts those.", st.WithValue, st.Valid) +
					leftOut(noValues, "builds no values", "build no values"),
			},
			Bands: []band{
				{Label: fmt.Sprintf("documents with a stated value: %d", st.WithValue), Columns: []column{
					{statusOK, "same", "The parser builds the value the corpus states."},
					{statusValExplained, "different, explained", "The parser builds another value, " +
						"and a departure in its stance file explains why."},
					{statusDiffTag, "different, tagged", "The parser builds another value, on a document " +
						"carrying a tag its stance file says nothing about. The tag marks a question the " +
						"specification leaves open."},
					{statusDiff, "different", "The parser builds another value, and nothing explains it. " +
						"Each of these is a defect in the parser or in the corpus."},
					{statusNoValue, "no value", "The parser accepts the document and returns no value for it."},
				}},
				{Label: "rejected, so no value: the verdict table counts these too", Columns: []column{
					{statusSettled, "settled", "The parser rejects the document, and a rule the specification " +
						"settles beyond the grammar forbids it."},
					{statusDeclared, "as declared", "The parser rejects a document carrying a tag " +
						"its stance file declares it refuses."},
					{statusUnscored, "after the parse", "The parser's composer or loader rejects the document, " +
						"past the stage the verdict is scored at."},
					{statusExplained, "explained", "The parser rejects the document, " +
						"and a departure in its stance file explains why."},
					{statusValRefTagged, "tagged", "The parser rejects a document carrying a tag " +
						"its stance file says nothing about."},
					{statusValRefused, "unexplained", "The parser rejects the document, and nothing explains it. " +
						"Each of these is a defect in the parser or in the corpus."},
				}},
				{Columns: []column{
					{statusError, "error", "The parser crashed, timed out, or wrote output the runner cannot read."},
				}},
			},
			Split: "The parsers build different values for %d documents, " +
				"counting the documents with no stated value.",
			Sources: values,
		},
		{
			ID:    "roundtrip",
			Title: "Round trip",
			Lead: []string{
				"Each parser writes the value it read back as YAML with its own encoder, reads that again, and " +
					"compares the two values in its own types, so an int that comes back as a float differs. " +
					"yaml.v3 uses yaml.Marshal and yaml.Unmarshal; go-yaml uses codec.NewDecoder, codec.Marshal and " +
					"codec.Unmarshal. The corpus states nothing here: the check is decode(encode(decode(d))) " +
					"against decode(d), on every document the parser accepts.",
			},
			Bands: []band{
				{Label: "documents the parser accepts", Columns: []column{
					{statusRTSame, "same", "Every document comes back as the value first read."},
					{statusRTDiff, "different", "A document comes back as another value. " +
						"The case page shows what the encoder wrote and what came back."},
					{statusRTUnwrite, "cannot write", "The encoder fails on a value the decoder built."},
					{statusRTUnread, "cannot read back", "The decoder rejects what the encoder wrote."},
					{statusRTCyclic, "cyclic", "The value holds a cycle, which the runner does not hand to the encoder."},
				}},
				{Columns: []column{
					{statusRefused, "rejected", "The parser rejects the document, so there is nothing to write back."},
					{statusError, "error", "The parser crashed, timed out, or wrote output the runner cannot read."},
				}},
			},
			Split:   "",
			Sources: trips,
		},
	}
}

// joinList joins ids as English does: "a", "a and b", "a, b and c".
func joinList(ids []string) string {
	switch len(ids) {
	case 0:
		return ""
	case 1:
		return ids[0]
	default:
		return strings.Join(ids[:len(ids)-1], ", ") + " and " + ids[len(ids)-1]
	}
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
	// Why is the reason for each source's verdict status, by source index, nil where none is needed.
	Why []*reason
	// ValueWhy is the reason for each source's value status.
	ValueWhy []*reason
}

// Short is the case's file name in the report: a digest of its name, because two cases may hold the same bytes and
// a seed and its mutant may be scored differently.
func (j judged) Short() string {
	sum := sha256.Sum256([]byte(j.Name))

	return hex.EncodeToString(sum[:8])
}

func (j judged) notable() bool {
	if j.Split["verdict"] || j.Split["value"] {
		return true
	}
	for _, statuses := range j.Status {
		for _, s := range statuses {
			if s != statusOK && s != statusRefusedOK && !slices.Contains(quietValues, s) {
				return true
			}
		}
	}

	return false
}

func judge(rec record, sources []sourceInfo, stances []parserStance) judged {
	j := judged{
		record: rec, Status: map[string][]string{}, Groups: map[string][]consensus.Group{}, Split: map[string]bool{},
		Why: make([]*reason, len(rec.Answers)), ValueWhy: make([]*reason, len(rec.Answers)),
	}

	verdicts, events, values := map[string]string{}, map[string]string{}, map[string]string{}
	// plain holds each verdict without the stage that refused, which decides whether the verdict is split.
	plain := map[string]string{}
	j.Status["verdict"] = make([]string, len(rec.Answers))
	j.Status["value"] = make([]string, len(rec.Answers))
	j.Status["events"] = make([]string, len(rec.Answers))
	j.Status["roundtrip"] = make([]string, len(rec.Answers))

	meaning := ""
	if rec.Meaning != nil && !rec.Meaning.Cyclic && len(rec.Meaning.JSON) > 0 {
		meaning, _ = consensus.CanonicalJSON(string(rec.Meaning.JSON))
	}

	for i, a := range rec.Answers {
		j.Status["roundtrip"][i] = roundTripStatus(sources[i], a)
		if a.Verdict == consensus.Error {
			j.Status["verdict"][i], j.Status["value"][i] = statusError, statusError
			if !sources[i].Values {
				j.Status["value"][i] = statusNoAnswer
			}

			continue
		}

		var ps parserStance
		if i < len(stances) {
			ps = stances[i]
		}

		// The groups hold the verdict the verdict table scores: a refusal counts up to the stance's stage, and
		// one past it, such as libfyaml's Python binding failing after fy-tool parsed the document, is an
		// acceptance. The group names the stage that refused. The events come from the parse, and a refusal by
		// the loader follows a clean one.
		parsed := a.Verdict == consensus.Accept || a.RefusedBy == consensus.Loader
		verdicts[a.Source], plain[a.Source] = verdictGroup(a, ps.Table.At)
		j.Status["verdict"][i], j.Why[i] = classifyVerdict(rec, a, ps)

		if parsed && len(a.Events) > 0 {
			events[a.Source] = strings.Join(a.Events, "\n")
		}
		if a.Verdict == consensus.Accept && a.JSON != nil {
			values[a.Source] = strings.Join(a.JSON, "\n")
		}

		// The value is scored on the valid documents the corpus states a value for, and on no other.
		switch {
		case !sources[i].Values:
			j.Status["value"][i] = statusNoAnswer
		case !rec.WellFormed || rec.Meaning == nil:
			j.Status["value"][i] = statusNotScored
		case a.Verdict != consensus.Accept:
			j.Status["value"][i], j.ValueWhy[i] = refusalReason(a, ps.Table.At, j.Status["verdict"][i], j.Why[i])
		case a.JSON == nil:
			j.Status["value"][i] = statusNoValue
		default:
			j.Status["value"][i], j.ValueWhy[i] = classifyValue(rec, a, ps, valueStatus(rec, a, meaning))
		}
	}

	for q, answers := range map[string]map[string]string{"verdict": verdicts, "events": events, "value": values} {
		groups := consensus.Groups(answers)
		j.Groups[q] = groups
		if len(groups) > 1 {
			j.Split[q] = true
		}
	}
	j.Split["verdict"] = len(consensus.Groups(plain)) > 1

	for i, a := range rec.Answers {
		j.Status["events"][i] = eventStatus(sources[i], a, j.Groups["events"])
	}

	return j
}

// roundTripStatus reads a source's round trip check.
func roundTripStatus(s sourceInfo, a consensus.Answer) string {
	switch {
	case !s.RoundTrip:
		return statusNoAnswer
	case a.Verdict == consensus.Error:
		return statusError
	case a.Verdict != consensus.Accept:
		return statusRefused
	case a.RoundTrip == nil:
		return statusNoAnswer
	default:
		return string(a.RoundTrip.Result)
	}
}

// eventStatus places a source's events among the answer groups of the sources that wrote events.
func eventStatus(s sourceInfo, a consensus.Answer, groups []consensus.Group) string {
	if !s.Events {
		return statusNoAnswer
	}
	own, largest, atLargest := 0, 0, 0
	for _, g := range groups {
		n := len(g.Sources)
		if slices.Contains(g.Sources, a.Source) {
			own = n
		}
		switch {
		case n > largest:
			largest, atLargest = n, 1
		case n == largest:
			atLargest++
		}
	}

	switch {
	case a.Verdict == consensus.Error:
		return statusError
	case own == 0:
		return statusNoEvents
	case len(groups) == 1 && own == 1:
		return statusOnly
	case len(groups) == 1:
		return statusAgree
	case atLargest > 1:
		return statusNoMajor
	case own == largest:
		return statusMajority
	default:
		return statusMinority
	}
}

// valueStatus scores the value a source built against the value the corpus states.
//
// The corpus states a value for one document, so a source that read a different number of documents differs.
func valueStatus(rec record, a consensus.Answer, meaning string) string {
	switch {
	case len(a.JSON) != 1:
	case rec.Meaning.Cyclic:
		if a.JSON[0] == "" {
			return statusOK
		}
	case a.JSON[0] == meaning:
		return statusOK
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
	for _, q := range []string{"verdict", "value", "events", "roundtrip"} {
		s.Counts[q] = make([]map[string]int, n)
		for i := range n {
			s.Counts[q][i] = map[string]int{}
		}
	}
	for _, c := range cases {
		for q, statuses := range c.Status {
			for i, st := range statuses {
				s.Counts[q][i][st]++
			}
		}
		for q, split := range c.Split {
			if !split {
				continue
			}
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
			if c.Split["verdict"] || c.Split["value"] {
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

// quietValues are the value and events statuses that make no case worth a page.
var quietValues = []string{
	statusRTSame, statusRTCyclic, statusRefused, statusValRefTagged, statusValRefused, statusNoValue, statusNotScored, statusNoAnswer,
	statusAgree, statusOnly, statusMajority, statusMinority, statusNoMajor, statusNoEvents,
}

// isDiff says whether a status is a difference from the corpus that nothing explains, or that an open tag may.
func isDiff(status string) bool {
	return isDefect(status) || status == statusDiffTag || status == statusAccTagged
}

// isDefect says whether a status is a difference from the corpus that nothing explains.
func isDefect(status string) bool {
	return status == statusDiff || status == statusAccepted || status == statusAgainst
}

// isExplained says whether a status is a difference from the grammar that a rule, a stance or a departure explains.
func isExplained(status string) bool {
	return status == statusSettled || status == statusDeclared || status == statusExplained ||
		status == statusAccExplained || status == statusValExplained
}

func pageName(name string) string {
	return unsafeName.ReplaceAllString(strings.ReplaceAll(name, "/", "--"), "_")
}

// view is what every page template receives.
type view struct {
	Head    head
	Reasons []reasonCount
	Reason  reasonCount
	// Groups are the reasons laid out by summary cell.
	Groups    []reasonGroup
	Stances   []parserStance
	Stats     corpusStats
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
	// Lead is the paragraph above a list page's cases, when the page lists more than one tag or feature.
	Lead string
	// Sections split a source's page: the differences on untagged cases, then those on tagged cases.
	Sections []section
}

// section is a titled list of cases on a page.
type section struct {
	Title, Lead string
	Cases       []judged
	Truncated   int
}

// writeReport reads a results file and writes the report into dir.
func writeReport(results, dir, stanceDir string, warn io.Writer) error {
	h, cases, err := readResults(results)
	if err != nil {
		return err
	}
	n := len(h.Sources)
	stances, err := loadStances(stanceDir, h)
	if err != nil {
		return err
	}

	judgedCases := make([]judged, len(cases))
	for i, c := range cases {
		judgedCases[i] = judge(c, h.Sources, stances)
	}
	reasons := countReasons(judgedCases, h.Sources, stances)
	for _, r := range reasons {
		if r.Cases == 0 {
			fmt.Fprintf(warn, "consensus: %s: departure %s matches no case\n", r.Source, r.ID)
		}
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"status":   func(c judged, q string, i int) string { return c.Status[q][i] },
		"count":    func(s summary, q string, i int, st string) int { return s.Counts[q][i][st] },
		"split":    func(c judged, q string) bool { return c.Split[q] },
		"groups":   func(c judged, q string) []consensus.Group { return c.Groups[q] },
		"answerOf": func(c judged, i int) consensus.Answer { return c.Answers[i] },
		"why":      func(c judged, i int) *reason { return c.Why[i] },
		"label":    statusLabel,
		"labelIn":  labelIn,
		"valueWhy": func(c judged, i int) *reason { return c.ValueWhy[i] },
		"lines":    func(s []string) string { return strings.Join(s, "\n") },
		"doc":      document,
		"size":     size,
		"rows": func(v view, cases []judged, truncated int) view {
			v.Cases, v.Truncated = cases, truncated

			return v
		},
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
		"cls":   statusClass,
		"ms":    milliseconds,
		"rules": func(c judged) []tagRule { return tagRules(h.Corpus.Vocabulary, c.Tags) },
		"groupsIn": func(groups []reasonGroup, question, source string) []reasonGroup {
			var out []reasonGroup
			for _, g := range groups {
				if (question == "" || g.Question == question) && (source == "" || g.Source == source) {
					out = append(out, g)
				}
			}

			return out
		},
		"withGroups": func(v view, groups []reasonGroup) view {
			v.Groups = groups

			return v
		},
		"cellLink": cellLink,
	}).ParseFS(templates, "templates/*.html")
	if err != nil {
		return err
	}

	sum := summarize(judgedCases, n)
	st := countCorpus(cases)
	qs := questionsFor(st, h.Sources)
	base := view{
		Head: h, Stats: st, Questions: qs, Summary: sum, Reasons: reasons, Stances: stances,
		Groups: groupReasons(reasons, sum, qs, h.Sources),
	}

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
		case strings.HasPrefix(name, "reason/"):
			page = "reason.html"
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
		var defects, open, explained, trips []judged
		for _, c := range notable {
			verdict, value := c.Status["verdict"][i], c.Status["value"][i]
			if rt := c.Status["roundtrip"][i]; rt == statusRTDiff || rt == statusRTUnwrite || rt == statusRTUnread {
				trips = append(trips, c)
			}
			switch {
			case isDefect(verdict) || value == statusDiff || verdict == statusError:
				defects = append(defects, c)
			case isDiff(verdict) || isDiff(value):
				open = append(open, c)
			case isExplained(verdict):
				explained = append(explained, c)
			}
		}
		// Every unexplained difference is listed: each is a defect in the source or in the corpus.
		v.Sections = append(v.Sections, section{
			Title: fmt.Sprintf("Unexplained: %d", len(defects)),
			Lead: "Cases the parser reads otherwise than the corpus, or fails on, " +
				"with no settled rule, declared stand, departure or open tag to explain it.",
			Cases: defects,
		})
		for _, sec := range []section{
			{
				Title: fmt.Sprintf("Open questions: %d", len(open)),
				Lead: "Cases carrying a tag the stance file says nothing about, which the parser reads " +
					"otherwise than the corpus. Declare a stand on the tag to move them out of this list.",
				Cases: open,
			},
			{
				Title: fmt.Sprintf("Round trip: %d", len(trips)),
				Lead:  "Cases whose value does not come back the same when the parser writes it as YAML and reads it again.",
				Cases: trips,
			},
			{
				Title: fmt.Sprintf("Explained: %d", len(explained)),
				Lead:  "Cases a settled rule, a declared stand or a departure explains. The reasons table counts them.",
				Cases: explained,
			},
		} {
			sec.Cases, sec.Truncated = capped(sec.Cases)
			v.Sections = append(v.Sections, sec)
		}
		pages["source/"+pageName(s.ID)+".html"] = v
	}

	cases := notable
	for name, v := range tripPages(base, judgedCases) {
		pages[name] = v
		cases = append(cases, v.Cases...)
	}
	for _, r := range base.Reasons {
		if len(r.Examples) == 0 {
			continue
		}
		v := base
		v.Root, v.Reason, v.Title = "../", r, r.Source+": "+labelIn(r.Question, r.Status)
		v.Cases, v.Truncated = r.Examples, r.Cases-len(r.Examples)
		pages[r.Page] = v
		// A reason may cover a case that is otherwise unremarkable, such as an acceptance under a declined rule.
		cases = append(cases, r.Examples...)
	}

	for _, c := range cases {
		v := base
		v.Root, v.Case, v.Title = "../", c, c.Name
		pages["case/"+c.Short()+".html"] = v
	}

	return pages
}

// tripListed are the round-trip statuses whose cases get a list page.
var tripListed = []string{statusRTDiff, statusRTUnwrite, statusRTUnread, statusRTCyclic}

// tripPage is the page listing a source's cases with one round-trip status.
func tripPage(source, status string) string {
	return "roundtrip/" + pageName(source) + "-" + strings.TrimPrefix(statusClass(status), "s-") + ".html"
}

// tripPages lays out a list page for each source that runs the round trip and each status of [tripListed].
func tripPages(base view, judgedCases []judged) map[string]view {
	columns := map[string]column{}
	for _, q := range base.Questions {
		if q.ID != "roundtrip" {
			continue
		}
		for _, col := range q.Columns() {
			columns[col.Status] = col
		}
	}

	pages := map[string]view{}
	for i, s := range base.Head.Sources {
		if !s.RoundTrip {
			continue
		}
		for _, status := range tripListed {
			var listed []judged
			for _, c := range judgedCases {
				if c.Status["roundtrip"][i] == status {
					listed = append(listed, c)
				}
			}
			if len(listed) == 0 {
				continue
			}
			v := base
			col := columns[status]
			v.Root, v.Title, v.Lead = "../", s.ID+": round trip, "+col.Label, col.Help
			v.Cases, v.Truncated = capped(listed)
			pages[tripPage(s.ID, status)] = v
		}
	}

	return pages
}

// cellLink is the link of a summary cell: its group in the reasons table below, or its round-trip page.
func cellLink(groups []reasonGroup, question, source, status string) string {
	if question == "roundtrip" {
		if slices.Contains(tripListed, status) {
			return tripPage(source, status)
		}

		return ""
	}
	for _, g := range groups {
		if g.Question == question && g.Source == source && g.Status == status {
			return "#" + g.Anchor()
		}
	}

	return ""
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

// statusClass is the CSS class of a status.
func statusClass(s string) string {
	return "s-" + strings.NewReplacer(" ", "", ",", "", "/", "").Replace(s)
}

// size writes a number of bytes in the largest unit that keeps it above one.
func size(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
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

// milliseconds writes a time in milliseconds with three significant digits below 100 ms, and whole above.
func milliseconds(ms float64) string {
	if ms >= 100 {
		return strconv.FormatFloat(ms, 'f', 0, 64)
	}

	return strconv.FormatFloat(ms, 'g', 3, 64)
}
