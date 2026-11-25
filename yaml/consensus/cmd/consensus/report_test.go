// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/conformance-suites/stance"
	"github.com/go-openapi/conformance-suites/suite"
	"github.com/go-openapi/conformance-suites/yaml/consensus"
)

func TestEventStatusPlacesASourceAmongTheGroups(t *testing.T) {
	writes := sourceInfo{ID: "a", Events: true}
	group := func(answer string, ids ...string) consensus.Group {
		return consensus.Group{Answer: answer, Sources: ids}
	}

	for _, tc := range []struct {
		name   string
		s      sourceInfo
		a      consensus.Answer
		groups []consensus.Group
		want   string
	}{
		{"a source writing no events", sourceInfo{ID: "a"}, consensus.Answer{Source: "a"}, nil, statusNoAnswer},
		{"an error", writes, consensus.Answer{Source: "a", Verdict: consensus.Error}, nil, statusError},
		{"a rejection", writes, consensus.Answer{Source: "a"}, []consensus.Group{group("x", "b", "c")}, statusNoEvents},
		{"alone to accept", writes, consensus.Answer{Source: "a"}, []consensus.Group{group("x", "a")}, statusOnly},
		{"unanimous", writes, consensus.Answer{Source: "a"}, []consensus.Group{group("x", "a", "b")}, statusAgree},
		{
			"two against one", writes,
			consensus.Answer{Source: "a"},
			[]consensus.Group{group("x", "a", "b"), group("y", "c")},
			statusMajority,
		},
		{
			"one against two", writes,
			consensus.Answer{Source: "a"},
			[]consensus.Group{group("x", "b", "c"), group("y", "a")},
			statusMinority,
		},
		{
			"one each", writes,
			consensus.Answer{Source: "a"},
			[]consensus.Group{group("x", "a"), group("y", "b")},
			statusNoMajor,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := eventStatus(tc.s, tc.a, tc.groups); got != tc.want {
				t.Errorf("eventStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJudgeScoresValuesOnlyWhereTheCorpusStatesOne(t *testing.T) {
	sources := []sourceInfo{{ID: "v", Values: true}}
	accept := []consensus.Answer{{Source: "v", Verdict: consensus.Accept, JSON: []string{`{"a":1}`}}}
	refuse := []consensus.Answer{{Source: "v", Verdict: consensus.Refuse, RefusedBy: consensus.Parser}}
	stated := &suite.Meaning{Under: "yaml-1.2-core", JSON: json.RawMessage(`{"a": 1}`)}

	for _, tc := range []struct {
		name string
		rec  record
		want string
	}{
		{"the stated value", record{WellFormed: true, Meaning: stated, Answers: accept}, statusOK},
		{
			"another value",
			record{WellFormed: true, Meaning: &suite.Meaning{JSON: json.RawMessage(`2`)}, Answers: accept},
			statusDiff,
		},
		{"a rejection", record{WellFormed: true, Meaning: stated, Answers: refuse}, statusRefused},
		{"no stated value", record{WellFormed: true, Answers: accept}, statusNotScored},
		{"an invalid document", record{Answers: refuse}, statusNotScored},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := judge(tc.rec, sources, []parserStance{{}}).Status["value"][0]; got != tc.want {
				t.Errorf("value status = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJudgeGroupsALoadersRefusalWithTheRefusals(t *testing.T) {
	sources := []sourceInfo{{ID: "a"}, {ID: "l"}, {ID: "c"}, {ID: "p"}}
	rec := record{WellFormed: true, Answers: []consensus.Answer{
		{Source: "a", Verdict: consensus.Accept},
		{Source: "l", Verdict: consensus.Refuse, RefusedBy: consensus.Loader},
		{Source: "c", Verdict: consensus.Refuse, RefusedBy: consensus.Composer},
		{Source: "p", Verdict: consensus.Refuse, RefusedBy: consensus.Loader},
	}}
	construct := parserStance{Table: stance.Table{At: stance.Construct}}
	stances := []parserStance{construct, construct, construct, {Table: stance.Table{At: stance.Parse}}}
	j := judge(rec, sources, stances)

	got := map[string]string{}
	for _, g := range j.Groups["verdict"] {
		for _, id := range g.Sources {
			got[id] = g.Answer
		}
	}
	want := map[string]string{
		"a": "accept", "l": "refuse by the loader", "c": "refuse by the composer",
		"p": "accept, then the loader refuses past parse",
	}
	for id, answer := range want {
		if got[id] != answer {
			t.Errorf("%s groups under %q, want %q", id, got[id], answer)
		}
	}
	if !j.Split["verdict"] {
		t.Error("the verdict is not split")
	}

	rec.Answers[0] = consensus.Answer{Source: "a", Verdict: consensus.Refuse, RefusedBy: consensus.Parser}
	rec.Answers[3] = consensus.Answer{Source: "p", Verdict: consensus.Refuse, RefusedBy: consensus.Parser}
	if judge(rec, sources, stances).Split["verdict"] {
		t.Error("refusals at different stages split the verdict")
	}
}

func TestTagRulesReadsTheHeader(t *testing.T) {
	vocabulary := []suite.TagSpec{
		{Tag: "anchor/alias-across-documents", Stage: "compose", Settled: "reject", Because: "3.2.2.2"},
		{Tag: "encoding/bom", Stage: "parse"},
	}
	got := tagRules(vocabulary, []string{"encoding/bom", "anchor/alias-across-documents", "unknown"})
	want := []tagRule{
		{Tag: "encoding/bom", Stage: "parse"},
		{Tag: "anchor/alias-across-documents", Stage: "compose", Settled: "reject", Because: "3.2.2.2"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("tagRules = %+v, want %+v", got, want)
	}
}

func TestClassifyVerdictSplitsByReason(t *testing.T) {
	h := head{
		Corpus: corpusInfo{Vocabulary: []suite.TagSpec{
			{Tag: "anchor/alias-undefined", Stage: "compose", Settled: "reject", Because: "7.1"},
			{Tag: "key/duplicate", Stage: "parse", Settled: "reject", Because: "3.2.1.1"},
			{Tag: "encoding/bom", Stage: "parse"},
			{Tag: "key/not-a-scalar", Stage: "construct"},
			{Tag: "anchor/alias-across-documents", Stage: "construct", Settled: "reject", Because: "3.2.2.2"},
		}},
		Sources: []sourceInfo{{ID: "p"}},
	}
	dir := t.TempDir()
	const file = `parser: p
at: compose
stands:
  key/duplicate: {stand: either, because: not checked}
  key/not-a-scalar: refuses
  anchor/alias-across-documents: {stand: either, because: by posture}
departures:
  - id: merge
    rejects: valid
    match: {message: 'merge key'}
    kind: position
    because: a ruling
  - id: lenient
    accepts: invalid
    match: {src: 'x'}
    kind: leniency
    because: unchecked
`
	if err := os.WriteFile(filepath.Join(dir, "p.yaml"), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	stances, err := loadStances(dir, h)
	if err != nil {
		t.Fatal(err)
	}
	ps := stances[0]

	accept := consensus.Answer{Verdict: consensus.Accept}
	byParser := func(msg string) consensus.Answer {
		return consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Parser, Message: msg}
	}
	byLoader := consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Loader}
	byComposer := consensus.Answer{Verdict: consensus.Refuse, RefusedBy: consensus.Composer}
	valid := func(tags ...string) record {
		return record{WellFormed: true, VerdictAt: "compose", Tags: tags, Src: []byte("a")}
	}

	for _, tc := range []struct {
		name string
		rec  record
		a    consensus.Answer
		want string
	}{
		{"a settled reject, accepted", valid("anchor/alias-undefined"), accept, statusAgainst},
		{"a settled reject, rejected", valid("anchor/alias-undefined"), byComposer, statusSettled},
		{"a declined rule, accepted", valid("key/duplicate"), accept, statusOK},
		{"a tag past the stage", valid("key/not-a-scalar"), byParser(""), statusDiff},
		{"a stand past the stage does not explain a rejection", valid("anchor/alias-across-documents"), byParser(""), statusDiff},
		{"a refusal past the stage counts as accepting", valid(), byLoader, statusOK},
		{"a refusal past the claim", record{WellFormed: true, Src: []byte("a")}, byComposer, statusUnscored},
		{"a departure", valid(), byParser("could not find merge key"), statusExplained},
		{"an open tag", valid("encoding/bom"), byParser(""), statusDiffTag},
		{"nothing explains it", valid(), byParser("no"), statusDiff},
		{"an invalid document, rejected", record{}, byParser(""), statusRefusedOK},
		{"a leniency", record{Src: []byte("x")}, accept, statusAccExplained},
		{"an open tag on an invalid document", record{Tags: []string{"encoding/bom"}}, accept, statusAccTagged},
		{"an invalid document, accepted", record{Src: []byte("y")}, accept, statusAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, _ := classifyVerdict(tc.rec, tc.a, ps); got != tc.want {
				t.Errorf("classifyVerdict = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("a stand past the stage explains an acceptance", func(t *testing.T) {
		got, why := classifyVerdict(valid("anchor/alias-across-documents"), accept, ps)
		if got != statusOK || why == nil || why.Text != "anchor/alias-across-documents: by posture" {
			t.Errorf("classifyVerdict = %q, %+v, want %q with the stand's because", got, why, statusOK)
		}
	})
}

func TestTheBuiltInStanceFilesLoad(t *testing.T) {
	h := head{Sources: []sourceInfo{{ID: "perlref"}, {ID: "libfyaml"}, {ID: "yamlstar"}, {ID: "yaml.v3"}, {ID: "go-yaml"}}}
	for _, tag := range []string{
		"key/duplicate", "encoding/not-utf8", "encoding/bom", "json/not-representable", "key/not-a-scalar",
		"anchor/cyclic-meaning", "directive/yaml-minor-version", "tag/bool-not-a-boolean", "tag/timestamp-not-a-date",
		"tag/binary-not-base64", "tag/int-not-an-integer", "tag/float-not-a-number", "tag/null-not-null",
		"tag/omap-not-a-sequence-of-pairs", "tag/kind-mismatch", "tag/local", "tag/named-handle", "tag/percent-escape",
		"directive/yaml-version", "merge/key", "merge/sequence", "merge/written-in-place", "merge/into-non-mapping",
		"merge/quoted", "merge/key-alone", "representation/binary", "representation/timestamp",
		"representation/ordered-map", "representation/integer-wide", "representation/float-out-of-range",
		"representation/float-inexact", "representation/float-non-finite", "anchor/alias-across-documents",
	} {
		h.Corpus.Vocabulary = append(h.Corpus.Vocabulary, suite.TagSpec{Tag: tag, Stage: "parse"})
	}
	stances, err := loadStances("", h)
	if err != nil {
		t.Fatal(err)
	}
	for i, ps := range stances {
		if !ps.Declared {
			t.Errorf("%s has no stance file", h.Sources[i].ID)
		}
	}
}

func TestRoundTripStatusReadsTheCheck(t *testing.T) {
	trips := sourceInfo{ID: "g", RoundTrip: true}
	for _, tc := range []struct {
		name string
		s    sourceInfo
		a    consensus.Answer
		want string
	}{
		{"no encoder", sourceInfo{ID: "p"}, consensus.Answer{Verdict: consensus.Accept}, statusNoAnswer},
		{"a rejection", trips, consensus.Answer{Verdict: consensus.Refuse}, statusRefused},
		{"an error", trips, consensus.Answer{Verdict: consensus.Error}, statusError},
		{"not checked", trips, consensus.Answer{Verdict: consensus.Accept}, statusNoAnswer},
		{"different", trips, consensus.Answer{
			Verdict: consensus.Accept, RoundTrip: &consensus.RoundTrip{Result: consensus.Different},
		}, statusRTDiff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := roundTripStatus(tc.s, tc.a); got != tc.want {
				t.Errorf("roundTripStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGroupReasonsAddsUpToTheSummaryCell(t *testing.T) {
	sources := []sourceInfo{{ID: "p", Values: true}}
	mk := func(status string, why *reason) judged {
		return judged{
			Status: map[string][]string{"verdict": {status}, "value": {statusNotScored}},
			Why:    []*reason{why}, ValueWhy: []*reason{nil},
		}
	}
	declined := &reason{Key: "declined key/duplicate", Text: "key/duplicate: not checked"}
	open := &reason{Key: "open encoding/bom", Text: "encoding/bom: p has not ruled on this"}
	cases := []judged{
		mk(statusOK, nil), mk(statusOK, declined), mk(statusDiffTag, open), mk(statusDiff, nil), mk(statusDiff, nil),
		mk(statusRefusedOK, nil),
	}
	stances := []parserStance{{}}
	sum := summarize(cases, 1)
	qs := questionsFor(corpusStats{}, sources)
	groups := groupReasons(countReasons(cases, sources, stances), sum, qs, sources)

	got := map[string][2]int{}
	for _, g := range groups {
		covered := 0
		for _, r := range g.Rows {
			covered += r.Cases
		}
		got[g.Status] = [2]int{g.Count, covered + g.Rest}
	}
	assert.Equal(t, map[string][2]int{
		statusOK: {2, 2}, statusDiffTag: {1, 1}, statusDiff: {2, 2},
	}, got, "one group per cell with a reason, each adding up to the cell; none for a correct rejection")
}
