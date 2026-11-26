// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/conformance-suites/suite"
	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

// The verdict statuses that split "rejected valid" and "accepted invalid" by reason.
const (
	statusAgainst      = "accepted, against the rules"
	statusSettled      = "rejected, settled"
	statusDeclared     = "rejected, as declared"
	statusExplained    = "rejected, explained"
	statusAccExplained = "accepted, explained"
	statusAccTagged    = "accepted, tagged"
	statusValExplained = "different, explained"
)

// The value statuses of a document the parser rejects, split by the verdict's reason. A rejection that a settled
// rule, a declared stand, a departure or the stage explains takes the verdict's own status.
const (
	statusValRefTagged = "rejected, tagged"
	statusValRefused   = "rejected"
)

// reason is why a source's verdict on a case falls in the column it does.
type reason struct {
	// Key groups the cases one reason explains: "settled anchor/alias-undefined", "departure merge-key-ruling".
	Key string
	// Text is the explanation a page shows.
	Text string
	// Kind is a departure's kind, empty for any other reason.
	Kind string
}

// stageOfRefusal places a refusal at the stage that made it.
func stageOfRefusal(by consensus.Stage) stance.Stage {
	switch by {
	case consensus.Composer:
		return stance.Compose
	case consensus.Loader:
		return stance.Construct
	default:
		return stance.Parse
	}
}

// rejects says whether a source rejected a document at a stage it is answerable for: a refusal past its stance's
// stage, such as libfyaml's Python binding failing after fy-tool parsed the document, counts as accepting.
func rejects(a consensus.Answer, at stance.Stage) bool {
	return a.Verdict == consensus.Refuse && stageOfRefusal(a.RefusedBy) <= at
}

// tagReasons walks a case's tags with a stance, as [stance.Table.Expect] does, and keeps what each finds.
//
// beyond holds the because of a stand on a tag decided past the parser's stage: the tag does not change the
// expectation, but the stance file explains what the parser does with it. early holds a settled rejection decided
// past the parser's stage: a parser that checks the rule before its stage rejects the document rightly.
type tagReasons struct {
	settled, declared, open, beyond, early *reason
}

func readTags(rec record, ps parserStance) tagReasons {
	t := ps.Table
	said := func(tag stance.Tag, otherwise string) string {
		if because, ok := ps.StandBecause[tag]; ok {
			return string(tag) + ": " + because
		}

		return string(tag) + ": " + t.Name + " " + otherwise
	}
	var out tagReasons
	for _, name := range rec.Tags {
		tag := stance.Tag(name)
		if stage, _ := t.Speaks.Of(tag); stage > t.At {
			if because, ok := ps.StandBecause[tag]; ok && out.beyond == nil {
				out.beyond = &reason{Key: "beyond " + name, Text: name + ": " + because}
			}
			if rule, settled := t.Requires.Of(tag); settled && rule.Then == stance.Reject && t.Stand(tag) != stance.Either &&
				out.early == nil {
				out.early = &reason{
					Key:  "settled early " + name,
					Text: name + ": " + rule.Because + ". The parser checks this " + stage.String() + " rule at " + t.At.String(),
				}
			}

			continue
		}

		if rule, settled := t.Requires.Of(tag); settled {
			switch {
			case t.Stand(tag) == stance.Either:
				if out.open == nil {
					out.open = &reason{Key: "declined " + name, Text: said(tag, "declines this settled rule")}
				}
			case rule.Then == stance.Reject && out.settled == nil:
				out.settled = &reason{Key: "settled " + name, Text: name + ": " + rule.Because}
			}

			continue
		}

		switch t.Stand(tag) {
		case stance.Refuses:
			if out.declared == nil {
				out.declared = &reason{Key: "declared " + name, Text: said(tag, "refuses this on purpose")}
			}
		case stance.Undeclared, stance.Either:
			if out.open == nil {
				text := name + ": " + t.Name + " has not ruled on this"
				if t.Stand(tag) == stance.Either {
					text = said(tag, "declines to guarantee this")
				}
				out.open = &reason{Key: "open " + name, Text: text}
			}
		default:
			// Accepts: the parser tolerates the property, and the grammar decides.
		}
	}

	return out
}

