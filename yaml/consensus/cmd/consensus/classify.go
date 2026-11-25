// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
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
type tagReasons struct {
	settled, declared, open *reason
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

		return statusOK, nil
	case rec.WellFormed:
		if tags.settled != nil {
			return statusSettled, tags.settled
		}
		if tags.declared != nil {
			return statusDeclared, tags.declared
		}
		if stageOfRefusal(a.RefusedBy) > suite.StageOf(rec.VerdictAt) {
			// The corpus vouches for the document only as far as it claims: a mutant's acceptance is the grammar's,
			// and a composer or a loader may rightly refuse what the grammar accepts.
			return statusUnscored, &reason{Key: "past the claim", Text: "the " + string(a.RefusedBy) +
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
	Source, Status, Key, Text, Kind string
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

// countReasons counts, per source, the cases each explaining reason covers: the settled rules, the declared stands
// and the departures. Every departure appears, so one matching no case shows as zero.
func countReasons(cases []judged, sources []sourceInfo, stances []parserStance) []reasonCount {
	type key struct{ source, status, key string }
	index := map[key]int{}
	var out []reasonCount
	add := func(k key, r reasonCount) {
		if _, ok := index[k]; !ok {
			index[k] = len(out)
			out = append(out, r)
		}
	}
	for i, ps := range stances {
		for _, d := range ps.Rejecting {
			add(key{sources[i].ID, statusExplained, "departure " + d.ID}, reasonCount{
				Source: sources[i].ID, Status: statusExplained, Key: "departure " + d.ID, ID: d.ID,
				Text: d.ID + ": " + d.Because, Kind: d.Kind,
			})
		}
		for _, d := range ps.Differing {
			add(key{sources[i].ID, statusValExplained, "departure " + d.ID}, reasonCount{
				Source: sources[i].ID, Status: statusValExplained, Key: "departure " + d.ID, ID: d.ID,
				Text: d.ID + ": " + d.Because, Kind: d.Kind,
			})
		}
		for _, d := range ps.Accepting {
			add(key{sources[i].ID, statusAccExplained, "departure " + d.ID}, reasonCount{
				Source: sources[i].ID, Status: statusAccExplained, Key: "departure " + d.ID, ID: d.ID,
				Text: d.ID + ": " + d.Because, Kind: d.Kind,
			})
		}
	}
	for _, c := range cases {
		for i, st := range c.Status["value"] {
			why := c.ValueWhy[i]
			if why == nil || st != statusValExplained {
				continue
			}
			k := key{sources[i].ID, st, why.Key}
			add(k, reasonCount{Source: sources[i].ID, Status: st, Key: why.Key, Text: why.Text, Kind: why.Kind})
			r := &out[index[k]]
			r.Cases++
			if len(r.Examples) < maxExamples {
				r.Examples = append(r.Examples, c)
			}
		}
		for i, st := range c.Status["verdict"] {
			why := c.Why[i]
			if why == nil || (!isExplained(st) && st != statusAgainst && !strings.HasPrefix(why.Key, "declined ")) {
				continue
			}
			k := key{sources[i].ID, st, why.Key}
			add(k, reasonCount{Source: sources[i].ID, Status: st, Key: why.Key, Text: why.Text, Kind: why.Kind})
			r := &out[index[k]]
			r.Cases++
			if len(r.Examples) < maxExamples {
				r.Examples = append(r.Examples, c)
			}
		}
	}
	for i := range out {
		sum := sha256.Sum256([]byte(out[i].Status + "\x00" + out[i].Key))
		out[i].Page = "reason/" + pageName(out[i].Source) + "--" + hex.EncodeToString(sum[:6]) + ".html"
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Source != out[b].Source {
			return slices.IndexFunc(sources, func(s sourceInfo) bool { return s.ID == out[a].Source }) <
				slices.IndexFunc(sources, func(s sourceInfo) bool { return s.ID == out[b].Source })
		}

		return out[a].Cases > out[b].Cases
	})

	return out
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