// classifyVerdict places a source's verdict on a case in a column of the verdict table, with the reason.
//
// The grammar's verdict picks the band, and the first reason that applies picks the column:
//
//   - on a valid document the parser accepts, a settled rule or a declared refusal makes the acceptance wrong;
//   - on a valid document the parser rejects: a settled rule, a declared refusal, a refusal past the stage the
//     case claims, a departure in the stance file, an open tag, and otherwise a defect;
//   - on an invalid document the parser accepts: a departure, an open tag or opaque bytes, and otherwise a defect.
func classifyVerdict(rec record, a consensus.Answer, ps parserStance) (string, *reason) {
	t := ps.Table
	tags := readTags(rec, ps)
	rejected := rejects(a, t.At)

	switch {
	case rec.WellFormed && !rejected:
		if tags.settled != nil {
			return statusAgainst, tags.settled
		}
		if tags.declared != nil {
			return statusAgainst, tags.declared
		}
		if tags.open != nil && strings.HasPrefix(tags.open.Key, "declined ") {
			// The parser declines the settled rule, so the acceptance is not held against it, and the report
			// keeps the reason so that the reasons table can count what the parser does not check.
			return statusOK, tags.open
		}
		if tags.beyond != nil {
			return statusOK, tags.beyond
		}

		return statusOK, nil
	case rec.WellFormed:
		if tags.settled != nil {
			return statusSettled, tags.settled
		}
		if tags.early != nil {
			return statusSettled, tags.early
		}
		if tags.declared != nil {
			return statusDeclared, tags.declared
		}
		if stageOfRefusal(a.RefusedBy) > suite.StageOf(rec.VerdictAt) {
			// The corpus vouches for the document only as far as it claims: a mutant's acceptance is the grammar's,
			// and a composer or a loader may rightly refuse what the grammar accepts.
			return statusUnscored, &reason{Key: "past the claim " + string(a.RefusedBy), Text: "the " + string(a.RefusedBy) +
				" rejects a document the corpus vouches for only up to " + suite.StageOf(rec.VerdictAt).String()}
		}
		for _, d := range ps.Rejecting {
			if d.Matches(rec, viewOf(a)) {
				return statusExplained, &reason{Key: "departure " + d.ID, Text: d.ID + ": " + d.Because, Kind: d.Kind}
			}
		}
		if tags.open != nil {
			return statusDiffTag, tags.open
		}

		return statusDiff, nil
	case rejected:
		return statusRefusedOK, nil
	default:
		for _, d := range ps.Accepting {
			if d.Matches(rec, viewOf(a)) {
				return statusAccExplained, &reason{Key: "departure " + d.ID, Text: d.ID + ": " + d.Because, Kind: d.Kind}
			}
		}
		if tags.open != nil {
			return statusAccTagged, tags.open
		}
		if rec.Opaque {
			return statusAccTagged, &reason{Key: "opaque", Text: "the corpus cannot read these bytes under any encoding it knows"}
		}

		return statusAccepted, nil
	}
}

// classifyValue splits a value difference by reason: a departure in the stance file, an open tag, or a defect.
func classifyValue(rec record, a consensus.Answer, ps parserStance, status string) (string, *reason) {
	if status != statusDiff {
		return status, nil
	}
	for _, d := range ps.Differing {
		if d.Matches(rec, viewOf(a)) {
			return statusValExplained, &reason{Key: "departure " + d.ID, Text: d.ID + ": " + d.Because, Kind: d.Kind}
		}
	}
	if open := readTags(rec, ps).open; open != nil {
		return statusDiffTag, open
	}

	return statusDiff, nil
}

func viewOf(a consensus.Answer) answerView {
	return answerView{Message: a.Message, Value: strings.Join(a.JSON, "\n")}
}

// reasonCount is how many cases one reason explains for one source, for the reasons tables.
type reasonCount struct {
	Question, Source, Status, Key, Text, Kind string
	// ID is the departure's id, empty for a settled rule or a declared stand.
	ID    string
	Cases int
	// Examples are the first cases the reason covers, at most maxExamples, for the reason's own page.
	Examples []judged
	// Page is the reason's page in the report.
	Page string
}

// maxExamples caps the cases a reason's page lists.
const maxExamples = 50

// Example is the first document the reason covers, cut to a tooltip's length.
func (r reasonCount) Example() string {
	if len(r.Examples) == 0 {
		return ""
	}
	text := document(r.Examples[0].Src)
	if len(text) > 600 {
		text = text[:600] + "\n..."
	}

	return text
}

// countReasons counts, per source, the cases each reason covers, on the verdict and the value questions: the
// settled rules, the declared stands, the departures, the open tags, the refusals past a case's claim, and a
// reason that says nothing explains the case. Every departure appears, so one matching no case shows as zero.
func countReasons(cases []judged, sources []sourceInfo, stances []parserStance) []reasonCount {
	type key struct{ question, source, status, key string }
	index := map[key]int{}
	var out []reasonCount
	add := func(k key, r reasonCount) *reasonCount {
		if _, ok := index[k]; !ok {
			index[k] = len(out)
			out = append(out, r)
		}

		return &out[index[k]]
	}
	// idOf names source i; a results file always holds one answer per source, so the index is in range.
	idOf := func(i int) string {
		if i < len(sources) {
			return sources[i].ID
		}

		return ""
	}
	for i, ps := range stances {
		for _, group := range []struct {
			question, status string
			deps             []*departure
		}{
			{"verdict", statusExplained, ps.Rejecting},
			{"verdict", statusAccExplained, ps.Accepting},
			{"value", statusValExplained, ps.Differing},
		} {
			for _, d := range group.deps {
				add(key{group.question, idOf(i), group.status, "departure " + d.ID}, reasonCount{
					Question: group.question, Source: idOf(i), Status: group.status, Key: "departure " + d.ID,
					ID: d.ID, Text: d.ID + ": " + d.Because, Kind: d.Kind,
				})
			}
		}
	}
	for _, c := range cases {
		for _, q := range []struct {
			id  string
			why []*reason
		}{{"verdict", c.Why}, {"value", c.ValueWhy}} {
			for i, st := range c.Status[q.id] {
				why := reasonFor(q.id, st, q.why[i])
				if why == nil {
					continue
				}
				r := add(key{q.id, idOf(i), st, why.Key}, reasonCount{
					Question: q.id, Source: idOf(i), Status: st, Key: why.Key, Text: why.Text, Kind: why.Kind,
				})
				r.Cases++
				if len(r.Examples) < maxExamples {
					r.Examples = append(r.Examples, c)
				}
			}
		}
	}
	for i := range out {
		sum := sha256.Sum256([]byte(out[i].Question + "\x00" + out[i].Status + "\x00" + out[i].Key))
		out[i].Page = "reason/" + pageName(out[i].Source) + "--" + hex.EncodeToString(sum[:6]) + ".html"
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Cases > out[b].Cases })

	return out
}

// Reasons for a status that has no reason of its own.
var (
	reasonUnexplained = reason{Key: "unexplained", Text: "nothing explains it: no settled rule, declared stand, " +
		"departure or open tag covers the case. A defect in the parser or in the corpus."}
	reasonError   = reason{Key: "error", Text: "the parser crashed, timed out, or wrote output the runner cannot read"}
	reasonNoValue = reason{Key: "no value", Text: "the parser accepts the document and returns no value for it"}
)

// reasonFor is the reason a status is counted under in the reasons table, or nil for a status the table leaves out:
// an agreement with the corpus that needs no reason, or a value the parser could not build because it rejected the
// document, which the verdict table counts.
func reasonFor(question, status string, why *reason) *reason {
	switch {
	case why != nil:
		return why
	case status == statusError:
		return &reasonError
	case status == statusDiff || status == statusAccepted:
		return &reasonUnexplained
	case question == "value" && status == statusNoValue:
		return &reasonNoValue
	default:
		return nil
	}
}

// reasonGroup is one cell of the verdict or the value table, and the reasons behind its count.
type reasonGroup struct {
	Question, Source, Status string
	// Count is the cell's count in the summary table.
	Count int
	// Rows are the reasons, most cases first.
	Rows []reasonCount
	// Rest counts the cases of the cell that need no reason: the agreements with the corpus around a declined rule.
	Rest int
}

// Anchor is the group's id in the index, which the summary table's cell links to.
func (g reasonGroup) Anchor() string { return anchorOf(g.Question, g.Source, g.Status) }

func anchorOf(question, source, status string) string {
	return "r-" + question + "-" + pageName(source) + "-" + strings.TrimPrefix(statusClass(status), "s-")
}

// Span is how many table rows the group takes, for the merged cells.
func (g reasonGroup) Span() int {
	n := len(g.Rows)
	if g.Rest > 0 {
		n++
	}

	return max(n, 1)
}

// groupReasons lays the reasons out by summary cell: for each question, each source and each column, in the
// summary table's order, the cell's count and the reasons behind it. A cell with no reason to give, such as
// "rejected" on an invalid document, gets no group.
func groupReasons(reasons []reasonCount, sum summary, questions []question, sources []sourceInfo) []reasonGroup {
	var out []reasonGroup
	for _, q := range questions {
		if q.ID != "verdict" && q.ID != "value" {
			continue
		}
		for _, i := range q.Sources {
			for _, col := range q.Columns() {
				g := reasonGroup{
					Question: q.ID, Source: sources[i].ID, Status: col.Status, Count: sum.Counts[q.ID][i][col.Status],
				}
				covered := 0
				for _, r := range reasons {
					if r.Question == q.ID && r.Source == g.Source && r.Status == g.Status {
						g.Rows = append(g.Rows, r)
						covered += r.Cases
					}
				}
				if len(g.Rows) == 0 {
					continue
				}
				g.Rest = g.Count - covered
				out = append(out, g)
			}
		}
	}

	return out
}

// labelIn is the label of a status on one question: a value status that shares a verdict status's name is
// labelled as a difference of value.
func labelIn(question, status string) string {
	if question == "value" {
		switch status {
		case statusDiff:
			return "different"
		case statusDiffTag:
			return "different, tagged"
		}
	}

	return statusLabel(status)
}

// statusLabel is the column label of a verdict status, or the status itself.
func statusLabel(status string) string {
	if label, ok := verdictLabels[status]; ok {
		return label
	}

	return status
}

var verdictLabels = map[string]string{
	statusOK: "accepted", statusRefusedOK: "rejected, invalid", statusAccepted: "accepted, invalid",
	statusDiff: "rejected", statusDiffTag: "rejected, tagged", statusUnscored: "rejected after the parse",
}

// tagRule is what the corpus header gives on one tag of a case: the stage that decides it, and the settled rule or
// none.
type tagRule struct {
	Tag, Stage, Settled, Because string
}

// tagRules lists the header's entry for each of a case's tags, in the case's order.
func tagRules(vocabulary []suite.TagSpec, tags []string) []tagRule {
	out := make([]tagRule, 0, len(tags))
	for _, tag := range tags {
		for _, spec := range vocabulary {
			if spec.Tag == tag {
				out = append(out, tagRule{
					Tag: tag, Stage: suite.StageOf(spec.Stage).String(), Settled: spec.Settled, Because: spec.Because,
				})

				break
			}
		}
	}

	return out
}

// verdictGroup returns the answer a source's verdict groups under on a case page, and the bare verdict that decides
// whether the sources split: a refusal names its stage, and a refusal past the stance's stage counts as accepting.
func verdictGroup(a consensus.Answer, at stance.Stage) (group, verdict string) {
	switch {
	case rejects(a, at):
		return "refuse by the " + string(a.RefusedBy), string(consensus.Refuse)
	case a.Verdict == consensus.Refuse:
		return "accept, then the " + string(a.RefusedBy) + " refuses past " + at.String(), string(consensus.Accept)
	default:
		return string(a.Verdict), string(a.Verdict)
	}
}

// refusalReason returns the value status and the reason of a valid document the parser rejects, taken from its
// verdict, so the value table splits its rejections by the same reasons as the verdict table.
func refusalReason(a consensus.Answer, at stance.Stage, verdict string, why *reason) (string, *reason) {
	switch verdict {
	case statusOK:
		// A refusal past the stance's stage: the verdict table counts an acceptance.
		return statusUnscored, &reason{
			Key:  "past the stage " + string(a.RefusedBy),
			Text: "the " + string(a.RefusedBy) + " rejects the document past " + at.String() + ", where the verdict is given",
		}
	case statusSettled, statusDeclared, statusUnscored, statusExplained:
		return verdict, why
	case statusDiffTag:
		return statusValRefTagged, why
	default:
		return statusValRefused, &reasonUnexplained
	}
}
